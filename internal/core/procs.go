//go:build windows

package core

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Process struct {
	PID     uint32  `json:"pid"`
	PPID    uint32  `json:"ppid"`
	Name    string  `json:"name"`
	Exe     string  `json:"exe"`
	CPU     float64 `json:"cpu"`
	Mem     uint64  `json:"mem"`
	Threads uint32  `json:"threads"`
	Started int64   `json:"started"`
	Session uint32  `json:"session"`
}

type AppGroup struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	Exe       string    `json:"exe"`
	Publisher string    `json:"publisher"`
	Kind      string    `json:"kind"` // app | background | system
	Count     int       `json:"count"`
	CPU       float64   `json:"cpu"`
	Mem       uint64    `json:"mem"`
	Protect   string    `json:"protect"` // "" | caution | blocked
	Reason    string    `json:"reason"`
	PIDs      []uint32  `json:"pids"`
	Procs     []Process `json:"procs,omitempty"`
}

type rawProc struct {
	pid, ppid, threads, session uint32
	name                        string
	ctime, cpuTime              int64
	privWS                      uint64
}

func querySystemProcesses() []rawProc {
	size := uint32(512 * 1024)
	var buf []byte
	for {
		buf = make([]byte, size)
		var ret uint32
		err := windows.NtQuerySystemInformation(windows.SystemProcessInformation, unsafe.Pointer(&buf[0]), size, &ret)
		if err == nil {
			break
		}
		if ret > size {
			size = ret + 64*1024
		} else {
			size *= 2
		}
		if size > 64*1024*1024 {
			return nil
		}
	}
	var out []rawProc
	off := uintptr(0)
	for {
		p := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(&buf[off]))
		name := ""
		if p.ImageName.Buffer != nil && p.ImageName.Length > 0 {
			name = windows.UTF16ToString(unsafe.Slice(p.ImageName.Buffer, p.ImageName.Length/2))
		}
		pid := uint32(p.UniqueProcessID)
		if pid == 0 {
			name = "System Idle Process"
		}
		out = append(out, rawProc{
			pid: pid, ppid: uint32(p.InheritedFromUniqueProcessID), threads: p.NumberOfThreads,
			session: p.SessionID, name: name, ctime: p.CreateTime,
			cpuTime: p.UserTime + p.KernelTime, privWS: uint64(p.WorkingSetPrivateSize),
		})
		if p.NextEntryOffset == 0 {
			break
		}
		off += uintptr(p.NextEntryOffset)
	}
	return out
}

type exeMeta struct {
	desc, publisher string
}

// ProcMonitor turns raw process snapshots into friendly, grouped data.
// It is refreshed lazily (at most about once per second) when callers ask.
type ProcMonitor struct {
	mu      sync.Mutex
	prev    map[uint32]rawProc
	prevAt  time.Time
	lastAt  time.Time
	last    []Process
	paths   map[[2]int64]string
	meta    map[string]exeMeta
	selfPID uint32
	winPIDs map[uint32]bool
	winAt   time.Time
}

func NewProcMonitor() *ProcMonitor {
	return &ProcMonitor{paths: map[[2]int64]string{}, meta: map[string]exeMeta{}, selfPID: uint32(os.Getpid())}
}

func (m *ProcMonitor) exePath(pid uint32, ctime int64) string {
	key := [2]int64{int64(pid), ctime}
	if p, ok := m.paths[key]; ok {
		return p
	}
	p := ""
	if h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid); err == nil {
		buf := make([]uint16, windows.MAX_LONG_PATH)
		n := uint32(len(buf))
		if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
			p = windows.UTF16ToString(buf[:n])
		}
		windows.CloseHandle(h)
	}
	m.paths[key] = p
	return p
}

func (m *ProcMonitor) exeMeta(path string) exeMeta {
	k := strings.ToLower(path)
	if v, ok := m.meta[k]; ok {
		return v
	}
	v := exeMeta{desc: versionString(path, "FileDescription"), publisher: versionString(path, "CompanyName")}
	if v.desc == "" {
		if pn := versionString(path, "ProductName"); !strings.Contains(pn, "Operating System") {
			v.desc = pn
		}
	}
	m.meta[k] = v
	return v
}

func versionString(path, field string) string {
	if path == "" {
		return ""
	}
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return ""
	}
	data := make([]byte, size)
	if windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&data[0])) != nil {
		return ""
	}
	var trans *[2]uint16
	var tl uint32
	langs := []string{"040904b0", "040904e4", "000004b0"}
	if windows.VerQueryValue(unsafe.Pointer(&data[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&trans), &tl) == nil && tl >= 4 {
		langs = append([]string{hex4(trans[0]) + hex4(trans[1])}, langs...)
	}
	for _, l := range langs {
		var p *uint16
		var n uint32
		if windows.VerQueryValue(unsafe.Pointer(&data[0]), `\StringFileInfo\`+l+`\`+field, unsafe.Pointer(&p), &n) == nil && n > 0 {
			s := strings.TrimSpace(windows.UTF16PtrToString(p))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func hex4(v uint16) string {
	const h = "0123456789abcdef"
	return string([]byte{h[v>>12&0xf], h[v>>8&0xf], h[v>>4&0xf], h[v&0xf]})
}

var (
	enumOnce sync.Once
	enumCB   uintptr
	enumMu   sync.Mutex
	enumHWND map[uint32][]windows.HWND
)

// windowsByPID lists visible, titled, top-level app windows per process.
func windowsByPID() map[uint32][]windows.HWND {
	enumOnce.Do(func() {
		enumCB = windows.NewCallback(func(h windows.HWND, _ uintptr) uintptr {
			if !windows.IsWindowVisible(h) {
				return 1
			}
			if owner, _, _ := procGetWindow.Call(uintptr(h), gwOwner); owner != 0 {
				return 1
			}
			if l, _, _ := procGetWindowTextLengthW.Call(uintptr(h)); l == 0 {
				return 1
			}
			ex, _, _ := procGetWindowLongPtrW.Call(uintptr(h), gwlExStyle)
			if ex&wsExToolWindow != 0 {
				return 1
			}
			var pid uint32
			windows.GetWindowThreadProcessId(h, &pid)
			enumHWND[pid] = append(enumHWND[pid], h)
			return 1
		})
	})
	enumMu.Lock()
	defer enumMu.Unlock()
	enumHWND = map[uint32][]windows.HWND{}
	windows.EnumWindows(enumCB, nil)
	return enumHWND
}

// Snapshot returns the process list with CPU% computed since the previous call.
func (m *ProcMonitor) Snapshot() []Process {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

func (m *ProcMonitor) snapshotLocked() []Process {
	if time.Since(m.lastAt) < 900*time.Millisecond && m.last != nil {
		return m.last
	}
	raw := querySystemProcesses()
	now := time.Now()
	wall := now.Sub(m.prevAt).Seconds() * 1e7 * float64(runtime.NumCPU())
	cur := make(map[uint32]rawProc, len(raw))
	out := make([]Process, 0, len(raw))
	live := map[[2]int64]bool{}
	for _, r := range raw {
		cur[r.pid] = r
		p := Process{PID: r.pid, PPID: r.ppid, Name: r.name, Mem: r.privWS, Threads: r.threads, Session: r.session}
		if r.ctime > 0 {
			p.Started = (r.ctime - 116444736000000000) / 10000
		}
		if prev, ok := m.prev[r.pid]; ok && prev.ctime == r.ctime && !m.prevAt.IsZero() && wall > 0 && r.pid != 0 {
			p.CPU = clamp(100 * float64(r.cpuTime-prev.cpuTime) / wall)
		}
		if r.pid > 4 {
			p.Exe = m.exePath(r.pid, r.ctime)
		}
		live[[2]int64{int64(r.pid), r.ctime}] = true
		out = append(out, p)
	}
	for k := range m.paths { // forget exited processes
		if !live[k] {
			delete(m.paths, k)
		}
	}
	m.prev, m.prevAt, m.last, m.lastAt = cur, now, out, now
	if time.Since(m.winAt) > 2*time.Second || m.winPIDs == nil {
		m.winPIDs = map[uint32]bool{}
		for pid := range windowsByPID() {
			m.winPIDs[pid] = true
		}
		m.winAt = now
	}
	return out
}

var sysRoot = strings.ToLower(os.Getenv("SystemRoot"))

// Groups folds processes into friendly app groups, heaviest first.
func (m *ProcMonitor) Groups(withProcs bool) []AppGroup {
	m.mu.Lock()
	defer m.mu.Unlock()
	procs := m.snapshotLocked()
	groups := map[string]*AppGroup{}
	for _, p := range procs {
		key := strings.ToLower(p.Exe)
		if key == "" {
			key = strings.ToLower(p.Name)
		}
		g := groups[key]
		if g == nil {
			g = &AppGroup{Key: key, Exe: p.Exe, Kind: "background"}
			if p.Exe != "" {
				meta := m.exeMeta(p.Exe)
				g.Name, g.Publisher = meta.desc, meta.publisher
			}
			if g.Name == "" {
				g.Name = prettyName(strings.TrimSuffix(p.Name, filepath.Ext(p.Name)))
			}
			if n, ok := friendlyOverrides[strings.ToLower(p.Name)]; ok {
				g.Name = n
			}
			groups[key] = g
		}
		g.Count++
		g.CPU += p.CPU
		g.Mem += p.Mem
		g.PIDs = append(g.PIDs, p.PID)
		if withProcs {
			g.Procs = append(g.Procs, p)
		}
		lexe := strings.ToLower(p.Exe)
		if m.winPIDs[p.PID] {
			g.Kind = "app"
		} else if g.Kind != "app" && (p.Session == 0 || p.Exe == "" || (sysRoot != "" && strings.HasPrefix(lexe, sysRoot+`\`))) {
			g.Kind = "system"
		}
		if pr, why := protection(p, m.selfPID); rank(pr) > rank(g.Protect) {
			g.Protect, g.Reason = pr, why
		}
	}
	out := make([]AppGroup, 0, len(groups))
	for _, g := range groups {
		if g.Key == "system idle process" {
			continue
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		si, sj := out[i].Mem+uint64(out[i].CPU*50e6), out[j].Mem+uint64(out[j].CPU*50e6)
		return si > sj
	})
	return out
}

func (m *ProcMonitor) Group(key string) (AppGroup, bool) {
	for _, g := range m.Groups(true) {
		if g.Key == key {
			sort.Slice(g.Procs, func(i, j int) bool { return g.Procs[i].Mem > g.Procs[j].Mem })
			return g, true
		}
	}
	return AppGroup{}, false
}

func rank(p string) int {
	switch p {
	case "blocked":
		return 2
	case "caution":
		return 1
	}
	return 0
}

var friendlyOverrides = map[string]string{
	"svchost.exe":        "Windows Service Host",
	"system":             "Windows Kernel",
	"registry":           "Windows Registry",
	"memory compression": "Memory Compression",
	"secure system":      "Secure System",
	"explorer.exe":       "File Explorer & Taskbar",
	"dwm.exe":            "Desktop Window Manager",
	"msmpeng.exe":        "Microsoft Defender",
	"csrss.exe":          "Windows Client Runtime",
}

func prettyName(base string) string {
	if base == "" {
		return "Unknown"
	}
	return strings.ToUpper(base[:1]) + base[1:]
}
