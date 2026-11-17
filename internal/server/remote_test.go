//go:build windows

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"Tidy/internal/core"
)

func newTestRemote(t *testing.T) (*Remote, *httptest.Server) {
	t.Setenv("APPDATA", t.TempDir()) // keep the real settings/audit log untouched
	procs := core.NewProcMonitor()
	api := &API{Procs: procs, Sampler: core.NewSampler(procs), Scanner: core.NewScanner(), Icons: core.NewIcons(), Settings: core.LoadSettings()}
	r := NewRemote(api, fstest.MapFS{"index.html": {Data: []byte("<html>")}})
	r.SetPIN("123456")
	ts := httptest.NewTLSServer(r.handler())
	t.Cleanup(ts.Close)
	return r, ts
}

func do(t *testing.T, c *http.Client, method, url, body string, csrf bool) *http.Response {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if csrf {
		req.Header.Set("X-Tidy", "1")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res
}

func TestRemoteAuthFlow(t *testing.T) {
	r, ts := newTestRemote(t)
	c := ts.Client()

	if res := do(t, c, "GET", ts.URL+"/api/metrics", "", false); res.StatusCode != 401 {
		t.Fatalf("unauthenticated metrics = %d", res.StatusCode)
	}
	if res := do(t, c, "GET", ts.URL+"/", "", false); res.StatusCode != 200 {
		t.Fatalf("static page = %d", res.StatusCode)
	}
	if res := do(t, c, "POST", ts.URL+"/api/login", `{"pin":"123456"}`, false); res.StatusCode != 403 {
		t.Fatalf("login without csrf header = %d", res.StatusCode)
	}
	if res := do(t, c, "POST", ts.URL+"/api/login", `{"pin":"000000"}`, true); res.StatusCode != 401 {
		t.Fatalf("wrong pin = %d", res.StatusCode)
	}
	res := do(t, c, "POST", ts.URL+"/api/login", `{"pin":"123456"}`, true)
	if res.StatusCode != 200 || len(res.Cookies()) == 0 {
		t.Fatalf("login = %d", res.StatusCode)
	}
	cookie := res.Cookies()[0]
	authed := func(method, path, body string, csrf bool) int {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		req.AddCookie(cookie)
		if csrf {
			req.Header.Set("X-Tidy", "1")
		}
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := authed("GET", "/api/metrics", "", false); code != 200 {
		t.Fatalf("authed metrics = %d", code)
	}
	if code := authed("POST", "/api/procs/stop", `{"key":"pid:4","force":true}`, false); code != 403 {
		t.Fatalf("post without csrf = %d", code)
	}
	if code := authed("POST", "/api/procs/stop", `{"key":"pid:4","force":true,"confirmed":true}`, true); code != 403 {
		t.Fatalf("killing System should be blocked, got %d", code)
	}
	if code := authed("POST", "/api/files/reveal", `{"path":"C:\\"}`, true); code != 403 {
		t.Fatalf("reveal remotely should be refused, got %d", code)
	}

	// view-only: destructive actions refused
	s := r.API.Settings.Get()
	orig := s
	s.RemoteViewOnly = true
	r.API.Settings.Set(s)
	defer r.API.Settings.Set(orig)
	if code := authed("POST", "/api/files/delete", `{"paths":["C:\\nope.txt"]}`, true); code != 403 {
		t.Fatalf("view-only delete = %d", code)
	}
	if code := authed("GET", "/api/procs", "", false); code != 200 {
		t.Fatalf("view-only can still view, got %d", code)
	}
}

func TestPinRateLimit(t *testing.T) {
	_, ts := newTestRemote(t)
	c := ts.Client()
	for i := 0; i < 5; i++ {
		do(t, c, "POST", ts.URL+"/api/login", `{"pin":"999999"}`, true)
	}
	if res := do(t, c, "POST", ts.URL+"/api/login", `{"pin":"123456"}`, true); res.StatusCode != 429 {
		t.Fatalf("expected lockout, got %d", res.StatusCode)
	}
}
