package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) isAdmin(userID int64) bool {
	ctx := context.Background()
	ok, err := h.store.IsAdmin(ctx, userID)
	if err != nil {
		log.Printf("Error checking admin: %v", err)
		return false
	}
	return ok
}

func (h *Handler) handleAdmin(msg *tgbotapi.Message) {
	if !h.isAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "You are not authorized to use admin commands.")
		return
	}

	text := `Admin Panel:

/allbookings [date] — View all bookings (e.g. /allbookings 2026-04-03)
/cancelbooking [id] — Cancel any booking
/togglemachine — Enable/disable a machine
/stats — View statistics
/postbooking — Post "Book Now" button to this chat (pin it in your group!)`

	h.sendText(msg.Chat.ID, text)
}

func (h *Handler) handlePostBooking(msg *tgbotapi.Message) {
	botUsername := h.bot.Self.UserName
	bookURL := fmt.Sprintf("https://t.me/%s?start=book", botUsername)

	text := "🏥 *EECP Booking System*\n\n" +
		"We have 5 EECP machines available 24/7\\.\n" +
		"Each session is 1 hour\\.\n\n" +
		"Tap the button below to book your session\\!"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📅 Book Now", bookURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📋 My Bookings", fmt.Sprintf("https://t.me/%s?start=mybookings", botUsername)),
			tgbotapi.NewInlineKeyboardButtonURL("❌ Cancel Booking", fmt.Sprintf("https://t.me/%s?start=cancel", botUsername)),
		),
	)

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdownV2
	reply.ReplyMarkup = keyboard

	sent, err := h.bot.Send(reply)
	if err != nil {
		log.Printf("Error sending booking post: %v", err)
		h.sendText(msg.Chat.ID, "Failed to post booking message.")
		return
	}

	pin := tgbotapi.PinChatMessageConfig{
		ChatID:              msg.Chat.ID,
		MessageID:           sent.MessageID,
		DisableNotification: true,
	}
	if _, err := h.bot.Request(pin); err != nil {
		log.Printf("Could not pin message (bot may need admin rights): %v", err)
	}
}

func (h *Handler) handleAllBookings(msg *tgbotapi.Message) {
	if !h.isAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "You are not authorized to use admin commands.")
		return
	}

	ctx := context.Background()
	dateStr := strings.TrimSpace(msg.CommandArguments())

	var date time.Time
	if dateStr == "" {
		date = time.Now()
	} else {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			h.sendText(msg.Chat.ID, "Invalid date format. Use: /allbookings 2026-04-03")
			return
		}
	}

	bookings, err := h.service.GetAllBookingsForDate(ctx, date)
	if err != nil {
		h.sendText(msg.Chat.ID, "Error fetching bookings.")
		log.Printf("Error fetching all bookings: %v", err)
		return
	}

	if len(bookings) == 0 {
		h.sendText(msg.Chat.ID, fmt.Sprintf("No bookings for %s.", date.Format("Jan 2, 2006")))
		return
	}

	byMachine := make(map[string][]string)
	for _, b := range bookings {
		short := b.ID
		if len(short) > 8 {
			short = short[:8]
		}
		line := fmt.Sprintf("  %s - %s  @%s (#%s)",
			b.StartTime.Format("15:04"),
			b.EndTime.Format("15:04"),
			b.TelegramUsername,
			short,
		)
		byMachine[b.MachineName] = append(byMachine[b.MachineName], line)
	}

	text := fmt.Sprintf("All bookings for %s:\n\n", date.Format("Jan 2, 2006"))

	machines, _ := h.service.GetMachines(ctx)
	for _, m := range machines {
		lines, ok := byMachine[m.Name]
		if ok {
			text += fmt.Sprintf("%s:\n", m.Name)
			for _, l := range lines {
				text += l + "\n"
			}
			text += "\n"
		} else {
			text += fmt.Sprintf("%s: No bookings\n\n", m.Name)
		}
	}

	text += fmt.Sprintf("Total: %d bookings", len(bookings))
	h.sendText(msg.Chat.ID, text)
}

func (h *Handler) handleCancelBookingAdmin(msg *tgbotapi.Message) {
	if !h.isAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "You are not authorized to use admin commands.")
		return
	}

	bookingID := strings.TrimSpace(msg.CommandArguments())
	if bookingID == "" {
		h.sendText(msg.Chat.ID, "Usage: /cancelbooking <booking_id>")
		return
	}

	ctx := context.Background()
	b, err := h.service.CancelBookingByAdmin(ctx, bookingID)
	if err != nil {
		h.sendText(msg.Chat.ID, fmt.Sprintf("Error: %v", err))
		return
	}

	short := b.ID
	if len(short) > 8 {
		short = short[:8]
	}

	h.sendText(msg.Chat.ID, fmt.Sprintf(
		"Booking #%s cancelled.\n\nUser: @%s\nMachine: %s\nTime: %s %s-%s",
		short, b.TelegramUsername, b.MachineName,
		b.StartTime.Format("Jan 2, 2006"),
		b.StartTime.Format("15:04"),
		b.EndTime.Format("15:04"),
	))

	h.notifier.NotifyCancellation(b, true)
}

func (h *Handler) handleToggleMachine(msg *tgbotapi.Message) {
	if !h.isAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "You are not authorized to use admin commands.")
		return
	}

	ctx := context.Background()
	machines, err := h.service.GetMachines(ctx)
	if err != nil {
		h.sendText(msg.Chat.ID, "Error fetching machines.")
		return
	}

	keyboard := buildMachineToggleKeyboard(machines)
	reply := tgbotapi.NewMessage(msg.Chat.ID, "Tap a machine to toggle its status:")
	reply.ReplyMarkup = keyboard
	if _, err := h.bot.Send(reply); err != nil {
		log.Printf("Error sending toggle keyboard: %v", err)
	}
}

func (h *Handler) handleStats(msg *tgbotapi.Message) {
	if !h.isAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "You are not authorized to use admin commands.")
		return
	}

	ctx := context.Background()
	stats, err := h.service.GetStats(ctx, 7)
	if err != nil {
		h.sendText(msg.Chat.ID, "Error fetching statistics.")
		log.Printf("Error fetching stats: %v", err)
		return
	}

	busiestMachine := stats.BusiestMachine
	if busiestMachine == "" {
		busiestMachine = "N/A"
	}
	busiestHour := stats.BusiestHour
	if busiestHour == "" {
		busiestHour = "N/A"
	}

	text := fmt.Sprintf(`Booking Statistics (Last 7 days):

Total bookings:    %d
Cancellations:     %d
Busiest machine:   %s
Busiest time:      %s
Avg daily usage:   %.1f sessions/day`,
		stats.TotalBookings,
		stats.Cancellations,
		busiestMachine,
		busiestHour,
		stats.AvgDailyUsage,
	)

	h.sendText(msg.Chat.ID, text)
}

func (h *Handler) handleChatID(msg *tgbotapi.Message) {
	chatType := msg.Chat.Type
	chatTitle := msg.Chat.Title
	if chatTitle == "" {
		chatTitle = "Private Chat"
	}

	text := fmt.Sprintf("Chat Info:\n\n  Chat ID: %d\n  Type: %s\n  Title: %s\n\nCopy the Chat ID to your .env file.",
		msg.Chat.ID, chatType, chatTitle)
	h.sendText(msg.Chat.ID, text)
}
