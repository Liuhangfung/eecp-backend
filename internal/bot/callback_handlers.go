package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/eecp/booking-bot/internal/model"
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
	case data == "menu:quickbook":
		h.handleQuickBook(fakeMsg)
	case data == "menu:disclaimer":
		h.handleDisclaimer(fakeMsg)
	case strings.HasPrefix(data, "date:"):
		h.onDateSelected(chatID, msgID, data)
	case strings.HasPrefix(data, "room:"):
		h.onRoomSelected(chatID, msgID, data)
	case strings.HasPrefix(data, "quickroom:"):
		h.onQuickRoomSelected(chatID, msgID, cq.From.ID, cq.From.UserName, data)
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
	case strings.HasPrefix(data, "booknow:"):
		h.onBookNow(chatID, msgID, cq.From.ID, cq.From.UserName, data)
	case strings.HasPrefix(data, "showmore:"):
		h.onShowMoreSlots(chatID, msgID, data)
	case strings.HasPrefix(data, "toggle:"):
		h.onToggleMachine(chatID, msgID, cq.From.ID, data)
	case data == "noop":
		// do nothing
	}
}

func roomLabel(room string) string {
	if room == "vip" {
		return "🌟 VIP Room"
	}
	return "🏥 Common Room"
}

func (h *Handler) onDateSelected(chatID int64, msgID int, data string) {
	dateStr := strings.TrimPrefix(data, "date:")
	_, err := time.ParseInLocation("2006-01-02", dateStr, hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid date selected.", nil)
		return
	}

	keyboard := buildRoomKeyboard(dateStr)
	h.editMessage(chatID, msgID, "🏠 Select a room:", &keyboard)
}

func (h *Handler) onRoomSelected(chatID int64, msgID int, data string) {
	// room:<vip|common>:<date>
	parts := strings.SplitN(strings.TrimPrefix(data, "room:"), ":", 2)
	if len(parts) != 2 {
		h.editMessage(chatID, msgID, "Invalid room selection.", nil)
		return
	}
	room, dateStr := parts[0], parts[1]

	date, err := time.ParseInLocation("2006-01-02", dateStr, hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid date.", nil)
		return
	}

	ctx := context.Background()
	slots, err := h.service.GetAvailableSlots(ctx, date, room)
	if err != nil {
		h.editMessage(chatID, msgID, "Error loading available slots.", nil)
		log.Printf("Error getting slots: %v", err)
		return
	}

	if len(slots) == 0 {
		h.editMessage(chatID, msgID, fmt.Sprintf("No available slots in %s for %s.", roomLabel(room), date.Format("Jan 2, 2006")), nil)
		return
	}

	keyboard := buildTimeKeyboard(slots, room, false)
	h.editMessage(chatID, msgID, fmt.Sprintf("%s — Available slots for %s:", roomLabel(room), date.Format("Jan 2, 2006")), &keyboard)
}

func (h *Handler) onQuickRoomSelected(chatID int64, msgID int, userID int64, username string, data string) {
	room := strings.TrimPrefix(data, "quickroom:")

	now := nowHKT()
	slotStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, hkt)

	ctx := context.Background()

	hasBooking, err := h.service.UserHasBookingForSlot(ctx, userID, slotStart)
	if err == nil && hasBooking {
		h.editMessage(chatID, msgID, "✅ You already have a booking for this hour. Use 📋 My Bookings to view it.", nil)
		return
	}

	machine, err := h.service.GetFreeMachineForSlot(ctx, slotStart, room)
	if err != nil {
		h.editMessage(chatID, msgID, fmt.Sprintf("😔 No machines available in %s right now. Try 📅 Book a Session.", roomLabel(room)), nil)
		return
	}

	endTime := slotStart.Add(model.SessionDuration)
	remaining := int(endTime.Sub(now).Minutes())

	if remaining < 10 {
		nextSlot := slotStart.Add(time.Hour)

		hasNext, _ := h.service.UserHasBookingForSlot(ctx, userID, nextSlot)
		if hasNext {
			h.editMessage(chatID, msgID, "✅ You already have a booking for the next slot. Use 📋 My Bookings to view it.", nil)
			return
		}

		nextMachine, nextErr := h.service.GetFreeMachineForSlot(ctx, nextSlot, room)
		if nextErr != nil {
			h.editMessage(chatID, msgID, fmt.Sprintf("⏱ Only %d min left and the next slot in %s is full.", remaining, roomLabel(room)), nil)
			return
		}
		machine = nextMachine
		slotStart = nextSlot
		endTime = nextSlot.Add(model.SessionDuration)
		remaining = 90
	}

	text := fmt.Sprintf("⚡ Quick Book — %s\n\n  Machine: %s\n  Date:    %s\n  Time:    %s - %s\n  ⏱ %d min remaining\n\nConfirm?",
		roomLabel(room),
		machine.Name,
		slotStart.Format("Jan 2, 2006"),
		slotStart.Format("15:04"),
		endTime.Format("15:04"),
		remaining,
	)

	keyboard := buildConfirmKeyboard(machine.ID, slotStart.Format("2006-01-02T15:04"))
	h.editMessage(chatID, msgID, text, &keyboard)
}

func (h *Handler) onShowMoreSlots(chatID int64, msgID int, data string) {
	// showmore:<room>:<date>
	parts := strings.SplitN(strings.TrimPrefix(data, "showmore:"), ":", 2)
	if len(parts) != 2 {
		h.editMessage(chatID, msgID, "Invalid data.", nil)
		return
	}
	room, dateStr := parts[0], parts[1]

	date, err := time.ParseInLocation("2006-01-02", dateStr, hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid date.", nil)
		return
	}

	ctx := context.Background()
	slots, err := h.service.GetAvailableSlots(ctx, date, room)
	if err != nil {
		h.editMessage(chatID, msgID, "Error loading available slots.", nil)
		log.Printf("Error getting slots: %v", err)
		return
	}

	keyboard := buildTimeKeyboard(slots, room, true)
	h.editMessage(chatID, msgID, fmt.Sprintf("%s — All slots for %s:", roomLabel(room), date.Format("Jan 2, 2006")), &keyboard)
}

func (h *Handler) onBookNow(chatID int64, msgID int, userID int64, username string, data string) {
	// booknow:<room>:<time>
	parts := strings.SplitN(strings.TrimPrefix(data, "booknow:"), ":", 2)
	if len(parts) != 2 {
		h.editMessage(chatID, msgID, "Invalid data.", nil)
		return
	}
	room, timeStr := parts[0], parts[1]

	startTime, err := time.ParseInLocation("2006-01-02T15:04", timeStr, hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid time.", nil)
		return
	}

	now := nowHKT()
	endTime := startTime.Add(model.SessionDuration)
	remaining := endTime.Sub(now)

	ctx := context.Background()
	machine, err := h.service.GetFreeMachineForSlot(ctx, startTime, room)
	if err != nil {
		h.editMessage(chatID, msgID, fmt.Sprintf("Sorry, no machines available in %s right now.", roomLabel(room)), nil)
		return
	}

	text := fmt.Sprintf("⚡ Book Now — %s\n\n  Machine: %s\n  Date:    %s\n  Time:    %s - %s\n  ⏱ %d min remaining",
		roomLabel(room),
		machine.Name,
		startTime.Format("Jan 2, 2006"),
		startTime.Format("15:04"),
		endTime.Format("15:04"),
		int(remaining.Minutes()),
	)

	keyboard := buildConfirmKeyboard(machine.ID, startTime.Format("2006-01-02T15:04"))
	h.editMessage(chatID, msgID, text, &keyboard)
}

func (h *Handler) onTimeSelected(chatID int64, msgID int, userID int64, username string, data string) {
	// time:<room>:<time>
	parts := strings.SplitN(strings.TrimPrefix(data, "time:"), ":", 2)
	if len(parts) != 2 {
		h.editMessage(chatID, msgID, "Invalid data.", nil)
		return
	}
	room, timeStr := parts[0], parts[1]

	startTime, err := time.ParseInLocation("2006-01-02T15:04", timeStr, hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid time selected.", nil)
		return
	}

	ctx := context.Background()
	machine, err := h.service.GetFreeMachineForSlot(ctx, startTime, room)
	if err != nil {
		h.editMessage(chatID, msgID, "Sorry, this slot is no longer available.", nil)
		return
	}

	text := fmt.Sprintf("Confirm your booking — %s\n\n  Machine: %s\n  Date:    %s\n  Time:    %s - %s",
		roomLabel(room),
		machine.Name,
		startTime.Format("Jan 2, 2006"),
		startTime.Format("15:04"),
		startTime.Add(model.SessionDuration).Format("15:04"),
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

	startTime, err := time.ParseInLocation("2006-01-02T15:04", parts[1], hkt)
	if err != nil {
		h.editMessage(chatID, msgID, "Invalid time.", nil)
		return
	}

	if username == "" {
		username = fmt.Sprintf("user_%d", userID)
	}

	ctx := context.Background()

	hasBooking, _ := h.service.UserHasBookingForSlot(ctx, userID, startTime)
	if hasBooking {
		h.editMessage(chatID, msgID, "✅ You already have a booking for this time slot. Use 📋 My Bookings to view it.", nil)
		return
	}

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
