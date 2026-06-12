// Package engine wraps anacrolix/torrent: add/drop/delete/read with RAM or disk
// storage, idempotent start, no zombie state (SPEC §5, §6, §9).
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"github.com/jodacame/fluxtorrent/internal/classify"
	"github.com/jodacame/fluxtorrent/internal/config"
	"github.com/jodacame/fluxtorrent/internal/rules"
)

// ErrRejected is returned when rules or compressed-detection reject a torrent.
type ErrRejected struct {
	Note     string
	Warnings []string
}

func (e *ErrRejected) Error() string { return e.Note }

// File describes one file in a torrent.
type File struct {
	Index    int    `json:"index"`
	Path     string `json:"path"`
	SizeB    int64  `json:"sizeBytes"`
	Playable bool   `json:"playable"`
}

// AddResult is returned by Add (SPEC §7 POST /api/torrents).
type AddResult struct {
	Hash     string   `json:"hash"`
	Name     string   `json:"name"`
	Files    []File   `json:"files"`
	Playable bool     `json:"playable"`
	Warnings []string `json:"warnings"`
}

// Stats is a live snapshot for one torrent (SPEC §7).
type Stats struct {
	Peers       int     `json:"peers"`
	Seeders     int     `json:"seeders"`
	DownKbps    int64   `json:"downKbps"`
	UpKbps      int64   `json:"upKbps"`
	Progress    float64 `json:"progress"`
	CacheFillMB int     `json:"cacheFillMB"`
	State       string  `json:"state"`
	Ratio       float64 `json:"ratio"`
	// UploadedBytes is the total shared (uploaded) so far, including bytes
	// accumulated in previous runs (persisted for keepSeed accounting).
	UploadedBytes int64 `json:"uploadedBytes"`
}

// Info is the list-view of a torrent (SPEC §7 GET /api/torrents).
type Info struct {
	Hash    string `json:"hash"`
	Name    string `json:"name"`
	SizeB   int64  `json:"sizeBytes"`
	Files   []File `json:"files"`
	Stats   Stats  `json:"stats"`
	Mode    string `json:"storageMode"` // "ram" | "disk"
	AddedAt int64  `json:"addedAt"`

	// mode & rule at a glance (SPEC §11b)
	Kind              string         `json:"kind"` // "stream" | "seeding"
	Private           bool           `json:"private"`
	KeepSeed          bool           `json:"keepSeed"`
	SeedTargetRatio   float64        `json:"seedTargetRatio"`
	SeedTargetMinutes int            `json:"seedTargetMinutes"`
	SeedElapsedMin    int            `json:"seedElapsedMin"`
	Clients           []StreamClient `json:"clients"`
}

type managed struct {
	t        *torrent.Torrent
	hash     string
	link     string
	mode     string
	addedAt  time.Time
	played   time.Time
	keepSeed bool
	sRatio   float64
	sMinutes int
	private  bool

	mu          sync.Mutex
	readers     int
	readHead    int64 // byte offset of the active RAM reader window center
	lastDown    int64
	lastUp      int64
	lastSampleT time.Time
	downKbps    int64
	upKbps      int64

	// keepSeed accounting (persisted so ratio/time survive restarts, SPEC §8).
	baseUp         int64     // bytes uploaded in prior sessions
	baseDown       int64     // bytes downloaded in prior sessions
	seedStartedAt  time.Time // when the torrent reached 100% (sharing begins)
	downloadAllSet bool

	// active stream consumers (who is playing, and which file)
	clientSeq int64
	clients   map[int64]StreamClient
}

// StreamClient describes a player currently consuming a stream (SPEC §11b:
// see who/what is streaming and from where).
type StreamClient struct {
	Addr      string `json:"addr"`      // remote IP:port of the player
	Agent     string `json:"agent"`     // User-Agent (player/app)
	FileIndex int    `json:"fileIndex"` // 0-based file being streamed
	File      string `json:"file"`      // file path
	Since     int64  `json:"since"`     // unix start time
}

func (m *managed) addClient(c StreamClient) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.clients == nil {
		m.clients = map[int64]StreamClient{}
	}
	m.clientSeq++
	id := m.clientSeq
	m.clients[id] = c
	return id
}

func (m *managed) removeClient(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, id)
}

// Engine is the torrent engine façade.
type Engine struct {
	cl    *torrent.Client
	store *config.Store
	ram   *ramStore

	// disk storage is created lazily so a pure-RAM setup never touches the disk
	// path (no piece-completion db in /downloads). Created on first disk torrent.
	diskPath string
	diskOnce sync.Once
	disk     storage.ClientImplCloser

	downLimiter *rate.Limiter // global download cap (SPEC §6.3)
	upLimiter   *rate.Limiter // global upload cap

	mu      sync.Mutex
	managed map[string]*managed
}

// New builds the anacrolix client from settings and returns an Engine.
func New(store *config.Store) (*Engine, error) {
	cfg := store.Get()

	cc := torrent.NewDefaultClientConfig()
	cc.ListenPort = cfg.Net.BTPort
	cc.NoDHT = !cfg.Net.DHT
	// Seeding capability is always on so `keepSeed` torrents can upload; RAM-mode
	// stream torrents never complete and are dropped after playback, so they don't
	// meaningfully seed regardless (SPEC §6.2, §6.4).
	cc.Seed = true
	cc.EstablishedConnsPerTorrent = cfg.Net.MaxConns
	cc.DataDir = cfg.Cache.Path
	// We always pass per-torrent storage; the RAM store is the default fallback so
	// a pure-RAM setup never opens anything on disk.
	ram := newRAMStore(int64(cfg.Cache.SizeMB) * 1 << 20)
	cc.DefaultStorage = ram

	// Advanced network options (SPEC §6.3). Client-level — applied at boot.
	cc.DisableIPv6 = !cfg.Net.IPv6
	cc.DisableUTP = !cfg.Net.UTP
	cc.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{
		Preferred:        true,                   // always prefer encrypted headers
		RequirePreferred: cfg.Net.EncryptHeaders, // force when enabled
	}

	// Present as qBittorrent. Many private trackers whitelist specific clients and
	// reject unknown ones (anacrolix's default identity) — this is what lets a
	// private-tracker announce succeed and return peers, exactly like TorrServer.
	cc.Bep20 = "-qB4390-"
	cc.PeerID = qbittorrentPeerID()
	cc.HTTPUserAgent = "qBittorrent/4.3.9"
	cc.ExtendedHandshakeClientVersion = "qBittorrent/4.3.9"
	cc.UpnpID = "qBittorrent/4.3.9"
	cc.TotalHalfOpenConns = 500 // connect to many peers quickly

	// Global rate limiters (runtime-adjustable via settings, SPEC §6.3).
	downLimiter := rate.NewLimiter(rate.Inf, 1<<20)
	upLimiter := rate.NewLimiter(rate.Inf, 1<<20)
	cc.DownloadRateLimiter = downLimiter
	cc.UploadRateLimiter = upLimiter

	cl, err := torrent.NewClient(cc)
	if err != nil {
		return nil, fmt.Errorf("torrent client: %w", err)
	}

	e := &Engine{
		cl:          cl,
		store:       store,
		ram:         ram,
		diskPath:    cfg.Cache.Path,
		downLimiter: downLimiter,
		upLimiter:   upLimiter,
		managed:     map[string]*managed{},
	}
	e.ApplyRateLimits(cfg.Net)
	e.startSeedEnforcer() // keepSeed ratio/time enforcement (SPEC §6.4)
	return e, nil
}

// diskStore lazily creates the on-disk storage backend on first use, so a
// pure-RAM setup never opens a piece-completion db in the disk path.
func (e *Engine) diskStore() storage.ClientImplCloser {
	e.diskOnce.Do(func() {
		d, err := newDiskStorage(e.diskPath)
		if err != nil {
			log.Printf("disk storage at %q unavailable: %v", e.diskPath, err)
			return
		}
		e.disk = d
	})
	return e.disk
}

// ApplyRateLimits updates the global up/down speed caps at runtime. Values are
// in KB/s; 0 means unlimited (SPEC §6.3).
func (e *Engine) ApplyRateLimits(net config.Net) {
	setLimiter(e.downLimiter, net.DownKbps)
	setLimiter(e.upLimiter, net.UpKbps)
}

func setLimiter(l *rate.Limiter, kbps int) {
	if l == nil {
		return
	}
	if kbps <= 0 {
		l.SetLimit(rate.Inf)
		l.SetBurst(1 << 20)
		return
	}
	bps := float64(kbps) * 1024
	burst := int(bps)
	if burst < 1<<18 { // allow short bursts even at low caps
		burst = 1 << 18
	}
	l.SetLimit(rate.Limit(bps))
	l.SetBurst(burst)
}

// Close drops all torrents and shuts the client down (SPEC §9 graceful shutdown).
func (e *Engine) Close() {
	e.mu.Lock()
	for _, m := range e.managed {
		m.t.Drop()
	}
	e.managed = map[string]*managed{}
	e.mu.Unlock()
	e.cl.Close()
	if e.disk != nil {
		_ = e.disk.Close()
	}
}

func (e *Engine) storageFor(mode string) storage.ClientImpl {
	if mode == "disk" {
		if d := e.diskStore(); d != nil {
			return d
		}
		return e.ram // disk unavailable → fall back to RAM
	}
	return e.ram
}

// Add ingests a magnet/infohash, applies rules, waits for metadata, classifies
// the payload and registers the torrent. Idempotent (SPEC §5).
func (e *Engine) Add(ctx context.Context, link string) (*AddResult, error) {
	cfg := e.store.Get()

	var spec *torrent.TorrentSpec
	var err error
	if isHTTPURL(link) {
		// .torrent download URL (e.g. Prowlarr/private trackers, which embed your
		// passkey and don't offer magnets) — fetch and parse it (SPEC §5).
		spec, err = specFromTorrentURL(ctx, link)
	} else {
		spec, err = specFromLink(link)
	}
	if err != nil {
		return nil, err
	}

	// Idempotent (SPEC §5): if this torrent is already active, return it as-is —
	// never restart its download. anacrolix keeps verifying/resuming/seeding the
	// existing data. Re-adds (LumoraTV polling, pack episodes, restarts) are no-ops.
	hash0 := spec.InfoHash.HexString()
	e.mu.Lock()
	existing, ok := e.managed[hash0]
	e.mu.Unlock()
	if ok && existing.t.Info() != nil {
		info := e.infoOf(existing)
		playable := false
		for _, f := range info.Files {
			if f.Playable {
				playable = true
				break
			}
		}
		return &AddResult{Hash: info.Hash, Name: info.Name, Files: info.Files, Playable: playable}, nil
	}

	ruleList, _ := e.store.Rules()
	subj := rules.Subject{Name: spec.DisplayName, Tracker: firstTracker(spec)}
	dec := rules.Evaluate(ruleList, subj, cfg.Seed)
	if dec.Reject {
		return nil, &ErrRejected{Note: dec.RejectNote}
	}

	mode := cfg.Cache.Mode
	if dec.ForceMode != "" {
		mode = dec.ForceMode
	}
	spec.Storage = e.storageFor(mode)

	t, _, err := e.cl.AddTorrentSpec(spec)
	if err != nil {
		return nil, err
	}

	// Wait for metadata (bounded).
	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		t.Drop()
		return nil, fmt.Errorf("timed out fetching torrent metadata")
	}

	// Smart private-tracker seeding (SPEC §6.2): a torrent flagged "private"
	// (BEP27) comes from a private tracker that enforces ratio/time. Auto-seed it
	// to the configured targets without needing an explicit rule.
	isPrivate := isPrivateInfo(t)
	if isPrivate && cfg.Seed.PrivateAuto && !dec.KeepSeed {
		dec.KeepSeed = true
		dec.SeedRatio = cfg.Seed.PrivateMaxRatio
		dec.SeedMinutes = cfg.Seed.PrivateMaxMinutes
	}
	// Seeding needs persisted pieces; if we landed in RAM, re-add on disk. Only
	// metadata has been fetched so far, so re-adding is cheap.
	if dec.KeepSeed && mode == "ram" {
		t.Drop()
		spec.Storage = e.storageFor("disk")
		t, _, err = e.cl.AddTorrentSpec(spec)
		if err != nil {
			return nil, err
		}
		select {
		case <-t.GotInfo():
		case <-ctx.Done():
			t.Drop()
			return nil, fmt.Errorf("timed out preparing disk seeding")
		}
		mode = "disk"
	}

	hash := t.InfoHash().HexString()
	files := buildFiles(t)

	// Compressed / playability classification (SPEC §6.5).
	cls := classify.Classify(toClassifyFiles(files))
	var warnings []string
	playable := cls.Playable
	if cls.Warning != "" {
		warnings = append(warnings, cls.Warning)
	}
	for i := range files {
		files[i].Playable = isVideo(files[i].Path)
	}
	if !playable && cfg.Compressed.Reject {
		t.Drop()
		return nil, &ErrRejected{Note: cls.Warning, Warnings: warnings}
	}

	m := &managed{
		t: t, hash: hash, link: link, mode: mode,
		addedAt: time.Now(), lastSampleT: time.Now(),
		keepSeed: dec.KeepSeed, sRatio: dec.SeedRatio, sMinutes: dec.SeedMinutes,
		private: isPrivate,
	}
	// Carry forward persisted ratio accounting so restarts don't reset progress
	// toward the seed target (SPEC §8).
	if rec, ok := e.store.GetTorrent(hash); ok {
		m.baseUp, m.baseDown = rec.BytesUp, rec.BytesDown
		if rec.SeedStartedAt > 0 {
			m.seedStartedAt = time.Unix(rec.SeedStartedAt, 0)
		}
	}

	// NOTE: we deliberately do NOT DownloadAll() here. While a file is streaming,
	// the reader must drive a sequential download (like TorrServer) so the bytes
	// just ahead of the playhead arrive fast and the player's buffer fills.
	// DownloadAll spreads bandwidth across every piece and starves the playhead,
	// which makes players sit forever waiting for their initial cache. For
	// keepSeed torrents the whole file is fetched later, only while idle (see
	// the seed enforcer).

	// Per-torrent connection cap override from a rule (SPEC §6.4).
	if dec.MaxConns > 0 {
		t.SetMaxEstablishedConns(dec.MaxConns)
	}

	e.mu.Lock()
	e.managed[hash] = m
	e.enforceActiveLimitLocked(cfg.Limits.MaxActiveTorrents)
	e.mu.Unlock()

	_ = e.store.SaveTorrent(config.TorrentRecord{
		Hash: hash, Link: link, Name: t.Name(), AddedAt: m.addedAt.Unix(),
		StorageMode: mode, SeedUntilRatio: dec.SeedRatio, SeedUntilMinutes: dec.SeedMinutes,
		BytesUp: m.baseUp, BytesDown: m.baseDown, SeedStartedAt: rec0(m.seedStartedAt),
	})

	return &AddResult{Hash: hash, Name: t.Name(), Files: files, Playable: playable, Warnings: warnings}, nil
}

// rec0 returns a unix timestamp or 0 for a zero time.
func rec0(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// enforceActiveLimitLocked drops the oldest idle torrent past the cap (SPEC §6.3).
func (e *Engine) enforceActiveLimitLocked(max int) {
	if len(e.managed) <= max {
		return
	}
	type cand struct {
		hash string
		when time.Time
	}
	var idle []cand
	for h, m := range e.managed {
		m.mu.Lock()
		busy := m.readers > 0 || m.keepSeed
		last := m.played
		if last.IsZero() {
			last = m.addedAt
		}
		m.mu.Unlock()
		if !busy {
			idle = append(idle, cand{h, last})
		}
	}
	sort.Slice(idle, func(i, j int) bool { return idle[i].when.Before(idle[j].when) })
	for len(e.managed) > max && len(idle) > 0 {
		victim := idle[0]
		idle = idle[1:]
		if m, ok := e.managed[victim.hash]; ok {
			m.t.Drop()
			delete(e.managed, victim.hash)
		}
	}
}

// Ensure returns the managed torrent, re-adding from the persisted record if it
// was dropped — the idempotent wake that eliminates zombie state (SPEC §5, §9).
func (e *Engine) Ensure(ctx context.Context, hash string) (*managed, error) {
	e.mu.Lock()
	m, ok := e.managed[hash]
	e.mu.Unlock()
	if ok {
		return m, nil
	}
	rec, found := e.store.GetTorrent(hash)
	if !found {
		return nil, errors.New("unknown torrent")
	}
	if _, err := e.Add(ctx, rec.Link); err != nil {
		return nil, err
	}
	e.mu.Lock()
	m = e.managed[hash]
	e.mu.Unlock()
	if m == nil {
		return nil, errors.New("failed to wake torrent")
	}
	return m, nil
}

// Get returns the live Info for a hash (must be active).
func (e *Engine) Get(hash string) (*Info, bool) {
	e.mu.Lock()
	m, ok := e.managed[hash]
	e.mu.Unlock()
	if !ok {
		return nil, false
	}
	return e.infoOf(m), true
}

// List returns all active torrents (SPEC §7 GET /api/torrents).
func (e *Engine) List() []Info {
	e.mu.Lock()
	ms := make([]*managed, 0, len(e.managed))
	for _, m := range e.managed {
		ms = append(ms, m)
	}
	e.mu.Unlock()
	out := make([]Info, 0, len(ms))
	for _, m := range ms {
		out = append(out, *e.infoOf(m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt > out[j].AddedAt })
	return out
}

func (e *Engine) infoOf(m *managed) *Info {
	files := buildFiles(m.t)
	for i := range files {
		files[i].Playable = isVideo(files[i].Path)
	}

	m.mu.Lock()
	clients := make([]StreamClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	elapsed := 0
	if !m.seedStartedAt.IsZero() {
		elapsed = int(time.Since(m.seedStartedAt).Minutes())
	}
	m.mu.Unlock()

	kind := "stream"
	if m.keepSeed {
		kind = "seeding"
	}

	return &Info{
		Hash: m.hash, Name: m.t.Name(), SizeB: m.t.Length(),
		Files: files, Stats: e.statsOf(m), Mode: m.mode, AddedAt: m.addedAt.Unix(),
		Kind: kind, Private: m.private, KeepSeed: m.keepSeed,
		SeedTargetRatio: m.sRatio, SeedTargetMinutes: m.sMinutes, SeedElapsedMin: elapsed,
		Clients: clients,
	}
}

func (e *Engine) statsOf(m *managed) Stats {
	ts := m.t.Stats()
	down := ts.ConnStats.BytesReadData.Int64()
	up := ts.ConnStats.BytesWrittenData.Int64()

	m.mu.Lock()
	now := time.Now()
	dt := now.Sub(m.lastSampleT).Seconds()
	if dt >= 0.5 {
		m.downKbps = int64(float64(down-m.lastDown) * 8 / 1000 / dt)
		m.upKbps = int64(float64(up-m.lastUp) * 8 / 1000 / dt)
		m.lastDown, m.lastUp, m.lastSampleT = down, up, now
	}
	dk, uk := m.downKbps, m.upKbps
	readers := m.readers
	baseUp := m.baseUp
	m.mu.Unlock()

	var progress float64
	if m.t.Length() > 0 {
		progress = float64(m.t.BytesCompleted()) / float64(m.t.Length())
	}
	ratio := 0.0
	if down > 0 {
		ratio = float64(up) / float64(down)
	}

	state := "downloading"
	switch {
	case m.t.Info() == nil:
		state = "fetching"
	case readers > 0:
		state = "playing"
	case progress >= 0.999 && m.t.Seeding():
		state = "seeding"
	case progress >= 0.999:
		state = "ready"
	case ts.ActivePeers == 0 && ts.TotalPeers == 0:
		state = "searching"
	}

	cacheFill := int(m.t.BytesCompleted() >> 20)
	if m.mode == "ram" {
		cacheFill = int(e.ram.usedBytes() >> 20)
	}

	return Stats{
		Peers: ts.ActivePeers, Seeders: ts.ConnectedSeeders,
		DownKbps: dk, UpKbps: uk, Progress: progress,
		CacheFillMB: cacheFill, State: state, Ratio: ratio,
		UploadedBytes: baseUp + up,
	}
}

// Drop releases peers + cache but keeps metadata for instant re-add (SPEC §5).
func (e *Engine) Drop(hash string) error {
	e.mu.Lock()
	m, ok := e.managed[hash]
	if ok {
		delete(e.managed, hash)
	}
	e.mu.Unlock()
	if !ok {
		return nil
	}
	m.t.Drop()
	return nil
}

// Delete removes the torrent and optionally its on-disk files (SPEC §5).
func (e *Engine) Delete(hash string, withFiles bool) error {
	e.mu.Lock()
	m, ok := e.managed[hash]
	if ok {
		delete(e.managed, hash)
	}
	e.mu.Unlock()

	rec, found := e.store.GetTorrent(hash)
	if ok {
		m.t.Drop()
	}
	_ = e.store.DeleteTorrent(hash)

	if withFiles && found && rec.StorageMode == "disk" {
		cfg := e.store.Get()
		_ = os.RemoveAll(filepath.Join(cfg.Cache.Path, rec.Name))
	}
	return nil
}

func (e *Engine) usedRAM() int64 { return e.ram.usedBytes() }

// --- helpers ---

func specFromLink(link string) (*torrent.TorrentSpec, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return nil, errors.New("empty link")
	}
	if strings.HasPrefix(link, "magnet:") {
		return torrent.TorrentSpecFromMagnetUri(link)
	}
	// bare infohash (40 hex or 32 base32)
	if len(link) == 40 {
		var h metainfo.Hash
		if err := h.FromHexString(link); err == nil {
			return &torrent.TorrentSpec{InfoHash: h, DisplayName: link}, nil
		}
	}
	return nil, fmt.Errorf("unrecognized link (expected magnet: or 40-char infohash)")
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// qbittorrentPeerID builds a 20-byte BEP20 peer id with qBittorrent's prefix so
// private trackers that whitelist clients accept us.
func qbittorrentPeerID() string {
	const prefix = "-qB4390-"
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 20)
	copy(b, prefix)
	for i := len(prefix); i < 20; i++ {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// specFromTorrentURL fetches a .torrent file over HTTP(S) and builds a spec from
// it. Used for indexer/private-tracker download links (Prowlarr, Jackett, …) that
// carry a passkey and provide no magnet. The metadata is embedded, so the torrent
// is ready immediately (no DHT wait), and the private flag is preserved.
func specFromTorrentURL(ctx context.Context, url string) (*torrent.TorrentSpec, error) {
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Some indexers return the .torrent bytes; others 302-redirect to a magnet.
	// Catch the magnet redirect so both kinds of indexers work.
	var magnet string
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme == "magnet" {
				magnet = req.URL.String()
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FluxTorrent/1.0")
	req.Header.Set("Accept", "application/x-bittorrent, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch .torrent: %w", err)
	}
	defer resp.Body.Close()

	if magnet != "" {
		return torrent.TorrentSpecFromMagnetUri(magnet)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch .torrent: HTTP %d from indexer", resp.StatusCode)
	}
	mi, err := metainfo.Load(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("parse .torrent (is the indexer link valid?): %w", err)
	}
	return torrent.TorrentSpecFromMetaInfo(mi), nil
}

func firstTracker(spec *torrent.TorrentSpec) string {
	for _, tier := range spec.Trackers {
		if len(tier) > 0 {
			return tier[0]
		}
	}
	return ""
}

func buildFiles(t *torrent.Torrent) []File {
	if t.Info() == nil {
		return nil
	}
	tf := t.Files()
	out := make([]File, 0, len(tf))
	for i, f := range tf {
		out = append(out, File{Index: i, Path: f.DisplayPath(), SizeB: f.Length()})
	}
	return out
}

func toClassifyFiles(fs []File) []classify.File {
	out := make([]classify.File, len(fs))
	for i, f := range fs {
		out[i] = classify.File{Path: f.Path, Size: f.SizeB}
	}
	return out
}

// isPrivateInfo reports whether the torrent is flagged private (BEP27).
func isPrivateInfo(t *torrent.Torrent) bool {
	info := t.Info()
	return info != nil && info.Private != nil && *info.Private
}

func isVideo(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".mkv", ".mp4", ".avi", ".m4v", ".ts", ".webm", ".mov":
		return true
	}
	return false
}
