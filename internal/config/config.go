// Package config holds FluxTorrent settings + rules and persists them in bbolt.
package config

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// Cache settings (SPEC §6.1).
type Cache struct {
	Mode        string `json:"mode"`        // "ram" | "disk"
	SizeMB      int    `json:"sizeMB"`      // RAM ring-buffer cap
	Path        string `json:"path"`        // disk storage root
	ReadaheadMB int    `json:"readaheadMB"` // bytes prioritized ahead of read head
}

// Seed settings (SPEC §6.2).
type Seed struct {
	Enabled           bool    `json:"enabled"`
	DropAfterPlayback bool    `json:"dropAfterPlayback"`
	MaxRatio          float64 `json:"maxRatio"`   // 0 = ∞
	MaxMinutes        int     `json:"maxMinutes"` // 0 = ∞

	// Smart private-tracker seeding (SPEC §6.2): torrents flagged "private"
	// (BEP27) come from private trackers that enforce ratio/time. When enabled,
	// FluxTorrent auto-seeds them to these targets (OR semantics) without needing
	// an explicit rule, and forces disk storage so they can be shared.
	PrivateAuto       bool    `json:"privateAuto"`
	PrivateMaxRatio   float64 `json:"privateMaxRatio"`
	PrivateMaxMinutes int     `json:"privateMaxMinutes"`
}

// Net settings (SPEC §6.3). The lower block is "advanced" — sensible defaults,
// rarely touched; some are client-level and apply on restart.
type Net struct {
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	BTPort     int    `json:"btPort"`
	DHT        bool   `json:"dht"`
	MaxConns   int    `json:"maxConns"`
	DownKbps   int    `json:"downKbps"`
	UpKbps     int    `json:"upKbps"`

	// Advanced (SPEC §6.3)
	EncryptHeaders       bool `json:"encryptHeaders"`       // force header obfuscation/encryption
	IPv6                 bool `json:"ipv6"`                 // allow IPv6 peers
	UTP                  bool `json:"utp"`                  // allow µTP transport
	DisconnectTimeoutSec int  `json:"disconnectTimeoutSec"` // drop an idle, non-seeding torrent after N s (0 = off)
}

// Limits settings (SPEC §6.3).
type Limits struct {
	MaxActiveTorrents int `json:"maxActiveTorrents"`
}

// Compressed-release handling (SPEC §6.5).
type Compressed struct {
	Reject bool `json:"reject"`
}

// Settings is the full config object persisted in bbolt.
type Settings struct {
	Cache             Cache      `json:"cache"`
	Seed              Seed       `json:"seed"`
	Net               Net        `json:"net"`
	Limits            Limits     `json:"limits"`
	Compressed        Compressed `json:"compressed"`
	NoPeersTimeoutSec int        `json:"noPeersTimeoutSec"`
	APIToken          string     `json:"apiToken"`
}

// SeedTarget overrides global seed limits for a keepSeed rule.
type SeedTarget struct {
	MaxRatio   float64 `json:"maxRatio"`
	MaxMinutes int     `json:"maxMinutes"`
}

// Rule is a single match→action entry (SPEC §6.4).
type Rule struct {
	Match struct {
		Field string `json:"field"` // indexer|tracker|name
		Op    string `json:"op"`    // equals|contains|regex
		Value string `json:"value"`
	} `json:"match"`
	Action   string      `json:"action"` // reject|prefer|forceDisk|forceRam|keepSeed
	Seed     *SeedTarget `json:"seed,omitempty"`
	MaxConns int         `json:"maxConns,omitempty"` // per-torrent connection cap override (0 = inherit)
	Note     string      `json:"note"`
}

// Defaults returns the SPEC default settings.
func Defaults() Settings {
	return Settings{
		Cache:             Cache{Mode: "ram", SizeMB: 1024, Path: "/downloads", ReadaheadMB: 64},
		Seed:              Seed{Enabled: false, DropAfterPlayback: true, MaxRatio: 0, MaxMinutes: 0, PrivateAuto: true, PrivateMaxRatio: 1.0, PrivateMaxMinutes: 4320},
		Net:               Net{ListenHost: "0.0.0.0", ListenPort: 7001, BTPort: 42069, DHT: true, MaxConns: 200, DownKbps: 0, UpKbps: 0, EncryptHeaders: false, IPv6: true, UTP: true, DisconnectTimeoutSec: 0},
		Limits:            Limits{MaxActiveTorrents: 8},
		Compressed:        Compressed{Reject: true},
		NoPeersTimeoutSec: 60,
		APIToken:          "",
	}
}

var (
	bSettings = []byte("settings")
	bRules    = []byte("rules")
	bTorrents = []byte("torrents")
	kSettings = []byte("current")
	kRules    = []byte("current")
)

// TorrentRecord is the persisted resume/seed state (SPEC §8).
type TorrentRecord struct {
	Hash             string  `json:"hash"`
	Link             string  `json:"link"`
	Name             string  `json:"name"`
	AddedAt          int64   `json:"addedAt"`
	LastPlayedAt     int64   `json:"lastPlayedAt"`
	StorageMode      string  `json:"storageMode"`
	BytesUp          int64   `json:"bytesUp"`
	BytesDown        int64   `json:"bytesDown"`
	SeedUntilRatio   float64 `json:"seedUntilRatio"`
	SeedUntilMinutes int     `json:"seedUntilMinutes"`
	SeedStartedAt    int64   `json:"seedStartedAt"`
}

// Store is the bbolt-backed config + state store. Safe for concurrent use.
type Store struct {
	db  *bolt.DB
	mu  sync.RWMutex
	cur Settings
}

// Open opens (or creates) the bbolt database at path and loads settings,
// applying env-var overrides and persisting the result.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bSettings, bRules, bTorrents} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		raw := tx.Bucket(bSettings).Get(kSettings)
		cfg := Defaults()
		if raw != nil {
			_ = json.Unmarshal(raw, &cfg)
		}
		applyEnv(&cfg)
		s.cur = cfg
		out, _ := json.Marshal(cfg)
		return tx.Bucket(bSettings).Put(kSettings, out)
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close flushes and closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Put validates, persists and hot-swaps the settings.
func (s *Store) Put(cfg Settings) error {
	normalize(&cfg)
	out, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bSettings).Put(kSettings, out)
	}); err != nil {
		return err
	}
	s.mu.Lock()
	s.cur = cfg
	s.mu.Unlock()
	return nil
}

// Rules returns the persisted ordered rule list.
func (s *Store) Rules() ([]Rule, error) {
	var rules []Rule
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bRules).Get(kRules)
		if raw == nil {
			return nil
		}
		return json.Unmarshal(raw, &rules)
	})
	if rules == nil {
		rules = []Rule{}
	}
	return rules, err
}

// PutRules persists the ordered rule list.
func (s *Store) PutRules(rules []Rule) error {
	out, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bRules).Put(kRules, out)
	})
}

// Torrents returns all persisted torrent records.
func (s *Store) Torrents() ([]TorrentRecord, error) {
	var recs []TorrentRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bTorrents).ForEach(func(_, v []byte) error {
			var r TorrentRecord
			if err := json.Unmarshal(v, &r); err != nil {
				return nil // skip corrupt entries
			}
			recs = append(recs, r)
			return nil
		})
	})
	return recs, err
}

// SaveTorrent upserts a torrent record.
func (s *Store) SaveTorrent(r TorrentRecord) error {
	out, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bTorrents).Put([]byte(r.Hash), out)
	})
}

// GetTorrent returns a persisted record by hash (ok=false if absent).
func (s *Store) GetTorrent(hash string) (TorrentRecord, bool) {
	var r TorrentRecord
	found := false
	_ = s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bTorrents).Get([]byte(hash))
		if raw == nil {
			return nil
		}
		if json.Unmarshal(raw, &r) == nil {
			found = true
		}
		return nil
	})
	return r, found
}

// DeleteTorrent removes a torrent record.
func (s *Store) DeleteTorrent(hash string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bTorrents).Delete([]byte(hash))
	})
}

func normalize(cfg *Settings) {
	if cfg.Cache.Mode != "disk" {
		cfg.Cache.Mode = "ram"
	}
	if cfg.Cache.SizeMB < 64 {
		cfg.Cache.SizeMB = 64
	}
	if cfg.Cache.ReadaheadMB < 1 {
		cfg.Cache.ReadaheadMB = 1
	}
	if cfg.Cache.Path == "" {
		cfg.Cache.Path = "/downloads"
	}
	if cfg.Net.ListenHost == "" {
		cfg.Net.ListenHost = "0.0.0.0"
	}
	if cfg.Net.ListenPort == 0 {
		cfg.Net.ListenPort = 7001
	}
	if cfg.Net.BTPort == 0 {
		cfg.Net.BTPort = 42069
	}
	if cfg.Net.MaxConns < 1 {
		cfg.Net.MaxConns = 200
	}
	if cfg.Limits.MaxActiveTorrents < 1 {
		cfg.Limits.MaxActiveTorrents = 8
	}
	if cfg.NoPeersTimeoutSec < 1 {
		cfg.NoPeersTimeoutSec = 60
	}
}

// applyEnv overlays FT_* environment variables onto the config (boot only).
func applyEnv(cfg *Settings) {
	if v := os.Getenv("FT_LISTEN_HOST"); v != "" {
		cfg.Net.ListenHost = v
	}
	if v := envInt("FT_LISTEN_PORT"); v != 0 {
		cfg.Net.ListenPort = v
	}
	if v := envInt("FT_BT_PORT"); v != 0 {
		cfg.Net.BTPort = v
	}
	if v := os.Getenv("FT_CACHE_MODE"); v != "" {
		cfg.Cache.Mode = v
	}
	if v := envInt("FT_CACHE_SIZE_MB"); v != 0 {
		cfg.Cache.SizeMB = v
	}
	if v := os.Getenv("FT_CACHE_PATH"); v != "" {
		cfg.Cache.Path = v
	}
	if v := envInt("FT_READAHEAD_MB"); v != 0 {
		cfg.Cache.ReadaheadMB = v
	}
	if v := os.Getenv("FT_API_TOKEN"); v != "" {
		cfg.APIToken = v
	}
	normalize(cfg)
}

func envInt(key string) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}
