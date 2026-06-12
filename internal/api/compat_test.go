package api

import "testing"

func TestIsInfoHash(t *testing.T) {
	valid := "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c" // 40 hex
	if !isInfoHash(valid) || !isInfoHash("DD8255ECDC7CA55FB0BBF81323D87062DB1F6D1C") {
		t.Error("expected valid infohashes to pass")
	}
	for _, bad := range []string{"", "abc", valid + "0", "zz8255ecdc7ca55fb0bbf81323d87062db1f6d1c", "assets"} {
		if isInfoHash(bad) {
			t.Errorf("isInfoHash(%q) should be false", bad)
		}
	}
}

func TestIsInt(t *testing.T) {
	for _, ok := range []string{"0", "1", "42"} {
		if !isInt(ok) {
			t.Errorf("isInt(%q) should be true", ok)
		}
	}
	for _, bad := range []string{"", "1a", "create", "stats.json", "-1"} {
		if isInt(bad) {
			t.Errorf("isInt(%q) should be false", bad)
		}
	}
}

func TestT2HState(t *testing.T) {
	cases := map[string]int{"fetching": 2, "searching": 3, "downloading": 3, "ready": 4, "seeding": 5}
	for state, want := range cases {
		if got, _ := t2hState(state); got != want {
			t.Errorf("t2hState(%q) = %d, want %d", state, got, want)
		}
	}
}
