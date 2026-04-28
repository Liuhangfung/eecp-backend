package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) handleCallback(cq *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cq.ID, "")
	h.bot.Send(callback)

	data := cq.Data
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID

	fakeMsg := &tgbotapi.Message{
		Chat: cq.Message.Chat,
		From: cq.From,
	}

	switch {
	case data == "menu:book":
		h.handleBook(fakeMsg)
	case data == "menu:start":
		h.handleStart(fakeMsg)
	case data == "menu:mybookings":
		h.handleMyBookings(fakeMsg)
	case data == "menu:cancel":
		h.handleCancel(fakeMsg)
	case data == "menu:disclaimer":
		h.handleDisclaimer(fakeMsg)
	case strings.HasPrefix(data, "date:"):
		h.onDateSelected(chatID, msgID, data)
	case strings.HasPrefix(data, "time:"):
		h.onTimeSelected(chatID, msgID, cq.From.ID, cq.From.UserName, data)
	case strings.HasPrefix(data, "confirm:"):
		h.onConfirm(chatID, msgID, cq.From.ID, cq.From.UserName, data)
	case strings.HasPrefix(data, "cancelsel:"):
		h.onCancelSelect(chatID, msgID, cq.From.ID, data)
	case strings.HasPrefix(data, "cancelconfirm:"):
		h.onCancelConfirm(chatID, msgID, cq.From.ID, data)
	case data == "cancel_booking_flow":
		h.editMessage(chatID, msgID, "Booking flow cancelled.", nil)
	case strings.HasPrefix(data, "showmore:"):
		h.onShowMoreSlots(chatID, msgID, data)
	case strings.HasPrefix(data, "toggle:"):
		h.onToggleMachine(chatID, msgID, cq.From.ID, data)
	case data == "noop":
		// do nothing
	}
}

func (h *Handler) onDateSelected(chatID int64, msgID int, data string) {
	dateStr := strings.TrimPrefix(data, "date:")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid date selected.", nil)
		return
	}

	ctx := context.Background()
	slots, err := h.service.GetAvailableSlots(ctx, date)
	if err != nil {
		h.editMessage(chatID, msgID, "Error loading available slots.", nil)
		log.Printf("Error getting slots: %v", err)
		return
	}

	if len(slots) == 0 {
		h.editMessage(chatID, msgID, fmt.Sprintf("No available slots for %s.", date.Format("Jan 2, 2006")), nil)
		return
	}

	keyboard := buildTimeKeyboard(slots, false)
	h.editMessage(chatID, msgID, fmt.Sprintf("Available slots for %s:", date.Format("Jan 2, 2006")), &keyboard)
}

func (h *Handler) onShowMoreSlots(chatID int64, msgID int, data string) {
	dateStr := strings.TrimPrefix(data, "showmore:")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid date.", nil)
		return
	}

	ctx := context.Background()
	slots, err := h.service.GetAvailableSlots(ctx, date)
	if err != nil {
		h.editMessage(chatID, msgID, "Error loading available slots.", nil)
		log.Printf("Error getting slots: %v", err)
		return
	}

	keyboard := buildTimeKeyboard(slots, true)
	h.editMessage(chatID, msgID, fmt.Sprintf("All available slots for %s:", date.Format("Jan 2, 2006")), &keyboard)
}

func (h *Handler) onTimeSelected(chatID int64, msgID int, userID int64, username string, data string) {
	timeStr := strings.TrimPrefix(data, "time:")
	startTime, err := time.Parse("2006-01-02T15:04", timeStr)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid time selected.", nil)
		return
	}

	ctx := context.Background()
	machine, err := h.service.GetFreeMachineForSlot(ctx, startTime)
	if err != nil {
		h.editMessage(chatID, msgID, "Sorry, this slot is no longer available.", nil)
		return
	}

	text := fmt.Sprintf("Confirm your booking:\n\n  Machine: %s\n  Date:    %s\n  Time:    %s - %s",
		machine.Name,
		startTime.Format("Jan 2, 2006"),
		startTime.Format("15:04"),
		startTime.Add(time.Hour).Format("15:04"),
	)

	keyboard := buildConfirmKeyboard(machine.ID, startTime.Format("2006-01-02T15:04"))
	h.editMessage(chatID, msgID, text, &keyboard)
}

func (h *Handler) onConfirm(chatID int64, msgID int, userID int64, username string, data string) {
	parts := strings.SplitN(strings.TrimPrefix(data, "confirm:"), ":", 2)
	if len(parts) != 2 {
		h.editMessage(chatID, msgID, "Invalid confirmation data.", nil)
		return
	}

	machineID, err := strconv.Atoi(parts[0])
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid machine ID.", nil)
		return
	}

	startTime, err := time.Parse("2006-01-02T15:04", parts[1])
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid time.", nil)
		return
	}

	if username == "" {
		username = fmt.Sprintf("user_%d", userID)
	}

	ctx := context.Background()
	b, err := h.service.CreateBooking(ctx, machineID, userID, username, startTime)
	if err != nil {
		h.editMessage(chatID, msgID, fmt.Sprintf("Booking failed: %v", err), nil)
		return
	}

	short := b.ID
	if len(short) > 8 {
		short = short[:8]
	}

	text := fmt.Sprintf("Booking confirmed!\n\n  Booking ID: #%s\n  Machine:    %s\n  Date:       %s\n  Time:       %s - %s\n\nTo view your bookings: /mybookings\nTo cancel: /cancel",
		short,
		b.MachineName,
		b.StartTime.Format("Jan 2, 2006"),
		b.StartTime.Format("15:04"),
		b.EndTime.Format("15:04"),
	)

	h.editMessage(chatID, msgID, text, nil)

	h.notifier.NotifyNewBooking(b)
}

func (h *Handler) onCancelSelect(chatID int64, msgID int, userID int64, data string) {
	bookingID := strings.TrimPrefix(data, "cancelsel:")

	ctx := context.Background()
	bookings, err := h.service.GetUserBookings(ctx, userID)
	if err != nil {
		h.editMessage(chatID, msgID, "Error loading booking.", nil)
		return
	}

	var found *struct {
		id, machineName string
		start, end      time.Time
	}
	for _, b := range bookings {
		if b.ID == bookingID {
			found = &struct {
				id, machineName string
				start, end      time.Time
			}{b.ID, b.MachineName, b.StartTime, b.EndTime}
			break
		}
	}

	if found == nil {
		h.editMessage(chatID, msgID, "Booking not found.", nil)
		return
	}

	short := found.id
	if len(short) > 8 {
		short = short[:8]
	}

	text := fmt.Sprintf("Are you sure you want to cancel?\n\n  #%s — %s\n  %s  %s - %s",
		short,
		found.machineName,
		found.start.Format("Jan 2, 2006"),
		found.start.Format("15:04"),
		found.end.Format("15:04"),
	)

	keyboard := buildCancelConfirmKeyboard(bookingID)
	h.editMessage(chatID, msgID, text, &keyboard)
}

func (h *Handler) onCancelConfirm(chatID int64, msgID int, userID int64, data string) {
	bookingID := strings.TrimPrefix(data, "cancelconfirm:")

	ctx := context.Background()
	b, err := h.service.CancelBooking(ctx, bookingID, userID)
	if err != nil {
		h.editMessage(chatID, msgID, fmt.Sprintf("Error: %v", err), nil)
		return
	}

	short := b.ID
	if len(short) > 8 {
		short = short[:8]
	}

	h.editMessage(chatID, msgID, fmt.Sprintf("Booking #%s has been cancelled.", short), nil)

	h.notifier.NotifyCancellation(b, false)
}

func (h *Handler) onToggleMachine(chatID int64, msgID int, userID int64, data string) {
	if !h.isAdmin(userID) {
		h.editMessage(chatID, msgID, "Unauthorized.", nil)
		return
	}

	idStr := strings.TrimPrefix(data, "toggle:")
	machineID, err := strconv.Atoi(idStr)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid machine ID.", nil)
		return
	}

	ctx := context.Background()
	m, err := h.service.ToggleMachine(ctx, machineID)
	if err != nil {
		h.editMessage(chatID, msgID, fmt.Sprintf("Error: %v", err), nil)
		return
	}

	status := "ACTIVE ✅"
	if !m.IsActive {
		status = "DISABLED 🔴"
	}

	h.editMessage(chatID, msgID, fmt.Sprintf("%s is now: %s", m.Name, status), nil)

	h.notifier.NotifyMachineToggle(m)
}
