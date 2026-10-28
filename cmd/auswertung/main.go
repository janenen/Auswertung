// Command auswertung starts the local web app and opens it in the browser.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/server"
	"github.com/janenen/auswertung/internal/store"
	"github.com/janenen/auswertung/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dataDir := flag.String("data", "db", "database directory")
	outDir := flag.String("out", "output", "directory for generated reports")
	leagueTemplate := flag.String("league-template", filepath.Join("extern", "formular_ligaergebnisse_kr_bez_ll.xlsx"), "RSB league result form (.xlsx)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser")
	simulator := flag.Bool("simulator", false, "offer a simulated RIKA SAG-2 for testing without hardware")
	flag.Parse()
	machine.SetSimulator(*simulator)

	url := "http://" + *addr
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		// Most likely the app is already running: just show it.
		log.Printf("listen on %s: %v", *addr, err)
		if !*noBrowser {
			openBrowser(url)
		}
		os.Exit(1)
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Handler: server.New(st, web.Dist(), server.Options{
		OutDir:         *outDir,
		LeagueTemplate: *leagueTemplate,
		Shutdown:       stop,
	})}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("Auswertung läuft auf %s (Daten: %s, Ausgabe: %s)", url, *dataDir, *outDir)
	if !*noBrowser {
		openBrowser(url)
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("opening browser: %v", err)
	}
}
