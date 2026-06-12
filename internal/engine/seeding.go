package engine

// keepSeed enforcement (SPEC §6.4).
//
// A `keepSeed` rule means: download the whole torrent, save it (disk mode), and
// keep sharing it until a ratio OR a time target is reached, then drop it. This
// file holds the background enforcer that:
//   - tracks uploaded/downloaded bytes (accumulated across restarts via bbolt),
//   - starts the seed timer when the torrent reaches 100%,
//   - drops the torrent once `maxRatio` OR `maxMinutes` is met (OR semantics),
//   - and resumes keepSeed torrents on startup so sharing survives restarts.
//
// Everything else (the default) streams in RAM and is dropped after playback —
// no rule needed for those, exactly as intended.

import (
	"context"
	"log"
	"time"
)

const seedTickInterval = 20 * time.Second

// startSeedEnforcer launches the periodic keepSeed enforcement loop.
func (e *Engine) startSeedEnforcer() {
	go func() {
		t := time.NewTicker(seedTickInterval)
		defer t.Stop()
		for range t.C {
			e.enforceSeeding()
			e.reapIdle()
		}
	}()
}

// reapIdle drops torrents that have sat idle (no readers, not seeding) longer
// than the configured disconnect timeout (SPEC §6.3, 0 = off).
func (e *Engine) reapIdle() {
	timeout := e.store.Get().Net.DisconnectTimeoutSec
	if timeout <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(timeout) * time.Second)
	e.mu.Lock()
	var drop []string
	for h, m := range e.managed {
		m.mu.Lock()
		last := m.played
		if last.IsZero() {
			last = m.addedAt
		}
		idle := m.readers == 0 && !m.keepSeed && last.Before(cutoff)
		m.mu.Unlock()
		if idle {
			drop = append(drop, h)
		}
	}
	e.mu.Unlock()
	for _, h := range drop {
		log.Printf("disconnect-timeout: dropping idle torrent %s", h)
		_ = e.Drop(h)
	}
}

// enforceSeeding checks every keepSeed torrent against its ratio/time target.
func (e *Engine) enforceSeeding() {
	e.mu.Lock()
	ms := make([]*managed, 0, len(e.managed))
	for _, m := range e.managed {
		if m.keepSeed {
			ms = append(ms, m)
		}
	}
	e.mu.Unlock()

	for _, m := range ms {
		if m.t.Info() == nil {
			continue
		}
		st := m.t.Stats()

		m.mu.Lock()
		// Keep fetching the whole torrent if a restart re-added it lazily.
		if !m.downloadAllSet {
			m.t.DownloadAll()
			m.downloadAllSet = true
		}
		totalDown := m.baseDown + st.ConnStats.BytesReadData.Int64()
		totalUp := m.baseUp + st.ConnStats.BytesWrittenData.Int64()
		complete := m.t.Length() > 0 && m.t.BytesCompleted() >= m.t.Length()
		if complete && m.seedStartedAt.IsZero() {
			m.seedStartedAt = time.Now() // sharing of the full content begins now
		}
		ratio := 0.0
		if totalDown > 0 {
			ratio = float64(totalUp) / float64(totalDown)
		}
		seedStarted := m.seedStartedAt
		sRatio, sMinutes := m.sRatio, m.sMinutes
		readers := m.readers
		m.mu.Unlock()

		// Persist accounting so ratio/time survive a restart (SPEC §8).
		e.updateSeedRecord(m.hash, totalUp, totalDown, seedStarted)

		met := false
		if sRatio > 0 && ratio >= sRatio {
			met = true
		}
		if sMinutes > 0 && complete && time.Since(seedStarted) >= time.Duration(sMinutes)*time.Minute {
			met = true
		}
		// Don't cut off an active viewer; drop once the target is met and idle.
		if met && readers == 0 {
			log.Printf("keepSeed target met for %s (ratio=%.2f) — dropping", m.hash, ratio)
			e.clearKeepSeed(m.hash)
			_ = e.Drop(m.hash)
		}
	}
}

// updateSeedRecord persists the running ratio accounting for a torrent.
func (e *Engine) updateSeedRecord(hash string, up, down int64, started time.Time) {
	rec, ok := e.store.GetTorrent(hash)
	if !ok {
		return
	}
	rec.BytesUp, rec.BytesDown, rec.SeedStartedAt = up, down, rec0(started)
	_ = e.store.SaveTorrent(rec)
}

// clearKeepSeed removes the seed target so a restart won't resume seeding it.
func (e *Engine) clearKeepSeed(hash string) {
	rec, ok := e.store.GetTorrent(hash)
	if !ok {
		return
	}
	rec.SeedUntilRatio, rec.SeedUntilMinutes = 0, 0
	_ = e.store.SaveTorrent(rec)
}

// ResumeKeepSeed re-adds persisted keepSeed torrents on startup so sharing
// continues across restarts. Their disk data is re-verified and seeded.
func (e *Engine) ResumeKeepSeed(ctx context.Context) {
	recs, _ := e.store.Torrents()
	for _, r := range recs {
		if r.SeedUntilRatio <= 0 && r.SeedUntilMinutes <= 0 {
			continue // not a keepSeed torrent
		}
		actx, cancel := context.WithTimeout(ctx, 120*time.Second)
		if _, err := e.Add(actx, r.Link); err != nil {
			log.Printf("resume keepSeed %s: %v", r.Hash, err)
		}
		cancel()
	}
}
