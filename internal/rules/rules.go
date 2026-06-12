// Package rules evaluates ordered match→action rules (SPEC §6.4).
package rules

import (
	"regexp"
	"strings"

	"github.com/jodacame/fluxtorrent/internal/config"
)

// Decision is the outcome of evaluating the rule list against a torrent.
type Decision struct {
	Reject      bool
	RejectNote  string
	ForceMode   string // "", "disk", "ram"
	KeepSeed    bool
	SeedRatio   float64
	SeedMinutes int
	Prefer      bool
	MaxConns    int // per-torrent connection cap override (0 = inherit)
}

// Subject carries the fields a rule can match on.
type Subject struct {
	Name    string
	Tracker string
	Indexer string
}

// Evaluate applies the rules in order; first match wins (SPEC §6.4).
// global supplies the default seed limits inherited by keepSeed rules.
func Evaluate(list []config.Rule, s Subject, global config.Seed) Decision {
	for _, r := range list {
		if !match(r, s) {
			continue
		}
		d := Decision{}
		switch r.Action {
		case "reject":
			d.Reject = true
			d.RejectNote = r.Note
		case "forceDisk":
			d.ForceMode = "disk"
		case "forceRam":
			d.ForceMode = "ram"
		case "prefer":
			d.Prefer = true
		case "keepSeed":
			d.KeepSeed = true
			d.ForceMode = "disk" // seeding requires persisted pieces
			d.SeedRatio = global.MaxRatio
			d.SeedMinutes = global.MaxMinutes
			if r.Seed != nil {
				d.SeedRatio = r.Seed.MaxRatio
				d.SeedMinutes = r.Seed.MaxMinutes
			}
		}
		d.MaxConns = r.MaxConns // per-torrent override, applies alongside any action
		return d
	}
	return Decision{}
}

func match(r config.Rule, s Subject) bool {
	var field string
	switch r.Match.Field {
	case "tracker":
		field = s.Tracker
	case "indexer":
		field = s.Indexer
	default: // "name"
		field = s.Name
	}
	val := r.Match.Value
	switch r.Match.Op {
	case "equals":
		return strings.EqualFold(field, val)
	case "regex":
		re, err := regexp.Compile(val)
		return err == nil && re.MatchString(field)
	default: // "contains"
		return strings.Contains(strings.ToLower(field), strings.ToLower(val))
	}
}
