//go:build windows

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type Settings struct {
	Theme          string `json:"theme"` // system | light | dark
	RemoteEnabled  bool   `json:"remoteEnabled"`
	RemotePort     int    `json:"remotePort"`
	RemoteViewOnly bool   `json:"remoteViewOnly"`
	ShowSystem     bool   `json:"showSystem"`
}

func DataDir() string {
	d := filepath.Join(os.Getenv("APPDATA"), "Tidy")
	os.MkdirAll(d, 0o700)
	return d
}

type SettingsStore struct {
	mu sync.Mutex
	s  Settings
}

func LoadSettings() *SettingsStore {
	st := &SettingsStore{s: Settings{Theme: "system", RemotePort: 7878}}
	if b, err := os.ReadFile(filepath.Join(DataDir(), "settings.json")); err == nil {
		json.Unmarshal(b, &st.s)
	}
	if st.s.RemotePort < 1024 || st.s.RemotePort > 65535 {
		st.s.RemotePort = 7878
	}
	return st
}

func (st *SettingsStore) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

func (st *SettingsStore) Set(s Settings) error {
	if s.RemotePort < 1024 || s.RemotePort > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535")
	}
	if s.Theme != "light" && s.Theme != "dark" {
		s.Theme = "system"
	}
	st.mu.Lock()
	st.s = s
	st.mu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(DataDir(), "settings.json"), b, 0o600)
}

var auditMu sync.Mutex

// Audit appends a line to %APPDATA%\Tidy\audit.log for every destructive action.
func Audit(who, action, target string) {
	auditMu.Lock()
	defer auditMu.Unlock()
	p := filepath.Join(DataDir(), "audit.log")
	if st, err := os.Stat(p); err == nil && st.Size() > 5<<20 {
		os.Rename(p, p+".old")
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s\t%s\t%s\n", time.Now().Format(time.RFC3339), who, action, strings.ReplaceAll(target, "\n", " "))
}

// RelaunchAsAdmin starts an elevated copy of Tidy (UAC prompt). The caller should quit after.
func RelaunchAsAdmin(args string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	return windows.ShellExecute(0, utf16Ptr("runas"), utf16Ptr(exe), utf16Ptr(args), utf16Ptr(cwd), windows.SW_SHOWNORMAL)
}
