package bot

import (
	"fmt"

	"cybarbot/internal/cyberark"
	"github.com/go-telegram/bot/models"
)

func buildPaginationKeyboard(page, totalPages int) models.InlineKeyboardMarkup {
	var row []models.InlineKeyboardButton

	if page > 1 {
		row = append(row, models.InlineKeyboardButton{Text: "◀ Prev", CallbackData: fmt.Sprintf("req_page_%d", page-1)})
	} else {
		row = append(row, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
	}

	row = append(row, models.InlineKeyboardButton{Text: fmt.Sprintf("Page %d/%d", page, totalPages), CallbackData: "noop"})

	if page < totalPages {
		row = append(row, models.InlineKeyboardButton{Text: "Next ▶", CallbackData: fmt.Sprintf("req_page_%d", page+1)})
	} else {
		row = append(row, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
	}

	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
}

func buildConfirmReasonKeyboard(reqID string) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Skip — No Reason", CallbackData: "confirm_skip_" + reqID},
				{Text: "✏️ Enter Reason", CallbackData: "confirm_reason_" + reqID},
			},
		},
	}
}

func buildBulkConfirmReasonKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Skip — No Reason", CallbackData: "bulk_confirm_skip"},
				{Text: "✏️ Enter Reason", CallbackData: "bulk_confirm_reason"},
			},
		},
	}
}

func buildBulkSelectKeyboard(requests []cyberark.IncomingRequest, selected map[string]bool, isReject bool, allSelected bool) models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	for _, req := range requests {
		check := "⬜"
		if selected[req.RequestID] {
			check = "✅"
		}
		
		text := fmt.Sprintf("%s %s (%s)", check, req.SafeName, req.AccountName)
		action := "toggle_" + req.RequestID
		
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: text, CallbackData: action},
		})
	}

	var allCheck string
	if allSelected {
		allCheck = "✅ Select All"
	} else {
		allCheck = "⬜ Select All"
	}

	rows = append(rows, []models.InlineKeyboardButton{
		{Text: allCheck, CallbackData: "toggle_all"},
	})

	var actionBtn models.InlineKeyboardButton
	if isReject {
		actionBtn = models.InlineKeyboardButton{Text: "❌ Reject Selected", CallbackData: "bulk_action_reject"}
	} else {
		actionBtn = models.InlineKeyboardButton{Text: "✅ Confirm Selected", CallbackData: "bulk_action_confirm"}
	}

	rows = append(rows, []models.InlineKeyboardButton{
		actionBtn,
		{Text: "🚫 Cancel", CallbackData: "cancel_bulk"},
	})

	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func buildNotificationKeyboard(reqID string) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Confirm", CallbackData: "notif_confirm_" + reqID},
				{Text: "❌ Reject", CallbackData: "notif_reject_" + reqID},
			},
			{
				{Text: "🔍 View Details", CallbackData: "notif_detail_" + reqID},
			},
		},
	}
}
