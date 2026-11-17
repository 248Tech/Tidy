//go:build windows

package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete          = 3
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
	fofAllowUndo      = 0x0040
	fofNoErrorUI      = 0x0400
	fofNoConfirmMkdir = 0x0200
)

type DeleteItem struct {
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Size  uint64 `json:"size"`
	Error string `json:"error,omitempty"`
	// TooBig means the Recycle Bin can't hold it; it must be deleted permanently.
	TooBig bool `json:"tooBig,omitempty"`
}

// PathSize returns the logical size of a file or folder tree.
func PathSize(path string) uint64 {
	st, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !st.IsDir() {
		return uint64(st.Size())
	}
	var total uint64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += uint64(info.Size())
			}
		}
		return nil
	})
	return total
}

// recycleLimit is a conservative guess of the largest item Windows will put in
// the Recycle Bin (default quota is ~5-10% of the drive). Bigger items are
// silently deleted forever by Windows, so we refuse instead and ask the user.
func recycleLimit(path string) uint64 {
	var free, total, totalFree uint64
	root := filepath.VolumeName(path) + `\`
	if windows.GetDiskFreeSpaceEx(utf16Ptr(root), &free, &total, &totalFree) != nil {
		return 1 << 62
	}
	return total / 20
}

// Delete moves each path to the Recycle Bin (or deletes it forever when
// permanent is true). sizeHint may provide known sizes to skip re-walking.
func Delete(paths []string, permanent bool, sizeHint func(string) (uint64, bool)) []DeleteItem {
	out := make([]DeleteItem, 0, len(paths))
	for _, p := range paths {
		it := DeleteItem{Path: p}
		if why := ProtectedPath(p); why != "" {
			it.Error = why
			out = append(out, it)
			continue
		}
		if _, err := os.Lstat(p); err != nil {
			it.Error = "It's already gone."
			out = append(out, it)
			continue
		}
		if sz, ok := sizeHint(p); ok {
			it.Size = sz
		} else {
			it.Size = PathSize(p)
		}
		if !permanent && it.Size > recycleLimit(p) {
			it.TooBig = true
			it.Error = "Too big for the Recycle Bin."
			out = append(out, it)
			continue
		}
		if err := shellDelete(p, !permanent); err != nil {
			it.Error = err.Error()
		} else if _, err := os.Lstat(p); err == nil {
			it.Error = "Windows couldn't remove it (it may be open in another program)."
		} else {
			it.OK = true
		}
		out = append(out, it)
	}
	return out
}

func shellDelete(path string, undo bool) error {
	from := append(utf16.Encode([]rune(filepath.Clean(path))), 0, 0)
	op := shFileOpStruct{wFunc: foDelete, pFrom: &from[0], fFlags: fofSilent | fofNoConfirmation | fofNoErrorUI | fofNoConfirmMkdir}
	if undo {
		op.fFlags |= fofAllowUndo
	}
	r, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		if r == 0x20 || r == 0x78 { // sharing violation / DE_ACCESSDENIEDSRC
			return errors.New("it's open in another program or needs admin rights")
		}
		if r == 5 {
			return errors.New("access denied — try restarting Tidy as admin")
		}
		return fmt.Errorf("Windows error 0x%x", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("cancelled")
	}
	return nil
}

// Restore puts recently recycled items back where they came from.
func Restore(paths []string) []DeleteItem {
	out := make([]DeleteItem, 0, len(paths))
	sid := ""
	if tu, err := windows.GetCurrentProcessToken().GetTokenUser(); err == nil {
		sid = tu.User.Sid.String()
	}
	for _, p := range paths {
		it := DeleteItem{Path: p}
		if err := restoreOne(p, sid); err != nil {
			it.Error = err.Error()
		} else {
			it.OK = true
		}
		out = append(out, it)
	}
	return out
}

type binEntry struct {
	iFile   string
	deleted int64
}

func restoreOne(orig, sid string) error {
	if _, err := os.Lstat(orig); err == nil {
		return errors.New("something with that name is already there")
	}
	dir := filepath.Join(filepath.VolumeName(orig)+`\`, "$Recycle.Bin", sid)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return errors.New("couldn't open the Recycle Bin")
	}
	var matches []binEntry
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), "$I") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(data) < 28 {
			continue
		}
		ver := binary.LittleEndian.Uint64(data[0:])
		deleted := int64(binary.LittleEndian.Uint64(data[16:]))
		var name []uint16
		switch ver {
		case 2:
			n := int(binary.LittleEndian.Uint32(data[24:]))
			if 28+n*2 > len(data) {
				continue
			}
			for i := 0; i < n; i++ {
				name = append(name, binary.LittleEndian.Uint16(data[28+i*2:]))
			}
		case 1:
			for i := 24; i+1 < len(data); i += 2 {
				name = append(name, binary.LittleEndian.Uint16(data[i:]))
			}
		default:
			continue
		}
		if strings.EqualFold(syscall.UTF16ToString(name), orig) {
			matches = append(matches, binEntry{filepath.Join(dir, e.Name()), deleted})
		}
	}
	if len(matches) == 0 {
		return errors.New("not found in the Recycle Bin")
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].deleted > matches[j].deleted })
	iFile := matches[0].iFile
	rFile := filepath.Join(filepath.Dir(iFile), "$R"+strings.TrimPrefix(filepath.Base(iFile), "$I"))
	os.MkdirAll(filepath.Dir(orig), 0o755)
	if err := os.Rename(rFile, orig); err != nil {
		return fmt.Errorf("couldn't restore: %v", err)
	}
	os.Remove(iFile)
	return nil
}

func Reveal(path string) error {
	if _, err := os.Stat(path); err != nil {
		return errors.New("it doesn't exist anymore")
	}
	cmd := exec.Command("explorer.exe", "/select,", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return cmd.Start()
}

type shQueryRBInfo struct {
	cbSize   uint32
	size     int64
	numItems int64
}

func RecycleBinSize() (uint64, uint64) {
	info := shQueryRBInfo{cbSize: uint32(unsafe.Sizeof(shQueryRBInfo{}))}
	procSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&info)))
	return uint64(info.size), uint64(info.numItems)
}

func EmptyRecycleBin() error {
	r, _, _ := procSHEmptyRecycleBinW.Call(0, 0, 0x1|0x2|0x4)
	if r != 0 && r != 0x8000FFFF { // E_UNEXPECTED = already empty
		return fmt.Errorf("couldn't empty the Recycle Bin (0x%x)", r)
	}
	return nil
}
