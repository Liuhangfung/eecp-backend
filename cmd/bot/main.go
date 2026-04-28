package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/eecp/booking-bot/internal/booking"
	bothandler "github.com/eecp/booking-bot/internal/bot"
	"github.com/eecp/booking-bot/internal/db"
	"github.com/eecp/booking-bot/internal/notifier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	maxAdvanceDays := getEnvInt("MAX_ADVANCE_DAYS", 7)
	maxActiveBookings := getEnvInt("MAX_ACTIVE_BOOKINGS", 5)
	notifyGroupChatID := getEnvInt64("NOTIFY_GROUP_CHAT_ID", 0)
	notifyEnabled := getEnvBool("NOTIFY_ENABLED", true)

	ctx := context.Background()

	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL")

	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "../../migrations"
	}

	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations applied")

	store := db.NewStore(pool)

	adminIDs := parseAdminIDs(os.Getenv("ADMIN_IDS"))
	if err := store.SeedAdmins(ctx, adminIDs); err != nil {
		log.Printf("Warning: failed to seed admins: %v", err)
	}
	log.Printf("Seeded %d admin(s)", len(adminIDs))

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	log.Printf("Bot authorized as @%s", bot.Self.UserName)

	commands := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "book", Description: "📅 Book an EECP session"},
		tgbotapi.BotCommand{Command: "mybookings", Description: "📋 View your upcoming bookings"},
		tgbotapi.BotCommand{Command: "cancel", Description: "❌ Cancel a booking"},
		tgbotapi.BotCommand{Command: "disclaimer", Description: "📄 Download disclaimer document"},
		tgbotapi.BotCommand{Command: "start", Description: "🏠 Main menu"},
	)
	if _, err := bot.Request(commands); err != nil {
		log.Printf("Warning: failed to set bot commands: %v", err)
	} else {
		log.Println("Bot commands registered")
	}

	groupNotifier := notifier.NewGroupNotifier(bot, notifyGroupChatID, notifyEnabled)

	svc := booking.NewService(store, maxAdvanceDays, maxActiveBookings)

	handler := bothandler.NewHandler(bot, svc, store, groupNotifier, maxAdvanceDays)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	log.Println("Bot is running. Waiting for updates...")

	for update := range updates {
		go handler.HandleUpdate(update)
	}
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvInt64(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func parseAdminIDs(s string) []int64 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			log.Printf("Warning: invalid admin ID '%s': %v", p, err)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
