package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myth815/tunescout/internal/api"
	"github.com/myth815/tunescout/internal/config"
	"github.com/myth815/tunescout/internal/federation"
	"github.com/myth815/tunescout/internal/music"
	"github.com/myth815/tunescout/internal/provider"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}
	cfg := config.FromEnv()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	client := &http.Client{Timeout: cfg.ProviderTimeout}

	providers := []provider.Provider{
		provider.NewMusicBrainz(client, cfg),
		provider.NewITunes(client, cfg),
		provider.NewLRCLIB(client, cfg),
		provider.NewAudius(client, cfg),
		provider.NewJamendo(client, cfg),
		provider.NewAcoustID(client, cfg),
		provider.NewAudD(client, cfg),
	}

	engine := federation.New(providers, music.Policy{}, cfg.SearchTimeout, logger)
	handler := api.New(engine, providers, cfg, logger)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.RequestTimeout,
		WriteTimeout:      cfg.RequestTimeout,
		IdleTimeout:       90 * time.Second,
	}

	serverError := make(chan error, 1)
	go func() {
		logger.Info("tunescout starting", "address", cfg.ListenAddress, "providers", len(providers))
		serverError <- server.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverError:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-signalContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("tunescout stopped")
	}
}

func healthcheck() {
	cfg := config.FromEnv()
	_, port, err := net.SplitHostPort(cfg.ListenAddress)
	if err != nil || port == "" {
		fmt.Fprintln(os.Stderr, "invalid TUNESCOUT_LISTEN address")
		os.Exit(1)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "health endpoint returned %s\n", response.Status)
		os.Exit(1)
	}
}
