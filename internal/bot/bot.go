package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"cybarbot/internal/config"
	"cybarbot/internal/cyberark"
	"cybarbot/internal/whitelist"
)

type Bot struct {
	api        *tgbotapi.BotAPI
	dispatcher *Dispatcher
	notifier   *Notifier
	cfg        *config.Config
	fsm        *FSMManager
	ws         *WebhookServer
}

func NewBot(cfg *config.Config, auth *cyberark.AuthManager, wl *whitelist.Whitelist, version string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to init bot api: %w", err)
	}
	
	api.Debug = (cfg.LogLevel == "debug")
	slog.Info("authorized on account", "username", api.Self.UserName)

	fsm := NewFSMManager()
	
	var notifier *Notifier
	if cfg.NotifyEnabled {
		notifier = NewNotifier(api, auth, cfg.PollIntervalSeconds, cfg.NotifyOnRestart, cfg.NotifyTelegramIDs, cfg.NotifyGroupIDs)
	}

	cmdHandler := NewCommandHandler(api, auth, fsm, cfg.RequestsPageSize, version, notifier)
	dispatcher := NewDispatcher(api, wl, cmdHandler, cfg.WhitelistSilent, cfg.WhitelistRejectMsg)

	bot := &Bot{
		api:        api,
		dispatcher: dispatcher,
		notifier:   notifier,
		cfg:        cfg,
		fsm:        fsm,
	}

	return bot, nil
}

func (b *Bot) Start() error {
	if b.notifier != nil {
		b.notifier.Start()
	}

	if b.cfg.BotMode == "webhook" {
		wh, err := tgbotapi.NewWebhook(b.cfg.TelegramWebhookURL)
		if err != nil {
			return err
		}
		
		_, err = b.api.Request(wh)
		if err != nil {
			return err
		}
		
		info, err := b.api.GetWebhookInfo()
		if err != nil {
			return err
		}
		
		if info.LastErrorDate != 0 {
			slog.Warn("telegram webhook last error", "msg", info.LastErrorMessage)
		}

		ws, err := StartWebhookServer(
			b.api, 
			b.cfg.WebhookListenAddr, 
			b.cfg.WebhookSecretToken, 
			b.cfg.WebhookTLSCert, 
			b.cfg.WebhookTLSKey, 
			b.dispatcher,
		)
		if err != nil {
			return err
		}
		b.ws = ws
	} else {
		// Long polling
		_, err := b.api.Request(tgbotapi.DeleteWebhookConfig{}) // Ensure webhook is disabled
		if err != nil {
			slog.Warn("failed to delete webhook before starting long polling", "error", err)
		}
		
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates := b.api.GetUpdatesChan(u)
		
		slog.Info("started long polling")
		go func() {
			for update := range updates {
				b.dispatcher.ProcessUpdate(update)
			}
		}()
	}

	return nil
}

func (b *Bot) Stop(ctx context.Context) {
	if b.notifier != nil {
		b.notifier.Stop()
	}
	
	if b.ws != nil {
		b.ws.Shutdown(ctx)
	} else {
		b.api.StopReceivingUpdates()
	}
}
