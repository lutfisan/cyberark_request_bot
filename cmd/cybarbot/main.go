package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cybarbot/internal/bot"
	"cybarbot/internal/config"
	"cybarbot/internal/cyberark"
	"cybarbot/internal/whitelist"
)

var Version = "1.2.0"

func setupLogger(cfg *config.Config) error {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	
	// Add file logging if configured
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	
	if cfg.LogAuditFile != "" {
		// Log Audit File would typically be a multi-writer, but for simplicity
		// we just output to stdout. Production apps could use lamberjack etc.
		// slog.NewJSONHandler(io.MultiWriter(os.Stdout, file))
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return nil
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}

	if err := setupLogger(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to setup logger: %v\n", err)
		os.Exit(1)
	}

	slog.Info("starting cybarbot", "version", Version)

	wl := whitelist.NewWhitelist(cfg.AllowedTelegramIDs, cfg.AllowedGroupIDs)

	cyberarkClient := cyberark.NewClient(cfg.CyberArkBaseURL, cfg.HTTPTimeoutSeconds, cfg.HTTPMaxRetries, cfg.CyberArkSkipTLSVerify)
	authManager := cyberark.NewAuthManager(cyberarkClient, cfg.CyberArkUsername, cfg.CyberArkPassword, cfg.SessionTTLMinutes)

	err = authManager.Logon()
	if err != nil {
		slog.Error("failed to logon to cyberark on startup", "error", err)
		// We could send a message to AdminTelegramID here, but we need the bot instance first.
		// For now we continue and rely on re-auth, or we can fail fast. 
		// PRD FR-01: MUST authenticate on startup.
		// We'll fail fast if it's completely unreachable.
		os.Exit(1)
	}

	authManager.StartRefreshLoop()

	tgBot, err := bot.NewBot(cfg, authManager, wl, Version)
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	// Create root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = tgBot.Start(ctx)
	if err != nil {
		slog.Error("failed to start bot", "error", err)
		os.Exit(1)
	}

	// SIGHUP handler for whitelist reload
	go func() {
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		for range hup {
			slog.Info("received SIGHUP, reloading whitelist from env")
			reloadCfg, err := config.LoadConfig()
			if err == nil {
				wl.Load(reloadCfg.AllowedTelegramIDs, reloadCfg.AllowedGroupIDs)
				slog.Info("whitelist reloaded")
			} else {
				slog.Error("failed to reload config on SIGHUP", "error", err)
			}
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	<-quit
	slog.Info("shutting down gracefully...")

	// Cancel context to stop go-telegram/bot polling/webhook
	cancel()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	tgBot.Stop(stopCtx)
	
	// Call Logoff on shutdown
	authManager.Logoff()

	slog.Info("shutdown complete")
}
