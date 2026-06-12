package engine

import "testing"

// TestRAMEviction verifies the RAM backend stays within its byte budget by
// evicting least-recently-used complete pieces (SPEC §6.1, §9 bounded memory).
func TestRAMEviction(t *testing.T) {
	const pieceLen = 1 << 20 // 1 MiB
	const budget = 3 * pieceLen
	s := newRAMStore(budget)

	pieces := make([]*ramPiece, 0, 8)
	for i := 0; i < 8; i++ {
		p := &ramPiece{store: s, length: pieceLen}
		buf := make([]byte, pieceLen)
		buf[0] = byte(i + 1)
		if _, err := p.WriteAt(buf, 0); err != nil {
			t.Fatalf("write piece %d: %v", i, err)
		}
		if err := p.MarkComplete(); err != nil {
			t.Fatalf("mark %d: %v", i, err)
		}
		// touch to force eviction accounting against the budget
		_, _ = p.ReadAt(make([]byte, 16), 0)
		pieces = append(pieces, p)
	}

	if got := s.usedBytes(); got > budget {
		t.Fatalf("used=%d exceeds budget=%d", got, budget)
	}

	// The most recently written/read pieces should still be resident…
	if pieces[7].Completion().Complete != true {
		t.Errorf("newest piece should remain complete")
	}
	// …and at least one old piece must have been evicted (marked not complete).
	evicted := 0
	for _, p := range pieces {
		if p.data == nil {
			evicted++
		}
	}
	if evicted == 0 {
		t.Errorf("expected eviction to free old pieces, none evicted")
	}
}

func TestRAMReadWrite(t *testing.T) {
	s := newRAMStore(10 << 20)
	p := &ramPiece{store: s, length: 64}
	data := []byte("fluxtorrent streaming bridge")
	if _, err := p.WriteAt(data, 4); err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(data))
	n, _ := p.ReadAt(out, 4)
	if n != len(data) || string(out) != string(data) {
		t.Fatalf("readback mismatch: %q", out[:n])
	}
}
