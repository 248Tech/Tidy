//go:build windows

package core

import (
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/sys/windows"
)

const Version = "1.0.0"

type Drive struct {
	Mount string `json:"mount"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // fixed | removable | network
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

type Metrics struct {
	Time       int64     `json:"time"`
	CPU        float64   `json:"cpu"`
	PerCore    []float64 `json:"perCore"`
	MemUsed    uint64    `json:"memUsed"`
	MemTotal   uint64    `json:"memTotal"`
	CommitUsed uint64    `json:"commitUsed"`
	CommitMax  uint64    `json:"commitMax"`
	Drives     []Drive   `json:"drives"`
	NetUp      float64   `json:"netUp"`
	NetDown    float64   `json:"netDown"`
	DiskRead   float64   `json:"diskRead"`
	DiskWrite  float64   `json:"diskWrite"`
	Uptime     uint64    `json:"uptime"`
	Procs      int       `json:"procs"`
}

type SystemInfo struct {
	Hostname string `json:"hostname"`
	CPUModel string `json:"cpuModel"`
	Cores    int    `json:"cores"`
	MemTotal uint64 `json:"memTotal"`
	IsAdmin  bool   `json:"isAdmin"`
	Version  string `json:"version"`
}

var (
	sysInfoOnce sync.Once
	sysInfo     SystemInfo
)

func GetSystemInfo() SystemInfo {
	sysInfoOnce.Do(func() {
		h, _ := os.Hostname()
		model := ""
		if ci, err := cpu.Info(); err == nil && len(ci) > 0 {
			model = ci[0].ModelName
		}
		sysInfo = SystemInfo{Hostname: h, CPUModel: model, Cores: runtime.NumCPU(), MemTotal: memStatus().TotalPhys,
			IsAdmin: windows.GetCurrentProcessToken().IsElevated(), Version: Version}
	})
	return sysInfo
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func memStatus() memoryStatusEx {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return m
}

// Sampler produces system metrics on demand, at most about once per second,
// so Tidy uses no CPU at all when nobody is looking.
type Sampler struct {
	mu        sync.Mutex
	last      Metrics
	lastAt    time.Time
	prevIdle  uint64
	prevTotal uint64
	prevNet   [2]uint64
	prevDisk  [2]uint64
	prevAt    time.Time
	drivesAt  time.Time
	drives    []Drive
	procs     *ProcMonitor
}

func NewSampler(p *ProcMonitor) *Sampler { return &Sampler{procs: p} }

func (s *Sampler) Get() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastAt) < 900*time.Millisecond {
		return s.last
	}
	now := time.Now()
	m := Metrics{Time: now.UnixMilli()}

	var idle, kern, user windows.Filetime
	procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kern)), uintptr(unsafe.Pointer(&user)))
	i, t := filetimeToInt(idle), filetimeToInt(kern)+filetimeToInt(user)
	if s.prevTotal != 0 && t > s.prevTotal {
		m.CPU = clamp(100 * (1 - float64(i-s.prevIdle)/float64(t-s.prevTotal)))
	}
	s.prevIdle, s.prevTotal = i, t
	if pc, err := cpu.Percent(0, true); err == nil {
		m.PerCore = pc
	}

	ms := memStatus()
	m.MemTotal, m.MemUsed = ms.TotalPhys, ms.TotalPhys-ms.AvailPhys
	m.CommitMax, m.CommitUsed = ms.TotalPageFile, ms.TotalPageFile-ms.AvailPageFile

	dt := now.Sub(s.prevAt).Seconds()
	if io, err := net.IOCounters(false); err == nil && len(io) > 0 {
		cur := [2]uint64{io[0].BytesSent, io[0].BytesRecv}
		if !s.prevAt.IsZero() && dt > 0 && cur[0] >= s.prevNet[0] && cur[1] >= s.prevNet[1] {
			m.NetUp = float64(cur[0]-s.prevNet[0]) / dt
			m.NetDown = float64(cur[1]-s.prevNet[1]) / dt
		}
		s.prevNet = cur
	}
	if io, err := disk.IOCounters(); err == nil {
		var cur [2]uint64
		for _, d := range io {
			cur[0] += d.ReadBytes
			cur[1] += d.WriteBytes
		}
		if !s.prevAt.IsZero() && dt > 0 && cur[0] >= s.prevDisk[0] && cur[1] >= s.prevDisk[1] {
			m.DiskRead = float64(cur[0]-s.prevDisk[0]) / dt
			m.DiskWrite = float64(cur[1]-s.prevDisk[1]) / dt
		}
		s.prevDisk = cur
	}
	s.prevAt = now

	if time.Since(s.drivesAt) > 5*time.Second {
		s.drives = ListDrives()
		s.drivesAt = now
	}
	m.Drives = s.drives
	up, _, _ := procGetTickCount64.Call()
	m.Uptime = uint64(up) / 1000
	if s.procs != nil {
		m.Procs = len(s.procs.Snapshot())
	}
	s.last, s.lastAt = m, now
	return m
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func ListDrives() []Drive {
	buf := make([]uint16, 256)
	n, err := windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0])
	if err != nil {
		return nil
	}
	var out []Drive
	start := 0
	for i := 0; i < int(n); i++ {
		if buf[i] != 0 {
			continue
		}
		root := windows.UTF16ToString(buf[start:i])
		start = i + 1
		if root == "" {
			continue
		}
		p := utf16Ptr(root)
		kind := ""
		switch windows.GetDriveType(p) {
		case windows.DRIVE_FIXED:
			kind = "fixed"
		case windows.DRIVE_REMOVABLE:
			kind = "removable"
		case windows.DRIVE_REMOTE:
			kind = "network"
		default:
			continue
		}
		var free, total, totalFree uint64
		if windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree) != nil || total == 0 {
			continue
		}
		label := make([]uint16, 261)
		windows.GetVolumeInformation(p, &label[0], uint32(len(label)), nil, nil, nil, nil, 0)
		out = append(out, Drive{Mount: root, Label: windows.UTF16ToString(label), Kind: kind, Total: total, Free: totalFree})
	}
	return out
}
