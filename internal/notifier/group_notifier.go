package notifier

import (
	"fmt"
	"log"

	"github.com/eecp/booking-bot/internal/model"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type GroupNotifier struct {
	bot     *tgbotapi.BotAPI
	chatID  int64
	enabled bool
}

func NewGroupNotifier(bot *tgbotapi.BotAPI, chatID int64, enabled bool) *GroupNotifier {
	return &GroupNotifier{bot: bot, chatID: chatID, enabled: enabled}
}

func (n *GroupNotifier) NotifyNewBooking(b *model.Booking) {
	if !n.enabled || n.chatID == 0 {
		return
	}

	text := fmt.Sprintf(
		"📋 *New EECP Booking*\n\n"+
			"👤 User: @%s\n"+
			"🔧 Machine: %s\n"+
			"📅 Date: %s\n"+
			"🕐 Time: %s \\- %s",
		escapeMarkdown(b.TelegramUsername),
		escapeMarkdown(b.MachineName),
		escapeMarkdown(b.StartTime.Format("Jan 2, 2006")),
		escapeMarkdown(b.StartTime.Format("15:04")),
		escapeMarkdown(b.EndTime.Format("15:04")),
	)

	n.send(text)
}

func (n *GroupNotifier) NotifyCancellation(b *model.Booking, byAdmin bool) {
	if !n.enabled || n.chatID == 0 {
		return
	}

	cancelledBy := "user"
	if byAdmin {
		cancelledBy = "admin"
	}

	text := fmt.Sprintf(
		"❌ *Booking Cancelled*\n\n"+
			"👤 User: @%s\n"+
			"🔧 Machine: %s\n"+
			"📅 Date: %s\n"+
			"🕐 Time: %s \\- %s\n"+
			"🔑 Cancelled by: %s\n\n"+
			"_This slot is now available again\\._",
		escapeMarkdown(b.TelegramUsername),
		escapeMarkdown(b.MachineName),
		escapeMarkdown(b.StartTime.Format("Jan 2, 2006")),
		escapeMarkdown(b.StartTime.Format("15:04")),
		escapeMarkdown(b.EndTime.Format("15:04")),
		cancelledBy,
	)

	n.send(text)
}

func (n *GroupNotifier) NotifyMachineToggle(m *model.Machine) {
	if !n.enabled || n.chatID == 0 {
		return
	}

	status := "ENABLED ✅"
	if !m.IsActive {
		status = "DISABLED 🔴"
	}

	text := fmt.Sprintf(
		"⚙️ *Machine Update*\n\n%s is now *%s*",
		escapeMarkdown(m.Name),
		status,
	)

	n.send(text)
}

func (n *GroupNotifier) send(text string) {
	msg := tgbotapi.NewMessage(n.chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	if _, err := n.bot.Send(msg); err != nil {
		log.Printf("Failed to send group notification: %v", err)
	}
}

func escapeMarkdown(s string) string {
	special := []byte{'_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!'}
	var result []byte
	for i := 0; i < len(s); i++ {
		for _, c := range special {
			if s[i] == c {
				result = append(result, '\\')
				break
			}
		}
		result = append(result, s[i])
	}
	return string(result)
}
