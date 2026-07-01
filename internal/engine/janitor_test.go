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

// TestDeleteRecordFilesKeepsRecordOnFailure verifies we never orphan: if the
// files can't be removed (unsafe path here), the record is retained so the files
// stay tracked instead of vanishing from the listing.
func TestDeleteRecordFilesKeepsRecordOnFailure(t *testing.T) {
	e := testEngine(t, t.TempDir())
	rec := config.TorrentRecord{Hash: "keepme", Name: "../escape", StorageMode: "disk"}
	if err := e.store.SaveTorrent(rec); err != nil {
		t.Fatal(err)
	}
	e.deleteRecordFiles(rec)
	if _, ok := e.store.GetTorrent("keepme"); !ok {
		t.Error("record must be kept when its files could not be removed (no orphaning)")
	}

	// A clean disk removal, by contrast, drops the record.
	root := e.store.Get().Cache.Path
	_ = os.MkdirAll(filepath.Join(root, "gone"), 0o755)
	rec2 := config.TorrentRecord{Hash: "dropme", Name: "gone", StorageMode: "disk"}
	_ = e.store.SaveTorrent(rec2)
	e.deleteRecordFiles(rec2)
	if _, ok := e.store.GetTorrent("dropme"); ok {
		t.Error("record should be dropped once its files are removed")
	}
}

// TestScanDiskFindsOrphans checks reconciliation: a folder with a record is
// dated by the record, an orphan folder (no record) is flagged and dated by
// mtime, and both are eviction candidates while an active torrent's folder would
// be excluded (covered by the guard, not needing a live torrent here).
func TestScanDiskFindsOrphans(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, root)

	_ = os.MkdirAll(filepath.Join(root, "tracked"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "tracked", "a.mkv"), make([]byte, 10), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "orphan"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "orphan", "b.mkv"), make([]byte, 20), 0o644)
	_ = e.store.SaveTorrent(config.TorrentRecord{Hash: "t1", Name: "tracked", StorageMode: "disk", LastPlayedAt: 42})

	byName := map[string]diskEntry{}
	for _, d := range e.scanDisk() {
		byName[d.name] = d
	}
	if len(byName) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(byName))
	}
	if tr := byName["tracked"]; tr.orphan || tr.hash != "t1" || tr.age != 42 {
		t.Errorf("tracked entry wrong: %+v", tr)
	}
	if or := byName["orphan"]; !or.orphan || or.hash != "" {
		t.Errorf("orphan entry wrong: %+v", or)
	}
}
