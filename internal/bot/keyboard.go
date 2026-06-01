package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"cybarbot/internal/cyberark"
)

func buildPaginationKeyboard(page, totalPages int) tgbotapi.InlineKeyboardMarkup {
	var row []tgbotapi.InlineKeyboardButton

	if page > 1 {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("◀ Prev", fmt.Sprintf("req_page_%d", page-1)))
	} else {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "noop"))
	}

	row = append(row, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Page %d/%d", page, totalPages), "noop"))

	if page < totalPages {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("Next ▶", fmt.Sprintf("req_page_%d", page+1)))
	} else {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "noop"))
	}

	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func buildConfirmReasonKeyboard(reqID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Skip — No Reason", "confirm_skip_"+reqID),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Enter Reason", "confirm_reason_"+reqID),
		),
	)
}

func buildBulkConfirmReasonKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Skip — No Reason", "bulk_confirm_skip"),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Enter Reason", "bulk_confirm_reason"),
		),
	)
}

func buildBulkSelectKeyboard(requests []cyberark.IncomingRequest, selected map[string]bool, isReject bool, allSelected bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, req := range requests {
		checkbox := "☐"
		if selected[req.RequestID] {
			checkbox = "☑"
		}
		text := fmt.Sprintf("%s %s — %s / %s", checkbox, req.RequestID, req.RequesterUserName, req.SafeName)
		action := fmt.Sprintf("toggle_%s", req.RequestID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(text, action),
		))
	}

	toggleAllText := "☑ Select All"
	if allSelected {
		toggleAllText = "☐ Deselect All"
	}

	var actionBtn tgbotapi.InlineKeyboardButton
	if isReject {
		actionBtn = tgbotapi.NewInlineKeyboardButtonData("❌ Reject Selected", "bulk_action_reject")
	} else {
		actionBtn = tgbotapi.NewInlineKeyboardButtonData("✅ Confirm Selected", "bulk_action_confirm")
	}

	controlRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(toggleAllText, "toggle_all"),
		actionBtn,
		tgbotapi.NewInlineKeyboardButtonData("🚫 Cancel", "cancel_bulk"),
	)
	rows = append(rows, controlRow)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildNotificationKeyboard(reqID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "notif_confirm_"+reqID),
			tgbotapi.NewInlineKeyboardButtonData("❌ Reject", "notif_reject_"+reqID),
			tgbotapi.NewInlineKeyboardButtonData("🔍 View Details", "notif_detail_"+reqID),
		),
	)
}
