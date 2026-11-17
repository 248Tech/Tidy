//go:build windows

package core

import (
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")

	procPostMessageW         = user32.NewProc("PostMessageW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetWindow            = user32.NewProc("GetWindow")
	procGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	procPrivateExtractIconsW = user32.NewProc("PrivateExtractIconsW")
	procDestroyIcon          = user32.NewProc("DestroyIcon")
	procGetIconInfo          = user32.NewProc("GetIconInfo")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procSHGetFileInfoW       = shell32.NewProc("SHGetFileInfoW")
	procSHFileOperationW     = shell32.NewProc("SHFileOperationW")
	procSHQueryRecycleBinW   = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW   = shell32.NewProc("SHEmptyRecycleBinW")
	procGetDIBits            = gdi32.NewProc("GetDIBits")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procGetObjectW           = gdi32.NewProc("GetObjectW")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	procAttachConsole        = kernel32.NewProc("AttachConsole")
	procCoInitializeEx       = ole32.NewProc("CoInitializeEx")
)

const (
	wmClose        = 0x0010
	gwOwner        = 4
	gwlExStyle     = ^uintptr(19) // -20
	wsExToolWindow = 0x00000080
)

func filetimeToInt(ft windows.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}

// AttachParentConsole lets a GUI-subsystem exe print to the terminal that launched it.
func AttachParentConsole() bool {
	r, _, _ := procAttachConsole.Call(uintptr(^uint32(0))) // ATTACH_PARENT_PROCESS
	return r != 0
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}
