package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Ensure environment is clean
	os.Clearenv()
	
	// We need these minimum required vars to not fail early
	os.Setenv("CYBERARK_BASE_URL", "https://localhost")
	os.Setenv("CYBERARK_USERNAME", "admin")
	os.Setenv("CYBERARK_PASSWORD", "secret")
	os.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	os.Setenv("ALLOWED_TELEGRAM_IDS", "12345")
	os.Setenv("BOT_MODE", "longpoll")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SessionTTLMinutes != 20 {
		t.Errorf("expected default SessionTTLMinutes to be 20, got %d", cfg.SessionTTLMinutes)
	}

	if cfg.BotMode != "longpoll" {
		t.Errorf("expected default BotMode to be longpoll, got %s", cfg.BotMode)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	os.Clearenv()

	os.Setenv("CYBERARK_BASE_URL", "https://localhost")
	os.Setenv("CYBERARK_USERNAME", "admin")
	os.Setenv("CYBERARK_PASSWORD", "secret")
	os.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	os.Setenv("ALLOWED_TELEGRAM_IDS", "12345")

	// Overrides
	os.Setenv("SESSION_TTL_MINUTES", "30")
	os.Setenv("BOT_MODE", "webhook")
	os.Setenv("TELEGRAM_WEBHOOK_URL", "https://example.com/webhook")
	os.Setenv("WEBHOOK_SECRET_TOKEN", "12345678901234567890123456789012")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SessionTTLMinutes != 30 {
		t.Errorf("expected SessionTTLMinutes to be 30, got %d", cfg.SessionTTLMinutes)
	}

	if cfg.BotMode != "webhook" {
		t.Errorf("expected BotMode to be webhook, got %s", cfg.BotMode)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	os.Clearenv()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing CYBERARK_BASE_URL, got nil")
	}
}
