package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jodacame/fluxtorrent/internal/config"
)

func testEngine(t *testing.T, cachePath string) *Engine {
	t.Helper()
	st, err := config.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := st.Get()
	cfg.Cache.Path = cachePath
	if err := st.Put(cfg); err != nil {
		t.Fatal(err)
	}
	return &Engine{store: st, managed: map[string]*managed{}}
}

// TestSafeDataPath locks the guards that make automatic deletion safe: an empty
// name (which would target the whole cache dir) and path traversal via a crafted
// torrent name must both be rejected.
func TestSafeDataPath(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, root)

	if p, ok := e.safeDataPath("My.Movie.2024"); !ok || p != filepath.Join(root, "My.Movie.2024") {
		t.Fatalf("valid name rejected: %q ok=%v", p, ok)
	}
	if p, ok := e.safeDataPath("show/S01E01"); !ok || p != filepath.Join(root, "show", "S01E01") {
		t.Fatalf("valid nested name rejected: %q ok=%v", p, ok)
	}
	for _, bad := range []string{"", "   ", "..", "../etc", "../../x", "foo/../../bar"} {
		if _, ok := e.safeDataPath(bad); ok {
			t.Errorf("unsafe name %q must be rejected", bad)
		}
	}
}

func TestDirSize(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "a"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "sub", "b"), make([]byte, 50), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := dirSize(d); got != 150 {
		t.Errorf("dirSize = %d, want 150", got)
	}
	if got := dirSize(filepath.Join(d, "missing")); got != 0 {
		t.Errorf("missing dir should be 0, got %d", got)
	}
}

func TestEvictKey(t *testing.T) {
	if got := evictKey(config.TorrentRecord{LastPlayedAt: 5, AddedAt: 1}); got != 5 {
		t.Errorf("should prefer LastPlayedAt, got %d", got)
	}
	if got := evictKey(config.TorrentRecord{AddedAt: 7}); got != 7 {
		t.Errorf("should fall back to AddedAt, got %d", got)
	}
}

// TestDeleteRecordFilesGuards ensures a non-disk record frees nothing and an
// unsafe name never calls RemoveAll, while both still drop the stale record.
func TestDeleteRecordFilesGuards(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, root)

	// A real disk file that should be removed and its size reported.
	name := "clip"
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name, "v.mkv"), make([]byte, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	freed := e.deleteRecordFiles(config.TorrentRecord{Hash: "h1", Name: name, StorageMode: "disk"})
	if freed != 200 {
		t.Errorf("freed = %d, want 200", freed)
	}
	if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
		t.Error("files should have been removed")
	}

	// RAM-mode record: nothing on disk to free.
	if freed := e.deleteRecordFiles(config.TorrentRecord{Hash: "h2", Name: "x", StorageMode: "ram"}); freed != 0 {
		t.Errorf("ram record should free 0, got %d", freed)
	}
	// Unsafe name must not escape the cache dir.
	if freed := e.deleteRecordFiles(config.TorrentRecord{Hash: "h3", Name: "../../etc", StorageMode: "disk"}); freed != 0 {
		t.Errorf("unsafe name should free 0, got %d", freed)
	}
}
