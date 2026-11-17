//go:build windows

package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Tidy/internal/core"
)

// Remote serves the same UI + API over HTTPS on the LAN, protected by a pairing PIN.
type Remote struct {
	API    *API
	Assets fs.FS

	mu       sync.Mutex
	srv      *http.Server
	port     int
	running  bool
	lastErr  string
	pin      string
	sessions map[string]time.Time
	fails    map[string][]time.Time
}

type RemoteStatus struct {
	Running  bool     `json:"running"`
	Port     int      `json:"port"`
	URLs     []string `json:"urls"`
	PIN      string   `json:"pin,omitempty"`
	ViewOnly bool     `json:"viewOnly"`
	Error    string   `json:"error,omitempty"`
	Sessions int      `json:"sessions"`
}

const sessionCookie = "tidy_session"
const sessionTTL = 30 * 24 * time.Hour

func NewRemote(api *API, assets fs.FS) *Remote {
	r := &Remote{API: api, Assets: assets, sessions: map[string]time.Time{}, fails: map[string][]time.Time{}}
	api.Remote = r
	r.NewPIN(false)
	return r
}

func randomPIN() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

// NewPIN makes a fresh pairing code; signOut also forgets every paired device.
func (r *Remote) NewPIN(signOut bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pin = randomPIN()
	if signOut {
		r.sessions = map[string]time.Time{}
	}
}

func (r *Remote) SetPIN(pin string) {
	r.mu.Lock()
	r.pin = pin
	r.mu.Unlock()
}

func (r *Remote) Status() RemoteStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := RemoteStatus{Running: r.running, Port: r.port, PIN: r.pin, Error: r.lastErr,
		ViewOnly: r.API.Settings.Get().RemoteViewOnly, Sessions: len(r.sessions)}
	if r.running {
		for _, ip := range lanIPs() {
			st.URLs = append(st.URLs, fmt.Sprintf("https://%s:%d", ip, r.port))
		}
	}
	return st
}

func lanIPs() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLinkLocalUnicast() {
				out = append(out, ipn.IP.String())
			}
		}
	}
	return out
}

func (r *Remote) Start(port int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}
	cert, err := loadOrCreateCert()
	if err != nil {
		r.lastErr = err.Error()
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		r.lastErr = fmt.Sprintf("Port %d is busy — pick another one in Settings.", port)
		return errors.New(r.lastErr)
	}
	r.srv = &http.Server{
		Handler:           r.handler(),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          nil,
	}
	r.port, r.running, r.lastErr = port, true, ""
	srv := r.srv
	go func() {
		err := srv.ServeTLS(ln, "", "")
		r.mu.Lock()
		if r.srv == srv {
			r.running = false
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				r.lastErr = err.Error()
			}
		}
		r.mu.Unlock()
	}()
	return nil
}

func (r *Remote) Stop() {
	r.mu.Lock()
	srv := r.srv
	r.srv, r.running = nil, false
	r.mu.Unlock()
	if srv != nil {
		srv.Close()
	}
}

func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func (r *Remote) authed(req *http.Request) bool {
	c, err := req.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	exp, ok := r.sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(r.sessions, c.Value)
		return false
	}
	return true
}

// tooManyFails rate-limits PIN guesses: 5 per IP per 5 minutes, 30 overall.
func (r *Remote) tooManyFails(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cut := time.Now().Add(-5 * time.Minute)
	total := 0
	for k, ts := range r.fails {
		keep := ts[:0]
		for _, t := range ts {
			if t.After(cut) {
				keep = append(keep, t)
			}
		}
		r.fails[k] = keep
		total += len(keep)
	}
	return len(r.fails[ip]) >= 5 || total >= 30
}

func (r *Remote) login(w http.ResponseWriter, req *http.Request) {
	ip := clientIP(req)
	if r.tooManyFails(ip) {
		fail(w, 429, "Too many wrong codes. Wait a few minutes and try again.")
		return
	}
	var body struct {
		PIN string `json:"pin"`
	}
	if readJSON(req, &body) != nil {
		fail(w, 400, "bad request")
		return
	}
	r.mu.Lock()
	ok := len(r.pin) == 6 && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.PIN)), []byte(r.pin)) == 1
	if !ok {
		r.fails[ip] = append(r.fails[ip], time.Now())
		r.mu.Unlock()
		time.Sleep(400 * time.Millisecond)
		fail(w, 401, "That code isn't right. Check the code shown in Tidy on the PC.")
		return
	}
	tok := make([]byte, 32)
	rand.Read(tok)
	t := hex.EncodeToString(tok)
	r.sessions[t] = time.Now().Add(sessionTTL)
	r.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: t, Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
	core.Audit("remote "+ip, "paired", "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (r *Remote) logout(w http.ResponseWriter, req *http.Request) {
	if c, err := req.Cookie(sessionCookie); err == nil {
		r.mu.Lock()
		delete(r.sessions, c.Value)
		r.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (r *Remote) handler() http.Handler {
	mux := http.NewServeMux()
	csrf := func(req *http.Request) bool {
		// Custom header can't be sent cross-site without a CORS preflight, which we never allow.
		return req.Method == http.MethodGet || req.Header.Get("X-Tidy") == "1"
	}
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, req *http.Request) {
		if !csrf(req) {
			fail(w, 403, "forbidden")
			return
		}
		r.login(w, req)
	})
	mux.HandleFunc("POST /api/logout", r.logout)
	r.API.Routes(mux, func(h handlerFunc, destructive bool) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			if !r.authed(req) {
				fail(w, 401, "Enter the pairing code to continue.", map[string]any{"code": "login"})
				return
			}
			if !csrf(req) {
				fail(w, 403, "forbidden")
				return
			}
			c := caller{ip: clientIP(req), viewer: r.API.Settings.Get().RemoteViewOnly}
			if destructive && c.viewer {
				fail(w, 403, "Remote access is view-only. Change this in Tidy's Settings on the PC.", map[string]any{"code": "viewonly"})
				return
			}
			h(w, req, c)
		}
	})
	files := http.FileServer(http.FS(r.Assets))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, req)
	}))
	return mux
}

func loadOrCreateCert() (tls.Certificate, error) {
	dir := core.DataDir()
	cp, kp := filepath.Join(dir, "remote-cert.pem"), filepath.Join(dir, "remote-key.pem")
	if c, err := tls.LoadX509KeyPair(cp, kp); err == nil {
		if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil && time.Now().Before(leaf.NotAfter.Add(-24*time.Hour)) {
			return c, nil
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	host, _ := os.Hostname()
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Tidy on " + host, Organization: []string{"Tidy"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(5, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host, "localhost"},
	}
	for _, ip := range lanIPs() {
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP(ip))
	}
	tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP("127.0.0.1"))
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	kb, _ := x509.MarshalECPrivateKey(key)
	os.WriteFile(cp, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(kp, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600)
	return tls.LoadX509KeyPair(cp, kp)
}
