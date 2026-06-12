// Command fluxtorrent is the single-binary torrent→HTTP streaming bridge
// (SPEC §10 entrypoint, wiring, graceful shutdown).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	fluxtorrent "github.com/jodacame/fluxtorrent"
	"github.com/jodacame/fluxtorrent/internal/api"
	"github.com/jodacame/fluxtorrent/internal/config"
	"github.com/jodacame/fluxtorrent/internal/engine"
)

func main() {
	configDir := getenv("FT_CONFIG_DIR", "/config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		log.Fatalf("config dir: %v", err)
	}

	store, err := config.Open(filepath.Join(configDir, "fluxtorrent.db"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	eng, err := engine.New(store)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}

	srv := api.New(eng, store, fluxtorrent.DistFS())

	// Resume keepSeed torrents so sharing survives restarts (SPEC §6.4).
	go eng.ResumeKeepSeed(context.Background())

	cfg := store.Get()
	addr := net.JoinHostPort(cfg.Net.ListenHost, strconv.Itoa(cfg.Net.ListenPort))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("FluxTorrent %s listening on http://%s (cache=%s)", api.Version, addr, cfg.Cache.Mode)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	// Graceful shutdown (SPEC §9): SIGTERM drops torrents cleanly, flushes bbolt.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	fmt.Println()
	log.Println("shutting down…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	eng.Close()
	log.Println("bye")
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
