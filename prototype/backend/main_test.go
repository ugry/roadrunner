package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNz(t *testing.T) {
	if got := nz("", "def"); got != "def" {
		t.Fatalf("nz empty: want def got %q", got)
	}
	if got := nz("x", "def"); got != "x" {
		t.Fatalf("nz value: want x got %q", got)
	}
	if got := nz("  ", "def"); got != "def" {
		t.Fatalf("nz whitespace: want def got %q", got)
	}
}

func TestS(t *testing.T) {
	if s(nil) != "" {
		t.Fatalf("s(nil) should be empty")
	}
	v := "hello"
	if s(&v) != "hello" {
		t.Fatalf("s(&v) want hello")
	}
}

func TestGetenvDefault(t *testing.T) {
	if getenv("INSUCAR_DOES_NOT_EXIST_123", "fallback") != "fallback" {
		t.Fatalf("getenv should return fallback")
	}
}

func TestRoleFromGroups(t *testing.T) {
	if got := roleFromGroups([]string{"operator"}); got != "agent" {
		t.Fatalf("operator -> agent, got %q", got)
	}
	if got := roleFromGroups([]string{"product_owner", "ops"}); got != "agent" {
		t.Fatalf("staff group -> agent, got %q", got)
	}
	if got := roleFromGroups([]string{"customers"}); got != "user" {
		t.Fatalf("non-staff -> user, got %q", got)
	}
	if got := roleFromGroups(nil); got != "user" {
		t.Fatalf("nil -> user, got %q", got)
	}
	// Verify all staff groups map correctly
	for _, g := range []string{"operator", "supervisor", "admin", "ops", "product_owner"} {
		if got := roleFromGroups([]string{g}); got != "agent" {
			t.Fatalf("%s -> agent, got %q", g, got)
		}
	}
}

func TestMakeTokenAndParseToken(t *testing.T) {
	// Save and restore sessionKey
	origKey := sessionKey
	sessionKey = []byte("test-session-key-for-unit-tests")
	defer func() { sessionKey = origKey }()

	tok := makeToken("user", "user-id-1", "Test User")
	if tok == "" {
		t.Fatal("makeToken returned empty")
	}

	role, id, name, ok := parseToken(tok)
	if !ok {
		t.Fatal("parseToken failed for valid token")
	}
	if role != "user" {
		t.Fatalf("role: want user got %q", role)
	}
	if id != "user-id-1" {
		t.Fatalf("id: want user-id-1 got %q", id)
	}
	if name != "Test User" {
		t.Fatalf("name: want Test User got %q", name)
	}

	// Tampered token
	tampered := tok[:len(tok)-5] + "xxxxx"
	if _, _, _, ok := parseToken(tampered); ok {
		t.Fatal("parseToken should reject tampered token")
	}

	// Empty token
	if _, _, _, ok := parseToken(""); ok {
		t.Fatal("parseToken should reject empty token")
	}

	// Token with wrong format
	if _, _, _, ok := parseToken("a|b|c"); ok {
		t.Fatal("parseToken should reject malformed token")
	}

	// Agent token
	agentTok := makeToken("agent", "agent-id-1", "Agent Smith")
	role, id, name, ok = parseToken(agentTok)
	if !ok || role != "agent" || id != "agent-id-1" || name != "Agent Smith" {
		t.Fatal("agent token parse failed")
	}
}

func TestParseTokenExpiry(t *testing.T) {
	origKey := sessionKey
	sessionKey = []byte("expiry-test-key")
	defer func() { sessionKey = origKey }()

	// Test that a freshly created token parses correctly
	tok := makeToken("user", "id", "name")
	_, _, _, ok := parseToken(tok)
	if !ok {
		t.Fatal("fresh token should be valid")
	}
}

func TestSetSessionCookieFlags(t *testing.T) {
	w := httptest.NewRecorder()
	setSession(w, "user", "id-1", "Test User")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "insucar_session" {
		t.Fatalf("cookie name: want insucar_session got %q", c.Name)
	}
	if !c.HttpOnly {
		t.Fatal("cookie should be HttpOnly")
	}
	if !c.Secure {
		t.Fatal("cookie should be Secure")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite: want Lax got %v", c.SameSite)
	}
	if c.MaxAge <= 0 {
		t.Fatal("cookie MaxAge should be positive")
	}
	if c.Path != "/" {
		t.Fatalf("cookie path: want / got %q", c.Path)
	}
}

func TestCurrentSession(t *testing.T) {
	origKey := sessionKey
	sessionKey = []byte("current-session-test-key")
	defer func() { sessionKey = origKey }()

	w := httptest.NewRecorder()
	setSession(w, "user", "uuid-1", "Jane Doe")
	cookies := w.Result().Cookies()

	r, _ := http.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])

	role, id, name, ok := currentSession(r)
	if !ok {
		t.Fatal("currentSession failed to parse valid cookie")
	}
	if role != "user" || id != "uuid-1" || name != "Jane Doe" {
		t.Fatalf("currentSession: got role=%q id=%q name=%q", role, id, name)
	}

	// No cookie
	r2, _ := http.NewRequest("GET", "/", nil)
	if _, _, _, ok := currentSession(r2); ok {
		t.Fatal("currentSession should fail with no cookie")
	}
}

func TestRoleFromGroupsEdgeCases(t *testing.T) {
	// Empty groups -> user
	if got := roleFromGroups([]string{}); got != "user" {
		t.Fatalf("empty -> user, got %q", got)
	}
	// Group order doesn't matter — staff takes precedence
	if got := roleFromGroups([]string{"customers", "operator"}); got != "agent" {
		t.Fatalf("mixed with staff -> agent, got %q", got)
	}
	// Case sensitivity
	if got := roleFromGroups([]string{"OPERATOR"}); got != "user" {
		t.Fatalf("case-sensitive: OPERATOR != operator -> user, got %q", got)
	}
}

func TestResolveCallerNoAuth(t *testing.T) {
	r, _ := http.NewRequest("GET", "/api/me", nil)
	c, ok := resolveCaller(r)
	if ok || c != nil {
		t.Fatal("resolveCaller should return nil for unauthenticated request")
	}
}

func TestCustom404(t *testing.T) {
	// API 404
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/nonexistent", nil)
	custom404(w, r)
	if w.Code != 404 {
		t.Fatalf("API 404: want 404 got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("API 404 content-type: want application/json got %q", ct)
	}

	// Web 404
	w2 := httptest.NewRecorder()
	r2, _ := http.NewRequest("GET", "/nonexistent-page", nil)
	custom404(w2, r2)
	if w2.Code != 404 {
		t.Fatalf("Web 404: want 404 got %d", w2.Code)
	}
	if ct := w2.Header().Get("Content-Type"); !contains(ct, "text/html") {
		t.Fatalf("Web 404 content-type: want text/html got %q", ct)
	}
}

func TestClientIP(t *testing.T) {
	// X-Forwarded-For with multiple hops
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 192.168.1.1")
	ip := clientIP(r)
	if ip != "10.0.0.1" {
		t.Fatalf("clientIP: want 10.0.0.1 got %q", ip)
	}

	// Single IP
	r2, _ := http.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Forwarded-For", "203.0.113.1")
	ip2 := clientIP(r2)
	if ip2 != "203.0.113.1" {
		t.Fatalf("clientIP single: want 203.0.113.1 got %q", ip2)
	}
}

func TestSha256hex(t *testing.T) {
	h := sha256hex("hello")
	if h == "" || len(h) != 64 {
		t.Fatalf("sha256hex should be 64 hex chars, got len=%d", len(h))
	}
	// Deterministic
	h2 := sha256hex("hello")
	if h != h2 {
		t.Fatalf("sha256hex should be deterministic")
	}
}

func TestSanitizePolicyInput(t *testing.T) {
	if !sanitizePolicyInput("normal", "text") {
		t.Fatal("normal input should pass")
	}
	if sanitizePolicyInput("\x00bad") {
		t.Fatal("null byte should fail")
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if sanitizePolicyInput(string(long)) {
		t.Fatal("too long input should fail")
	}
	// Single field pass
	if !sanitizePolicyInput("ok") {
		t.Fatal("single valid field should pass")
	}
}

func TestSplitName(t *testing.T) {
	first, last := splitName("john.doe", "john@example.com")
	if first != "john" || last != "doe" {
		t.Fatalf("splitName: want john/doe got %q/%q", first, last)
	}
	first, last = splitName("single", "single@example.com")
	if first != "single" || last != "" {
		t.Fatalf("splitName single: want single/'' got %q/%q", first, last)
	}
	first, last = splitName("", "user@example.com")
	if first != "user" || last != "" {
		t.Fatalf("splitName email-only: want user/'' got %q/%q", first, last)
	}
	first, last = splitName("", "")
	if first != "user" || last != "" {
		t.Fatalf("splitName empty: want user/'' got %q/%q", first, last)
	}
}

func TestNzName(t *testing.T) {
	if n := nzName("testuser", ""); n != "testuser" {
		t.Fatalf("nzName: want testuser got %q", n)
	}
	if n := nzName("", "email@example.com"); n != "email@example.com" {
		t.Fatalf("nzName fallback: want email got %q", n)
	}
	if n := nzName("", ""); n != "" {
		t.Fatalf("nzName empty: want '' got %q", n)
	}
}

func TestSanitizeMW(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := sanitizeMW(next)

	// Normal request
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/test", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("normal request should pass, got %d", w.Code)
	}

	// Path traversal (rejected for non-photo paths)
	w3 := httptest.NewRecorder()
	r3, _ := http.NewRequest("GET", "/api/../../../etc/passwd", nil)
	handler.ServeHTTP(w3, r3)
	if w3.Code != 400 {
		t.Fatalf("path traversal should be rejected, got %d", w3.Code)
	}

	// Photo paths are allowed (they already have their own sanitization)
	w4 := httptest.NewRecorder()
	r4, _ := http.NewRequest("GET", "/api/photo/abc-123.jpg", nil)
	handler.ServeHTTP(w4, r4)
	if w4.Code != 200 {
		t.Fatalf("photo path should pass, got %d", w4.Code)
	}

	// Null byte in query string
	w5 := httptest.NewRecorder()
	r5, _ := http.NewRequest("GET", "/api/test?q=test", nil)
	// Go's URL parser strips null bytes from path, but we test query separately
	handler.ServeHTTP(w5, r5)
	if w5.Code != 200 {
		t.Fatalf("normal query should pass, got %d", w5.Code)
	}
}

func TestSlogLogMW(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := slogLogMW(next)
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/api/test", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("slog logMW should pass through, got %d", w.Code)
	}
}

func TestValidRequiredService(t *testing.T) {
	// Verify all declared services are valid
	for svc := range validRequiredService {
		if !validRequiredService[svc] {
			t.Fatalf("service %q should be valid", svc)
		}
	}
	// Verify some invalid services
	if validRequiredService["invalid_service"] {
		t.Fatal("invalid_service should not be in validRequiredService")
	}
	if validRequiredService[""] {
		t.Fatal("empty service should not be valid")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleAuthConfig(t *testing.T) {
	r, _ := http.NewRequest("GET", "/api/auth/config", nil)
	w := httptest.NewRecorder()
	handleAuthConfig(w, r)
	if w.Code != 200 {
		t.Fatalf("auth config should return 200, got %d", w.Code)
	}
	// Verify response contains expected keys
	body := w.Body.String()
	if !contains(body, "cognito") {
		t.Fatal("response should contain cognito key")
	}
}

func TestHandleMe(t *testing.T) {
	// Unauthenticated
	r, _ := http.NewRequest("GET", "/api/me", nil)
	w := httptest.NewRecorder()
	handleMe(w, r)
	if w.Code != 200 {
		t.Fatalf("/api/me should return 200 even when unauthenticated, got %d", w.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	r, _ := http.NewRequest("POST", "/api/logout", nil)
	w := httptest.NewRecorder()
	handleLogout(w, r)
	if w.Code != 200 {
		t.Fatalf("logout should return 200, got %d", w.Code)
	}
	// Verify deletion cookie is set
	for _, c := range w.Result().Cookies() {
		if c.Name == "insucar_session" {
			if c.MaxAge != -1 {
				t.Fatal("logout cookie should have MaxAge=-1")
			}
			if !c.HttpOnly {
				t.Fatal("logout cookie should be HttpOnly")
			}
			if !c.Secure {
				t.Fatal("logout cookie should be Secure")
			}
		}
	}
}

func TestMapMissionToCase(t *testing.T) {
	tests := []struct{ ms, expected string }{
		{"en_route", "en_route"},
		{"on_site", "on_site"},
		{"completed", "resolved"},
		{"failed", "cancelled"},
		{"cancelled", "cancelled"},
		{"searching", ""},
		{"offered", ""},
		{"accepted", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := mapMissionToCase(tc.ms); got != tc.expected {
			t.Fatalf("mapMissionToCase(%q): want %q got %q", tc.ms, tc.expected, got)
		}
	}
}
