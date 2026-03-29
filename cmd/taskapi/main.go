package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/envload"
	"github.com/AlexsanderHamir/T2A/pkgs/repo"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/handler"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const cmdName = "taskapi"

// Server timeouts: WriteTimeout is left unset so long-lived SSE streams are not cut off.
// ReadHeaderTimeout mitigates slowloris; IdleTimeout limits idle keep-alive connections.
const (
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxRequestHeaders = 1 << 20
)

func main() {
	port := flag.String("port", "8080", "HTTP listen port")
	envPath := flag.String("env", "", "path to .env (default: <repo-root>/.env)")
	migrate := flag.Bool("migrate", false, "run GORM AutoMigrate before serving")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	path, err := envload.Load(*envPath)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.env", "err", err)
		os.Exit(1)
	}
	slog.Info("env loaded", "cmd", cmdName, "operation", "taskapi.startup", "path", path)

	db, err := postgres.Open(os.Getenv("DATABASE_URL"), nil)
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.db", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if *migrate {
		if err := postgres.Migrate(ctx, db); err != nil {
			slog.Error("migrate failed", "cmd", cmdName, "operation", "taskapi.migrate", "err", err)
			os.Exit(1)
		}
		slog.Info("migrate ok", "cmd", cmdName, "operation", "taskapi.migrate")
	}

	taskStore := store.NewStore(db)
	hub := handler.NewSSEHub()
	var rep *repo.Root
	if root := strings.TrimSpace(os.Getenv("REPO_ROOT")); root != "" {
		r, err := repo.OpenRoot(root)
		if err != nil {
			slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.repo_root", "err", err)
			os.Exit(1)
		}
		rep = r
		slog.Info("repo root configured", "cmd", cmdName, "operation", "taskapi.startup", "path", rep.Abs())
	}
	api := handler.WithRecovery(handler.NewHandler(taskStore, hub, rep))
	mux := http.NewServeMux()
	mux.Handle("/", api)

	ln, err := net.Listen("tcp", net.JoinHostPort("", *port))
	if err != nil {
		slog.Error("startup failed", "cmd", cmdName, "operation", "taskapi.listen", "err", err)
		os.Exit(1)
	}

	baseURL := fmt.Sprintf("http://localhost:%s/", *port)
	slog.Info("listening", "cmd", cmdName, "operation", "taskapi.serve", "addr", ln.Addr().String(), "url", baseURL)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxRequestHeaders,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "cmd", cmdName, "operation", "taskapi.serve", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "cmd", cmdName, "operation", "taskapi.shutdown", "err", err)
		os.Exit(1)
	}
	if sqlDB, err := db.DB(); err != nil {
		slog.Error("database close skipped", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
	} else if err := sqlDB.Close(); err != nil {
		slog.Error("database close", "cmd", cmdName, "operation", "taskapi.db_close", "err", err)
		os.Exit(1)
	}
}
