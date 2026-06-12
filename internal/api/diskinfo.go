package api

import (
	"net/http"
	"syscall"
)

// diskInfo reports total/used/free bytes for the configured disk-cache path, so
// the UI can show available space before saving torrents to disk (SPEC §6.1).
func (s *Server) diskInfo(w http.ResponseWriter, _ *http.Request) {
	path := s.store.Get().Cache.Path

	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"path": path, "available": false})
		return
	}
	bsize := int64(st.Bsize)
	total := int64(st.Blocks) * bsize
	free := int64(st.Bavail) * bsize      // available to unprivileged users
	used := total - int64(st.Bfree)*bsize // reserved-aware used

	writeJSON(w, http.StatusOK, map[string]any{
		"path":       path,
		"available":  true,
		"totalBytes": total,
		"freeBytes":  free,
		"usedBytes":  used,
	})
}
