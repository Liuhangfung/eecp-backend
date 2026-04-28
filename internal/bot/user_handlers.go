package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

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
		"🏥 We have *5 EECP machines* available *24/7*\n"+
		"⏱ Each session is *1 hour*\n"+
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
	ctx := context.Background()
	now := time.Now().In(hkt)
	slotStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, hkt)

	hasBooking, err := h.service.UserHasBookingForSlot(ctx, msg.From.ID, slotStart)
	if err == nil && hasBooking {
		h.sendText(msg.Chat.ID, "✅ You already have a booking for this hour. Use 📋 My Bookings to view it.")
		return
	}

	machine, err := h.service.GetFreeMachineForSlot(ctx, slotStart)
	if err != nil {
		h.sendText(msg.Chat.ID, "😔 Sorry, no machines are available right now. Try booking a later slot with 📅 Book a Session.")
		return
	}

	endTime := slotStart.Add(time.Hour)
	remaining := int(endTime.Sub(now).Minutes())

	if remaining < 10 {
		nextSlot := slotStart.Add(time.Hour)
		nextMachine, nextErr := h.service.GetFreeMachineForSlot(ctx, nextSlot)
		if nextErr != nil {
			h.sendText(msg.Chat.ID, fmt.Sprintf("⏱ Only %d min left in this hour and the next slot is full. Try 📅 Book a Session.", remaining))
			return
		}
		machine = nextMachine
		slotStart = nextSlot
		endTime = nextSlot.Add(time.Hour)
		remaining = 60
	}

	text := fmt.Sprintf("⚡ *Quick Book — Right Now*\n\n"+
		"  Machine: %s\n"+
		"  Date:    %s\n"+
		"  Time:    %s - %s\n"+
		"  ⏱ %d min remaining in this slot\n\n"+
		"Confirm?",
		machine.Name,
		slotStart.Format("Jan 2, 2006"),
		slotStart.Format("15:04"),
		endTime.Format("15:04"),
		remaining,
	)

	keyboard := buildConfirmKeyboard(machine.ID, slotStart.Format("2006-01-02T15:04"))
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending quick book: %v", err)
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
