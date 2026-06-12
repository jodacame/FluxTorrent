// Package api exposes the REST + WebSocket surface and serves the embedded UI
// (SPEC §7). One process, one port.
package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jodacame/fluxtorrent/internal/config"
	"github.com/jodacame/fluxtorrent/internal/engine"
)

// Server wires the engine, store and UI assets into an http.Handler.
type Server struct {
	eng   *engine.Engine
	store *config.Store
	ui    fs.FS
	hub   *hub
	start time.Time
}

// New builds the API server. ui is the embedded web/dist filesystem (may be nil).
func New(eng *engine.Engine, store *config.Store, ui fs.FS) *Server {
	s := &Server{eng: eng, store: store, ui: ui, hub: newHub(), start: time.Now()}
	go s.hub.run()
	go s.broadcastLoop()
	return s
}

// Handler returns the root http.Handler with routing + auth.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/torrents", s.addTorrent)
	mux.HandleFunc("GET /api/torrents", s.listTorrents)
	mux.HandleFunc("GET /api/torrents/{hash}", s.getTorrent)
	mux.HandleFunc("POST /api/torrents/{hash}/drop", s.dropTorrent)
	mux.HandleFunc("DELETE /api/torrents/{hash}", s.deleteTorrent)

	mux.HandleFunc("GET /stream/{hash}/{index}", s.stream)

	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("GET /api/rules", s.getRules)
	mux.HandleFunc("PUT /api/rules", s.putRules)

	mux.HandleFunc("GET /api/events", s.hub.serveWS)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/disk", s.diskInfo)

	// Drop-in compatibility for existing clients (point them here unchanged):
	s.registerTorrServer(mux)   // TorrServer (MatriX) — /echo, /torrents, /stream, /play
	s.registerTorrent2HTTP(mux) // torrent2http (Kodi/Quasar) — /status, /ls, /files, /get

	// Root handler: Stremio streaming-server compat (root-level paths) → SPA.
	mux.Handle("/", s.rootHandler())

	h := s.withAuth(s.withCORS(mux))
	if httpLogOn {
		h = s.withLogging(h) // FT_HTTP_LOG=1 — per-request logging for debugging clients
	}
	return h
}

// --- middleware ---

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.store.Get().APIToken
		// Stream + UI stay open so saved player URLs survive without tokens (SPEC §7).
		if token == "" || strings.HasPrefix(r.URL.Path, "/stream/") || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			writeErr(w, http.StatusUnauthorized, "invalid or missing API token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- torrent handlers ---

func (s *Server) addTorrent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Link) == "" {
		writeErr(w, http.StatusBadRequest, "body must be { \"link\": \"magnet:...\" | infohash }")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	res, err := s.eng.Add(ctx, body.Link)
	if err != nil {
		var rej *engine.ErrRejected
		if errors.As(err, &rej) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": rej.Note, "warnings": rej.Warnings,
			})
			return
		}
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.hub.broadcast(event{Type: "added", Hash: res.Hash, Name: res.Name})
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listTorrents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.eng.List())
}

func (s *Server) getTorrent(w http.ResponseWriter, r *http.Request) {
	info, ok := s.eng.Get(r.PathValue("hash"))
	if !ok {
		writeErr(w, http.StatusNotFound, "torrent not active")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) dropTorrent(w http.ResponseWriter, r *http.Request) {
	_ = s.eng.Drop(r.PathValue("hash"))
	s.hub.broadcast(event{Type: "dropped", Hash: r.PathValue("hash")})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteTorrent(w http.ResponseWriter, r *http.Request) {
	withFiles := r.URL.Query().Get("withFiles") == "true"
	_ = s.eng.Delete(r.PathValue("hash"), withFiles)
	s.hub.broadcast(event{Type: "dropped", Hash: r.PathValue("hash")})
	w.WriteHeader(http.StatusNoContent)
}

// --- stream handler (SPEC §7) ---

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "index must be an integer")
		return
	}
	s.serveStream(w, r, r.PathValue("hash"), index)
}

// serveStream opens a Range-capable reader and streams file `index` (0-based) of
// `hash`. Shared by the native `/stream/{hash}/{index}` route and the
// TorrServer-compatible routes so streaming behaves identically everywhere.
func (s *Server) serveStream(w http.ResponseWriter, r *http.Request, hash string, index int) {
	reqStart := parseRangeStart(r.Header.Get("Range"))

	client := engine.StreamClient{Addr: clientAddr(r), Agent: r.UserAgent()}

	// Fully downloaded files go straight from disk via http.ServeFile (kernel
	// sendfile) — byte-identical to a plain static file server. The torrent
	// reader below only serves data still being downloaded.
	if path, done, ok := s.eng.OpenDiskFile(r.Context(), hash, index, client); ok {
		defer done()
		if httpLogOn {
			log.Printf("stream: disk-serve %s idx=%d → %s", hash[:8], index, filepath.Base(path))
		}
		h := w.Header()
		if ct := mediaContentType(path); ct != "" {
			h.Set("Content-Type", ct)
		}
		h.Set("Connection", "close")
		h.Set("Server", "FluxTorrent")
		h.Set("transferMode.dlna.org", "Streaming")
		h.Set("ETag", `"`+hex.EncodeToString([]byte(hash+"/"+filepath.Base(path)))+`"`)
		http.ServeFile(w, r, path)
		return
	}

	sr, err := s.eng.OpenStream(r.Context(), hash, index, reqStart, client)
	if err != nil {
		switch {
		case errors.Is(err, engine.ErrNoPeers):
			writeErr(w, http.StatusGatewayTimeout, "Couldn't find anyone sharing this right now. Try again or pick a different source.")
		case errors.Is(err, engine.ErrNotFound):
			writeErr(w, http.StatusNotFound, err.Error())
		default:
			writeErr(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	defer sr.Close()

	h := w.Header()
	// Correct media MIME by extension before ServeContent (which only sniffs when
	// Content-Type is unset). Go's sniffer mislabels Matroska as `video/webm`,
	// making players reject non-WebM codecs (HEVC/x264 in MKV). TorrServer maps
	// explicitly too.
	if ct := mediaContentType(sr.Name); ct != "" {
		h.Set("Content-Type", ct)
	}
	// Match TorrServer's streaming response so the same players behave the same:
	// `Connection: close` gives each Range request a clean connection (ffmpeg/mpv
	// reuse a keep-alive mid-body connection badly when seeking); the DLNA/Server
	// hints mark it as a streaming source.
	h.Set("Connection", "close")
	h.Set("Server", "FluxTorrent")
	h.Set("transferMode.dlna.org", "Streaming")
	// Strong ETag (hash/path, hex) exactly like TorrServer — lets players that
	// correlate Range connections to one resource see the same identity.
	h.Set("ETag", `"`+hex.EncodeToString([]byte(hash+"/"+sr.Name))+`"`)

	http.ServeContent(w, r, sr.Name, sr.ModTime, sr.ReadSeeker)
}

// mediaContentType returns the MIME type for a media/subtitle file by extension,
// or "" to let net/http decide.
func mediaContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mkv":
		return "video/x-matroska"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".ts", ".m2ts", ".mts":
		return "video/mp2t"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".mpg", ".mpeg":
		return "video/mpeg"
	case ".ogv":
		return "video/ogg"
	case ".srt":
		return "application/x-subrip"
	case ".ass", ".ssa":
		return "text/x-ssa"
	case ".vtt":
		return "text/vtt"
	case ".sub":
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

// --- settings & rules ---

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Get())
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var cfg config.Settings
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid settings body")
		return
	}
	if err := s.store.Put(cfg); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eng.ApplyRateLimits(s.store.Get().Net) // hot-apply speed caps (SPEC §6.3)
	writeJSON(w, http.StatusOK, s.store.Get())
}

func (s *Server) getRules(w http.ResponseWriter, r *http.Request) {
	rl, _ := s.store.Rules()
	writeJSON(w, http.StatusOK, rl)
}

func (s *Server) putRules(w http.ResponseWriter, r *http.Request) {
	var rl []config.Rule
	if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
		writeErr(w, http.StatusBadRequest, "rules must be a JSON array")
		return
	}
	if err := s.store.PutRules(rl); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rl)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	cfg := s.store.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        Version,
		"uptime":         int(time.Since(s.start).Seconds()),
		"activeTorrents": len(s.eng.List()),
		"cacheMode":      cfg.Cache.Mode,
	})
}

// --- stats broadcast loop (SPEC §7 ~1/s) ---

func (s *Server) broadcastLoop() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		if s.hub.count() == 0 {
			continue // stay idle when nobody's watching (SPEC §9 ultra-light idle)
		}
		s.hub.broadcast(event{Type: "stats", Torrents: s.eng.List()})
	}
}

// --- helpers ---

// Version is set at build time via -ldflags, defaults below.
var Version = "0.2.1"

// clientAddr returns the player's source address, honoring X-Forwarded-For when
// FluxTorrent sits behind a reverse proxy.
func clientAddr(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}

func parseRangeStart(h string) int64 {
	if !strings.HasPrefix(h, "bytes=") {
		return 0
	}
	spec := strings.TrimPrefix(h, "bytes=")
	if i := strings.IndexByte(spec, '-'); i > 0 {
		if n, err := strconv.ParseInt(spec[:i], 10, 64); err == nil {
			return n
		}
	}
	return 0
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
