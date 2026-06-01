package bot

import (
	"log/slog"
	"runtime/debug"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func AuditLog(action string, reqID string, bulk bool, actorID int64, actorUsername string, reason string) {
	slog.Info("AUDIT",
		"action", action,
		"request_id", reqID,
		"bulk", bulk,
		"actor_telegram_id", actorID,
		"actor_username", actorUsername,
		"reason", reason,
	)
}

func PanicRecovery(bot *tgbotapi.BotAPI, chatID int64) {
	if r := recover(); r != nil {
		slog.Error("handler panic", "recover", r, "stack", string(debug.Stack()))
		if bot != nil && chatID != 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "🔴 An internal error occurred. Please try again."))
		}
	}
}

func WithLogging(handler func() error, component string, update tgbotapi.Update) func() {
	return func() {
		start := time.Now()
		var userID int64
		var username string

		if update.Message != nil {
			userID = update.Message.From.ID
			username = update.Message.From.UserName
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
			username = update.CallbackQuery.From.UserName
		}

		err := handler()

		duration := time.Since(start).Milliseconds()
		if err != nil {
			slog.Error("handler error",
				"component", component,
				"telegram_user_id", userID,
				"telegram_username", username,
				"error", err,
				"duration_ms", duration,
			)
		} else {
			slog.Info("handler success",
				"component", component,
				"telegram_user_id", userID,
				"telegram_username", username,
				"duration_ms", duration,
			)
		}
	}
}
