package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// authState holds the optional single-password UI authentication (SPEC §7).
//
// Authentication is OFF by default and turns ON only when FT_AUTH_PASSWORD is
// set, so existing open deployments keep working unchanged. When on, the UI and
// /api/* require either a valid session cookie (browser login) or the API
// bearer token (machine clients). /stream and the player-compat endpoints stay
// open regardless, because external players cannot log in interactively.
type authState struct {
	password   string        // empty → UI login disabled
	secret     []byte        // random per-boot HMAC key for session cookies
	sessionTTL time.Duration // how long a login lasts

	mu    sync.Mutex
	fails map[string]*failInfo // per-IP login throttling
}

type failInfo struct {
	count       int
	lockedUntil time.Time
	seen        time.Time
}

const (
	sessionCookie = "ft_session"
	maxLoginFails = 5                // failed attempts before a lockout kicks in
	loginWindow   = 15 * time.Minute // failures older than this are forgotten
	loginLockout  = 5 * time.Minute  // lockout duration once the limit is hit
)

// newAuthState reads FT_AUTH_PASSWORD / FT_AUTH_SESSION_HOURS and generates a
// random session-signing secret for this process.
func newAuthState() *authState {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		// crypto/rand failing is fatal-grade; refuse to issue forgeable cookies.
		panic("fluxtorrent: cannot read random secret: " + err.Error())
	}
	ttl := 7 * 24 * time.Hour
	if v := os.Getenv("FT_AUTH_SESSION_HOURS"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			ttl = time.Duration(h) * time.Hour
		}
	}
	return &authState{
		password:   os.Getenv("FT_AUTH_PASSWORD"),
		secret:     secret,
		sessionTTL: ttl,
		fails:      map[string]*failInfo{},
	}
}

// loginEnabled reports whether a UI password is configured.
func (a *authState) loginEnabled() bool { return a.password != "" }

// sign returns the cookie value for a session expiring at exp (unix seconds):
// "<exp>.<hmac-hex>". The HMAC over the expiry binds it to this process secret.
func (a *authState) sign(exp int64) string {
	payload := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

// validToken verifies a cookie value: correct signature and not expired.
func (a *authState) validToken(v string) bool {
	dot := strings.IndexByte(v, '.')
	if dot <= 0 {
		return false
	}
	payload, sig := v[:dot], v[dot+1:]
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	want := mac.Sum(nil)
	got, err := hex.DecodeString(sig)
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		return false
	}
	return time.Now().Unix() < exp
}

// validSession reports whether the request carries a valid session cookie.
func (a *authState) validSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return a.validToken(c.Value)
}

// issueCookie writes a fresh session cookie on the response.
func (a *authState) issueCookie(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(a.sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    a.sign(exp.Unix()),
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
		MaxAge:   int(a.sessionTTL.Seconds()),
	})
}

// clearCookie expires the session cookie.
func (a *authState) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// throttle returns (locked, retryAfter) for a login attempt from ip, pruning
// stale entries as it goes.
func (a *authState) throttle(ip string) (bool, time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	f := a.fails[ip]
	if f == nil {
		return false, 0
	}
	if now.After(f.lockedUntil) && now.Sub(f.seen) > loginWindow {
		delete(a.fails, ip)
		return false, 0
	}
	if now.Before(f.lockedUntil) {
		return true, time.Until(f.lockedUntil)
	}
	return false, 0
}

// recordFail registers a failed attempt and locks the IP once the limit is hit.
func (a *authState) recordFail(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	f := a.fails[ip]
	if f == nil || now.Sub(f.seen) > loginWindow {
		f = &failInfo{}
		a.fails[ip] = f
	}
	f.count++
	f.seen = now
	if f.count >= maxLoginFails {
		f.lockedUntil = now.Add(loginLockout)
		f.count = 0 // reset the counter; the lockout is the penalty now
	}
}

// clearFails drops the throttle record for ip after a successful login.
func (a *authState) clearFails(ip string) {
	a.mu.Lock()
	delete(a.fails, ip)
	a.mu.Unlock()
}

// --- middleware + handlers (wired in server.go) ---

// authConfigured reports whether any authentication is active (a UI password
// and/or an API bearer token).
func (s *Server) authConfigured() bool {
	return s.auth.loginEnabled() || s.store.Get().APIToken != ""
}

// requestAuthorized reports whether r carries valid credentials: a session
// cookie (browser) or the API bearer token (machine clients). Returns true when
// no authentication is configured.
func (s *Server) requestAuthorized(r *http.Request) bool {
	if !s.authConfigured() {
		return true
	}
	if s.auth.validSession(r) {
		return true
	}
	token := s.store.Get().APIToken
	return token != "" && r.Header.Get("Authorization") == "Bearer "+token
}

// requireAuth writes a 401 and returns false when the request lacks credentials.
// Used to gate mutating player-compat actions (rem/set) that no player needs but
// that would otherwise let anyone reaching the port modify server state.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.requestAuthorized(r) {
		return true
	}
	writeErr(w, http.StatusUnauthorized, "authentication required")
	return false
}

// withAuth gates the UI and /api/* when authentication is configured. It accepts
// either a session cookie (browser) or the API bearer token (machine clients).
// /stream and the player-compat endpoints (non-/api paths) always pass through;
// the few mutating compat actions are gated inside their handlers instead.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Auth is active only when a password and/or an API token is configured.
		if !s.authConfigured() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		// Only /api/* is gated. The SPA shell, /stream and player-compat routes
		// stay open (the login screen lives in the open SPA bundle; players and
		// saved stream URLs keep working without a session — SPEC §7).
		if !strings.HasPrefix(path, "/api/") || isAuthOpenPath(path) {
			next.ServeHTTP(w, r)
			return
		}
		if s.requestAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeErr(w, http.StatusUnauthorized, "authentication required")
	})
}

// isAuthOpenPath lists the /api/* endpoints that must stay reachable without a
// session so the login flow and health checks work.
func isAuthOpenPath(path string) bool {
	switch path {
	case "/api/login", "/api/logout", "/api/auth", "/api/health":
		return true
	}
	return false
}

// handleAuthStatus tells the UI whether to show a login screen and whether the
// current request is already authenticated.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"required":      s.auth.loginEnabled(),
		"authenticated": !s.auth.loginEnabled() || s.auth.validSession(r),
	})
}

// handleLogin verifies the password and, on success, issues a session cookie.
// Failed attempts are throttled per client IP to resist brute force.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.auth.loginEnabled() {
		writeErr(w, http.StatusBadRequest, "password authentication is not configured")
		return
	}
	ip := loginIP(r)
	if locked, retry := s.auth.throttle(ip); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeErr(w, http.StatusTooManyRequests, "too many attempts, try again later")
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "body must be { \"password\": \"…\" }")
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Password), []byte(s.auth.password)) != 1 {
		s.auth.recordFail(ip)
		writeErr(w, http.StatusUnauthorized, "invalid password")
		return
	}
	s.auth.clearFails(ip)
	s.auth.issueCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.clearCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// isHTTPS reports whether the request reached us over TLS, honoring a TLS-
// terminating reverse proxy's X-Forwarded-Proto.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// loginIP is the throttling key: the real client IP, honoring X-Forwarded-For.
func loginIP(r *http.Request) string {
	host := clientAddr(r)
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
