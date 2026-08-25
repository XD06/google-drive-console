package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dsk/drive-backup-console/internal/api"
	"github.com/dsk/drive-backup-console/internal/apikey"
	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
	"github.com/dsk/drive-backup-console/internal/download"
	"github.com/dsk/drive-backup-console/internal/upload"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("dotenv: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Set up optimized HTTP transport with connection pooling for Google API calls
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	if cfg.HTTPProxy != "" {
		proxyURL, err := url.Parse(cfg.HTTPProxy)
		if err != nil {
			log.Fatalf("invalid HTTP_PROXY: %v", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		log.Printf("HTTP proxy set to %s", cfg.HTTPProxy)
	}
	http.DefaultTransport = transport
	log.Printf("HTTP transport: MaxIdleConnsPerHost=100, HTTP2=on")

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		log.Fatalf("data dir: %v", err)
	}

	authSvc, err := auth.NewService(cfg)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	uploadStore := upload.NewPersistentStore(filepath.Join(cfg.DataDir, "uploads.json"))
	// Pass PersistentStore itself so Put/Update schedule debounced disk saves (not only on shutdown).
	uploadSvc := &upload.Service{Store: uploadStore}
	uploadStore.StartReaper(10*time.Minute, time.Hour)

	apiKeys := apikey.NewStore(cfg.APIKeysPath)

	// Download store (only if yt-dlp is configured)
	var downloadStore *download.PersistentStore
	if cfg.YtDlpPath != "" {
		if err := os.MkdirAll(cfg.DownloadTmpDir, 0o700); err != nil {
			log.Fatalf("download tmp dir: %v", err)
		}
		downloadStore = download.NewPersistentStore(filepath.Join(cfg.DataDir, "downloads.json"))
		downloadStore.StartReaper(10*time.Minute, time.Hour)
		log.Printf("download feature enabled: yt-dlp=%s tmpDir=%s", cfg.YtDlpPath, cfg.DownloadTmpDir)
		if cfg.DownloadProxy != "" {
			log.Printf("download proxy: %s", cfg.DownloadProxy)
		}
	} else {
		log.Printf("download feature disabled (set YTDLP_PATH to enable)")
	}

	handler := api.NewRouter(api.Deps{
		Config:    cfg,
		Auth:      authSvc,
		Uploads:   uploadSvc,
		Keys:      apiKeys,
		Downloads: downloadStore,
	})

	// Wrap with gzip compression (before logging, so compressed size is logged)
	compressed := api.GzipMiddleware(handler)
	// Wrap with logging middleware
	wrapped := api.LoggingMiddleware(compressed)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           wrapped,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0, // 0 = no timeout; upload chunks (16 MiB) on slow links can exceed 60s
		WriteTimeout:      0, // streaming uploads/downloads can be long
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("drive-backup-console listening on http://localhost%s (devMode=%v oauth=%v)", addr, cfg.DevMode, authSvc.Enabled)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received, draining connections…")
	uploadStore.StopReaper()
	if downloadStore != nil {
		downloadStore.StopReaper()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	uploadStore.SaveNow()
	if downloadStore != nil {
		downloadStore.SaveNow()
	}
	log.Println("server stopped")
}
