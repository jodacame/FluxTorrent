package engine

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// ramStore is a bounded in-memory storage backend (SPEC §6.1 RAM mode).
// It holds piece data in memory and evicts the least-recently-used *complete*
// pieces once the global byte budget is exceeded, so memory stays bounded
// regardless of torrent size. Evicted pieces are marked not-complete so the
// engine re-fetches them on demand — the ring-buffer-around-the-read-head
// behaviour that keeps RAM mode predictable.
type ramStore struct {
	mu       sync.Mutex
	maxBytes int64
	used     int64
	clock    uint64
	pieces   map[*ramPiece]struct{}
}

func newRAMStore(maxBytes int64) *ramStore {
	return &ramStore{maxBytes: maxBytes, pieces: map[*ramPiece]struct{}{}}
}

func (s *ramStore) OpenTorrent(_ context.Context, info *metainfo.Info, _ metainfo.Hash) (storage.TorrentImpl, error) {
	t := &ramTorrent{store: s, pieces: map[int]*ramPiece{}}
	capFn := func() (int64, bool) { return s.maxBytes, true }
	return storage.TorrentImpl{
		Piece:    func(p metainfo.Piece) storage.PieceImpl { return t.piece(p) },
		Close:    t.Close,
		Capacity: &capFn,
	}, nil
}

func (s *ramStore) Close() error { return nil }

// usedBytes reports current resident bytes (for cache-fill stats).
func (s *ramStore) usedBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used
}

type ramTorrent struct {
	store  *ramStore
	mu     sync.Mutex
	pieces map[int]*ramPiece
}

func (t *ramTorrent) piece(p metainfo.Piece) *ramPiece {
	t.mu.Lock()
	defer t.mu.Unlock()
	idx := p.Index()
	if rp, ok := t.pieces[idx]; ok {
		return rp
	}
	rp := &ramPiece{store: t.store, length: p.Length()}
	t.pieces[idx] = rp
	return rp
}

func (t *ramTorrent) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	for _, rp := range t.pieces {
		if rp.data != nil {
			t.store.used -= rp.length
			delete(t.store.pieces, rp)
			rp.data = nil
		}
	}
	t.pieces = map[int]*ramPiece{}
	return nil
}

type ramPiece struct {
	store    *ramStore
	length   int64
	data     []byte // nil = not resident
	complete bool
	lastUsed uint64
}

func (p *ramPiece) Completion() storage.Completion {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	return storage.Completion{Complete: p.complete && p.data != nil, Ok: true}
}

func (p *ramPiece) MarkComplete() error {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	p.complete = true
	return nil
}

func (p *ramPiece) MarkNotComplete() error {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	p.complete = false
	return nil
}

func (p *ramPiece) ReadAt(b []byte, off int64) (int, error) {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	if p.data == nil {
		return 0, io.EOF
	}
	if off >= int64(len(p.data)) {
		return 0, io.EOF
	}
	n := copy(b, p.data[off:])
	p.store.clock++
	p.lastUsed = p.store.clock
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

func (p *ramPiece) WriteAt(b []byte, off int64) (int, error) {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	if p.data == nil {
		p.data = make([]byte, p.length)
		p.store.used += p.length
		p.store.pieces[p] = struct{}{}
	}
	if off < 0 || off+int64(len(b)) > p.length {
		return 0, fmt.Errorf("ram piece write out of range")
	}
	n := copy(p.data[off:], b)
	p.store.clock++
	p.lastUsed = p.store.clock
	p.store.evictLocked(p)
	return n, nil
}

// evictLocked frees least-recently-used complete pieces until under budget.
// Must be called with store.mu held. keep is never evicted.
func (s *ramStore) evictLocked(keep *ramPiece) {
	for s.used > s.maxBytes {
		var victim *ramPiece
		var oldest uint64
		for rp := range s.pieces {
			if rp == keep || rp.data == nil || !rp.complete {
				continue
			}
			if victim == nil || rp.lastUsed < oldest {
				victim, oldest = rp, rp.lastUsed
			}
		}
		if victim == nil {
			return // nothing evictable; in-flight pieces still resident
		}
		s.used -= victim.length
		victim.data = nil
		victim.complete = false
		delete(s.pieces, victim)
	}
}

// compile-time interface checks
var (
	_ storage.ClientImplCloser = (*ramStore)(nil)
	_ storage.PieceImpl        = (*ramPiece)(nil)
)
