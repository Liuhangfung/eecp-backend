package booking

import (
	"context"
	"fmt"
	"time"

	"github.com/eecp/booking-bot/internal/db"
	"github.com/eecp/booking-bot/internal/model"
)

var hkt = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		loc = time.FixedZone("HKT", 8*60*60)
	}
	return loc
}()

type Service struct {
	store             *db.Store
	maxAdvanceDays    int
	maxActiveBookings int
}

func NewService(store *db.Store, maxAdvanceDays, maxActiveBookings int) *Service {
	return &Service{
		store:             store,
		maxAdvanceDays:    maxAdvanceDays,
		maxActiveBookings: maxActiveBookings,
	}
}

func (s *Service) GetAvailableSlots(ctx context.Context, date time.Time, room string) ([]model.SlotAvailability, error) {
	activeMachines, err := s.store.CountActiveMachines(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("count active machines: %w", err)
	}

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, hkt)
	end := start.Add(24 * time.Hour)

	now := time.Now().In(hkt)
	if start.Before(now) {
		start = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, hkt)
	}

	bookedCounts, err := s.store.GetBookedCountsByHour(ctx, start, end, room)
	if err != nil {
		return nil, fmt.Errorf("get booked counts: %w", err)
	}

	var slots []model.SlotAvailability
	for t := start; t.Before(end); t = t.Add(time.Hour) {
		booked := bookedCounts[t.Hour()]
		free := activeMachines - booked
		if free > 0 {
			slots = append(slots, model.SlotAvailability{
				StartTime:  t,
				EndTime:    t.Add(time.Hour),
				FreeCount:  free,
				TotalCount: activeMachines,
			})
		}
	}

	return slots, nil
}

func (s *Service) GetFreeMachineForSlot(ctx context.Context, startTime time.Time, room string) (*model.Machine, error) {
	return s.store.GetFreeMachineForSlot(ctx, startTime, room)
}

func (s *Service) UserHasBookingForSlot(ctx context.Context, userID int64, startTime time.Time) (bool, error) {
	return s.store.UserHasBookingForSlot(ctx, userID, startTime)
}

func (s *Service) CreateBooking(ctx context.Context, machineID int, userID int64, username string, startTime time.Time) (*model.Booking, error) {
	active, err := s.store.CountActiveBookingsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count active bookings: %w", err)
	}
	if active >= s.maxActiveBookings {
		return nil, fmt.Errorf("you already have %d active bookings (max: %d)", active, s.maxActiveBookings)
	}

	now := time.Now().In(hkt)
	maxDate := time.Date(now.Year(), now.Month(), now.Day()+s.maxAdvanceDays, 23, 59, 59, 0, hkt)
	if startTime.After(maxDate) {
		return nil, fmt.Errorf("cannot book more than %d days in advance", s.maxAdvanceDays)
	}

	currentSlotStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, hkt)
	if startTime.Before(currentSlotStart) {
		return nil, fmt.Errorf("cannot book a slot in the past")
	}

	booking, err := s.store.CreateBooking(ctx, machineID, userID, username, startTime)
	if err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}

	return booking, nil
}

func (s *Service) GetUserBookings(ctx context.Context, userID int64) ([]model.Booking, error) {
	return s.store.GetUserBookings(ctx, userID)
}

func (s *Service) CancelBooking(ctx context.Context, bookingID string, userID int64) (*model.Booking, error) {
	return s.store.CancelBooking(ctx, bookingID, userID)
}

func (s *Service) CancelBookingByAdmin(ctx context.Context, bookingID string) (*model.Booking, error) {
	return s.store.CancelBookingByAdmin(ctx, bookingID)
}

func (s *Service) GetAllBookingsForDate(ctx context.Context, date time.Time) ([]model.Booking, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, hkt)
	end := start.Add(24 * time.Hour)
	return s.store.GetAllBookingsForRange(ctx, start, end)
}

func (s *Service) GetMachines(ctx context.Context) ([]model.Machine, error) {
	return s.store.GetMachines(ctx)
}

func (s *Service) ToggleMachine(ctx context.Context, machineID int) (*model.Machine, error) {
	return s.store.ToggleMachine(ctx, machineID)
}

func (s *Service) GetStats(ctx context.Context, days int) (*model.Stats, error) {
	return s.store.GetStats(ctx, days)
}
