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

// enforceDiskCap evicts the oldest inactive pending content until the disk-cache
// dir is back under Disk.MaxGB. The hard cap overrides the grace window.
func (e *Engine) enforceDiskCap() {
	cfg := e.store.Get()
	if cfg.Disk.MaxGB <= 0 {
		return
	}
	limit := int64(cfg.Disk.MaxGB) << 30
	used := dirSize(cfg.Cache.Path)
	if used <= limit {
		return
	}

	recs, _ := e.store.Torrents()
	cand := make([]config.TorrentRecord, 0, len(recs))
	for _, r := range recs {
		if r.PendingDeleteAt == 0 || e.isActive(r.Hash) {
			continue // only inactive, already-retired content is evictable
		}
		cand = append(cand, r)
	}
	// Oldest first: by last use, falling back to when it was added.
	sort.Slice(cand, func(i, j int) bool { return evictKey(cand[i]) < evictKey(cand[j]) })

	for _, r := range cand {
		if used <= limit {
			break
		}
		freed := e.deleteRecordFiles(r)
		if freed > 0 {
			used -= freed
		}
		log.Printf("disk cap: evicted %s (%s), freed %dMB", shortHash(r.Hash), r.Name, freed>>20)
	}
	if used > limit {
		// Everything evictable is gone but active/seeding content still exceeds
		// the cap — surface it rather than silently over-run.
		log.Printf("disk cap: still over budget (%dMB > %dMB) — only active/seeding content remains",
			used>>20, limit>>20)
	}
}

// deleteRecordFiles removes a record's on-disk files (guarded) and the record
// itself, returning the bytes freed. Only disk-mode records with a path that
// stays strictly inside the cache dir are touched.
func (e *Engine) deleteRecordFiles(r config.TorrentRecord) int64 {
	defer func() { _ = e.store.DeleteTorrent(r.Hash) }()
	if r.StorageMode != "disk" {
		return 0
	}
	path, ok := e.safeDataPath(r.Name)
	if !ok {
		log.Printf("janitor: refusing unsafe delete path for %s (name=%q)", shortHash(r.Hash), r.Name)
		return 0
	}
	size := dirSize(path)
	if err := os.RemoveAll(path); err != nil {
		log.Printf("janitor: remove %s: %v", path, err)
		return 0
	}
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
