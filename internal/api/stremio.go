package api

// Stremio streaming-server compatibility layer.
//
// Stremio's local streaming server (default port 11470) serves torrents from
// root-level paths: `GET /{infoHash}/{fileIdx}` streams a file (fileIdx is
// 0-based, same as FluxTorrent's native index). Because those paths live at the
// root — where the embedded UI is served — they are matched here inside the root
// handler (tryStremio) rather than via the ServeMux, so they never collide with
// static assets or SPA routes.
//
// Implemented:
//   GET  /{infoHash}/{fileIdx}        → stream a file (Range-capable)
//   GET|POST /{infoHash}/create       → ensure the torrent is added (by infohash/DHT)
//   GET  /{infoHash}/stats.json       → torrent stats JSON
//   GET  /{infoHash}/{fileIdx}/stats.json → per-file stats JSON
//   GET  /stats.json                  → global stats JSON
//
// Transcoding routes (/hlsv2, /transcode) are intentionally unsupported —
// FluxTorrent never transcodes (the player handles codecs, SPEC §2).

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// rootHandler dispatches a root request to Stremio compat, then the SPA.
func (s *Server) rootHandler() http.Handler {
	var spa http.Handler
	if s.ui != nil {
		spa = s.spaHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.tryStremio(w, r) {
			return
		}
		if spa != nil {
			spa.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

// tryStremio handles a Stremio streaming-server request, returning true if it
// matched (and was served). Non-matching paths fall through to the UI.
func (s *Server) tryStremio(w http.ResponseWriter, r *http.Request) bool {
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		return false
	}
	if path == "stats.json" {
		s.stremioGlobalStats(w)
		return true
	}
	parts := strings.Split(path, "/")
	if !isInfoHash(parts[0]) {
		return false
	}
	hash := strings.ToLower(parts[0])

	switch {
	case len(parts) == 2 && parts[1] == "create":
		s.stremioEnsure(w, r, hash)
		return true
	case len(parts) == 2 && parts[1] == "stats.json":
		s.stremioStats(w, hash)
		return true
	case len(parts) == 3 && parts[2] == "stats.json" && isInt(parts[1]):
		s.stremioStats(w, hash)
		return true
	case len(parts) == 2 && isInt(parts[1]):
		idx, _ := strconv.Atoi(parts[1])
		s.stremioStream(w, r, hash, idx)
		return true
	}
	return false
}

// stremioStream ensures the torrent exists (adding by infohash if needed) and
// streams the 0-based file index.
func (s *Server) stremioStream(w http.ResponseWriter, r *http.Request, hash string, idx int) {
	if _, ok := s.eng.Get(hash); !ok {
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		if _, err := s.eng.Add(ctx, hash); err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	s.serveStream(w, r, hash, idx)
}

// stremioEnsure adds the torrent by infohash (DHT metadata) and returns it.
func (s *Server) stremioEnsure(w http.ResponseWriter, r *http.Request, hash string) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if _, err := s.eng.Add(ctx, hash); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.stremioStats(w, hash)
}

func (s *Server) stremioStats(w http.ResponseWriter, hash string) {
	info, ok := s.eng.Get(hash)
	if !ok {
		writeErr(w, http.StatusNotFound, "torrent not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"infoHash":      info.Hash,
		"name":          info.Name,
		"downloaded":    int64(info.Stats.Progress * float64(info.SizeB)),
		"downloadSpeed": info.Stats.DownKbps * 1000 / 8,
		"uploadSpeed":   info.Stats.UpKbps * 1000 / 8,
		"peers":         info.Stats.Peers,
		"seeds":         info.Stats.Seeders,
		"progress":      info.Stats.Progress,
		"streamLen":     info.SizeB,
	})
}

func (s *Server) stremioGlobalStats(w http.ResponseWriter) {
	list := s.eng.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"all":               len(list),
		"peerSearchRunning": false,
	})
}

// --- helpers ---

func isInfoHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
