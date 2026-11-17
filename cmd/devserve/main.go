// devserve runs Tidy's UI + API in a normal browser at http://127.0.0.1:5199
// without the desktop window or pairing — for UI development and screenshots.
// It only listens on localhost.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"Tidy/internal/core"
	"Tidy/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5199", "listen address (localhost only)")
	dist := flag.String("dist", "frontend/dist", "built frontend folder")
	flag.Parse()
	if !strings.HasPrefix(*addr, "127.0.0.1:") && !strings.HasPrefix(*addr, "localhost:") {
		log.Fatal("devserve only listens on localhost")
	}
	procs := core.NewProcMonitor()
	api := &server.API{Procs: procs, Sampler: core.NewSampler(procs), Scanner: core.NewScanner(), Icons: core.NewIcons(), Settings: core.LoadSettings()}
	server.NewRemote(api, os.DirFS(*dist))
	local := api.LocalHandler()
	files := http.FileServer(http.Dir(*dist))
	log.Printf("Tidy dev UI on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			local.ServeHTTP(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})))
}
