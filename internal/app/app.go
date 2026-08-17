package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"arham-gateway/internal/adminapi"
	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
	"arham-gateway/internal/logger"
	"arham-gateway/internal/publicapi"
	"arham-gateway/internal/routing"
	"arham-gateway/web"
)

type App struct {
	cfg       *config.Config
	db        *database.DB
	masterKey []byte
	router    *routing.Router
	server    *http.Server
	log       *slog.Logger
}

func New(configPath string) (*App, error) {
	log := logger.New(os.Stdout, logger.LevelInfo)

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Open database
	db, err := database.Open(cfg.Database.Path, cfg.Database.BusyTimeoutMs)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Run migrations and seed defaults
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	if err := db.SeedDefaults(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seeding defaults: %w", err)
	}

	// Load master key
	masterKey, err := crypto.LoadMasterKey(cfg.Security.MasterKeyPath)
	if err != nil {
		// Fallback: check if local master.key exists or create for dev
		if errors.Is(err, os.ErrNotExist) {
			_ = db.Close()
			return nil, fmt.Errorf("master key not found at %s. Please run 'gateway setup' or generate master.key: %w", cfg.Security.MasterKeyPath, err)
		}
		_ = db.Close()
		return nil, fmt.Errorf("loading master key: %w", err)
	}

	router := routing.NewRouter(db, masterKey, cfg)

	publicHandler := publicapi.NewHandler(db, router, cfg)
	adminHandler := adminapi.NewHandler(db, masterKey, cfg)
	assetHandler := web.AssetHandler()

	rootMux := http.NewServeMux()

	// Route public API endpoints
	rootMux.Handle("/healthz", publicHandler)
	rootMux.Handle("/readyz", publicHandler)
	rootMux.Handle("/v1/", publicHandler)

	// Route admin API endpoints
	rootMux.Handle("/api/", adminHandler)

	// Route dashboard static assets and SPA fallback
	rootMux.Handle("/", assetHandler)

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      rootMux,
		ReadTimeout:  cfg.Server.ReadTimeout.Duration(),
		WriteTimeout: cfg.Server.WriteTimeout.Duration(),
		IdleTimeout:  cfg.Server.IdleTimeout.Duration(),
	}

	return &App{
		cfg:       cfg,
		db:        db,
		masterKey: masterKey,
		router:    router,
		server:    srv,
		log:       log,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("starting Arham Gateway server",
		"listen_addr", a.cfg.Server.ListenAddr,
	)

	// Background log retention pruning loop
	pruneCtx, pruneCancel := context.WithCancel(context.Background())
	defer pruneCancel()
	go func() {
		ticker := time.NewTicker(a.cfg.Retention.CleanupInterval.Duration())
		defer ticker.Stop()
		for {
			select {
			case <-pruneCtx.Done():
				return
			case <-ticker.C:
				if deleted, err := a.db.PruneDetailedLogs(a.cfg.Retention.DetailedLogDays); err == nil && deleted > 0 {
					a.log.Info("pruned old detailed request logs", "deleted_rows", deleted)
				}
			}
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Listen for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		pruneCancel()
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		pruneCancel()
		a.log.Info("received shutdown signal, beginning graceful drain", "signal", sig.String())
	}

	drainTimeout := a.cfg.Timeouts.StreamDrainTimeout.Duration()
	if drainTimeout <= 0 {
		drainTimeout = 15 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.log.Error("server shutdown error", "error", err.Error())
	}

	if err := a.db.Close(); err != nil {
		a.log.Error("database close error", "error", err.Error())
	}

	a.log.Info("gateway server stopped gracefully")
	return nil
}

func CheckLocalStatus(listenAddr string) (string, error) {
	if strings.HasPrefix(listenAddr, ":") {
		listenAddr = "127.0.0.1" + listenAddr
	}
	url := fmt.Sprintf("http://%s/healthz", listenAddr)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "offline", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return "healthy", nil
	}
	return fmt.Sprintf("unhealthy (status %d)", resp.StatusCode), nil
}
