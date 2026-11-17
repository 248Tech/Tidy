package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows/registry"

	"Tidy/internal/core"
	"Tidy/internal/server"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	serve := flag.Bool("serve", false, "run without a window and serve Tidy to other devices (remote only)")
	port := flag.Int("port", 0, "port for remote access (default from settings, 7878)")
	pin := flag.String("pin", "", "fixed 6-digit pairing code for remote access (default: random)")
	flag.Parse()

	dist, _ := fs.Sub(assets, "frontend/dist")
	procs := core.NewProcMonitor()
	settings := core.LoadSettings()
	api := &server.API{
		Procs:    procs,
		Sampler:  core.NewSampler(procs),
		Scanner:  core.NewScanner(),
		Icons:    core.NewIcons(),
		Settings: settings,
		Headless: *serve,
	}
	remote := server.NewRemote(api, dist)
	if len(*pin) == 6 && strings.Trim(*pin, "0123456789") == "" {
		remote.SetPIN(*pin)
	}
	p := settings.Get().RemotePort
	if *port > 0 {
		p = *port
	}

	if *serve {
		runHeadless(remote, p)
		return
	}

	if settings.Get().RemoteEnabled {
		remote.Start(p) // errors are shown in Settings
	}

	var ctx context.Context
	api.Quit = func() {
		if ctx != nil {
			runtime.Quit(ctx)
		}
	}
	bg := &options.RGBA{R: 12, G: 13, B: 18, A: 255}
	if lightTheme(settings.Get().Theme) {
		bg = &options.RGBA{R: 246, G: 246, B: 249, A: 255}
	}
	err := wails.Run(&options.App{
		Title:            "Tidy",
		Width:            1240,
		Height:           800,
		MinWidth:         880,
		MinHeight:        600,
		Frameless:        true,
		BackgroundColour: bg,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: api.LocalHandler(),
		},
		OnStartup:  func(c context.Context) { ctx = c },
		OnShutdown: func(context.Context) { remote.Stop() },
		Windows: &windows.Options{
			Theme:               windows.SystemDefault,
			DisableWindowIcon:   false,
			WebviewUserDataPath: filepath.Join(core.DataDir(), "webview"),
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

func lightTheme(pref string) bool {
	switch pref {
	case "light":
		return true
	case "dark":
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 1
}

func runHeadless(remote *server.Remote, port int) {
	core.AttachParentConsole()
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		out = os.Stdout
	}
	if err := remote.Start(port); err != nil {
		fmt.Fprintln(out, "Tidy could not start remote access:", err)
		os.Exit(1)
	}
	st := remote.Status()
	var b strings.Builder
	fmt.Fprintf(&b, "\nTidy is running in remote-only mode.\n\nOpen one of these on your phone or another PC:\n")
	for _, u := range st.URLs {
		fmt.Fprintf(&b, "   %s\n", u)
	}
	fmt.Fprintf(&b, "\nPairing code:  %s\n\nYour browser will warn about the certificate the first time - that's expected.\nPress Ctrl+C to stop.\n", st.PIN)
	fmt.Fprint(out, b.String())
	os.WriteFile(filepath.Join(core.DataDir(), "remote-info.txt"), []byte(b.String()), 0o600)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	remote.Stop()
}
