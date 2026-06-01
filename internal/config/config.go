package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	CyberArkBaseURL         string
	CyberArkUsername        string
	CyberArkPassword        string
	CyberArkSkipTLSVerify   bool
	SessionTTLMinutes       int

	TelegramBotToken        string
	AdminTelegramID         int64
	BotMode                 string

	TelegramWebhookURL      string
	WebhookListenAddr       string
	WebhookSecretToken      string
	WebhookTLSCert          string
	WebhookTLSKey           string

	AllowedTelegramIDs      []int64
	AllowedGroupIDs         []int64
	WhitelistSilent         bool
	WhitelistRejectMsg      string

	RequestsPageSize        int
	HTTPTimeoutSeconds      int
	HTTPMaxRetries          int

	NotifyEnabled           bool
	PollIntervalSeconds     int
	NotifyOnRestart         bool
	NotifyTelegramIDs       []int64
	NotifyGroupIDs          []int64

	LogLevel                string
	LogAuditFile            string
}

func LoadConfig() (*Config, error) {
	// Try loading .env file, ignore error if it doesn't exist as env vars might be set in environment
	_ = godotenv.Load()

	cfg := &Config{}
	var errs []string

	cfg.CyberArkBaseURL = os.Getenv("CYBERARK_BASE_URL")
	if cfg.CyberArkBaseURL == "" {
		errs = append(errs, "missing required env variable CYBERARK_BASE_URL")
	} else if !strings.HasPrefix(cfg.CyberArkBaseURL, "https://") {
		errs = append(errs, "CYBERARK_BASE_URL must be a valid HTTPS URL")
	}

	cfg.CyberArkUsername = os.Getenv("CYBERARK_USERNAME")
	if cfg.CyberArkUsername == "" {
		errs = append(errs, "missing required env variable CYBERARK_USERNAME")
	}
	cfg.CyberArkPassword = os.Getenv("CYBERARK_PASSWORD")
	if cfg.CyberArkPassword == "" {
		errs = append(errs, "missing required env variable CYBERARK_PASSWORD")
	}

	cfg.CyberArkSkipTLSVerify = parseBool(os.Getenv("CYBERARK_SKIP_TLS_VERIFY"), false)
	cfg.SessionTTLMinutes = parseInt(os.Getenv("SESSION_TTL_MINUTES"), 20)

	cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if cfg.TelegramBotToken == "" {
		errs = append(errs, "missing required env variable TELEGRAM_BOT_TOKEN")
	}

	cfg.AdminTelegramID = parseInt64(os.Getenv("ADMIN_TELEGRAM_ID"), 0)

	cfg.BotMode = os.Getenv("BOT_MODE")
	if cfg.BotMode != "longpoll" && cfg.BotMode != "webhook" {
		errs = append(errs, "BOT_MODE must be one of: longpoll, webhook")
	}

	if cfg.BotMode == "webhook" {
		cfg.TelegramWebhookURL = os.Getenv("TELEGRAM_WEBHOOK_URL")
		if cfg.TelegramWebhookURL == "" {
			errs = append(errs, "TELEGRAM_WEBHOOK_URL is required when BOT_MODE=webhook")
		}
		cfg.WebhookListenAddr = os.Getenv("WEBHOOK_LISTEN_ADDR")
		if cfg.WebhookListenAddr == "" {
			cfg.WebhookListenAddr = ":8443"
		}
		cfg.WebhookSecretToken = os.Getenv("WEBHOOK_SECRET_TOKEN")
		if len(cfg.WebhookSecretToken) < 32 {
			errs = append(errs, "WEBHOOK_SECRET_TOKEN must be at least 32 characters when BOT_MODE=webhook")
		}
		cfg.WebhookTLSCert = os.Getenv("WEBHOOK_TLS_CERT")
		cfg.WebhookTLSKey = os.Getenv("WEBHOOK_TLS_KEY")
	}

	cfg.AllowedTelegramIDs = parseInt64Slice(os.Getenv("ALLOWED_TELEGRAM_IDS"))
	cfg.AllowedGroupIDs = parseInt64Slice(os.Getenv("ALLOWED_GROUP_IDS"))
	if len(cfg.AllowedTelegramIDs) == 0 && len(cfg.AllowedGroupIDs) == 0 {
		errs = append(errs, "ALLOWED_TELEGRAM_IDS and ALLOWED_GROUP_IDS cannot both be empty")
	}

	cfg.WhitelistSilent = parseBool(os.Getenv("WHITELIST_SILENT"), true)
	cfg.WhitelistRejectMsg = os.Getenv("WHITELIST_REJECT_MSG")
	if cfg.WhitelistRejectMsg == "" {
		cfg.WhitelistRejectMsg = "⛔ You are not authorised to use this bot."
	}

	cfg.RequestsPageSize = parseInt(os.Getenv("REQUESTS_PAGE_SIZE"), 10)
	cfg.HTTPTimeoutSeconds = parseInt(os.Getenv("HTTP_TIMEOUT_SECONDS"), 30)
	cfg.HTTPMaxRetries = parseInt(os.Getenv("HTTP_MAX_RETRIES"), 3)

	cfg.NotifyEnabled = parseBool(os.Getenv("NOTIFY_ENABLED"), true)
	cfg.PollIntervalSeconds = parseInt(os.Getenv("POLL_INTERVAL_SECONDS"), 60)
	if cfg.PollIntervalSeconds < 60 || cfg.PollIntervalSeconds > 180 {
		errs = append(errs, fmt.Sprintf("POLL_INTERVAL_SECONDS must be between 60 and 180 (got: %d)", cfg.PollIntervalSeconds))
	}
	cfg.NotifyOnRestart = parseBool(os.Getenv("NOTIFY_ON_RESTART"), false)

	notifyUsersStr := os.Getenv("NOTIFY_TELEGRAM_IDS")
	if notifyUsersStr == "" {
		cfg.NotifyTelegramIDs = cfg.AllowedTelegramIDs
		slog.Info("NOTIFY_TELEGRAM_IDS is empty, defaulting to ALLOWED_TELEGRAM_IDS", "count", len(cfg.NotifyTelegramIDs))
	} else {
		cfg.NotifyTelegramIDs = parseInt64Slice(notifyUsersStr)
	}

	notifyGroupsStr := os.Getenv("NOTIFY_GROUP_IDS")
	if notifyGroupsStr == "" {
		cfg.NotifyGroupIDs = cfg.AllowedGroupIDs
		slog.Info("NOTIFY_GROUP_IDS is empty, defaulting to ALLOWED_GROUP_IDS", "count", len(cfg.NotifyGroupIDs))
	} else {
		cfg.NotifyGroupIDs = parseInt64Slice(notifyGroupsStr)
	}

	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	cfg.LogAuditFile = os.Getenv("LOG_AUDIT_FILE")

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors:\n%s", strings.Join(errs, "\n"))
	}

	return cfg, nil
}

func parseBool(val string, def bool) bool {
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return b
}

func parseInt(val string, def int) int {
	if val == "" {
		return def
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return i
}

func parseInt64(val string, def int64) int64 {
	if val == "" {
		return def
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return def
	}
	return i
}

func parseInt64Slice(val string) []int64 {
	if val == "" {
		return []int64{}
	}
	parts := strings.Split(val, ",")
	var result []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i, err := strconv.ParseInt(p, 10, 64); err == nil {
			result = append(result, i)
		}
	}
	return result
}
