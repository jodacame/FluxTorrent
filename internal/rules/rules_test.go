package rules

import (
	"testing"

	"github.com/jodacame/fluxtorrent/internal/config"
)

func mkRule(field, op, val, action string, seed *config.SeedTarget) config.Rule {
	var r config.Rule
	r.Match.Field = field
	r.Match.Op = op
	r.Match.Value = val
	r.Action = action
	r.Seed = seed
	r.Note = "test note"
	return r
}

func TestEvaluate(t *testing.T) {
	global := config.Seed{MaxRatio: 1.5, MaxMinutes: 120}

	t.Run("reject by tracker contains", func(t *testing.T) {
		d := Evaluate([]config.Rule{mkRule("tracker", "contains", "flaky", "reject", nil)},
			Subject{Tracker: "udp://flaky.example:6969"}, global)
		if !d.Reject || d.RejectNote != "test note" {
			t.Fatalf("expected reject with note, got %+v", d)
		}
	})

	t.Run("forceDisk", func(t *testing.T) {
		d := Evaluate([]config.Rule{mkRule("name", "contains", "remux", "forceDisk", nil)},
			Subject{Name: "Movie.2160p.REMUX"}, global)
		if d.ForceMode != "disk" {
			t.Fatalf("expected disk, got %q", d.ForceMode)
		}
	})

	t.Run("keepSeed inherits global and forces disk", func(t *testing.T) {
		d := Evaluate([]config.Rule{mkRule("tracker", "contains", "private", "keepSeed", nil)},
			Subject{Tracker: "https://private.tracker/announce"}, global)
		if !d.KeepSeed || d.ForceMode != "disk" || d.SeedRatio != 1.5 || d.SeedMinutes != 120 {
			t.Fatalf("keepSeed inheritance wrong: %+v", d)
		}
	})

	t.Run("keepSeed override", func(t *testing.T) {
		d := Evaluate([]config.Rule{mkRule("tracker", "equals", "T", "keepSeed", &config.SeedTarget{MaxRatio: 2, MaxMinutes: 0})},
			Subject{Tracker: "T"}, global)
		if d.SeedRatio != 2 || d.SeedMinutes != 0 {
			t.Fatalf("override wrong: %+v", d)
		}
	})

	t.Run("first match wins", func(t *testing.T) {
		d := Evaluate([]config.Rule{
			mkRule("name", "regex", `(?i)4k`, "forceRam", nil),
			mkRule("name", "contains", "x", "reject", nil),
		}, Subject{Name: "Movie.4K.x"}, global)
		if d.ForceMode != "ram" || d.Reject {
			t.Fatalf("expected first rule (ram) to win, got %+v", d)
		}
	})

	t.Run("no match → zero decision", func(t *testing.T) {
		d := Evaluate([]config.Rule{mkRule("name", "equals", "nope", "reject", nil)},
			Subject{Name: "something"}, global)
		if d.Reject || d.ForceMode != "" || d.KeepSeed {
			t.Fatalf("expected empty decision, got %+v", d)
		}
	})
}
