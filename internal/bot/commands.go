package bot

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"cybarbot/internal/cyberark"
)

type CommandHandler struct {
	bot        *tgbotapi.BotAPI
	auth       *cyberark.AuthManager
	fsm        *FSMManager
	pageSize   int
	botVersion string
	startTime  time.Time
	notifier   *Notifier // reference for status command
}

func NewCommandHandler(bot *tgbotapi.BotAPI, auth *cyberark.AuthManager, fsm *FSMManager, pageSize int, version string, notifier *Notifier) *CommandHandler {
	return &CommandHandler{
		bot:        bot,
		auth:       auth,
		fsm:        fsm,
		pageSize:   pageSize,
		botVersion: version,
		startTime:  time.Now(),
		notifier:   notifier,
	}
}

func (h *CommandHandler) HandleCommand(update tgbotapi.Update) {
	defer PanicRecovery(h.bot, update.Message.Chat.ID)
	
	msg := update.Message
	chatID := msg.Chat.ID
	command := msg.Command()

	// Any command resets FSM
	h.fsm.Reset(chatID)

	var err error
	switch command {
	case "start", "help":
		err = h.handleHelp(chatID)
	case "status":
		err = h.handleStatus(chatID)
	case "notify_status":
		err = h.handleNotifyStatus(chatID)
	case "requests":
		err = h.handleRequests(chatID, 1)
	case "detail":
		args := msg.CommandArguments()
		if args == "" {
			h.sendMessage(chatID, "Usage: /detail <RequestID>")
			return
		}
		err = h.handleDetail(chatID, args)
	case "confirm":
		args := msg.CommandArguments()
		if args == "" {
			h.sendMessage(chatID, "Usage: /confirm <RequestID>")
			return
		}
		err = h.handleConfirm(chatID, args)
	case "reject":
		args := msg.CommandArguments()
		if args == "" {
			h.sendMessage(chatID, "Usage: /reject <RequestID>")
			return
		}
		err = h.handleReject(chatID, args)
	case "cancel":
		h.sendMessage(chatID, "✅ Operation cancelled. State reset to IDLE.")
	default:
		h.sendMessage(chatID, "Unknown command. Use /help for a list of commands.")
	}

	if err != nil {
		slog.Error("command execution failed", "command", command, "error", err)
		h.sendMessage(chatID, fmt.Sprintf("🔴 Error executing command: %v", err))
	}
}

func (h *CommandHandler) HandleCallback(update tgbotapi.Update) {
	defer PanicRecovery(h.bot, update.CallbackQuery.Message.Chat.ID)
	
	cb := update.CallbackQuery
	chatID := cb.Message.Chat.ID
	data := cb.Data

	// Acknowledge callback immediately
	callbackCfg := tgbotapi.NewCallback(cb.ID, "")
	h.bot.Request(callbackCfg)

	var err error
	if strings.HasPrefix(data, "req_page_") {
		var page int
		fmt.Sscanf(data, "req_page_%d", &page)
		err = h.handleRequests(chatID, page)
	} else if strings.HasPrefix(data, "confirm_skip_") {
		reqID := strings.TrimPrefix(data, "confirm_skip_")
		err = h.executeConfirm(chatID, reqID, cb.From.UserName, int64(cb.From.ID), "")
	} else if strings.HasPrefix(data, "confirm_reason_") {
		reqID := strings.TrimPrefix(data, "confirm_reason_")
		ctx := h.fsm.SetState(chatID, StateWaitingConfirmReason)
		ctx.RequestID = reqID
		h.sendMessage(chatID, "Please type your reason:")
	} else if strings.HasPrefix(data, "notif_confirm_") {
		reqID := strings.TrimPrefix(data, "notif_confirm_")
		err = h.handleConfirm(chatID, reqID)
	} else if strings.HasPrefix(data, "notif_reject_") {
		reqID := strings.TrimPrefix(data, "notif_reject_")
		err = h.handleReject(chatID, reqID)
	} else if strings.HasPrefix(data, "notif_detail_") {
		reqID := strings.TrimPrefix(data, "notif_detail_")
		err = h.handleDetail(chatID, reqID)
	} else if data == "noop" {
		// Do nothing
	} else {
		slog.Warn("unhandled callback data", "data", data)
	}

	if err != nil {
		slog.Error("callback execution failed", "data", data, "error", err)
		h.sendMessage(chatID, fmt.Sprintf("🔴 Error processing action: %v", err))
	}
}

func (h *CommandHandler) HandleTextMessage(update tgbotapi.Update) {
	defer PanicRecovery(h.bot, update.Message.Chat.ID)
	
	msg := update.Message
	chatID := msg.Chat.ID
	text := msg.Text

	ctx := h.fsm.GetContext(chatID)
	if ctx.State == StateIdle {
		return // Ignore random text messages
	}

	var err error
	switch ctx.State {
	case StateWaitingConfirmReason:
		err = h.executeConfirm(chatID, ctx.RequestID, msg.From.UserName, int64(msg.From.ID), text)
	case StateWaitingRejectReason:
		err = h.executeReject(chatID, ctx.RequestID, msg.From.UserName, int64(msg.From.ID), text)
	}

	if err != nil {
		slog.Error("state execution failed", "state", ctx.State, "error", err)
		h.sendMessage(chatID, fmt.Sprintf("🔴 Error processing action: %v", err))
	}
	h.fsm.Reset(chatID)
}

func (h *CommandHandler) handleHelp(chatID int64) error {
	helpText := `🤖 **CybArBot Command Reference**

/start - Welcome message and command list
/help - Full command reference
/status - Bot health, session status, active delivery mode
/notify_status - Notification watcher health
/requests - List all pending incoming requests (paginated)
/detail <id> - Show full confirmation details for a request
/confirm <id> - Confirm a single request (optional reason)
/reject <id> - Reject a single request (mandatory reason)
/cancel - Abort any active multi-step operation
`
	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	_, err := h.bot.Send(msg)
	return err
}

func (h *CommandHandler) handleStatus(chatID int64) error {
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

	h.sendMessage(chatID, statusText)
	return nil
}

func (h *CommandHandler) handleNotifyStatus(chatID int64) error {
	if h.notifier == nil {
		h.sendMessage(chatID, "Notification watcher is disabled.")
		return nil
	}
	
	// Assuming notifier has a way to get stats
	stats := h.notifier.GetStats()
	
	statusText := fmt.Sprintf(`🔔 Notification Watcher Status
────────────────────────────────────
Enabled        : ✅ Yes
Poll Interval  : %d seconds
Last Poll      : %s
Last Result    : %d seen, %d new, %d stale-edited
Cache Size     : %d requests
`, stats.PollInterval, stats.LastPoll.Format("2006-01-02 15:04:05 UTC"), stats.LastSeen, stats.LastNew, stats.LastStaleEdited, stats.CacheSize)

	h.sendMessage(chatID, statusText)
	return nil
}

func (h *CommandHandler) handleRequests(chatID int64, page int) error {
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

	msg := tgbotapi.NewMessage(chatID, text)
	if total > 0 {
		msg.ReplyMarkup = buildPaginationKeyboard(page, totalPages)
	}

	_, err = h.bot.Send(msg)
	return err
}

func (h *CommandHandler) handleDetail(chatID int64, reqID string) error {
	detail, err := h.auth.GetIncomingRequestDetail(reqID)
	if err != nil {
		return err
	}

	text := formatRequestDetail(detail)
	h.sendMessage(chatID, text)
	return nil
}

func (h *CommandHandler) handleConfirm(chatID int64, reqID string) error {
	detail, err := h.auth.GetIncomingRequestDetail(reqID)
	if err != nil {
		return err
	}

	text := fmt.Sprintf(`⚠️ Confirm Request %s?
Requester : %s
Safe      : %s
Account   : %s
─────────────────────────
Add a reason?`, reqID, detail.RequesterUserName, detail.SafeName, detail.AccountName)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = buildConfirmReasonKeyboard(reqID)
	_, err = h.bot.Send(msg)
	return err
}

func (h *CommandHandler) executeConfirm(chatID int64, reqID, username string, userID int64, reason string) error {
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
		h.notifier.UpdateNotificationMessage(reqID, fmt.Sprintf("✅ CONFIRMED by @%s at %s", username, time.Now().UTC().Format("2006-01-02 15:04:05 UTC")))
	}

	text := fmt.Sprintf(`✅ Request %s Confirmed
Reason : %s
By     : @%s
At     : %s`, reqID, finalReason, username, time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	h.sendMessage(chatID, text)
	return nil
}

func (h *CommandHandler) handleReject(chatID int64, reqID string) error {
	ctx := h.fsm.SetState(chatID, StateWaitingRejectReason)
	ctx.RequestID = reqID

	text := fmt.Sprintf("✏️ Please provide a rejection reason for %s\n(This field is mandatory):", reqID)
	h.sendMessage(chatID, text)
	return nil
}

func (h *CommandHandler) executeReject(chatID int64, reqID, username string, userID int64, reason string) error {
	finalReason := "[CybArBot] " + reason

	err := h.auth.RejectRequest(reqID, finalReason)
	if err != nil {
		return err
	}

	AuditLog("REJECT", reqID, false, userID, username, finalReason)
	
	if h.notifier != nil {
		h.notifier.UpdateNotificationMessage(reqID, fmt.Sprintf("❌ REJECTED by @%s at %s — Reason: %s", username, time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), finalReason))
	}

	text := fmt.Sprintf(`❌ Request %s Rejected
Reason : %s
By     : @%s
At     : %s`, reqID, finalReason, username, time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	h.sendMessage(chatID, text)
	return nil
}

func (h *CommandHandler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	h.bot.Send(msg)
}
