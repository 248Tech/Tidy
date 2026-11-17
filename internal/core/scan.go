//go:build windows

package core

import (
	"container/heap"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var categoryOf = map[string]string{}

func init() {
	add := func(cat string, exts ...string) {
		for _, e := range exts {
			categoryOf["."+e] = cat
		}
	}
	add("video", "mp4", "mkv", "avi", "mov", "wmv", "webm", "m4v", "flv", "mpg", "mpeg", "ts", "m2ts", "3gp")
	add("audio", "mp3", "wav", "flac", "aac", "ogg", "m4a", "wma", "opus", "aiff")
	add("image", "jpg", "jpeg", "png", "gif", "bmp", "webp", "heic", "tif", "tiff", "raw", "cr2", "nef", "arw", "psd", "svg", "ico")
	add("archive", "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst", "cab", "tgz")
	add("installer", "msi", "msix", "appx", "iso", "img", "dmg", "msp")
	add("document", "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "rtf", "odt", "ods", "csv", "epub", "md")
	add("app", "exe", "dll", "sys", "pak", "bin", "dat", "vpk", "bundle", "assets", "ucas", "utoc", "sav")
	add("disk", "vhd", "vhdx", "vmdk", "vdi", "qcow2", "wim", "esd")
	add("model", "gguf", "safetensors", "ckpt", "pt", "pth", "onnx", "h5", "tflite")
	add("code", "js", "ts", "go", "py", "java", "c", "cpp", "h", "cs", "rs", "json", "xml", "html", "css", "map", "pdb", "obj", "lib", "o", "a", "jar", "class", "node")
}

var Categories = []string{"video", "audio", "image", "archive", "installer", "document", "app", "disk", "model", "code", "other"}

const numCats = 11

func Category(name string) string {
	if c, ok := categoryOf[strings.ToLower(filepath.Ext(name))]; ok {
		return c
	}
	return "other"
}

func catIndex(c string) int {
	for i, x := range Categories {
		if x == c {
			return i
		}
	}
	return len(Categories) - 1
}

type FileEntry struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     uint64 `json:"size"`
	Modified int64  `json:"modified"`
	Category string `json:"category"`
}

type dirNode struct {
	name     string
	parent   *dirNode
	size     uint64
	own      uint64 // bytes of files directly inside
	files    uint32
	dirs     uint32
	modified int64
	children []*dirNode
	top      []FileEntry // largest direct files
	denied   bool
	cats     [numCats]uint32 // KB per category, for colouring folders
	dom      int8            // dominant category index, -1 = mixed
}

const topPerDir = 24

type fileHeap []FileEntry

func (h fileHeap) Len() int           { return len(h) }
func (h fileHeap) Less(i, j int) bool { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x any)        { *h = append(*h, x.(FileEntry)) }
func (h *fileHeap) Pop() any          { o := *h; n := len(o); x := o[n-1]; *h = o[:n-1]; return x }

type ScanStatus struct {
	Running  bool   `json:"running"`
	Done     bool   `json:"done"`
	Root     string `json:"root"`
	Files    int64  `json:"files"`
	Dirs     int64  `json:"dirs"`
	Bytes    int64  `json:"bytes"`
	Current  string `json:"current"`
	Elapsed  int64  `json:"elapsed"`
	Error    string `json:"error,omitempty"`
	Denied   int64  `json:"denied"`
	Expected uint64 `json:"expected"` // used bytes on the drive, for a progress bar
}

type CategoryTotal struct {
	Category string `json:"category"`
	Size     uint64 `json:"size"`
	Count    int64  `json:"count"`
}

// Scanner runs one disk scan at a time and answers questions about its result.
type Scanner struct {
	mu       sync.RWMutex
	cancel   context.CancelFunc
	root     *dirNode
	rootPath string
	started  time.Time
	finished time.Time
	running  bool
	done     bool
	errMsg   string
	expected uint64

	files, dirs, bytes, denied atomic.Int64
	current                    atomic.Value
	catSize                    [numCats]atomic.Uint64
	catCount                   [numCats]atomic.Int64

	bigMu  sync.Mutex
	big    fileHeap
	bigMin atomic.Uint64
}

const bigKeep = 1500

func NewScanner() *Scanner { return &Scanner{} }

func (s *Scanner) Start(path string) error {
	path = filepath.Clean(path)
	if len(path) == 2 && path[1] == ':' {
		path += `\`
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return errors.New("that folder doesn't exist")
	}
	s.Cancel()
	s.mu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.root = &dirNode{name: path}
	s.rootPath = path
	s.started = time.Now()
	s.running, s.done, s.errMsg = true, false, ""
	s.expected = 0
	for _, d := range ListDrives() {
		if strings.EqualFold(d.Mount, path) {
			s.expected = d.Total - d.Free
		}
	}
	s.files.Store(0)
	s.dirs.Store(0)
	s.bytes.Store(0)
	s.denied.Store(0)
	for i := range s.catSize {
		s.catSize[i].Store(0)
		s.catCount[i].Store(0)
	}
	s.big = nil
	s.bigMin.Store(0)
	root := s.root
	s.mu.Unlock()

	go func() {
		sem := make(chan struct{}, runtime.NumCPU()*2)
		var wg sync.WaitGroup
		s.walk(ctx, root, path, sem, &wg)
		wg.Wait()
		finalize(root)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.root != root {
			return // superseded by a newer scan
		}
		s.running = false
		s.finished = time.Now()
		if ctx.Err() != nil {
			s.errMsg = "cancelled"
		} else {
			s.done = true
		}
		runtime.GC()
	}()
	return nil
}

func (s *Scanner) Cancel() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

const (
	attrOffline        = 0x1000
	attrRecallOnOpen   = 0x40000
	attrRecallOnAccess = 0x400000
)

func (s *Scanner) walk(ctx context.Context, node *dirNode, path string, sem chan struct{}, wg *sync.WaitGroup) {
	if ctx.Err() != nil {
		return
	}
	s.current.Store(path)
	ents, err := os.ReadDir(path)
	if err != nil {
		node.denied = true
		s.denied.Add(1)
		return
	}
	var top fileHeap
	for _, e := range ents {
		if e.IsDir() {
			child := &dirNode{name: e.Name(), parent: node}
			if info, err := e.Info(); err == nil {
				child.modified = info.ModTime().UnixMilli()
			}
			node.children = append(node.children, child)
			continue
		}
		if !e.Type().IsRegular() {
			continue // symlinks, junctions, devices
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := uint64(info.Size())
		if a, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && a.FileAttributes&(attrOffline|attrRecallOnOpen|attrRecallOnAccess) != 0 {
			size = 0 // cloud-only file: takes no space on this PC
		}
		mod := info.ModTime().UnixMilli()
		cat := Category(e.Name())
		ci := catIndex(cat)
		s.catSize[ci].Add(size)
		s.catCount[ci].Add(1)
		node.own += size
		node.cats[ci] = satAdd(node.cats[ci], uint32(min(size>>10, 1<<32-1)))
		node.files++
		s.files.Add(1)
		s.bytes.Add(int64(size))
		if len(top) < topPerDir || size > top[0].Size {
			fe := FileEntry{Name: e.Name(), Size: size, Modified: mod, Category: cat}
			if len(top) < topPerDir {
				heap.Push(&top, fe)
			} else {
				top[0] = fe
				heap.Fix(&top, 0)
			}
		}
		if size > s.bigMin.Load() {
			s.addBig(FileEntry{Path: filepath.Join(path, e.Name()), Name: e.Name(), Size: size, Modified: mod, Category: cat})
		}
	}
	node.top = top
	node.dirs = uint32(len(node.children))
	s.dirs.Add(int64(len(node.children)))
	for _, c := range node.children {
		cp := filepath.Join(path, c.name)
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func(c *dirNode, cp string) {
				defer func() { <-sem; wg.Done() }()
				s.walk(ctx, c, cp, sem, wg)
			}(c, cp)
		default:
			s.walk(ctx, c, cp, sem, wg)
		}
	}
}

func (s *Scanner) addBig(f FileEntry) {
	s.bigMu.Lock()
	defer s.bigMu.Unlock()
	if len(s.big) < bigKeep {
		heap.Push(&s.big, f)
	} else if f.Size > s.big[0].Size {
		s.big[0] = f
		heap.Fix(&s.big, 0)
	}
	if len(s.big) >= bigKeep {
		s.bigMin.Store(s.big[0].Size)
	}
}

// finalize sums sizes bottom-up and forgets detail inside tiny folders to keep memory low.
func finalize(n *dirNode) {
	n.size = n.own
	files, dirs := n.files, n.dirs
	for _, c := range n.children {
		finalize(c)
		n.size += c.size
		for i, v := range c.cats {
			n.cats[i] = satAdd(n.cats[i], v)
		}
		files += c.files
		dirs += c.dirs
	}
	n.files, n.dirs = files, dirs
	n.dom = -1
	var total, best uint64
	for i, v := range n.cats {
		total += uint64(v)
		if uint64(v) > best {
			best, n.dom = uint64(v), int8(i)
		}
	}
	if total == 0 || best*100 < total*45 || Categories[n.dom] == "other" {
		n.dom = -1
	}
	sort.Slice(n.children, func(i, j int) bool { return n.children[i].size > n.children[j].size })
	if n.size < 256*1024 && n.parent != nil {
		n.children = nil
		n.top = nil
	}
}

func (s *Scanner) Status() ScanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := ScanStatus{Running: s.running, Done: s.done, Root: s.rootPath, Error: s.errMsg, Expected: s.expected,
		Files: s.files.Load(), Dirs: s.dirs.Load(), Bytes: s.bytes.Load(), Denied: s.denied.Load()}
	if cur, ok := s.current.Load().(string); ok && s.running {
		st.Current = cur
	}
	if !s.started.IsZero() {
		end := time.Now()
		if !s.running {
			end = s.finished
		}
		st.Elapsed = end.Sub(s.started).Milliseconds()
	}
	return st
}

type TreeItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     uint64 `json:"size"`
	IsDir    bool   `json:"isDir"`
	Files    uint32 `json:"files,omitempty"`
	Category string `json:"category"`
	Modified int64  `json:"modified,omitempty"`
	Other    bool   `json:"other,omitempty"` // aggregated "smaller files"
}

type TreeView struct {
	Path   string     `json:"path"`
	Name   string     `json:"name"`
	Size   uint64     `json:"size"`
	Files  uint32     `json:"files"`
	Dirs   uint32     `json:"dirs"`
	Crumbs []TreeItem `json:"crumbs"`
	Items  []TreeItem `json:"items"`
}

func (s *Scanner) find(path string) *dirNode {
	if s.root == nil {
		return nil
	}
	rel, err := filepath.Rel(s.rootPath, filepath.Clean(path))
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil
	}
	n := s.root
	if rel == "." {
		return n
	}
	for _, part := range strings.Split(rel, `\`) {
		var next *dirNode
		for _, c := range n.children {
			if strings.EqualFold(c.name, part) {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

func (n *dirNode) path(rootPath string) string {
	if n.parent == nil {
		return rootPath
	}
	return filepath.Join(n.parent.path(rootPath), n.name)
}

// Tree returns one level of a finished scan: sub-folders and big files.
func (s *Scanner) Tree(path string) (TreeView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.done {
		return TreeView{}, errors.New("no finished scan yet")
	}
	if path == "" {
		path = s.rootPath
	}
	n := s.find(path)
	if n == nil {
		return TreeView{}, errors.New("folder not found in scan")
	}
	p := n.path(s.rootPath)
	v := TreeView{Path: p, Name: n.name, Size: n.size, Files: n.files, Dirs: n.dirs}
	for c := n; c != nil; c = c.parent {
		v.Crumbs = append([]TreeItem{{Name: c.name, Path: c.path(s.rootPath), Size: c.size, IsDir: true}}, v.Crumbs...)
	}
	var shownFiles uint64
	for _, c := range n.children {
		v.Items = append(v.Items, TreeItem{Name: c.name, Path: filepath.Join(p, c.name), Size: c.size, IsDir: true, Files: c.files, Category: c.category(), Modified: c.modified})
	}
	for _, f := range n.top {
		shownFiles += f.Size
		v.Items = append(v.Items, TreeItem{Name: f.Name, Path: filepath.Join(p, f.Name), Size: f.Size, Category: f.Category, Modified: f.Modified})
	}
	if rest := n.own - min(n.own, shownFiles); rest > 0 {
		v.Items = append(v.Items, TreeItem{Name: "Smaller files", Size: rest, Category: "other", Other: true})
	}
	sort.Slice(v.Items, func(i, j int) bool { return v.Items[i].Size > v.Items[j].Size })
	if len(v.Items) > 300 {
		v.Items = v.Items[:300]
	}
	return v, nil
}

func (s *Scanner) Biggest(category string, limit int) []FileEntry {
	s.bigMu.Lock()
	all := append([]FileEntry(nil), s.big...)
	s.bigMu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].Size > all[j].Size })
	out := make([]FileEntry, 0, limit)
	for _, f := range all {
		if category != "" && category != "all" && f.Category != category {
			continue
		}
		out = append(out, f)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Scanner) CategoryTotals() []CategoryTotal {
	out := make([]CategoryTotal, 0, len(Categories))
	for i, c := range Categories {
		out = append(out, CategoryTotal{Category: c, Size: s.catSize[i].Load(), Count: s.catCount[i].Load()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// SizeOf returns a known size for a path from the last scan.
func (s *Scanner) SizeOf(path string) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.done {
		return 0, false
	}
	if n := s.find(path); n != nil {
		return n.size, true
	}
	if n := s.find(filepath.Dir(path)); n != nil {
		base := filepath.Base(path)
		for _, f := range n.top {
			if strings.EqualFold(f.Name, base) {
				return f.Size, true
			}
		}
	}
	return 0, false
}

// Forget updates the scan result after a path was deleted.
func (s *Scanner) Forget(path string, size uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		parent := s.find(filepath.Dir(path))
		if parent != nil {
			base := filepath.Base(path)
			removed := false
			for i, c := range parent.children {
				if strings.EqualFold(c.name, base) {
					parent.children = append(parent.children[:i], parent.children[i+1:]...)
					removed = true
					break
				}
			}
			if !removed {
				for i, f := range parent.top {
					if strings.EqualFold(f.Name, base) {
						parent.top = append(parent.top[:i], parent.top[i+1:]...)
						parent.own -= min(parent.own, size)
						break
					}
				}
			}
			for n := parent; n != nil; n = n.parent {
				n.size -= min(n.size, size)
			}
		}
	}
	s.bigMu.Lock()
	lp := strings.ToLower(path)
	keep := s.big[:0]
	for _, f := range s.big {
		fp := strings.ToLower(f.Path)
		if fp == lp || strings.HasPrefix(fp, lp+`\`) {
			ci := catIndex(f.Category)
			s.catSize[ci].Add(^(f.Size - 1))
			continue
		}
		keep = append(keep, f)
	}
	s.big = keep
	heap.Init(&s.big)
	s.bigMu.Unlock()
}

func satAdd(a, b uint32) uint32 {
	if a > ^uint32(0)-b {
		return ^uint32(0)
	}
	return a + b
}

// category is the folder's dominant file type, or "folder" when it's a mix.
func (n *dirNode) category() string {
	if n.dom < 0 {
		return "folder"
	}
	return Categories[n.dom]
}
