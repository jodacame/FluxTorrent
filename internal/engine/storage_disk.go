package engine

import (
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent/storage"
)

// newDiskStorage returns a file-backed storage rooted at path (SPEC §6.1 disk mode).
func newDiskStorage(path string) (storage.ClientImplCloser, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	return storage.NewFile(filepath.Clean(path)), nil
}
