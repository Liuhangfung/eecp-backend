package db

import (
	"context"
	"fmt"
	"time"

	"github.com/eecp/booking-bot/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CountActiveMachines(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM machines WHERE is_active = true").Scan(&count)
	return count, err
}

func (s *Store) GetBookedCountsByHour(ctx context.Context, start, end time.Time) (map[int]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT EXTRACT(HOUR FROM start_time)::int AS hour, COUNT(*)
		FROM bookings
		WHERE status = 'confirmed'
		  AND start_time >= $1
		  AND start_time < $2
		GROUP BY hour
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var hour, count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, err
		}
		counts[hour] = count
	}
	return counts, rows.Err()
}

func (s *Store) GetFreeMachineForSlot(ctx context.Context, startTime time.Time) (*model.Machine, error) {
	var m model.Machine
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.name, m.is_active
		FROM machines m
		WHERE m.is_active = true
		  AND m.id NOT IN (
		      SELECT machine_id FROM bookings
		      WHERE status = 'confirmed' AND start_time = $1
		  )
		ORDER BY m.id
		LIMIT 1
	`, startTime).Scan(&m.ID, &m.Name, &m.IsActive)
	if err != nil {
		return nil, fmt.Errorf("no free machine available for this slot")
	}
	return &m, nil
}

func (s *Store) UserHasBookingForSlot(ctx context.Context, userID int64, startTime time.Time) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings
			WHERE telegram_user_id = $1 AND start_time = $2 AND status = 'confirmed'
		)
	`, userID, startTime).Scan(&exists)
	return exists, err
}

func (s *Store) CreateBooking(ctx context.Context, machineID int, userID int64, username string, startTime time.Time) (*model.Booking, error) {
	endTime := startTime.Add(time.Hour)
	b := &model.Booking{}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO bookings (machine_id, telegram_user_id, telegram_username, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5, 'confirmed')
		RETURNING id, machine_id, telegram_user_id, telegram_username, start_time, end_time, status, created_at
	`, machineID, userID, username, startTime, endTime).Scan(
		&b.ID, &b.MachineID, &b.TelegramUserID, &b.TelegramUsername,
		&b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	var machineName string
	_ = s.pool.QueryRow(ctx, "SELECT name FROM machines WHERE id=$1", machineID).Scan(&machineName)
	b.MachineName = machineName

	return b, nil
}

func (s *Store) CountActiveBookingsForUser(ctx context.Context, userID int64) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE telegram_user_id = $1
		  AND status = 'confirmed'
		  AND start_time > now()
	`, userID).Scan(&count)
	return count, err
}

func (s *Store) GetUserBookings(ctx context.Context, userID int64) ([]model.Booking, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.machine_id, b.telegram_user_id, b.telegram_username,
		       b.start_time, b.end_time, b.status, b.created_at, m.name
		FROM bookings b
		JOIN machines m ON m.id = b.machine_id
		WHERE b.telegram_user_id = $1
		  AND b.status = 'confirmed'
		  AND b.start_time > now()
		ORDER BY b.start_time
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(&b.ID, &b.MachineID, &b.TelegramUserID, &b.TelegramUsername,
			&b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt, &b.MachineName); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func (s *Store) CancelBooking(ctx context.Context, bookingID string, userID int64) (*model.Booking, error) {
	b := &model.Booking{}
	err := s.pool.QueryRow(ctx, `
		UPDATE bookings SET status = 'cancelled'
		WHERE id = $1 AND telegram_user_id = $2 AND status = 'confirmed'
		RETURNING id, machine_id, telegram_user_id, telegram_username, start_time, end_time, status, created_at
	`, bookingID, userID).Scan(
		&b.ID, &b.MachineID, &b.TelegramUserID, &b.TelegramUsername,
		&b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("booking not found or already cancelled")
	}

	var machineName string
	_ = s.pool.QueryRow(ctx, "SELECT name FROM machines WHERE id=$1", b.MachineID).Scan(&machineName)
	b.MachineName = machineName

	return b, nil
}

func (s *Store) CancelBookingByAdmin(ctx context.Context, bookingID string) (*model.Booking, error) {
	b := &model.Booking{}
	err := s.pool.QueryRow(ctx, `
		UPDATE bookings SET status = 'cancelled'
		WHERE id = $1 AND status = 'confirmed'
		RETURNING id, machine_id, telegram_user_id, telegram_username, start_time, end_time, status, created_at
	`, bookingID).Scan(
		&b.ID, &b.MachineID, &b.TelegramUserID, &b.TelegramUsername,
		&b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("booking not found or already cancelled")
	}

	var machineName string
	_ = s.pool.QueryRow(ctx, "SELECT name FROM machines WHERE id=$1", b.MachineID).Scan(&machineName)
	b.MachineName = machineName

	return b, nil
}

func (s *Store) GetAllBookingsForRange(ctx context.Context, start, end time.Time) ([]model.Booking, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.machine_id, b.telegram_user_id, b.telegram_username,
		       b.start_time, b.end_time, b.status, b.created_at, m.name
		FROM bookings b
		JOIN machines m ON m.id = b.machine_id
		WHERE b.start_time >= $1 AND b.start_time < $2
		  AND b.status = 'confirmed'
		ORDER BY b.start_time, m.id
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.Booking
	for rows.Next() {
		var b model.Booking
		if err := rows.Scan(&b.ID, &b.MachineID, &b.TelegramUserID, &b.TelegramUsername,
			&b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt, &b.MachineName); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func (s *Store) GetMachines(ctx context.Context) ([]model.Machine, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, name, is_active FROM machines ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []model.Machine
	for rows.Next() {
		var m model.Machine
		if err := rows.Scan(&m.ID, &m.Name, &m.IsActive); err != nil {
			return nil, err
		}
		machines = append(machines, m)
	}
	return machines, rows.Err()
}

func (s *Store) ToggleMachine(ctx context.Context, machineID int) (*model.Machine, error) {
	m := &model.Machine{}
	err := s.pool.QueryRow(ctx, `
		UPDATE machines SET is_active = NOT is_active
		WHERE id = $1
		RETURNING id, name, is_active
	`, machineID).Scan(&m.ID, &m.Name, &m.IsActive)
	if err != nil {
		return nil, fmt.Errorf("machine not found")
	}
	return m, nil
}

func (s *Store) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admins WHERE telegram_user_id=$1)", userID).Scan(&exists)
	return exists, err
}

func (s *Store) SeedAdmins(ctx context.Context, adminIDs []int64) error {
	for _, id := range adminIDs {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO admins (telegram_user_id) VALUES ($1)
			ON CONFLICT DO NOTHING
		`, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetStats(ctx context.Context, days int) (*model.Stats, error) {
	since := time.Now().AddDate(0, 0, -days)
	stats := &model.Stats{}

	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE created_at >= $1 AND status = 'confirmed'
	`, since).Scan(&stats.TotalBookings)
	if err != nil {
		return nil, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE created_at >= $1 AND status = 'cancelled'
	`, since).Scan(&stats.Cancellations)
	if err != nil {
		return nil, err
	}

	_ = s.pool.QueryRow(ctx, `
		SELECT m.name FROM bookings b
		JOIN machines m ON m.id = b.machine_id
		WHERE b.created_at >= $1 AND b.status = 'confirmed'
		GROUP BY m.name
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, since).Scan(&stats.BusiestMachine)

	var busiestHour int
	err = s.pool.QueryRow(ctx, `
		SELECT EXTRACT(HOUR FROM start_time)::int FROM bookings
		WHERE created_at >= $1 AND status = 'confirmed'
		GROUP BY EXTRACT(HOUR FROM start_time)
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, since).Scan(&busiestHour)
	if err == nil {
		stats.BusiestHour = fmt.Sprintf("%02d:00 - %02d:00", busiestHour, busiestHour+1)
	}

	if days > 0 {
		stats.AvgDailyUsage = float64(stats.TotalBookings) / float64(days)
	}

	return stats, nil
}
