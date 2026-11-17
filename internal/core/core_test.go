//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanAggregates(t *testing.T) {
	root := t.TempDir()
	write := func(rel string, n int) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, make([]byte, n), 0o644)
	}
	write(`a\big.mp4`, 400_000)
	write(`a\b\song.mp3`, 300_000)
	write(`c\doc.pdf`, 100)
	write(`top.zip`, 1000)

	s := NewScanner()
	if err := s.Start(root); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !s.Status().Done {
		if time.Now().After(deadline) {
			t.Fatal("scan timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	tv, err := s.Tree("")
	if err != nil {
		t.Fatal(err)
	}
	if tv.Size != 701_100 || tv.Files != 4 {
		t.Fatalf("root size=%d files=%d", tv.Size, tv.Files)
	}
	if tv.Items[0].Name != "a" || tv.Items[0].Size != 700_000 {
		t.Fatalf("first item %+v", tv.Items[0])
	}
	big := s.Biggest("", 10)
	if len(big) != 4 || big[0].Name != "big.mp4" || big[0].Category != "video" {
		t.Fatalf("biggest %+v", big)
	}
	if sz, ok := s.SizeOf(filepath.Join(root, "a")); !ok || sz != 700_000 {
		t.Fatalf("sizeof a = %d %v", sz, ok)
	}
	s.Forget(filepath.Join(root, `a\big.mp4`), 400_000)
	tv, _ = s.Tree("")
	if tv.Size != 301_100 {
		t.Fatalf("after forget size=%d", tv.Size)
	}
	sub, err := s.Tree(filepath.Join(root, "A")) // case-insensitive
	if err != nil || len(sub.Crumbs) != 2 {
		t.Fatalf("subtree %v %+v", err, sub.Crumbs)
	}
}

func TestProtectedPath(t *testing.T) {
	sr := os.Getenv("SystemRoot")
	cases := map[string]bool{
		`C:\`:                               true,
		sr:                                  true,
		filepath.Join(sr, `System32\x.dll`): true,
		`C:\Program Files`:                  true,
		`C:\pagefile.sys`:                   true,
		os.Getenv("USERPROFILE"):            true,
		filepath.Join(os.Getenv("USERPROFILE"), `Downloads\setup.exe`): false,
		`D:\Games\big.iso`: false,
		`relative\path`:    true,
	}
	for p, want := range cases {
		if got := ProtectedPath(p) != ""; got != want {
			t.Errorf("ProtectedPath(%q) protected=%v want %v", p, got, want)
		}
	}
}

func TestRecycleAndRestore(t *testing.T) {
	dir, _ := os.MkdirTemp(filepath.Join(os.Getenv("USERPROFILE")), "tidy-test-")
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "hello.txt")
	os.WriteFile(f, []byte("hi"), 0o644)
	res := Delete([]string{f}, false, func(string) (uint64, bool) { return 0, false })
	if !res[0].OK {
		t.Fatalf("delete: %+v", res[0])
	}
	if _, err := os.Stat(f); err == nil {
		t.Fatal("file still exists")
	}
	rr := Restore([]string{f})
	if !rr[0].OK {
		t.Fatalf("restore: %+v", rr[0])
	}
	if b, _ := os.ReadFile(f); string(b) != "hi" {
		t.Fatal("restored content mismatch")
	}
}

func TestProcessGroups(t *testing.T) {
	m := NewProcMonitor()
	m.Snapshot()
	time.Sleep(1100 * time.Millisecond)
	gs := m.Groups(false)
	if len(gs) < 10 {
		t.Fatalf("only %d groups", len(gs))
	}
	var sawSvc, sawCsrss bool
	for _, g := range gs {
		if strings.HasSuffix(g.Key, `svchost.exe`) {
			sawSvc = g.Count > 1 && g.Protect == "caution"
		}
		if strings.HasSuffix(g.Key, "csrss.exe") {
			sawCsrss = g.Protect == "blocked"
		}
	}
	if !sawSvc || !sawCsrss {
		t.Fatalf("svchost grouped/caution=%v csrss blocked=%v", sawSvc, sawCsrss)
	}
	for _, g := range gs {
		if strings.HasSuffix(g.Key, "svchost.exe") {
			if _, _, err := m.StopGroup(g.Key, true, false); err != ErrCaution {
				t.Fatalf("svchost force-quit without confirmation should need caution, got %v", err)
			}
		}
		if strings.HasSuffix(g.Key, "csrss.exe") {
			if _, _, err := m.StopGroup(g.Key, true, true); err != ErrBlocked {
				t.Fatalf("csrss kill should be blocked, got %v", err)
			}
		}
	}
}

func TestIcon(t *testing.T) {
	ic := NewIcons()
	b, err := ic.Get(filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"))
	if err != nil || len(b) < 100 || string(b[1:4]) != "PNG" {
		t.Fatalf("icon err=%v len=%d", err, len(b))
	}
}
