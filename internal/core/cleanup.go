//go:build windows

package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CleanItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Icon     string `json:"icon"`
	Size     uint64 `json:"size"`
	Count    int64  `json:"count"`
	Note     string `json:"note,omitempty"`
	ReviewIn string `json:"reviewIn,omitempty"` // path to review instead of auto-cleaning
}

type cleanTarget struct {
	id, title, desc, icon, note string
	dirs                        func() []string
	olderThan                   time.Duration
	extensions                  []string
	review                      bool
}

func env(k string) string { return os.Getenv(k) }

var cleanTargets = []cleanTarget{
	{id: "temp", title: "Temporary files", icon: "flame",
		desc: "Leftovers apps create while they work. Windows and your apps make new ones when needed.",
		dirs: func() []string { return []string{env("TEMP"), filepath.Join(env("SystemRoot"), "Temp")} }, olderThan: 24 * time.Hour},
	{id: "browser", title: "Browser caches", icon: "globe",
		desc: "Copies of websites saved to load pages faster. Safe to clear; sites may load a bit slower the first time.",
		note: "Close your browsers first to clear the most.",
		dirs: func() []string {
			la := env("LOCALAPPDATA")
			var out []string
			for _, b := range []string{`Google\Chrome`, `Microsoft\Edge`, `BraveSoftware\Brave-Browser`, `Vivaldi`, `Opera Software\Opera Stable`} {
				ud := filepath.Join(la, b, "User Data")
				profiles, _ := filepath.Glob(filepath.Join(ud, "*", "Cache"))
				out = append(out, profiles...)
				cc, _ := filepath.Glob(filepath.Join(ud, "*", "Code Cache"))
				out = append(out, cc...)
			}
			ff, _ := filepath.Glob(filepath.Join(la, `Mozilla\Firefox\Profiles\*\cache2`))
			return append(out, ff...)
		}},
	{id: "crash", title: "Crash reports & logs", icon: "file-warning",
		desc: "Reports saved when apps crashed. Only useful to developers.",
		dirs: func() []string {
			return []string{filepath.Join(env("LOCALAPPDATA"), "CrashDumps"), filepath.Join(env("ProgramData"), `Microsoft\Windows\WER\ReportArchive`), filepath.Join(env("LOCALAPPDATA"), `Microsoft\Windows\WER\ReportArchive`)}
		}},
	{id: "thumbs", title: "Thumbnail cache", icon: "image",
		desc: "Little previews of your pictures. Windows rebuilds them automatically.",
		dirs: func() []string { return []string{filepath.Join(env("LOCALAPPDATA"), `Microsoft\Windows\Explorer`)} }, extensions: []string{".db"}},
	{id: "installers", title: "Old installers in Downloads", icon: "package",
		desc: "Setup files you downloaded more than 3 months ago. Once an app is installed you rarely need these.",
		dirs: func() []string { return []string{filepath.Join(env("USERPROFILE"), "Downloads")} }, olderThan: 90 * 24 * time.Hour,
		extensions: []string{".exe", ".msi", ".msix", ".zip", ".iso"}, review: true},
}

func (t cleanTarget) visit(fn func(path string, size uint64)) {
	cutoff := time.Now().Add(-t.olderThan)
	for _, root := range t.dirs() {
		if root == "" {
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			if t.id == "installers" && filepath.Dir(p) != root {
				return nil // only top level of Downloads
			}
			if len(t.extensions) > 0 {
				ok := false
				for _, e := range t.extensions {
					if strings.EqualFold(filepath.Ext(p), e) && (t.id != "thumbs" || strings.HasPrefix(strings.ToLower(d.Name()), "thumbcache_")) {
						ok = true
					}
				}
				if !ok {
					return nil
				}
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if t.olderThan > 0 && info.ModTime().After(cutoff) {
				return nil
			}
			fn(p, uint64(info.Size()))
			return nil
		})
	}
}

// CleanSuggestions measures each junk location (in parallel).
func CleanSuggestions() []CleanItem {
	out := make([]CleanItem, len(cleanTargets)+1)
	var wg sync.WaitGroup
	for i, t := range cleanTargets {
		wg.Add(1)
		go func(i int, t cleanTarget) {
			defer wg.Done()
			it := CleanItem{ID: t.id, Title: t.title, Desc: t.desc, Icon: t.icon, Note: t.note}
			if t.review {
				it.ReviewIn = t.dirs()[0]
			}
			t.visit(func(_ string, size uint64) { it.Size += size; it.Count++ })
			out[i] = it
		}(i, t)
	}
	rbSize, rbCount := RecycleBinSize()
	out[len(cleanTargets)] = CleanItem{ID: "recycle", Title: "Recycle Bin", Icon: "trash-2", Size: rbSize, Count: int64(rbCount),
		Desc: "Things you already deleted. Emptying it frees the space for good."}
	wg.Wait()
	return out
}

// Clean removes junk for the given suggestion IDs; files in use are skipped.
func Clean(ids []string) (freed uint64, removed int64, skipped int64) {
	for _, id := range ids {
		if id == "recycle" {
			size, n := RecycleBinSize()
			if EmptyRecycleBin() == nil {
				freed += size
				removed += int64(n)
			}
			continue
		}
		for _, t := range cleanTargets {
			if t.id != id || t.review {
				continue
			}
			t.visit(func(p string, size uint64) {
				if ProtectedPath(p) != "" && !strings.HasPrefix(strings.ToLower(p), sysRoot+`\temp\`) {
					skipped++
					return
				}
				if os.Remove(p) == nil {
					freed += size
					removed++
				} else {
					skipped++
				}
			})
			if t.id == "temp" { // tidy up now-empty folders inside temp locations
				for _, root := range t.dirs() {
					removeEmptyDirs(root)
				}
			}
		}
	}
	return
}

func removeEmptyDirs(root string) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.IsDir() {
			p := filepath.Join(root, e.Name())
			removeEmptyDirs(p)
			os.Remove(p) // fails harmlessly if not empty
		}
	}
}
