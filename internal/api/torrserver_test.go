package api

import (
	"testing"

	"github.com/jodacame/fluxtorrent/internal/engine"
)

// TestTSIndex locks the 1-based↔0-based mapping that keeps TorrServer clients
// streaming the right file without changes.
func TestTSIndex(t *testing.T) {
	cases := map[string]int{"1": 0, "2": 1, "10": 9, "0": 0, "": 0, "-3": 0, "x": 0}
	for in, want := range cases {
		if got := tsIndex(in); got != want {
			t.Errorf("tsIndex(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestTSStat(t *testing.T) {
	cases := map[string]int{"fetching": 2, "searching": 3, "downloading": 4, "playing": 4, "ready": 4, "seeding": 4}
	for state, want := range cases {
		if got, _ := tsStat(state); got != want {
			t.Errorf("tsStat(%q) = %d, want %d", state, got, want)
		}
	}
}

// TestTSObject verifies file ids are emitted 1-based for TorrServer clients.
func TestTSObject(t *testing.T) {
	info := engine.Info{
		Hash: "abc", Name: "X", SizeB: 100,
		Files: []engine.File{{Index: 0, Path: "a.mkv", SizeB: 60, Playable: true}},
		Stats: engine.Stats{State: "downloading"},
	}
	obj := tsObject(info)
	fs := obj["file_stats"].([]map[string]any)
	if fs[0]["id"].(int) != 1 {
		t.Fatalf("expected 1-based id=1, got %v", fs[0]["id"])
	}
	if obj["hash"] != "abc" || obj["stat"].(int) != 4 {
		t.Errorf("unexpected object: %+v", obj)
	}
}
