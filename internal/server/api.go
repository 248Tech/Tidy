//go:build windows

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Tidy/internal/core"
)

// API holds every service the UI talks to. The same handler is mounted inside
// the desktop window (trusted) and behind the remote server (authenticated).
type API struct {
	Procs    *core.ProcMonitor
	Sampler  *core.Sampler
	Scanner  *core.Scanner
	Icons    *core.Icons
	Settings *core.SettingsStore
	Remote   *Remote
	Headless bool
	Quit     func()
}

type caller struct {
	local  bool
	viewer bool
	ip     string
}

func (c caller) who() string {
	if c.local {
		return "this PC"
	}
	return "remote " + c.ip
}

type handlerFunc func(w http.ResponseWriter, r *http.Request, c caller)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string, extra ...map[string]any) {
	body := map[string]any{"error": msg}
	for _, e := range extra {
		for k, v := range e {
			body[k] = v
		}
	}
	writeJSON(w, code, body)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

// Routes registers all API endpoints. wrap decides who the caller is (and may reject).
func (a *API) Routes(mux *http.ServeMux, wrap func(handlerFunc, bool) http.HandlerFunc) {
	type route struct {
		pattern     string
		h           handlerFunc
		destructive bool
	}
	routes := []route{
		{"GET /api/info", a.info, false},
		{"GET /api/metrics", a.metrics, false},
		{"GET /api/procs", a.procs, false},
		{"GET /api/procs/detail", a.procDetail, false},
		{"POST /api/procs/stop", a.procStop, true},
		{"GET /api/icon", a.icon, false},
		{"GET /api/drives", func(w http.ResponseWriter, r *http.Request, c caller) { writeJSON(w, 200, core.ListDrives()) }, false},
		{"POST /api/scan/start", a.scanStart, false},
		{"POST /api/scan/cancel", func(w http.ResponseWriter, r *http.Request, c caller) {
			a.Scanner.Cancel()
			writeJSON(w, 200, map[string]bool{"ok": true})
		}, false},
		{"GET /api/scan/status", func(w http.ResponseWriter, r *http.Request, c caller) { writeJSON(w, 200, a.Scanner.Status()) }, false},
		{"GET /api/scan/tree", a.scanTree, false},
		{"GET /api/scan/biggest", a.scanBiggest, false},
		{"GET /api/scan/categories", func(w http.ResponseWriter, r *http.Request, c caller) { writeJSON(w, 200, a.Scanner.CategoryTotals()) }, false},
		{"POST /api/files/delete", a.filesDelete, true},
		{"POST /api/files/restore", a.filesRestore, true},
		{"POST /api/files/reveal", a.filesReveal, false},
		{"GET /api/clean", func(w http.ResponseWriter, r *http.Request, c caller) { writeJSON(w, 200, core.CleanSuggestions()) }, false},
		{"POST /api/clean", a.clean, true},
		{"GET /api/settings", func(w http.ResponseWriter, r *http.Request, c caller) { writeJSON(w, 200, a.Settings.Get()) }, false},
		{"POST /api/settings", a.saveSettings, true},
		{"GET /api/remote", a.remoteStatus, false},
		{"POST /api/remote/pin", a.remoteNewPin, true},
		{"POST /api/admin/restart", a.adminRestart, true},
	}
	for _, rt := range routes {
		mux.HandleFunc(rt.pattern, wrap(rt.h, rt.destructive))
	}
}

func (a *API) info(w http.ResponseWriter, r *http.Request, c caller) {
	writeJSON(w, 200, map[string]any{
		"system": core.GetSystemInfo(), "local": c.local, "viewOnly": c.viewer, "headless": a.Headless,
		"theme": a.Settings.Get().Theme,
	})
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request, c caller) {
	writeJSON(w, 200, a.Sampler.Get())
}

func (a *API) procs(w http.ResponseWriter, r *http.Request, c caller) {
	writeJSON(w, 200, a.Procs.Groups(false))
}

func (a *API) procDetail(w http.ResponseWriter, r *http.Request, c caller) {
	g, ok := a.Procs.Group(r.URL.Query().Get("key"))
	if !ok {
		fail(w, 404, core.ErrGone.Error())
		return
	}
	writeJSON(w, 200, g)
}

func (a *API) procStop(w http.ResponseWriter, r *http.Request, c caller) {
	var req struct {
		Key       string `json:"key"`
		Force     bool   `json:"force"`
		Confirmed bool   `json:"confirmed"`
	}
	if readJSON(r, &req) != nil || req.Key == "" {
		fail(w, 400, "bad request")
		return
	}
	res, g, err := a.Procs.StopGroup(req.Key, req.Force, req.Confirmed)
	switch {
	case errors.Is(err, core.ErrBlocked):
		fail(w, 403, g.Reason, map[string]any{"code": "blocked"})
		return
	case errors.Is(err, core.ErrCaution):
		fail(w, 409, g.Reason, map[string]any{"code": "caution"})
		return
	case err != nil:
		fail(w, 400, err.Error())
		return
	}
	verb := "close"
	if req.Force {
		verb = "force-quit"
	}
	core.Audit(c.who(), verb, g.Name+" ("+g.Exe+")")
	writeJSON(w, 200, map[string]any{"result": res, "name": g.Name, "freed": g.Mem})
}

func (a *API) icon(w http.ResponseWriter, r *http.Request, c caller) {
	b, err := a.Icons.Get(r.URL.Query().Get("path"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(b)
}

func (a *API) scanStart(w http.ResponseWriter, r *http.Request, c caller) {
	var req struct {
		Path string `json:"path"`
	}
	if readJSON(r, &req) != nil || req.Path == "" {
		fail(w, 400, "bad request")
		return
	}
	if err := a.Scanner.Start(req.Path); err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, a.Scanner.Status())
}

func (a *API) scanTree(w http.ResponseWriter, r *http.Request, c caller) {
	v, err := a.Scanner.Tree(r.URL.Query().Get("path"))
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, v)
}

func (a *API) scanBiggest(w http.ResponseWriter, r *http.Request, c caller) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	writeJSON(w, 200, a.Scanner.Biggest(r.URL.Query().Get("category"), limit))
}

func (a *API) filesDelete(w http.ResponseWriter, r *http.Request, c caller) {
	var req struct {
		Paths     []string `json:"paths"`
		Permanent bool     `json:"permanent"`
	}
	if readJSON(r, &req) != nil || len(req.Paths) == 0 || len(req.Paths) > 500 {
		fail(w, 400, "bad request")
		return
	}
	items := core.Delete(req.Paths, req.Permanent, a.Scanner.SizeOf)
	var freed uint64
	for _, it := range items {
		if it.OK {
			freed += it.Size
			a.Scanner.Forget(it.Path, it.Size)
			mode := "recycle"
			if req.Permanent {
				mode = "delete-permanently"
			}
			core.Audit(c.who(), mode, it.Path)
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "freed": freed})
}

func (a *API) filesRestore(w http.ResponseWriter, r *http.Request, c caller) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if readJSON(r, &req) != nil || len(req.Paths) == 0 {
		fail(w, 400, "bad request")
		return
	}
	items := core.Restore(req.Paths)
	for _, it := range items {
		if it.OK {
			core.Audit(c.who(), "restore", it.Path)
		}
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *API) filesReveal(w http.ResponseWriter, r *http.Request, c caller) {
	if !c.local {
		fail(w, 403, "Only available on the PC itself.")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if readJSON(r, &req) != nil {
		fail(w, 400, "bad request")
		return
	}
	if err := core.Reveal(req.Path); err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *API) clean(w http.ResponseWriter, r *http.Request, c caller) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if readJSON(r, &req) != nil || len(req.IDs) == 0 {
		fail(w, 400, "bad request")
		return
	}
	freed, removed, skipped := core.Clean(req.IDs)
	core.Audit(c.who(), "clean", strings.Join(req.IDs, ","))
	writeJSON(w, 200, map[string]any{"freed": freed, "removed": removed, "skipped": skipped})
}

func (a *API) saveSettings(w http.ResponseWriter, r *http.Request, c caller) {
	var s core.Settings
	if readJSON(r, &s) != nil {
		fail(w, 400, "bad request")
		return
	}
	old := a.Settings.Get()
	if !c.local { // remote users can't change remote-access settings
		s.RemoteEnabled, s.RemotePort, s.RemoteViewOnly = old.RemoteEnabled, old.RemotePort, old.RemoteViewOnly
	}
	if a.Headless {
		s.RemoteEnabled, s.RemotePort = old.RemoteEnabled, old.RemotePort
	}
	if err := a.Settings.Set(s); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if c.local && !a.Headless && (s.RemoteEnabled != old.RemoteEnabled || s.RemotePort != old.RemotePort) {
		a.Remote.Stop()
		if s.RemoteEnabled {
			if err := a.Remote.Start(s.RemotePort); err != nil {
				fail(w, 400, err.Error())
				return
			}
		}
		core.Audit(c.who(), "remote-access", map[bool]string{true: "on", false: "off"}[s.RemoteEnabled])
	}
	writeJSON(w, 200, a.Settings.Get())
}

func (a *API) remoteStatus(w http.ResponseWriter, r *http.Request, c caller) {
	st := a.Remote.Status()
	if !c.local {
		st.PIN = "" // never hand the PIN to a remote viewer
	}
	writeJSON(w, 200, st)
}

func (a *API) remoteNewPin(w http.ResponseWriter, r *http.Request, c caller) {
	if !c.local {
		fail(w, 403, "Only available on the PC itself.")
		return
	}
	a.Remote.NewPIN(true)
	writeJSON(w, 200, a.Remote.Status())
}

func (a *API) adminRestart(w http.ResponseWriter, r *http.Request, c caller) {
	if !c.local || a.Headless {
		fail(w, 403, "Only available on the PC itself.")
		return
	}
	if err := core.RelaunchAsAdmin(""); err != nil {
		fail(w, 400, "Admin restart was cancelled.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	if a.Quit != nil {
		go func() { time.Sleep(300 * time.Millisecond); a.Quit() }()
	}
}

// LocalHandler is mounted inside the desktop window: the caller is the PC's own user.
func (a *API) LocalHandler() http.Handler {
	mux := http.NewServeMux()
	a.Routes(mux, func(h handlerFunc, _ bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h(w, r, caller{local: true}) }
	})
	return mux
}
