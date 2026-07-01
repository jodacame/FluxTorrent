package api

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func newTestAuth(password string) *authState {
	return &authState{
		password:   password,
		secret:     []byte("test-secret-0123456789abcdef0123"),
		sessionTTL: time.Hour,
		fails:      map[string]*failInfo{},
	}
}

// TestSessionTokenRoundTrip verifies a freshly signed token validates and that
// tampering or expiry rejects it.
func TestSessionTokenRoundTrip(t *testing.T) {
	a := newTestAuth("hunter2")

	valid := a.sign(time.Now().Add(time.Hour).Unix())
	if !a.validToken(valid) {
		t.Fatal("freshly signed token should validate")
	}

	expired := a.sign(time.Now().Add(-time.Minute).Unix())
	if a.validToken(expired) {
		t.Error("expired token must be rejected")
	}

	// A token signed with a different secret must not validate here.
	other := newTestAuth("hunter2")
	other.secret = []byte("a-completely-different-secret-32b")
	if a.validToken(other.sign(time.Now().Add(time.Hour).Unix())) {
		t.Error("token from a different secret must be rejected")
	}

	for _, bad := range []string{"", "no-dot", "abc.def", valid + "x"} {
		if a.validToken(bad) {
			t.Errorf("malformed token %q must be rejected", bad)
		}
	}
}

// TestThrottleLocksAfterLimit ensures repeated failures lock the client IP.
func TestThrottleLocksAfterLimit(t *testing.T) {
	a := newTestAuth("pw")
	ip := "1.2.3.4"

	for i := 0; i < maxLoginFails; i++ {
		if locked, _ := a.throttle(ip); locked {
			t.Fatalf("locked too early at attempt %d", i)
		}
		a.recordFail(ip)
	}
	locked, retry := a.throttle(ip)
	if !locked || retry <= 0 {
		t.Fatalf("expected lockout after %d fails, got locked=%v retry=%v", maxLoginFails, locked, retry)
	}

	// A successful login clears the record.
	a.clearFails(ip)
	if locked, _ := a.throttle(ip); locked {
		t.Error("clearFails should release the lockout")
	}
}

func TestIsAuthOpenPath(t *testing.T) {
	open := []string{"/api/login", "/api/logout", "/api/auth", "/api/health"}
	for _, p := range open {
		if !isAuthOpenPath(p) {
			t.Errorf("%s should stay open", p)
		}
	}
	for _, p := range []string{"/api/torrents", "/api/settings", "/api/events"} {
		if isAuthOpenPath(p) {
			t.Errorf("%s must not be open", p)
		}
	}
}

func TestIsHTTPS(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://x/api", nil)
	if isHTTPS(r) {
		t.Error("plain request should not be https")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !isHTTPS(r) {
		t.Error("X-Forwarded-Proto=https should be treated as https")
	}
}

// TestIssuedCookieValidates ties issueCookie to validSession end to end.
func TestIssuedCookieValidates(t *testing.T) {
	a := newTestAuth("pw")
	rw := &cookieRecorder{header: http.Header{}}
	req, _ := http.NewRequest(http.MethodPost, "http://x/api/login", nil)
	a.issueCookie(rw, req)

	set := rw.header.Get("Set-Cookie")
	if !strings.Contains(set, sessionCookie+"=") || !strings.Contains(set, "HttpOnly") {
		t.Fatalf("unexpected Set-Cookie: %q", set)
	}
	// Replay the cookie on a new request.
	val := set[strings.Index(set, "=")+1 : strings.IndexByte(set, ';')]
	req2, _ := http.NewRequest(http.MethodGet, "http://x/api/torrents", nil)
	req2.AddCookie(&http.Cookie{Name: sessionCookie, Value: val})
	if !a.validSession(req2) {
		t.Error("issued cookie should produce a valid session")
	}
}

type cookieRecorder struct{ header http.Header }

func (c *cookieRecorder) Header() http.Header       { return c.header }
func (c *cookieRecorder) Write([]byte) (int, error) { return 0, nil }
func (c *cookieRecorder) WriteHeader(int)           {}
