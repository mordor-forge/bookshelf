package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bookshelf/internal/api"
	"bookshelf/internal/config"
	"bookshelf/internal/scanner"
	"bookshelf/internal/store"
	"bookshelf/internal/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// seed library_dir setting from env on first boot if not yet set.
	current, err := st.GetLibraryDir(ctx)
	if err != nil {
		return err
	}
	if current == "" && cfg.LibraryDir != "" {
		if err := st.SetLibraryDir(ctx, cfg.LibraryDir); err != nil {
			log.Warn("seed library_dir from env failed", "err", err)
		} else {
			current = cfg.LibraryDir
		}
	}
	if current == "" {
		log.Info("starting", "library", "(not configured)", "db", cfg.DBPath, "listen", cfg.Listen)
	} else {
		log.Info("starting", "library", current, "db", cfg.DBPath, "listen", cfg.Listen)
	}

	sc := scanner.New(st, log)

	// initial scan in the background so startup is fast. no-op if library is unset.
	go func() {
		dir, err := st.GetLibraryDir(ctx)
		if err != nil {
			log.Error("read library_dir for initial scan", "err", err)
			return
		}
		if dir == "" {
			return
		}
		if _, err := sc.Run(ctx, dir); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("initial scan failed", "err", err)
		}
	}()

	handler := api.New(st, sc, web.Dist(), log)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
	return nil
}
