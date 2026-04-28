# EECP Booking Telegram Bot

A Telegram bot for booking 1-hour EECP machine sessions. Supports 5 machines running 24/7 with inline-button booking, admin management, and group notifications.

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 16+
- A Telegram Bot Token (from [@BotFather](https://t.me/BotFather))

### 1. Configure

```bash
cp .env.example .env
# Edit .env with your bot token, database URL, and admin Telegram IDs
```

### 2. Run with Docker Compose

```bash
docker compose up -d
```

### 3. Run locally (without Docker)

```bash
# Start PostgreSQL separately, then:
go run ./cmd/bot
```

## Bot Commands

### User Commands

| Command | Description |
|---|---|
| `/start` | Welcome message |
| `/book` | Book an EECP session |
| `/mybookings` | View upcoming bookings |
| `/cancel` | Cancel a booking |

### Admin Commands

| Command | Description |
|---|---|
| `/admin` | Admin menu |
| `/allbookings` | View all bookings for a date |
| `/cancelbooking` | Cancel any booking |
| `/togglemachine` | Enable/disable a machine |
| `/stats` | Booking statistics |

## Configuration

| Variable | Description |
|---|---|
| `BOT_TOKEN` | Telegram bot token |
| `DATABASE_URL` | PostgreSQL connection string |
| `ADMIN_IDS` | Comma-separated admin Telegram user IDs |
| `NOTIFY_GROUP_CHAT_ID` | Telegram group chat ID for notifications |
| `NOTIFY_ENABLED` | Enable/disable group notifications |
| `MAX_ADVANCE_DAYS` | How many days ahead users can book (default: 7) |
| `MAX_ACTIVE_BOOKINGS` | Max active bookings per user (default: 5) |
