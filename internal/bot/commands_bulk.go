package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"cybarbot/internal/cyberark"
)

// Bulk operations extensions to CommandHandler

func (h *CommandHandler) ConfirmAllHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleConfirmAll(ctx, b, chatID)
}

func (h *CommandHandler) RejectAllHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleRejectAll(ctx, b, chatID)
}

func (h *CommandHandler) handleConfirmAll(ctx context.Context, b *bot.Bot, chatID int64) error {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		return h.sendMessage(ctx, b, chatID, "✅ No pending requests to confirm.")
	}

	fsmCtx := h.fsm.SetState(chatID, StateBulkConfirmSelect)
	fsmCtx.RequestIDs = make([]string, 0)
	
	selected := make(map[string]bool)

	text := "Select requests to confirm:\n──────────────────────────────────────────"
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: buildBulkSelectKeyboard(requests, selected, false, false),
	})
	
	if err == nil {
		fsmCtx.MessageID = msg.ID
	}
	return err
}

func (h *CommandHandler) handleRejectAll(ctx context.Context, b *bot.Bot, chatID int64) error {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		return h.sendMessage(ctx, b, chatID, "✅ No pending requests to reject.")
	}

	fsmCtx := h.fsm.SetState(chatID, StateBulkRejectSelect)
	fsmCtx.RequestIDs = make([]string, 0)
	
	selected := make(map[string]bool)

	text := "Select requests to reject:\n──────────────────────────────────────────"
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: buildBulkSelectKeyboard(requests, selected, true, false),
	})
	
	if err == nil {
		fsmCtx.MessageID = msg.ID
	}
	return err
}

func (h *CommandHandler) handleBulkCallback(ctx context.Context, b *bot.Bot, update *models.Update) error {
	cb := update.CallbackQuery
	
	var chatID int64
	var messageID int
	if msg := cb.Message.Message; msg != nil {
		chatID = msg.Chat.ID
		messageID = msg.ID
	} else if msg := cb.Message.InaccessibleMessage; msg != nil {
		chatID = msg.Chat.ID
		messageID = msg.MessageID
	}

	data := cb.Data

	if strings.HasPrefix(data, "toggle_") {
		reqID := strings.TrimPrefix(data, "toggle_")
		if reqID == "all" {
			return h.handleBulkToggle(ctx, b, chatID, messageID, "", true)
		}
		return h.handleBulkToggle(ctx, b, chatID, messageID, reqID, false)
	} else if data == "bulk_action_confirm" {
		return h.handleBulkActionInit(ctx, b, chatID, false)
	} else if data == "bulk_action_reject" {
		return h.handleBulkActionInit(ctx, b, chatID, true)
	} else if data == "cancel_bulk" {
		h.fsm.Reset(chatID)
		_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      "🚫 Bulk operation cancelled.",
		})
		return err
	} else if data == "bulk_confirm_skip" {
		// execute bulk confirm
		return h.executeBulkAction(ctx, b, chatID, cb.From.Username, cb.From.ID, "", false)
	} else if data == "bulk_confirm_reason" {
		fsmCtx := h.fsm.GetContext(chatID)
		if fsmCtx.State == StateBulkConfirmSelect {
			h.fsm.SetState(chatID, StateWaitingConfirmReason)
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Please type your bulk confirmation reason:",
				ReplyMarkup: &models.ForceReply{
					ForceReply: true,
				},
			})
			return err
		}
		return nil
	}

	return nil
}

func (h *CommandHandler) handleBulkToggle(ctx context.Context, b *bot.Bot, chatID int64, messageID int, reqID string, toggleAll bool) error {
	fsmCtx := h.fsm.GetContext(chatID)
	if fsmCtx.State != StateBulkConfirmSelect && fsmCtx.State != StateBulkRejectSelect {
		return nil
	}

	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	selected := make(map[string]bool)
	for _, id := range fsmCtx.RequestIDs {
		selected[id] = true
	}

	allSelected := true
	for _, req := range requests {
		if !selected[req.RequestID] {
			allSelected = false
			break
		}
	}

	if toggleAll {
		if allSelected {
			// deselect all
			selected = make(map[string]bool)
			fsmCtx.RequestIDs = []string{}
			allSelected = false
		} else {
			// select all
			fsmCtx.RequestIDs = []string{}
			for _, req := range requests {
				selected[req.RequestID] = true
				fsmCtx.RequestIDs = append(fsmCtx.RequestIDs, req.RequestID)
			}
			allSelected = true
		}
	} else {
		// toggle specific
		if selected[reqID] {
			delete(selected, reqID)
			var newIDs []string
			for _, id := range fsmCtx.RequestIDs {
				if id != reqID {
					newIDs = append(newIDs, id)
				}
			}
			fsmCtx.RequestIDs = newIDs
		} else {
			selected[reqID] = true
			fsmCtx.RequestIDs = append(fsmCtx.RequestIDs, reqID)
		}

		// Re-evaluate allSelected
		allSelected = true
		for _, req := range requests {
			if !selected[req.RequestID] {
				allSelected = false
				break
			}
		}
	}

	isReject := fsmCtx.State == StateBulkRejectSelect

	_, err = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: buildBulkSelectKeyboard(requests, selected, isReject, allSelected),
	})

	return err
}

func (h *CommandHandler) handleBulkActionInit(ctx context.Context, b *bot.Bot, chatID int64, isReject bool) error {
	fsmCtx := h.fsm.GetContext(chatID)
	slog.Info("handleBulkActionInit",
		"chatID", chatID,
		"isReject", isReject,
		"state", fsmCtx.State,
		"requestIDs", fsmCtx.RequestIDs,
		"requestIDs_count", len(fsmCtx.RequestIDs),
	)
	if len(fsmCtx.RequestIDs) == 0 {
		return h.sendMessage(ctx, b, chatID, "⚠️ No requests selected.")
	}

	if isReject {
		newCtx := h.fsm.SetState(chatID, StateWaitingRejectReason)
		slog.Info("handleBulkActionInit: state set to WaitingRejectReason",
			"newState", newCtx.State,
			"requestIDs_after", newCtx.RequestIDs,
		)
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "✏️ Please provide a rejection reason for the selected requests:",
			ReplyMarkup: &models.ForceReply{
				ForceReply: true,
			},
		})
		return err
	} else {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        "⚠️ Confirm selected requests?\n─────────────────────────\nAdd a reason?",
			ReplyMarkup: buildBulkConfirmReasonKeyboard(),
		})
		return err
	}
}

func (h *CommandHandler) executeBulkAction(ctx context.Context, b *bot.Bot, chatID int64, username string, userID int64, reason string, isReject bool) error {
	fsmCtx := h.fsm.GetContext(chatID)
	slog.Info("executeBulkAction",
		"chatID", chatID,
		"isReject", isReject,
		"reason", reason,
		"state", fsmCtx.State,
		"requestIDs", fsmCtx.RequestIDs,
		"requestIDs_count", len(fsmCtx.RequestIDs),
	)
	if len(fsmCtx.RequestIDs) == 0 {
		return h.sendMessage(ctx, b, chatID, "⚠️ No requests selected for bulk action.")
	}

	finalReason := ""
	if reason == "" {
		finalReason = "[CybArBot] Bulk Approved"
	} else {
		finalReason = "[CybArBot] " + reason
	}

	var err error
	var resp *cyberark.BulkActionResponse
	if isReject {
		resp, err = h.auth.BulkRejectRequests(fsmCtx.RequestIDs, finalReason)
	} else {
		resp, err = h.auth.BulkConfirmRequests(fsmCtx.RequestIDs, finalReason)
	}

	if resp == nil && err != nil {
		return err
	}

	actionStr := "CONFIRM"
	if isReject {
		actionStr = "REJECT"
	}

	for _, reqID := range resp.Successful {
		AuditLog(actionStr, reqID, true, userID, username, finalReason)
		if h.notifier != nil {
			statusStr := "✅ CONFIRMED"
			if isReject {
				statusStr = "❌ REJECTED"
			}
			h.notifier.UpdateNotificationMessage(ctx, reqID, fmt.Sprintf("%s by @%s at %s (Bulk)", statusStr, username, time.Now().In(tzLocation).Format("2006-01-02 15:04:05 MST")))
		}
	}

	for _, failed := range resp.Failed {
		AuditLog(actionStr, failed.RequestID, false, userID, username, failed.Error)
	}

	statusStr := "Confirmed"
	if isReject {
		statusStr = "Rejected"
	}

	msgText := fmt.Sprintf("✅ Successfully %s %d requests.", statusStr, len(resp.Successful))
	if len(resp.Failed) > 0 {
		msgText += fmt.Sprintf("\n❌ Failed to %s %d requests.", strings.ToLower(statusStr), len(resp.Failed))
	}

	h.sendMessage(ctx, b, chatID, msgText)
	h.fsm.Reset(chatID)
	return nil
}


