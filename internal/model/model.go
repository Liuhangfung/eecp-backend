package model

import "time"

type Machine struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type BookingStatus string

const (
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
)

type Booking struct {
	ID               string        `json:"id"`
	MachineID        int           `json:"machine_id"`
	TelegramUserID   int64         `json:"telegram_user_id"`
	TelegramUsername string        `json:"telegram_username"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	Status           BookingStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	MachineName      string        `json:"machine_name,omitempty"`
}

type SlotAvailability struct {
	StartTime  time.Time
	EndTime    time.Time
	FreeCount  int
	TotalCount int
}

type Stats struct {
	TotalBookings  int     `json:"total_bookings"`
	Cancellations  int     `json:"cancellations"`
	BusiestMachine string  `json:"busiest_machine"`
	BusiestHour    string  `json:"busiest_hour"`
	AvgDailyUsage  float64 `json:"avg_daily_usage"`
}
