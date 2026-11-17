//go:build windows

package core

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

var (
	ErrBlocked = errors.New("blocked")
	ErrCaution = errors.New("needs confirmation")
	ErrGone    = errors.New("that app isn't running anymore")
)

type KillResult struct {
	Closed int      `json:"closed"` // windows asked to close nicely
	Killed int      `json:"killed"` // processes force-stopped
	Failed int      `json:"failed"` // processes we couldn't stop
	Denied bool     `json:"denied"` // some failures were "access denied" (needs admin)
	Errors []string `json:"errors,omitempty"`
}

// StopGroup closes (gracefully) or force-quits every process of an app group.
// key may be a group key or "pid:<n>" to target a single process.
func (m *ProcMonitor) StopGroup(key string, force, confirmed bool) (KillResult, AppGroup, error) {
	var res KillResult
	var g AppGroup
	var pids []uint32
	if strings.HasPrefix(key, "pid:") {
		var pid uint32
		fmt.Sscanf(key, "pid:%d", &pid)
		found := false
		for _, p := range m.Snapshot() {
			if p.PID == pid {
				pr, why := protection(p, m.selfPID)
				g = AppGroup{Key: key, Name: p.Name, Exe: p.Exe, Protect: pr, Reason: why, Count: 1, PIDs: []uint32{pid}, Mem: p.Mem}
				found = true
				break
			}
		}
		if !found {
			return res, g, ErrGone
		}
		pids = g.PIDs
	} else {
		var ok bool
		g, ok = m.Group(key)
		if !ok {
			return res, g, ErrGone
		}
		pids = g.PIDs
	}
	if g.Protect == "blocked" {
		return res, g, ErrBlocked
	}
	if g.Protect == "caution" && !confirmed {
		return res, g, ErrCaution
	}

	if !force {
		wins := windowsByPID()
		for _, pid := range pids {
			for _, h := range wins[pid] {
				if r, _, _ := procPostMessageW.Call(uintptr(h), wmClose, 0, 0); r != 0 {
					res.Closed++
				}
			}
		}
		if res.Closed == 0 {
			return res, g, errors.New("this app has no window to close — try Force quit")
		}
		return res, g, nil
	}

	// Force: include children (e.g. a browser's helper processes) so nothing is orphaned.
	targets := withDescendants(m.Snapshot(), pids)
	for _, pid := range targets {
		if pid == m.selfPID || pid <= 4 {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
		if err == nil {
			err = windows.TerminateProcess(h, 1)
			windows.CloseHandle(h)
		}
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) { // already exited
				continue
			}
			res.Failed++
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				res.Denied = true
			}
			if len(res.Errors) < 5 {
				res.Errors = append(res.Errors, fmt.Sprintf("PID %d: %v", pid, err))
			}
			continue
		}
		res.Killed++
	}
	return res, g, nil
}

func withDescendants(procs []Process, roots []uint32) []uint32 {
	started := map[uint32]int64{}
	for _, p := range procs {
		started[p.PID] = p.Started
	}
	children := map[uint32][]uint32{}
	for _, p := range procs {
		// Guard against PID reuse: a real child can't be older than its parent.
		if ps, ok := started[p.PPID]; ok && p.Started >= ps {
			children[p.PPID] = append(children[p.PPID], p.PID)
		}
	}
	seen := map[uint32]bool{}
	var out []uint32
	var walk func(uint32)
	walk = func(pid uint32) {
		if seen[pid] {
			return
		}
		seen[pid] = true
		out = append(out, pid)
		for _, c := range children[pid] {
			if c != pid && c > 4 {
				walk(c)
			}
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return out
}
