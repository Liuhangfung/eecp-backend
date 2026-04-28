package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) handleStart(msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	switch args {
	case "book":
		h.handleBook(msg)
		return
	case "mybookings":
		h.handleMyBookings(msg)
		return
	case "cancel":
		h.handleCancel(msg)
		return
	}

	firstName := msg.From.FirstName
	if firstName == "" {
		firstName = "there"
	}

	greeting := fmt.Sprintf("👋 Hello %s!\n\n"+
		"Welcome to the EECP Booking System — your one-stop solution for booking EECP treatment sessions.\n\n"+
		"🌟 *VIP Room* — 2 machines\n"+
		"🏥 *Common Room* — 3 machines\n"+
		"⏱ Each session is *1.5 hours*\n"+
		"📅 Book up to *%d days* in advance\n\n"+
		"How can we assist you today?",
		firstName, h.maxDays)

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚡ Quick Book (Now)", "menu:quickbook"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Book a Session", "menu:book"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 My Bookings", "menu:mybookings"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Booking", "menu:cancel"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📄 Disclaimer", "menu:disclaimer"),
		),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, greeting)
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = inlineKeyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending start message: %v", err)
	}

	h.sendPersistentMenu(msg.Chat.ID)
}

func (h *Handler) sendPersistentMenu(chatID int64) {
	replyKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("⚡ Quick Book"),
			tgbotapi.NewKeyboardButton("📅 Book a Session"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📋 My Bookings"),
			tgbotapi.NewKeyboardButton("❌ Cancel Booking"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📄 Disclaimer"),
			tgbotapi.NewKeyboardButton("🏠 Main Menu"),
		),
	)
	replyKeyboard.ResizeKeyboard = true

	menuMsg := tgbotapi.NewMessage(chatID, "Use the menu below or tap the buttons above 👆")
	menuMsg.ReplyMarkup = replyKeyboard
	if _, err := h.bot.Send(menuMsg); err != nil {
		log.Printf("Error sending persistent menu: %v", err)
	}
}

func (h *Handler) handleQuickBook(msg *tgbotapi.Message) {
	keyboard := buildQuickBookRoomKeyboard()
	reply := tgbotapi.NewMessage(msg.Chat.ID, "⚡ *Quick Book — Select a room:*")
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending quick book room selection: %v", err)
	}
}

func (h *Handler) handleBook(msg *tgbotapi.Message) {
	keyboard := buildDateKeyboard(h.maxDays)

	reply := tgbotapi.NewMessage(msg.Chat.ID, "📅 *Book a Session*\n\nSelect a date:")
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending date picker: %v", err)
	}
}

func (h *Handler) handleMyBookings(msg *tgbotapi.Message) {
	ctx := context.Background()
	bookings, err := h.service.GetUserBookings(ctx, msg.From.ID)
	if err != nil {
		h.sendText(msg.Chat.ID, "Error fetching your bookings. Please try again.")
		log.Printf("Error fetching user bookings: %v", err)
		return
	}

	if len(bookings) == 0 {
		text := "📋 *My Bookings*\n\nYou have no upcoming bookings.\n\nTap 📅 *Book a Session* to reserve one!"
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ParseMode = tgbotapi.ModeMarkdown
		if _, err := h.bot.Send(reply); err != nil {
			log.Printf("Error sending empty bookings: %v", err)
		}
		return
	}

	text := "📋 *My Bookings*\n\n"
	for i, b := range bookings {
		short := b.ID
		if len(short) > 8 {
			short = short[:8]
		}
		text += fmt.Sprintf("*%d.* `#%s`\n   🔧 %s\n   📅 %s\n   🕐 %s - %s\n\n",
			i+1,
			short,
			b.MachineName,
			b.StartTime.Format("Jan 2, 2006"),
			b.StartTime.Format("15:04"),
			b.EndTime.Format("15:04"),
		)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel Booking", "menu:cancel"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Book a Session", "menu:book"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Main Menu", "menu:start"),
		),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending bookings: %v", err)
	}
}

func (h *Handler) handleDisclaimer(msg *tgbotapi.Message) {
	disclaimerPath := "disclaimer/免責聲明.docx"
	if _, err := os.Stat(disclaimerPath); os.IsNotExist(err) {
		disclaimerPath = filepath.Join("..", "..", "disclaimer", "免責聲明.docx")
	}

	if _, err := os.Stat(disclaimerPath); os.IsNotExist(err) {
		h.sendText(msg.Chat.ID, "Sorry, the disclaimer document is currently unavailable.")
		log.Printf("Disclaimer file not found at %s", disclaimerPath)
		return
	}

	doc := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FilePath(disclaimerPath))
	doc.Caption = "📄 EECP Treatment Disclaimer (免責聲明)"
	if _, err := h.bot.Send(doc); err != nil {
		log.Printf("Error sending disclaimer document: %v", err)
		h.sendText(msg.Chat.ID, "Sorry, failed to send the disclaimer document. Please try again.")
	}
}

func (h *Handler) handleCancel(msg *tgbotapi.Message) {
	ctx := context.Background()
	bookings, err := h.service.GetUserBookings(ctx, msg.From.ID)
	if err != nil {
		h.sendText(msg.Chat.ID, "Error fetching your bookings. Please try again.")
		return
	}

	if len(bookings) == 0 {
		h.sendText(msg.Chat.ID, "You have no bookings to cancel.")
		return
	}

	keyboard := buildCancelKeyboard(bookings)
	reply := tgbotapi.NewMessage(msg.Chat.ID, "❌ *Cancel Booking*\n\nWhich booking do you want to cancel?")
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending cancel picker: %v", err)
	}
}
