// Command eurofish is the self-contained eurofish server: HTTP API, web UI,
// migrations and background jobs in one static binary.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"eurofish/internal/cache"
	"eurofish/internal/config"
	"eurofish/internal/db"
	"eurofish/internal/server"
)

// version is set at build time: -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve":
		if err := serve(); err != nil {
			fmt.Fprintln(os.Stderr, "eurofish:", err)
			os.Exit(1)
		}
	case "migrate":
		if err := migrateOnly(); err != nil {
			fmt.Fprintln(os.Stderr, "eurofish:", err)
			os.Exit(1)
		}
	case "healthcheck":
		os.Exit(healthcheck())
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "usage: eurofish [serve|migrate|healthcheck|version]\n")
		os.Exit(2)
	}
}

func serve() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	c, err := cache.New(cfg.CacheURL)
	if err != nil {
		return err
	}
	defer c.Close()

	srv := server.New(cfg, database, c, log, version)
	go srv.RunJanitor(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Listen() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	return srv.Shutdown(15 * time.Second)
}

func migrateOnly() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := openDB(context.Background(), cfg)
	if err != nil {
		return err
	}
	return database.Close()
}

func openDB(ctx context.Context, cfg *config.Config) (*db.DB, error) {
	if cfg.DBDriver == "sqlite" {
		if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
		if err := adoptLegacyDB(ctx, cfg); err != nil {
			return nil, err
		}
	}
	database, err := db.Open(ctx, cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		return nil, err
	}
	if cfg.DBDriver == "sqlite" {
		database.BackupDir = filepath.Join(cfg.DataDir, "backups")
	}
	if err := database.Migrate(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return database, nil
}

// adoptLegacyDB copies ./data/eurofish.db from the working directory into the
// data directory when the latter has no database yet. The source is left as is.
func adoptLegacyDB(ctx context.Context, cfg *config.Config) error {
	if cfg.LegacyDataDir == "" {
		return nil
	}
	src := filepath.Join(cfg.LegacyDataDir, "eurofish.db")
	dst := filepath.Join(cfg.DataDir, "eurofish.db")
	srcAbs, err1 := filepath.Abs(src)
	dstAbs, err2 := filepath.Abs(dst)
	if err1 != nil || err2 != nil || srcAbs == dstAbs {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if fi, err := os.Stat(src); err != nil || fi.IsDir() {
		return nil
	}
	if err := db.CopySQLite(ctx, src, dst); err != nil {
		return fmt.Errorf("copy database from %s to %s: %w", src, dst, err)
	}
	fmt.Fprintf(os.Stderr, "eurofish: copied existing database %s to %s\n", srcAbs, dstAbs)
	return nil
}

// healthcheck is used by container HEALTHCHECKs (the scratch image has no curl).
func healthcheck() int {
	addr := os.Getenv("EUROFISH_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	port := addr[strings.LastIndex(addr, ":")+1:]
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
