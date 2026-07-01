package engine

// Disk retention (SPEC §6.1 disk mode).
//
// Goal: keep only what is in use or seeding toward a live target, and free
// everything else — while guaranteeing the disk-cache dir never exceeds
// Disk.MaxGB (the disk analogue of the RAM ring-buffer cap).
//
// Two phases keep it safe:
//   - retire(): a torrent that is no longer needed. Private torrents are deleted
//     immediately (their tracker ratio/time already served as the retention
//     window); public torrents are dropped from the active set but kept on disk
//     and marked PendingDeleteAt = now + grace, so a returning viewer resumes
//     instantly from the retained files.
//   - the janitor loop: deletes pending content once its grace window elapses,
//     and, if the dir is over the cap, evicts the oldest inactive content first
//     (the hard cap overrides the grace window).

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jodacame/fluxtorrent/internal/config"
)

const janitorInterval = 30 * time.Second

// startJanitor launches the periodic disk-retention loop.
func (e *Engine) startJanitor() {
	go func() {
		t := time.NewTicker(janitorInterval)
		defer t.Stop()
		for range t.C {
			e.sweepPending()
			e.enforceDiskCap()
		}
	}()
}

// retire disposes of a torrent that is no longer needed (see file header).
func (e *Engine) retire(hash string, private bool) {
	if private {
		_ = e.Delete(hash, true) // immediate: private manages its own window
		return
	}
	grace := time.Duration(e.store.Get().Disk.GraceMinutes) * time.Minute
	_ = e.Drop(hash) // stop sharing/downloading, keep the files on disk
	rec, ok := e.store.GetTorrent(hash)
	if !ok {
		return
	}
	rec.PendingDeleteAt = time.Now().Add(grace).Unix()
	if rec.LastPlayedAt == 0 {
		rec.LastPlayedAt = time.Now().Unix()
	}
	_ = e.store.SaveTorrent(rec)
	log.Printf("retire: %s kept ~%dm before deletion", shortHash(hash), e.store.Get().Disk.GraceMinutes)
}

// sweepPending deletes public records whose grace window has elapsed and that
// are not currently active (an active torrent means a viewer rescued it).
func (e *Engine) sweepPending() {
	now := time.Now().Unix()
	recs, _ := e.store.Torrents()
	for _, r := range recs {
		if r.PendingDeleteAt == 0 || r.PendingDeleteAt > now {
			continue
		}
		if e.isActive(r.Hash) {
			continue // rescued
		}
		if freed := e.deleteRecordFiles(r); freed >= 0 {
			log.Printf("janitor: deleted %s (%s)", shortHash(r.Hash), r.Name)
		}
	}
}

// enforceDiskCap keeps the disk-cache dir under Disk.MaxGB. It reconciles against
// what is actually on disk — so it reclaims retired content AND untracked orphans
// (files left behind by a client "rem", a delete-without-files, or a past run) —
// evicting the oldest inactive item first. Content that is being watched or
// seeding toward a live target (i.e. an active torrent) is never touched. The
// hard cap overrides the grace window.
func (e *Engine) enforceDiskCap() {
	cfg := e.store.Get()
	if cfg.Disk.MaxGB <= 0 {
		return
	}
	limit := int64(cfg.Disk.MaxGB) << 30
	root := filepath.Clean(cfg.Cache.Path)
	used := dirSize(root)
	if used <= limit {
		return
	}

	cand := e.scanDisk() // everything on disk that is NOT an active torrent
	sort.Slice(cand, func(i, j int) bool { return cand[i].age < cand[j].age })

	for _, d := range cand {
		if used <= limit {
			break
		}
		if e.evictDiskEntry(d) {
			used -= d.size
			kind := "retired"
			if d.orphan {
				kind = "orphan"
			}
			log.Printf("disk cap: evicted %s %s (%s), freed %dMB", kind, shortHash(d.hash), d.name, d.size>>20)
		}
	}
	if used > limit {
		// Only active/seeding content remains — surface it rather than silently
		// over-run the cap.
		log.Printf("disk cap: still over budget (%dMB > %dMB) — only active/seeding content remains",
			used>>20, limit>>20)
	}
}

// diskEntry is one top-level item in the download folder during reconciliation.
type diskEntry struct {
	name   string
	path   string
	size   int64
	age    int64  // unix seconds; smaller = older = evicted first
	hash   string // owning record's hash, or "" for an orphan
	orphan bool
}

// scanDisk lists the download folder's top-level entries that are NOT backed by
// an active (running) torrent — i.e. retired content and untracked orphans. Each
// is dated for eviction: known records by last use, orphans by file mtime.
func (e *Engine) scanDisk() []diskEntry {
	root := filepath.Clean(e.store.Get().Cache.Path)
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	active := e.activeDiskNames()
	byName := map[string]config.TorrentRecord{}
	if recs, err := e.store.Torrents(); err == nil {
		for _, r := range recs {
			byName[r.Name] = r
		}
	}
	out := make([]diskEntry, 0, len(ents))
	for _, ent := range ents {
		name := ent.Name()
		if active[name] {
			continue // in use / seeding toward a live target — protected
		}
		p := filepath.Join(root, name)
		d := diskEntry{name: name, path: p, size: dirSize(p)}
		if r, ok := byName[name]; ok {
			d.hash, d.age = r.Hash, evictKey(r)
		} else {
			d.orphan, d.age = true, entryMtime(p)
		}
		out = append(out, d)
	}
	return out
}

// evictDiskEntry removes one reconciled entry (re-checking it isn't active and
// stays inside the cache dir) and drops its record if any.
func (e *Engine) evictDiskEntry(d diskEntry) bool {
	if e.activeDiskNames()[d.name] {
		return false // became active between scan and eviction
	}
	path, ok := e.safeDataPath(d.name)
	if !ok {
		return false
	}
	if err := os.RemoveAll(path); err != nil {
		log.Printf("janitor: remove %s: %v", path, err)
		return false
	}
	if d.hash != "" {
		_ = e.store.DeleteTorrent(d.hash)
	}
	return true
}

// activeDiskNames is the set of on-disk folder names backed by a running
// disk-mode torrent (never evicted).
func (e *Engine) activeDiskNames() map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make(map[string]bool, len(e.managed))
	for _, m := range e.managed {
		if m.mode == "disk" && m.t.Info() != nil {
			names[m.t.Name()] = true
		}
	}
	return names
}

// Retire disposes of a torrent from an external control request (e.g. a
// TorrServer "rem"): it is dropped and its files are cleaned up on the normal
// schedule (private immediately, public after the grace window) rather than
// being left behind untracked as an orphan.
func (e *Engine) Retire(hash string) {
	private := false
	if m, ok := e.managedOf(hash); ok {
		private = m.private
	}
	e.retire(hash, private)
}

// managedOf returns the active managed torrent for hash, if any.
func (e *Engine) managedOf(hash string) (*managed, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.managed[hash]
	return m, ok
}

// OrphanInfo describes an on-disk item with no backing record and no active
// torrent — i.e. something not shown in the listing: leftovers from before
// retention existed, a client "rem", or a delete-without-files.
type OrphanInfo struct {
	Name  string `json:"name"`
	SizeB int64  `json:"sizeBytes"`
}

// ListOrphans returns the disk-cache entries that are safe to remove on demand
// (no record, not active). Used to preview an orphan cleanup before running it.
func (e *Engine) ListOrphans() []OrphanInfo {
	out := []OrphanInfo{}
	for _, d := range e.scanDisk() {
		if d.orphan {
			out = append(out, OrphanInfo{Name: d.name, SizeB: d.size})
		}
	}
	return out
}

// CleanOrphans deletes every orphan (see ListOrphans) and reports how many were
// removed and the bytes freed. It never touches active/seeding content or
// records still in the listing.
func (e *Engine) CleanOrphans() (removed int, freed int64) {
	for _, d := range e.scanDisk() {
		if !d.orphan {
			continue
		}
		if e.evictDiskEntry(d) {
			removed++
			freed += d.size
			log.Printf("cleanup: removed orphan %s (%dMB)", d.name, d.size>>20)
		}
	}
	return removed, freed
}

// entryMtime returns a path's modification time in unix seconds (0 if missing).
func entryMtime(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime().Unix()
	}
	return 0
}

// deleteRecordFiles removes a record's on-disk files (guarded) and the record
// itself, returning the bytes freed. Only disk-mode records with a path that
// stays strictly inside the cache dir are touched. The record is dropped ONLY
// once the files are actually gone — so a failed or refused removal keeps the
// record (and thus the files stay tracked) instead of orphaning them on disk.
func (e *Engine) deleteRecordFiles(r config.TorrentRecord) int64 {
	if r.StorageMode != "disk" {
		_ = e.store.DeleteTorrent(r.Hash) // nothing on disk to reconcile
		return 0
	}
	path, ok := e.safeDataPath(r.Name)
	if !ok {
		log.Printf("janitor: refusing unsafe delete path for %s (name=%q) — keeping record", shortHash(r.Hash), r.Name)
		return 0
	}
	size := dirSize(path)
	if err := os.RemoveAll(path); err != nil {
		log.Printf("janitor: remove %s: %v — keeping record", path, err)
		return 0
	}
	_ = e.store.DeleteTorrent(r.Hash) // files gone → drop the record
	return size
}

// safeDataPath resolves a torrent's on-disk root and verifies it stays strictly
// inside the disk-cache dir — guarding against an empty name (which would target
// the whole dir) and path traversal via a crafted torrent name.
func (e *Engine) safeDataPath(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	root := filepath.Clean(e.store.Get().Cache.Path)
	p := filepath.Clean(filepath.Join(root, name))
	if p == root || !strings.HasPrefix(p, root+string(os.PathSeparator)) {
		return "", false
	}
	return p, true
}

// isActive reports whether the hash is currently a managed (running) torrent.
func (e *Engine) isActive(hash string) bool {
	e.mu.Lock()
	_, ok := e.managed[hash]
	e.mu.Unlock()
	return ok
}

// evictKey is the eviction ordering: least-recently-used first (fall back to
// add time for never-played content).
func evictKey(r config.TorrentRecord) int64 {
	if r.LastPlayedAt > 0 {
		return r.LastPlayedAt
	}
	return r.AddedAt
}

// dirSize sums the byte size of every file under path (0 if it doesn't exist).
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func shortHash(h string) string {
	if len(h) >= 8 {
		return h[:8]
	}
	return h
}
