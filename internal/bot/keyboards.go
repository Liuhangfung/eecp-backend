package bot

import (
	"fmt"
	"time"

	"github.com/eecp/booking-bot/internal/model"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func buildDateKeyboard(maxDays int) tgbotapi.InlineKeyboardMarkup {
	now := time.Now()
	var rows [][]tgbotapi.InlineKeyboardButton

	for i := 0; i < maxDays; i++ {
		date := now.AddDate(0, 0, i)
		label := date.Format("Jan 2 (Mon)")
		if i == 0 {
			label = "Today (" + date.Format("Jan 2") + ")"
		} else if i == 1 {
			label = "Tomorrow (" + date.Format("Jan 2") + ")"
		}

		callbackData := fmt.Sprintf("date:%s", date.Format("2006-01-02"))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, callbackData),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildTimeKeyboard(slots []model.SlotAvailability) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, slot := range slots {
		label := fmt.Sprintf("%s - %s  (%d free)",
			slot.StartTime.Format("15:04"),
			slot.EndTime.Format("15:04"),
			slot.FreeCount,
		)
		callbackData := fmt.Sprintf("time:%s", slot.StartTime.Format("2006-01-02T15:04"))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, callbackData),
		))
	}

	if len(rows) == 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("No available slots", "noop"),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildConfirmKeyboard(machineID int, startTime string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", fmt.Sprintf("confirm:%d:%s", machineID, startTime)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_booking_flow"),
		),
	)
}

func buildCancelKeyboard(bookings []model.Booking) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, b := range bookings {
		short := b.ID
		if len(short) > 8 {
			short = short[:8]
		}
		label := fmt.Sprintf("#%s — %s %s-%s",
			short,
			b.StartTime.Format("Jan 2"),
			b.StartTime.Format("15:04"),
			b.EndTime.Format("15:04"),
		)
		callbackData := fmt.Sprintf("cancelsel:%s", b.ID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, callbackData),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildCancelConfirmKeyboard(bookingID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Yes, cancel", fmt.Sprintf("cancelconfirm:%s", bookingID)),
			tgbotapi.NewInlineKeyboardButtonData("Keep it", "cancel_booking_flow"),
		),
	)
}

func buildMachineToggleKeyboard(machines []model.Machine) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for _, m := range machines {
		status := "✅"
		if !m.IsActive {
			status = "🔴"
		}
		label := fmt.Sprintf("%s %s", status, m.Name)
		callbackData := fmt.Sprintf("toggle:%d", m.ID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, callbackData),
		))
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
