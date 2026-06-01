package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bulk operations extensions to CommandHandler

func (h *CommandHandler) handleConfirmAll(chatID int64) error {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		h.sendMessage(chatID, "✅ No pending requests to confirm.")
		return nil
	}

	ctx := h.fsm.SetState(chatID, StateBulkConfirmSelect)
	ctx.RequestIDs = make([]string, 0)
	
	// Create an empty selection map
	selected := make(map[string]bool)

	text := "Select requests to confirm:\n──────────────────────────────────────────"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = buildBulkSelectKeyboard(requests, selected, false, false)
	
	sentMsg, err := h.bot.Send(msg)
	if err == nil {
		ctx.MessageID = sentMsg.MessageID
	}
	return err
}

func (h *CommandHandler) handleRejectAll(chatID int64) error {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		h.sendMessage(chatID, "✅ No pending requests to reject.")
		return nil
	}

	ctx := h.fsm.SetState(chatID, StateBulkRejectSelect)
	ctx.RequestIDs = make([]string, 0)
	
	// Create an empty selection map
	selected := make(map[string]bool)

	text := "Select requests to reject:\n──────────────────────────────────────────"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = buildBulkSelectKeyboard(requests, selected, true, false)
	
	sentMsg, err := h.bot.Send(msg)
	if err == nil {
		ctx.MessageID = sentMsg.MessageID
	}
	return err
}

// Handle toggle selections in bulk mode
// In commands.go HandleCallback:
// else if strings.HasPrefix(data, "toggle_") {
//     reqID := strings.TrimPrefix(data, "toggle_")
//     h.handleBulkToggle(chatID, cb.Message.MessageID, reqID, false) // toggle specific
// } else if data == "toggle_all" {
//     h.handleBulkToggle(chatID, cb.Message.MessageID, "", true) // toggle all
// } else if data == "bulk_action_confirm" {
//     h.handleBulkActionInit(chatID, false)
// } else if data == "bulk_action_reject" {
//     h.handleBulkActionInit(chatID, true)
// } else if data == "cancel_bulk" {
//     h.fsm.Reset(chatID)
//     edit := tgbotapi.NewEditMessageText(chatID, cb.Message.MessageID, "🚫 Bulk operation cancelled.")
//     h.bot.Send(edit)
// }
