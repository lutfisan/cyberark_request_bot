package bot

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"cybarbot/internal/whitelist"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

func PanicRecoveryMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("handler panic", "recover", r, "stack", string(debug.Stack()))
				
				chatID := getChatID(update)
				if chatID != 0 {
					b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: chatID,
						Text:   "🔴 An internal error occurred. Please try again.",
					})
				}
			}
		}()
		next(ctx, b, update)
	}
}

func LoggingMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		start := time.Now()
		userID := getUserID(update)
		username := getUsername(update)

		next(ctx, b, update)

		duration := time.Since(start).Milliseconds()
		slog.Info("handler executed",
			"telegram_user_id", userID,
			"telegram_username", username,
			"duration_ms", duration,
		)
	}
}

func WhitelistMiddleware(wl *whitelist.Whitelist, silent bool, rejectMsg string) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			userID := getUserID(update)
			chatID := getChatID(update)
			
			// For groups, check chat ID as well
			if update.Message != nil && (update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup") {
				if wl.IsAllowed(chatID) {
					userID = chatID
				}
			} else if update.CallbackQuery != nil {
				if msg := update.CallbackQuery.Message.Message; msg != nil && (msg.Chat.Type == "group" || msg.Chat.Type == "supergroup") {
					if wl.IsAllowed(chatID) {
						userID = chatID
					}
				}
			}

			if !wl.IsAllowed(userID) {
				slog.Warn("unauthorized access attempt", "sender_id", userID)
				if !silent && chatID != 0 {
					b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: chatID,
						Text:   rejectMsg,
					})
				}
				return
			}
			next(ctx, b, update)
		}
	}
}

func getChatID(update *models.Update) int64 {
	if update.Message != nil {
		return update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		if msg := update.CallbackQuery.Message.Message; msg != nil {
			return msg.Chat.ID
		} else if inaccMsg := update.CallbackQuery.Message.InaccessibleMessage; inaccMsg != nil {
			return inaccMsg.Chat.ID
		}
	}
	return 0
}

func getUserID(update *models.Update) int64 {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.ID
	} else if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	return 0
}

func getUsername(update *models.Update) string {
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.Username
	} else if update.CallbackQuery != nil {
		return update.CallbackQuery.From.Username
	}
	return ""
}
