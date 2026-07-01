package api

// TorrServer compatibility layer.
//
// This module exposes a subset of the TorrServer (MatriX) HTTP API mapped onto
// the FluxTorrent engine, so existing TorrServer clients (LumoraTV, TorrServe,
// Lampa, TorrServe-ktor, etc.) can point at FluxTorrent unchanged. It is kept
// in its own file so the native `/api/*` surface stays clean and the mapping is
// easy to audit or extend for other servers.
//
// Implemented endpoints:
//   GET  /echo                         → version probe (clients use it to detect the server)
//   POST /torrents  {action,...}       → list | add | get | rem | drop | set
//   GET  /stream/{name}?link&index&play→ stream a file (link = hash or magnet)
//   GET  /stream?link&index&play       → same, nameless variant
//   GET  /play/{hash}/{index}          → stream shortcut
//   POST /settings  {action,sets}      → get | set (best-effort field mapping)
//
// Compatibility notes:
//   • TorrServer file indexes are 1-based (file_stats[].id); FluxTorrent is
//     0-based. The mapping is handled here (engineIndex = tsIndex - 1).
//   • These routes stay open (no bearer token), matching TorrServer's default
//     and keeping saved player URLs working without credentials.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jodacame/fluxtorrent/internal/config"
	"github.com/jodacame/fluxtorrent/internal/engine"
)

// registerTorrServer wires the compatibility routes onto the mux.
func (s *Server) registerTorrServer(mux *http.ServeMux) {
	mux.HandleFunc("GET /echo", s.tsEcho)
	mux.HandleFunc("POST /torrents", s.tsTorrents)
	mux.HandleFunc("GET /stream/{name}", s.tsStream)
	mux.HandleFunc("GET /stream", s.tsStream)
	mux.HandleFunc("GET /play/{hash}/{index}", s.tsPlay)
	mux.HandleFunc("POST /settings", s.tsSettings)
}

// GET /echo — version string used by clients to detect the server.
func (s *Server) tsEcho(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "FluxTorrent %s (TorrServer-compatible)", Version)
}

// tsTorrentRequest is the TorrServer /torrents control body.
type tsTorrentRequest struct {
	Action string `json:"action"`
	Link   string `json:"link"`
	Hash   string `json:"hash"`
	Title  string `json:"title"`
}

// POST /torrents — the main TorrServer control endpoint.
func (s *Server) tsTorrents(w http.ResponseWriter, r *http.Request) {
	var req tsTorrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	switch req.Action {
	case "list":
		out := make([]map[string]any, 0)
		for _, info := range s.eng.List() {
			out = append(out, tsObject(info))
		}
		writeJSON(w, http.StatusOK, out)

	case "add":
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		res, err := s.eng.Add(ctx, req.Link)
		if err != nil {
			log.Printf("torrserver add failed: link=%s err=%v", redactedLink(req.Link), err)
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		log.Printf("torrserver add: hash=%s name=%q files=%d", res.Hash, res.Name, len(res.Files))
		if info, ok := s.eng.Get(res.Hash); ok {
			writeJSON(w, http.StatusOK, tsObject(*info))
			return
		}
		writeJSON(w, http.StatusOK, tsAddResult(res))

	case "get":
		info, ok := s.eng.Get(req.Hash)
		if !ok {
			writeErr(w, http.StatusNotFound, "torrent not found")
			return
		}
		writeJSON(w, http.StatusOK, tsObject(*info))

	case "rem":
		// Destructive: never needed to play, so it requires credentials when auth
		// is on (a bare player only does add/get/drop).
		if !s.requireAuth(w, r) {
			return
		}
		// Torrents under a seeding obligation (private auto-seed or a keepSeed
		// rule) have their own ratio/time targets — a client "remove" must not
		// delete them; the seed enforcer drops them once the target is met.
		// Everything else is removed as asked.
		if s.isSeedProtected(req.Hash) {
			log.Printf("torrserver rem ignored: %s is seed-protected (keeps seeding)", req.Hash)
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = s.eng.Delete(req.Hash, false)
		w.WriteHeader(http.StatusOK)

	case "drop":
		if s.isSeedProtected(req.Hash) {
			log.Printf("torrserver drop ignored: %s is seed-protected (keeps seeding)", req.Hash)
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = s.eng.Drop(req.Hash)
		w.WriteHeader(http.StatusOK)

	case "set":
		// Server-state change: gated behind auth. Title/category edits are
		// cosmetic in FluxTorrent, so this stays a no-op once authorized.
		if !s.requireAuth(w, r) {
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		writeErr(w, http.StatusBadRequest, "unknown action: "+req.Action)
	}
}

// redactedLink makes a torrent link safe to log. Indexer download URLs carry
// apikeys in the query and magnet tracker params can embed passkeys, so only
// the identifying part is kept: scheme://host/path for URLs, the btih for
// magnets, and plain hashes as-is.
func redactedLink(link string) string {
	if strings.HasPrefix(link, "magnet:") {
		if m := btihRe.FindString(link); m != "" {
			return "magnet:?xt=" + m
		}
		return "magnet:?REDACTED"
	}
	if u, err := url.Parse(link); err == nil && u.Scheme != "" {
		u.RawQuery, u.Fragment, u.User = "", "", nil
		return u.String()
	}
	return link // bare infohash
}

var btihRe = regexp.MustCompile(`urn:btih:[0-9a-zA-Z]+`)

// isSeedProtected reports whether the hash is an active torrent under a seeding
// obligation — private auto-seed OR a `keepSeed` rule (both set KeepSeed). Such
// torrents must survive a client drop/remove; the seed enforcer drops them only
// once their configured ratio/time target is met.
func (s *Server) isSeedProtected(hash string) bool {
	info, ok := s.eng.Get(hash)
	return ok && info.KeepSeed
}

// GET /stream/{name}?link={hash|magnet}&index={1-based}&play
func (s *Server) tsStream(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	link := q.Get("link")
	if link == "" {
		writeErr(w, http.StatusBadRequest, "missing link parameter")
		return
	}
	idx := tsIndex(q.Get("index"))

	// Resolve the hash, adding the torrent if a magnet/infohash was given.
	hash := link
	if strings.HasPrefix(link, "magnet:") || len(link) != 40 {
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		res, err := s.eng.Add(ctx, link)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		hash = res.Hash
	}

	// ?stat → JSON torrent object; ?m3u → playlist; otherwise stream bytes.
	if q.Has("stat") {
		if info, ok := s.eng.Get(hash); ok {
			writeJSON(w, http.StatusOK, tsObject(*info))
			return
		}
		writeErr(w, http.StatusNotFound, "torrent not found")
		return
	}
	if q.Has("m3u") {
		s.tsPlaylist(w, r, hash)
		return
	}

	s.serveStream(w, r, hash, idx)
}

// GET /play/{hash}/{index} — streaming shortcut (1-based index).
func (s *Server) tsPlay(w http.ResponseWriter, r *http.Request) {
	idx := tsIndex(r.PathValue("index"))
	s.serveStream(w, r, r.PathValue("hash"), idx)
}

// tsPlaylist returns an M3U playlist of the playable files.
func (s *Server) tsPlaylist(w http.ResponseWriter, r *http.Request, hash string) {
	info, ok := s.eng.Get(hash)
	if !ok {
		writeErr(w, http.StatusNotFound, "torrent not found")
		return
	}
	origin := "http://" + r.Host
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, f := range info.Files {
		if !f.Playable {
			continue
		}
		b.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", f.Path))
		b.WriteString(fmt.Sprintf("%s/stream/%s?link=%s&index=%d&play\n",
			origin, urlSafe(f.Path), hash, f.Index+1))
	}
	_, _ = w.Write([]byte(b.String()))
}

// POST /settings — get/set in TorrServer field names (best-effort mapping).
func (s *Server) tsSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string         `json:"action"`
		Sets   map[string]any `json:"sets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	cfg := s.store.Get()
	if req.Action == "set" && req.Sets != nil {
		// Changing server settings requires credentials when auth is on; players
		// only ever read settings (action "get").
		if !s.requireAuth(w, r) {
			return
		}
		if v, ok := req.Sets["CacheSize"]; ok {
			if mb, ok := toInt(v); ok && mb > 0 {
				cfg.Cache.SizeMB = mb / (1 << 20) // TorrServer CacheSize is bytes
				if cfg.Cache.SizeMB < 64 {
					cfg.Cache.SizeMB = mb // tolerate clients that send MB
				}
			}
		}
		if v, ok := req.Sets["ReaderReadAHead"]; ok {
			if pct, ok := toInt(v); ok && pct > 0 {
				cfg.Cache.ReadaheadMB = pct
			}
		}
		_ = s.store.Put(cfg)
		cfg = s.store.Get()
	}
	writeJSON(w, http.StatusOK, tsSettingsObject(cfg))
}

// --- mapping helpers ---

// tsObject maps a FluxTorrent Info to a TorrServer torrent object.
func tsObject(info engine.Info) map[string]any {
	files := make([]map[string]any, 0, len(info.Files))
	for _, f := range info.Files {
		files = append(files, map[string]any{
			"id":     f.Index + 1, // TorrServer is 1-based
			"path":   f.Path,
			"length": f.SizeB,
		})
	}
	stat, statStr := tsStat(info.Stats.State)
	return map[string]any{
		"title":        info.Name,
		"name":         info.Name,
		"poster":       "",
		"hash":         info.Hash,
		"stat":         stat,
		"stat_string":  statStr,
		"torrent_size": info.SizeB,
		"file_stats":   files,
		// extra live fields some clients surface
		"download_speed":    info.Stats.DownKbps * 1000 / 8,
		"upload_speed":      info.Stats.UpKbps * 1000 / 8,
		"active_peers":      info.Stats.Peers,
		"total_peers":       info.Stats.Peers,
		"connected_seeders": info.Stats.Seeders,
		"preloaded_bytes":   int64(info.Stats.Progress * float64(info.SizeB)),
	}
}

// tsAddResult maps an AddResult (when not yet listed) to a minimal TS object.
func tsAddResult(res *engine.AddResult) map[string]any {
	files := make([]map[string]any, 0, len(res.Files))
	for _, f := range res.Files {
		files = append(files, map[string]any{"id": f.Index + 1, "path": f.Path, "length": f.SizeB})
	}
	return map[string]any{
		"title": res.Name, "name": res.Name, "hash": res.Hash,
		"stat": 4, "stat_string": "Torrent working", "file_stats": files,
	}
}

// tsStat maps an engine state to TorrServer's numeric stat + string.
// TorrServer enum: 1 added, 2 getting info, 3 preload, 4 working, 5 closed.
func tsStat(state string) (int, string) {
	switch state {
	case "fetching":
		return 2, "Torrent getting info"
	case "searching":
		return 3, "Torrent preload"
	default: // downloading/playing/ready/seeding
		return 4, "Torrent working"
	}
}

func tsSettingsObject(cfg config.Settings) map[string]any {
	return map[string]any{
		"CacheSize":       cfg.Cache.SizeMB * (1 << 20),
		"ReaderReadAHead": cfg.Cache.ReadaheadMB,
		"PreloadBuffer":   false,
		"EnableDebug":     false,
	}
}

// tsIndex parses a 1-based TorrServer index into a 0-based engine index.
func tsIndex(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0
	}
	return n - 1
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

func urlSafe(p string) string {
	return strings.ReplaceAll(strings.ReplaceAll(p, " ", "%20"), "#", "%23")
}
