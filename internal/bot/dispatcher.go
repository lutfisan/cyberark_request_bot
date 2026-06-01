package bot

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"cybarbot/internal/whitelist"
)

type Dispatcher struct {
	bot        *tgbotapi.BotAPI
	whitelist  *whitelist.Whitelist
	cmdHandler *CommandHandler
	silent     bool
	rejectMsg  string
}

func NewDispatcher(bot *tgbotapi.BotAPI, wl *whitelist.Whitelist, cmdHandler *CommandHandler, silent bool, rejectMsg string) *Dispatcher {
	return &Dispatcher{
		bot:        bot,
		whitelist:  wl,
		cmdHandler: cmdHandler,
		silent:     silent,
		rejectMsg:  rejectMsg,
	}
}

func (d *Dispatcher) ProcessUpdate(update tgbotapi.Update) {
	var senderID int64
	var chatID int64

	if update.Message != nil {
		senderID = update.Message.From.ID
		chatID = update.Message.Chat.ID
		// For groups, check chat ID as well
		if update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup() {
			if d.whitelist.IsAllowed(chatID) {
				senderID = chatID // Use chat ID for whitelist check if group is allowed
			}
		}
	} else if update.CallbackQuery != nil {
		senderID = update.CallbackQuery.From.ID
		chatID = update.CallbackQuery.Message.Chat.ID
		if update.CallbackQuery.Message.Chat.IsGroup() || update.CallbackQuery.Message.Chat.IsSuperGroup() {
			if d.whitelist.IsAllowed(chatID) {
				senderID = chatID
			}
		}
	} else {
		// Unhandled update type
		return
	}

	if !d.whitelist.IsAllowed(senderID) {
		slog.Warn("unauthorized access attempt", "sender_id", senderID)
		if !d.silent && chatID != 0 {
			msg := tgbotapi.NewMessage(chatID, d.rejectMsg)
			d.bot.Send(msg)
		}
		return
	}

	if update.Message != nil {
		if update.Message.IsCommand() {
			go WithLogging(func() error {
				d.cmdHandler.HandleCommand(update)
				return nil
			}, "command_"+update.Message.Command(), update)()
		} else if update.Message.Text != "" {
			go WithLogging(func() error {
				d.cmdHandler.HandleTextMessage(update)
				return nil
			}, "text_message", update)()
		}
	} else if update.CallbackQuery != nil {
		go WithLogging(func() error {
			d.cmdHandler.HandleCallback(update)
			return nil
		}, "callback_query", update)()
	}
}
