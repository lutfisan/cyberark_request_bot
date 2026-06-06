package bot

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"cybarbot/internal/cyberark"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type CommandHandler struct {
	auth       *cyberark.AuthManager
	fsm        *FSMManager
	pageSize   int
	botVersion string
	startTime  time.Time
	notifier   *Notifier // reference for status command
}

func NewCommandHandler(auth *cyberark.AuthManager, fsm *FSMManager, pageSize int, version string, notifier *Notifier) *CommandHandler {
	return &CommandHandler{
		auth:       auth,
		fsm:        fsm,
		pageSize:   pageSize,
		botVersion: version,
		startTime:  time.Now(),
		notifier:   notifier,
	}
}

// DefaultHandler handles unknown commands or FSM text states
func (h *CommandHandler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}
	
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	if strings.HasPrefix(text, "/") {
		h.sendMessage(ctx, b, chatID, "Unknown command. Use /help for a list of commands.")
		h.fsm.Reset(chatID)
		return
	}

	fsmCtx := h.fsm.GetContext(chatID)
	slog.Info("DefaultHandler: FSM state check",
		"chatID", chatID,
		"state", fsmCtx.State,
		"requestID", fsmCtx.RequestID,
		"requestIDs_count", len(fsmCtx.RequestIDs),
		"text", text,
	)
	if fsmCtx.State == StateIdle {
		return // Ignore random text messages
	}

	var err error
	username := update.Message.From.Username
	userID := update.Message.From.ID

	switch fsmCtx.State {
	case StateWaitingConfirmReason:
		if len(fsmCtx.RequestIDs) > 0 {
			err = h.executeBulkAction(ctx, b, chatID, username, userID, text, false)
		} else {
			err = h.executeConfirm(ctx, b, chatID, fsmCtx.RequestID, username, userID, text)
		}
	case StateWaitingRejectReason:
		if len(fsmCtx.RequestIDs) > 0 {
			err = h.executeBulkAction(ctx, b, chatID, username, userID, text, true)
		} else {
			err = h.executeReject(ctx, b, chatID, fsmCtx.RequestID, username, userID, text)
		}
	case StateBulkConfirmSelect, StateBulkRejectSelect:
		// Ignoring text while in bulk select mode
		slog.Info("DefaultHandler: ignoring text in bulk select mode", "state", fsmCtx.State)
	default:
		slog.Info("DefaultHandler: unhandled FSM state", "state", fsmCtx.State)
	}

	if err != nil {
		slog.Error("state execution failed", "state", fsmCtx.State, "error", err)
		h.sendMessage(ctx, b, chatID, fmt.Sprintf("🔴 Error processing action: %v", err))
	}
	h.fsm.Reset(chatID)
}

func (h *CommandHandler) HelpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleHelp(ctx, b, chatID)
}

func (h *CommandHandler) StatusHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleStatus(ctx, b, chatID)
}

func (h *CommandHandler) NotifyStatusHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleNotifyStatus(ctx, b, chatID)
}

func (h *CommandHandler) RequestsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.handleRequests(ctx, b, chatID, 1)
}

func (h *CommandHandler) sendRequestSelection(ctx context.Context, b *bot.Bot, chatID int64, text, actionPrefix string) {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		h.sendMessage(ctx, b, chatID, "🔴 Failed to fetch requests: "+err.Error())
		return
	}
	if len(requests) == 0 {
		h.sendMessage(ctx, b, chatID, "✅ No pending requests available.")
		return
	}

	kb := buildRequestSelectionKeyboard(requests, actionPrefix, 1)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: kb,
	})
	if err != nil {
		h.sendMessage(ctx, b, chatID, "🔴 Failed to send selection menu.")
	}
}

func (h *CommandHandler) DetailHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	
	args := getCommandArgs(update.Message.Text)
	if args == "" {
		h.sendRequestSelection(ctx, b, chatID, "🔍 Select a request to view details:", "notif_detail_")
		return
	}
	h.handleDetail(ctx, b, chatID, args)
}

func (h *CommandHandler) ConfirmHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	
	args := getCommandArgs(update.Message.Text)
	if args == "" {
		h.sendRequestSelection(ctx, b, chatID, "✅ Select a request to confirm:", "notif_confirm_")
		return
	}
	h.handleConfirm(ctx, b, chatID, args)
}

func (h *CommandHandler) RejectHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	
	args := getCommandArgs(update.Message.Text)
	if args == "" {
		h.sendRequestSelection(ctx, b, chatID, "❌ Select a request to reject:", "notif_reject_")
		return
	}
	h.handleReject(ctx, b, chatID, args)
}

func (h *CommandHandler) CancelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	h.fsm.Reset(chatID)
	h.sendMessage(ctx, b, chatID, "✅ Operation cancelled. State reset to IDLE.")
}

func (h *CommandHandler) CallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery
	var chatID int64
	if msg := cb.Message.Message; msg != nil {
		chatID = msg.Chat.ID
	} else if msg := cb.Message.InaccessibleMessage; msg != nil {
		chatID = msg.Chat.ID
	}
	data := cb.Data

	// Acknowledge callback immediately
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	var err error
	if strings.HasPrefix(data, "req_page_") {
		var page int
		fmt.Sscanf(data, "req_page_%d", &page)
		err = h.handleRequests(ctx, b, chatID, page)
	} else if strings.HasPrefix(data, "confirm_skip_") {
		reqID := strings.TrimPrefix(data, "confirm_skip_")
		err = h.executeConfirm(ctx, b, chatID, reqID, cb.From.Username, cb.From.ID, "")
	} else if strings.HasPrefix(data, "confirm_reason_") {
		reqID := strings.TrimPrefix(data, "confirm_reason_")
		fsmCtx := h.fsm.SetState(chatID, StateWaitingConfirmReason)
		fsmCtx.RequestID = reqID
		
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please type your reason:",
			ReplyMarkup: &models.ForceReply{
				ForceReply: true,
			},
		})
	} else if strings.HasPrefix(data, "notif_confirm_") {
		reqID := strings.TrimPrefix(data, "notif_confirm_")
		err = h.handleConfirm(ctx, b, chatID, reqID)
	} else if strings.HasPrefix(data, "notif_reject_") {
		reqID := strings.TrimPrefix(data, "notif_reject_")
		err = h.handleReject(ctx, b, chatID, reqID)
	} else if strings.HasPrefix(data, "notif_detail_") {
		reqID := strings.TrimPrefix(data, "notif_detail_")
		err = h.handleDetail(ctx, b, chatID, reqID)
	} else if strings.HasPrefix(data, "sel_page_") {
		// format: sel_page_<page>_<actionPrefix>
		parts := strings.SplitN(strings.TrimPrefix(data, "sel_page_"), "_", 2)
		if len(parts) == 2 {
			var page int
			fmt.Sscanf(parts[0], "%d", &page)
			actionPrefix := parts[1]
			
			requests, errReq := h.auth.GetIncomingRequests()
			if errReq == nil {
				_, err = b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
					ChatID:      chatID,
					MessageID:   cb.Message.Message.ID,
					ReplyMarkup: buildRequestSelectionKeyboard(requests, actionPrefix, page),
				})
			} else {
				err = errReq
			}
		}
	} else if strings.HasPrefix(data, "toggle_") || strings.HasPrefix(data, "bulk_page_") || data == "bulk_action_confirm" || data == "bulk_action_reject" || data == "cancel_bulk" || data == "bulk_confirm_skip" || data == "bulk_confirm_reason" {
		// Bulk handlers will be moved to commands_bulk.go but called from here
		err = h.handleBulkCallback(ctx, b, update)
	} else if data == "noop" {
		// Do nothing
	} else {
		slog.Warn("unhandled callback data", "data", data)
	}

	if err != nil {
		slog.Error("callback execution failed", "data", data, "error", err)
		h.sendMessage(ctx, b, chatID, fmt.Sprintf("🔴 Error processing action: %v", err))
	}
}

// Helpers
func getCommandArgs(text string) string {
	parts := strings.SplitN(text, " ", 2)
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func (h *CommandHandler) sendMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) error {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func (h *CommandHandler) handleHelp(ctx context.Context, b *bot.Bot, chatID int64) error {
	helpText := `🤖 <b>CybArBot Command Reference</b>

/start - Welcome message and command list
/help - Full command reference
/status - Bot health, session status, active delivery mode
/notify_status - Notification watcher health
/requests - List all pending incoming requests (paginated)
/detail &lt;id&gt; - Show full confirmation details for a request
/confirm &lt;id&gt; - Confirm a single request (optional reason)
/reject &lt;id&gt; - Reject a single request (mandatory reason)
/confirmall - Bulk confirm multiple requests
/rejectall - Bulk reject multiple requests
/cancel - Abort any active multi-step operation
`
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      helpText,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		slog.Error("failed to send help message", "error", err)
	}
	return err
}

func (h *CommandHandler) handleStatus(ctx context.Context, b *bot.Bot, chatID int64) error {
	uptime := time.Since(h.startTime)
	sessionStatus := "✅ Active"
	if h.auth.Token() == "" {
		sessionStatus = "❌ Inactive"
	}

	statusText := fmt.Sprintf(`🤖 CybArBot v%s
──────────────────────────────────────
Uptime           : %s
CyberArk Session : %s
`, h.botVersion, uptime.Round(time.Second), sessionStatus)

	return h.sendMessage(ctx, b, chatID, statusText)
}

func (h *CommandHandler) handleNotifyStatus(ctx context.Context, b *bot.Bot, chatID int64) error {
	if h.notifier == nil {
		return h.sendMessage(ctx, b, chatID, "Notification watcher is disabled.")
	}
	
	stats := h.notifier.GetStats()
	
	statusText := fmt.Sprintf(`🔔 Notification Watcher Status
────────────────────────────────────
Enabled        : ✅ Yes
Poll Interval  : %d seconds
Last Poll      : %s
Last Result    : %d seen, %d new, %d stale-edited
Cache Size     : %d requests
`, stats.PollInterval, stats.LastPoll.In(tzLocation).Format("2006-01-02 15:04:05 MST"), stats.LastSeen, stats.LastNew, stats.LastStaleEdited, stats.CacheSize)

	return h.sendMessage(ctx, b, chatID, statusText)
}

func (h *CommandHandler) handleRequests(ctx context.Context, b *bot.Bot, chatID int64, page int) error {
	requests, err := h.auth.GetIncomingRequests()
	if err != nil {
		return err
	}

	total := len(requests)
	totalPages := int(math.Ceil(float64(total) / float64(h.pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	if page < 1 {
		page = 1
	} else if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * h.pageSize
	endIdx := startIdx + h.pageSize
	if endIdx > total {
		endIdx = total
	}

	pageRequests := requests[startIdx:endIdx]
	text := formatRequestList(pageRequests, page, totalPages)

	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}

	if total > 0 {
		params.ReplyMarkup = buildPaginationKeyboard(page, totalPages)
	}

	_, err = b.SendMessage(ctx, params)
	return err
}

func (h *CommandHandler) handleDetail(ctx context.Context, b *bot.Bot, chatID int64, reqID string) error {
	detail, err := h.auth.GetIncomingRequestDetail(reqID)
	if err != nil {
		return err
	}

	text := formatRequestDetail(detail)
	return h.sendMessage(ctx, b, chatID, text)
}

func (h *CommandHandler) handleConfirm(ctx context.Context, b *bot.Bot, chatID int64, reqID string) error {
	detail, err := h.auth.GetIncomingRequestDetail(reqID)
	if err != nil {
		return err
	}

	requester := getRequester(detail.RequestorUserName)
	_, addr := getAccountStr(detail.AccountDetails, detail.Operation)
	text := fmt.Sprintf(`⚠️ Confirm Request %s?
Requester : %s
Safe      : %s
Account   : %s
─────────────────────────
Add a reason?`, reqID, requester, detail.SafeName, addr)

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: buildConfirmReasonKeyboard(reqID),
	})
	return err
}

func (h *CommandHandler) executeConfirm(ctx context.Context, b *bot.Bot, chatID int64, reqID, username string, userID int64, reason string) error {
	finalReason := ""
	if reason == "" {
		finalReason = "[CybArBot] Approved via CybArBot"
	} else {
		finalReason = "[CybArBot] " + reason
	}

	err := h.auth.ConfirmRequest(reqID, finalReason)
	if err != nil {
		return err
	}

	AuditLog("CONFIRM", reqID, false, userID, username, finalReason)
	
	if h.notifier != nil {
		h.notifier.UpdateNotificationMessage(ctx, reqID, fmt.Sprintf("✅ CONFIRMED by @%s at %s", username, time.Now().In(tzLocation).Format("2006-01-02 15:04:05 MST")))
	}

	text := fmt.Sprintf(`✅ Request %s Confirmed
Reason : %s
By     : @%s
At     : %s`, reqID, finalReason, username, time.Now().In(tzLocation).Format("2006-01-02 15:04:05 MST"))
	return h.sendMessage(ctx, b, chatID, text)
}

func (h *CommandHandler) handleReject(ctx context.Context, b *bot.Bot, chatID int64, reqID string) error {
	fsmCtx := h.fsm.SetState(chatID, StateWaitingRejectReason)
	fsmCtx.RequestID = reqID

	text := fmt.Sprintf("✏️ Please provide a rejection reason for %s\n(This field is mandatory):", reqID)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		ReplyMarkup: &models.ForceReply{
			ForceReply: true,
		},
	})
	return err
}

func (h *CommandHandler) executeReject(ctx context.Context, b *bot.Bot, chatID int64, reqID, username string, userID int64, reason string) error {
	finalReason := "[CybArBot] " + reason

	err := h.auth.RejectRequest(reqID, finalReason)
	if err != nil {
		return err
	}

	AuditLog("REJECT", reqID, false, userID, username, finalReason)
	
	if h.notifier != nil {
		h.notifier.UpdateNotificationMessage(ctx, reqID, fmt.Sprintf("❌ REJECTED by @%s at %s — Reason: %s", username, time.Now().In(tzLocation).Format("2006-01-02 15:04:05 MST"), finalReason))
	}

	text := fmt.Sprintf(`❌ Request %s Rejected
Reason : %s
By     : @%s
At     : %s`, reqID, finalReason, username, time.Now().In(tzLocation).Format("2006-01-02 15:04:05 MST"))
	return h.sendMessage(ctx, b, chatID, text)
}
