package bot

import (
	"log"

	"github.com/eecp/booking-bot/internal/booking"
	"github.com/eecp/booking-bot/internal/db"
	"github.com/eecp/booking-bot/internal/notifier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	bot      *tgbotapi.BotAPI
	service  *booking.Service
	store    *db.Store
	notifier *notifier.GroupNotifier
	maxDays  int
}

func NewHandler(bot *tgbotapi.BotAPI, service *booking.Service, store *db.Store, notifier *notifier.GroupNotifier, maxDays int) *Handler {
	return &Handler{
		bot:      bot,
		service:  service,
		store:    store,
		notifier: notifier,
		maxDays:  maxDays,
	}
}

func (h *Handler) HandleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	if update.Message.IsCommand() {
		h.handleCommand(update.Message)
		return
	}

	h.handleTextMessage(update.Message)
}

func (h *Handler) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.handleStart(msg)
	case "book":
		h.handleBook(msg)
	case "mybookings":
		h.handleMyBookings(msg)
	case "cancel":
		h.handleCancel(msg)
	case "admin":
		h.handleAdmin(msg)
	case "allbookings":
		h.handleAllBookings(msg)
	case "cancelbooking":
		h.handleCancelBookingAdmin(msg)
	case "togglemachine":
		h.handleToggleMachine(msg)
	case "stats":
		h.handleStats(msg)
	case "disclaimer":
		h.handleDisclaimer(msg)
	case "postbooking":
		h.handlePostBooking(msg)
	case "chatid":
		h.handleChatID(msg)
	}
}

func (h *Handler) handleTextMessage(msg *tgbotapi.Message) {
	switch msg.Text {
	case "📅 Book a Session":
		h.handleBook(msg)
	case "📋 My Bookings":
		h.handleMyBookings(msg)
	case "❌ Cancel Booking":
		h.handleCancel(msg)
	case "📄 Disclaimer":
		h.handleDisclaimer(msg)
	case "🏠 Main Menu":
		h.handleStart(msg)
	}
}

func (h *Handler) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (h *Handler) sendMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Error sending markdown message: %v", err)
	}
}

func (h *Handler) editMessage(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if keyboard != nil {
		edit.ReplyMarkup = keyboard
	}
	if _, err := h.bot.Send(edit); err != nil {
		log.Printf("Error editing message: %v", err)
	}
}
