package api

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// HTTP request logging is opt-in via the FT_HTTP_LOG env var (1/true). It logs
// method, path, query, Range and User-Agent on the way in, and status, bytes,
// duration and the first write error (who dropped the connection) on the way
// out; long transfers also get a 10s progress line with the send rate. This is
// the tool for debugging player/client integrations — off by default because
// real players open many connections per playback.
var httpLogOn = func() bool {
	v := os.Getenv("FT_HTTP_LOG")
	return v == "1" || v == "true" || v == "yes"
}()

// secretParams hides credential-bearing query values (indexer apikeys, tracker
// passkeys) so request logs are safe to share.
var secretParams = regexp.MustCompile(`(?i)(apikey|api_key|passkey|token)=[^&]+`)

func redactQuery(q string) string {
	return secretParams.ReplaceAllString(q, "$1=REDACTED")
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := ""
		if r.URL.RawQuery != "" {
			q = "?" + redactQuery(r.URL.RawQuery)
		}
		rng := r.Header.Get("Range")
		if rng != "" {
			rng = " Range=" + rng
		}
		ua := r.UserAgent()
		if len(ua) > 40 {
			ua = ua[:40]
		}
		addr := clientAddr(r)
		log.Printf("→ %s %s%s%s  UA=%q ip=%s", r.Method, r.URL.Path, q, rng, ua, addr)

		start := time.Now()
		lw := &logRW{ResponseWriter: w}

		// Periodic progress for long transfers (streams): rate seen by the client.
		stop := make(chan struct{})
		go func() {
			t := time.NewTicker(10 * time.Second)
			defer t.Stop()
			var last int64
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					cur := atomic.LoadInt64(&lw.bytes)
					rate := float64(cur-last) / 10 / 1024 / 1024
					last = cur
					log.Printf("… %s %s  ip=%s sent=%dMB rate=%.2fMB/s", r.Method, r.URL.Path, addr, cur>>20, rate)
				}
			}
		}()

		next.ServeHTTP(lw, r)
		close(stop)

		werr := ""
		if e := lw.writeErr(); e != nil {
			werr = fmt.Sprintf(" werr=%q", e.Error())
		}
		log.Printf("← %s %s%s  %d (%dB) %s ip=%s%s", r.Method, r.URL.Path, q, lw.status, atomic.LoadInt64(&lw.bytes), time.Since(start).Round(time.Millisecond), addr, werr)
	})
}

type logRW struct {
	http.ResponseWriter
	status int
	bytes  int64

	mu   sync.Mutex
	werr error // first write error (e.g. client reset), nil if clean
}

func (w *logRW) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *logRW) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	atomic.AddInt64(&w.bytes, int64(n))
	if err != nil {
		w.mu.Lock()
		if w.werr == nil {
			w.werr = err
		}
		w.mu.Unlock()
	}
	return n, err
}

func (w *logRW) writeErr() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.werr
}

// Hijack/Flush forwarded so WebSocket upgrades and streaming still work.
func (w *logRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter is not a Hijacker")
}

func (w *logRW) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
