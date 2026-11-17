//go:build windows

package core

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// Icons extracts and caches app icons as PNG. All GDI/shell work runs on one
// dedicated, COM-initialised OS thread.
type Icons struct {
	mu    sync.Mutex
	cache map[string][]byte
	jobs  chan iconJob
}

type iconJob struct {
	path string
	out  chan []byte
}

func NewIcons() *Icons {
	ic := &Icons{cache: map[string][]byte{}, jobs: make(chan iconJob, 64)}
	go ic.worker()
	return ic
}

func (ic *Icons) Get(path string) ([]byte, error) {
	if path == "" || !strings.HasSuffix(strings.ToLower(path), ".exe") {
		return nil, errors.New("not an exe")
	}
	key := strings.ToLower(path)
	ic.mu.Lock()
	if b, ok := ic.cache[key]; ok {
		ic.mu.Unlock()
		if b == nil {
			return nil, errors.New("no icon")
		}
		return b, nil
	}
	ic.mu.Unlock()
	out := make(chan []byte, 1)
	ic.jobs <- iconJob{path, out}
	b := <-out
	ic.mu.Lock()
	if len(ic.cache) > 2000 {
		ic.cache = map[string][]byte{}
	}
	ic.cache[key] = b
	ic.mu.Unlock()
	if b == nil {
		return nil, errors.New("no icon")
	}
	return b, nil
}

func (ic *Icons) worker() {
	runtime.LockOSThread()
	procCoInitializeEx.Call(0, 2) // COINIT_APARTMENTTHREADED
	for j := range ic.jobs {
		j.out <- extractIconPNG(j.path)
	}
}

type shFileInfo struct {
	hIcon       uintptr
	iIcon       int32
	attributes  uint32
	displayName [260]uint16
	typeName    [80]uint16
}

type iconInfo struct {
	fIcon    int32
	xHot     uint32
	yHot     uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type bitmap struct {
	bmType       int32
	bmWidth      int32
	bmHeight     int32
	bmWidthBytes int32
	bmPlanes     uint16
	bmBitsPixel  uint16
	bmBits       uintptr
}

type bitmapInfo struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
	colors          [4]uint32
}

func extractIconPNG(path string) []byte {
	var hicon uintptr
	var id uint32
	n, _, _ := procPrivateExtractIconsW.Call(uintptr(unsafe.Pointer(utf16Ptr(path))), 0, 64, 64,
		uintptr(unsafe.Pointer(&hicon)), uintptr(unsafe.Pointer(&id)), 1, 0)
	if n == 0 || n == 0xFFFFFFFF || hicon == 0 {
		var sfi shFileInfo
		r, _, _ := procSHGetFileInfoW.Call(uintptr(unsafe.Pointer(utf16Ptr(path))), 0,
			uintptr(unsafe.Pointer(&sfi)), unsafe.Sizeof(sfi), 0x100) // SHGFI_ICON | SHGFI_LARGEICON
		if r == 0 || sfi.hIcon == 0 {
			return nil
		}
		hicon = sfi.hIcon
	}
	defer procDestroyIcon.Call(hicon)

	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(hicon, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil
	}
	defer procDeleteObject.Call(ii.hbmMask)
	if ii.hbmColor == 0 {
		return nil
	}
	defer procDeleteObject.Call(ii.hbmColor)

	var bm bitmap
	procGetObjectW.Call(ii.hbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
	w, h := int(bm.bmWidth), int(bm.bmHeight)
	if w <= 0 || h <= 0 || w > 512 || h > 512 {
		return nil
	}
	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)

	bits := func(hbm uintptr) []byte {
		bi := bitmapInfo{biWidth: int32(w), biHeight: -int32(h), biPlanes: 1, biBitCount: 32}
		bi.biSize = 40
		buf := make([]byte, w*h*4)
		if r, _, _ := procGetDIBits.Call(hdc, hbm, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0); r == 0 {
			return nil
		}
		return buf
	}
	color := bits(ii.hbmColor)
	if color == nil {
		return nil
	}
	hasAlpha := false
	for i := 3; i < len(color); i += 4 {
		if color[i] != 0 {
			hasAlpha = true
			break
		}
	}
	var mask []byte
	if !hasAlpha && ii.hbmMask != 0 {
		mask = bits(ii.hbmMask)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		b, g, r, a := color[i*4], color[i*4+1], color[i*4+2], color[i*4+3]
		if !hasAlpha {
			a = 255
			if mask != nil && mask[i*4] != 0 {
				a = 0
			}
		}
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r, g, b, a
	}
	var out bytes.Buffer
	if png.Encode(&out, img) != nil {
		return nil
	}
	return out.Bytes()
}
