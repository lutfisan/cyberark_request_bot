package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"cybarbot/internal/config"
	"cybarbot/internal/cyberark"
	"cybarbot/internal/whitelist"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Bot struct {
	api      *bot.Bot
	notifier *Notifier
	cfg      *config.Config
	fsm      *FSMManager
	ws       *WebhookServer
}

func NewBot(cfg *config.Config, auth *cyberark.AuthManager, wl *whitelist.Whitelist, version string) (*Bot, error) {
	fsm := NewFSMManager()
	
	// We need cmdHandler for DefaultHandler, but cmdHandler needs notifier. Let's pre-declare them.
	var cmdHandler *CommandHandler
	
	// 1. Setup Options
	opts := []bot.Option{
		bot.WithNotAsyncHandlers(),
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if cmdHandler != nil {
				cmdHandler.DefaultHandler(ctx, b, update)
			}
		}),
		bot.WithErrorsHandler(func(err error) {
			slog.Error("telegram api error", "error", err)
		}),
		bot.WithMiddlewares(
			PanicRecoveryMiddleware,
			LoggingMiddleware,
			WhitelistMiddleware(wl, cfg.WhitelistSilent, cfg.WhitelistRejectMsg),
		),
	}

	// 2. Init Bot instance
	b, err := bot.New(cfg.TelegramBotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to init bot: %w", err)
	}

	// 3. Setup Handlers
	var notifier *Notifier
	if cfg.NotifyEnabled {
		notifier = NewNotifier(b, auth, cfg.PollIntervalSeconds, cfg.NotifyOnRestart, cfg.NotifyTelegramIDs, cfg.NotifyGroupIDs)
	}
	cmdHandler = NewCommandHandler(auth, fsm, cfg.RequestsPageSize, version, notifier)

	// Routing setup
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && (update.Message.Text == "/start" || update.Message.Text == "/help") },
		cmdHandler.HelpHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/status" },
		cmdHandler.StatusHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/notify_status" },
		cmdHandler.NotifyStatusHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/requests" },
		cmdHandler.RequestsHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/confirmall" },
		cmdHandler.ConfirmAllHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/rejectall" },
		cmdHandler.RejectAllHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && (update.Message.Text == "/search" || strings.HasPrefix(update.Message.Text, "/search ")) },
		cmdHandler.SearchHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.Message != nil && update.Message.Text == "/cancel" },
		cmdHandler.CancelHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil && (update.Message.Text == "/detail" || strings.HasPrefix(update.Message.Text, "/detail "))
		},
		cmdHandler.DetailHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil && (update.Message.Text == "/confirm" || strings.HasPrefix(update.Message.Text, "/confirm "))
		},
		cmdHandler.ConfirmHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil && (update.Message.Text == "/reject" || strings.HasPrefix(update.Message.Text, "/reject "))
		},
		cmdHandler.RejectHandler,
	)
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool { return update.CallbackQuery != nil },
		cmdHandler.CallbackHandler,
	)

	// In `go-telegram/bot`, the DefaultHandler is used for everything else.
	// We need to re-assign it. Since `b.DefaultHandler` is private, we should actually pass it in `bot.New`
	// but we couldn't because it depends on `cmdHandler` which requires `b` to send messages... actually `cmdHandler` doesn't hold `b` anymore!
	// Yes it does not! So we can set it.
	// Let's create `b` with all options together by doing it properly.

	return &Bot{
		api:      b,
		notifier: notifier,
		cfg:      cfg,
		fsm:      fsm,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	if b.notifier != nil {
		b.notifier.Start()
	}

	// Set Bot Commands for hamburger menu
	_, err := b.api.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "help", Description: "Show available commands"},
			{Command: "requests", Description: "List all pending requests"},
			{Command: "detail", Description: "View single request details"},
			{Command: "confirm", Description: "Confirm a single request"},
			{Command: "reject", Description: "Reject a single request"},
			{Command: "confirmall", Description: "Bulk confirm multiple requests"},
			{Command: "rejectall", Description: "Bulk reject multiple requests"},
			{Command: "search", Description: "Search requests by Requester/Address"},
			{Command: "status", Description: "Show session info & bot health"},
			{Command: "notify_status", Description: "Notification watcher health"},
			{Command: "cancel", Description: "Abort current operation"},
		},
	})
	if err != nil {
		slog.Warn("failed to set bot commands", "error", err)
	}

	if b.cfg.BotMode == "webhook" {
		ws, err := StartWebhookServer(
			b.api,
			b.cfg.WebhookListenAddr,
			b.cfg.WebhookTLSCert,
			b.cfg.WebhookTLSKey,
		)
		if err != nil {
			return err
		}
		b.ws = ws
		
		_, err = b.api.SetWebhook(ctx, &bot.SetWebhookParams{
			URL:         b.cfg.TelegramWebhookURL,
			SecretToken: b.cfg.WebhookSecretToken,
		})
		if err != nil {
			return err
		}
		
		slog.Info("started telegram bot in webhook mode")
		go b.api.StartWebhook(ctx)
	} else {
		_, err := b.api.DeleteWebhook(ctx, &bot.DeleteWebhookParams{DropPendingUpdates: true})
		if err != nil {
			slog.Warn("failed to delete webhook", "error", err)
		}
		
		slog.Info("started long polling")
		go b.api.Start(ctx)
	}

	return nil
}

func (b *Bot) Stop(ctx context.Context) {
	if b.notifier != nil {
		b.notifier.Stop()
	}
	
	if b.ws != nil {
		b.ws.Shutdown(ctx)
	}
}
