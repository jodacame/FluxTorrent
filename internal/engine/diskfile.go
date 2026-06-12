package engine

// Direct-from-disk serving for fully downloaded files.
//
// When a file inside a torrent is 100% downloaded and verified, streaming it
// through a torrent.Reader adds nothing — every byte is already on disk. The
// HTTP layer can hand the actual file to http.ServeFile and get the kernel
// sendfile path, exactly like a plain static file server (and like TorrServer
// behaves once its cache is warm). Incomplete or RAM-backed torrents keep
// using OpenStream's torrent reader.

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// OpenDiskFile returns the on-disk path for file `index` of `hash` when that
// file is fully downloaded and present in the disk storage. It registers
// `client` with the same bookkeeping as OpenStream (UI player list, reader
// count for keepSeed/idle/drop logic); call done() when the response ends.
// ok=false → caller must fall back to OpenStream.
func (e *Engine) OpenDiskFile(ctx context.Context, hash string, index int, client StreamClient) (path string, done func(), ok bool) {
	m, err := e.Ensure(ctx, hash)
	if err != nil || m.t.Info() == nil {
		return "", nil, false
	}
	files := m.t.Files()
	if index < 0 || index >= len(files) {
		return "", nil, false
	}
	f := files[index]
	if f.Length() == 0 || f.BytesCompleted() < f.Length() {
		return "", nil, false
	}

	// Locate the file as written by storage.NewFile: multi-file torrents live
	// under "<dir>/<torrent name>/", single-file torrents directly in "<dir>/".
	// The size check guards against name sanitization mismatches — on any
	// doubt we fall back to the torrent reader.
	for _, p := range []string{
		filepath.Join(e.diskPath, m.t.Info().Name, filepath.FromSlash(f.Path())),
		filepath.Join(e.diskPath, filepath.FromSlash(f.Path())),
	} {
		st, err := os.Stat(p)
		if err != nil || st.Size() != f.Length() {
			continue
		}
		m.mu.Lock()
		m.readers++
		m.played = time.Now()
		m.mu.Unlock()
		client.FileIndex = index
		client.File = f.DisplayPath()
		client.Since = time.Now().Unix()
		id := m.addClient(client)
		return p, func() {
			m.removeClient(id)
			e.releaseReader(m)
		}, true
	}
	return "", nil, false
}
