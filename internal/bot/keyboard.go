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

const bulkPageSize = 20

func buildBulkSelectKeyboard(requests []cyberark.IncomingRequest, selected map[string]bool, isReject bool, allSelected bool, page int) models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	total := len(requests)
	totalPages := (total + bulkPageSize - 1) / bulkPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * bulkPageSize
	endIdx := startIdx + bulkPageSize
	if endIdx > total {
		endIdx = total
	}

	pageRequests := requests[startIdx:endIdx]

	for _, req := range pageRequests {
		check := "⬜"
		if selected[req.RequestID] {
			check = "✅"
		}
		
		_, addr := getAccountStr(req.AccountDetails, req.Operation)
		text := fmt.Sprintf("%s [%s] %s -> %s", check, req.RequestID, req.RequestorUserName, addr)
		action := "toggle_" + req.RequestID
		
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: text, CallbackData: action},
		})
	}

	// Pagination nav row (only if multiple pages)
	if totalPages > 1 {
		var navRow []models.InlineKeyboardButton
		if page > 1 {
			navRow = append(navRow, models.InlineKeyboardButton{Text: "◀ Prev", CallbackData: fmt.Sprintf("bulk_page_%d", page-1)})
		} else {
			navRow = append(navRow, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
		}
		selectedCount := len(selected)
		navRow = append(navRow, models.InlineKeyboardButton{Text: fmt.Sprintf("Page %d/%d (%d sel)", page, totalPages, selectedCount), CallbackData: "noop"})
		if page < totalPages {
			navRow = append(navRow, models.InlineKeyboardButton{Text: "Next ▶", CallbackData: fmt.Sprintf("bulk_page_%d", page+1)})
		} else {
			navRow = append(navRow, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
		}
		rows = append(rows, navRow)
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

const selectPageSize = 20

func buildRequestSelectionKeyboard(requests []cyberark.IncomingRequest, actionPrefix string, page int) models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	total := len(requests)
	totalPages := (total + selectPageSize - 1) / selectPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * selectPageSize
	endIdx := startIdx + selectPageSize
	if endIdx > total {
		endIdx = total
	}

	pageRequests := requests[startIdx:endIdx]

	for _, req := range pageRequests {
		_, addr := getAccountStr(req.AccountDetails, req.Operation)
		text := fmt.Sprintf("[%s] %s -> %s", req.RequestID, getRequester(req.RequestorUserName), addr)
		
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: text, CallbackData: actionPrefix + req.RequestID},
		})
	}

	// Pagination nav row (only if multiple pages)
	if totalPages > 1 {
		var navRow []models.InlineKeyboardButton
		if page > 1 {
			navRow = append(navRow, models.InlineKeyboardButton{Text: "◀ Prev", CallbackData: fmt.Sprintf("sel_page_%d_%s", page-1, actionPrefix)})
		} else {
			navRow = append(navRow, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
		}
		navRow = append(navRow, models.InlineKeyboardButton{Text: fmt.Sprintf("Page %d/%d", page, totalPages), CallbackData: "noop"})
		if page < totalPages {
			navRow = append(navRow, models.InlineKeyboardButton{Text: "Next ▶", CallbackData: fmt.Sprintf("sel_page_%d_%s", page+1, actionPrefix)})
		} else {
			navRow = append(navRow, models.InlineKeyboardButton{Text: " ", CallbackData: "noop"})
		}
		rows = append(rows, navRow)
	}

	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
