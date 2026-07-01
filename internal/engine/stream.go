package engine

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/anacrolix/torrent"
)

// Streaming sentinel errors (mapped to HTTP status in the stream handler).
var (
	ErrNotFound = errors.New("torrent or file not found")
	ErrNoPeers  = errors.New("no peers found within the timeout")
	ErrNoMeta   = errors.New("torrent metadata not available")
)

// StreamReader is a seekable, Range-friendly reader over one torrent file.
type StreamReader struct {
	io.ReadSeeker
	Name    string
	Length  int64
	ModTime time.Time
	OnWrite func(n int) // report bytes sent to the client (for live send-rate)
	closer  func()
}

// Close releases the reader and applies drop-after-playback policy.
func (s *StreamReader) Close() error { s.closer(); return nil }

// OpenStream returns a reader for file `index`, waking the torrent if idle.
// reqStart is the requested Range start, used for RAM single-reader gating
// (SPEC §6.1, §9). client identifies the consuming player (shown in the UI).
// It blocks until the first bytes are reachable or the no-peers timeout (504).
func (e *Engine) OpenStream(ctx context.Context, hash string, index int, reqStart int64, client StreamClient) (*StreamReader, error) {
	m, err := e.Ensure(ctx, hash)
	if err != nil {
		return nil, ErrNotFound
	}
	if m.t.Info() == nil {
		select {
		case <-m.t.GotInfo():
		case <-ctx.Done():
			return nil, ErrNoMeta
		}
	}
	files := m.t.Files()
	if index < 0 || index >= len(files) {
		return nil, ErrNotFound
	}
	file := files[index]

	cfg := e.store.Get()
	readahead := int64(cfg.Cache.ReadaheadMB) << 20

	// Concurrent readers are allowed in every mode. Real players (AVPlayer, VLC,
	// Infuse…) routinely open several Range connections at once — e.g. one for the
	// MP4 `moov` atom at the tail and one for sequential playback at the head — so
	// rejecting the second reader would break playback (and TorrServer/Stremio
	// compatibility). The bounded RAM ring buffer still caps total memory; extra
	// distant read heads just cause LRU eviction churn, never a hard failure.
	m.mu.Lock()
	m.readers++
	m.readHead = reqStart
	m.played = time.Now()
	m.mu.Unlock()

	// Register the consuming player (shown in the UI; SPEC §11b).
	client.FileIndex = index
	client.File = file.DisplayPath()
	client.Since = time.Now().Unix()
	clientID := m.addClient(client)

	r := file.NewReader()
	r.SetReadahead(readahead)
	if reqStart > 0 {
		if _, err := r.Seek(reqStart, io.SeekStart); err != nil {
			m.removeClient(clientID)
			e.closeReader(m, r)
			return nil, err
		}
	}

	// No-peers timeout (SPEC §9): wait for the first byte to be reachable.
	if err := e.waitFirstByte(ctx, m, r, reqStart, cfg.NoPeersTimeoutSec); err != nil {
		m.removeClient(clientID)
		e.closeReader(m, r)
		return nil, err
	}

	sr := &StreamReader{
		ReadSeeker: r,
		Name:       file.DisplayPath(),
		Length:     file.Length(),
		ModTime:    m.addedAt,
		OnWrite:    func(n int) { m.addSent(clientID, n) },
		closer: func() {
			m.removeClient(clientID)
			e.closeReader(m, r)
		},
	}
	return sr, nil
}

// waitFirstByte blocks until the byte at reqStart is available or the no-peers
// timeout elapses with zero peers.
func (e *Engine) waitFirstByte(ctx context.Context, m *managed, r torrent.Reader, _ int64, timeoutSec int) error {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	rctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := r.ReadContext(rctx, buf)
		if err == nil || errors.Is(err, io.EOF) {
			// rewind the one byte we peeked
			_, _ = r.Seek(-1, io.SeekCurrent)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) {
			if m.t.Stats().TotalPeers == 0 {
				return ErrNoPeers
			}
			return err
		}
		return nil
	case <-rctx.Done():
		if m.t.Stats().TotalPeers == 0 {
			return ErrNoPeers
		}
		return nil // peers exist, let the stream proceed
	}
}

func (e *Engine) closeReader(m *managed, r torrent.Reader) {
	_ = r.Close()
	e.releaseReader(m)
}

// releaseReader undoes one reader registration: shared by torrent readers and
// the direct-from-disk path so drop-after-playback/keepSeed logic sees both.
func (e *Engine) releaseReader(m *managed) {
	m.mu.Lock()
	if m.readers > 0 {
		m.readers--
	}
	last := m.readers == 0
	keep := m.keepSeed
	m.mu.Unlock()

	if !last {
		return
	}
	cfg := e.store.Get()
	// keepSeed / global seeding retains it — the seed enforcer disposes of it
	// once its target is met.
	if keep || cfg.Seed.Enabled {
		return
	}
	if m.mode == "disk" {
		// Disk leftover: retire so the space is freed (public keeps a grace
		// window for a returning viewer; private is deleted immediately).
		if cfg.Disk.DeleteAfterPlayback {
			e.retire(m.hash, m.private)
		}
		return
	}
	// RAM: drop-after-playback frees the in-memory buffer (SPEC §6.2).
	if cfg.Seed.DropAfterPlayback {
		_ = e.Drop(m.hash)
	}
}
