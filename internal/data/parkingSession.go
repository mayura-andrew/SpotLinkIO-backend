package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/validator"
)

const (
	SessionStatusActive    = "active"
	SessionStatusCompleted = "completed"
	SessionStatusViolated  = "violated"
)

type ParkingSession struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	ReservationID *uuid.UUID `json:"reservation_id" db:"reservation_id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	VehicleID     uuid.UUID  `json:"vehicle_id" db:"vehicle_id"`
	ParkingSpotID uuid.UUID  `json:"parking_spot_id" db:"parking_spot_id"`
	CheckInTime   time.Time  `json:"check_in_time" db:"check_in_time"`
	CheckOutTime  *time.Time `json:"check_out_time" db:"check_out_time"`
	Status        string     `json:"status" db:"status"`
	TotalDuration *int       `json:"total_duration" db:"total_duration"` // in minutes
	TotalAmount   *float64   `json:"total_amount" db:"total_amount"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	Version       int        `json:"version" db:"version"`
}

func ValidateParkingSession(v *validator.Validator, session *ParkingSession) {
	v.Check(!session.CheckInTime.IsZero(), "check_in_time", "must be provided")

	if session.CheckOutTime != nil {
		v.Check(session.CheckOutTime.After(session.CheckInTime), "check_out_time", "must be after check-in time")
	}

	v.Check(validator.PermittedValue(session.Status,
		SessionStatusActive,
		SessionStatusCompleted,
		SessionStatusViolated), "status", "must be a valid status")

	if session.TotalDuration != nil {
		v.Check(*session.TotalDuration >= 0, "total_duration", "must not be negative")
	}

	if session.TotalAmount != nil {
		v.Check(*session.TotalAmount >= 0, "total_amount", "must not be negative")
		v.Check(*session.TotalAmount <= 100000, "total_amount", "must not exceed 100,000")
	}
}

type ParkingSessionModel struct {
	DB *sql.DB
}

func (m ParkingSessionModel) Insert(session *ParkingSession) error {
	query := `
		INSERT INTO parking_sessions (reservation_id, user_id, vehicle_id, parking_spot_id, check_in_time, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		session.ReservationID,
		session.UserID,
		session.VehicleID,
		session.ParkingSpotID,
		session.CheckInTime,
		session.Status,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&session.ID,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m ParkingSessionModel) Get(id uuid.UUID) (*ParkingSession, error) {
	query := `
		SELECT id, reservation_id, user_id, vehicle_id, parking_spot_id, check_in_time, check_out_time, status, total_duration, total_amount, created_at, updated_at, version
		FROM parking_sessions
		WHERE id = $1`

	var session ParkingSession

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&session.ReservationID,
		&session.UserID,
		&session.VehicleID,
		&session.ParkingSpotID,
		&session.CheckInTime,
		&session.CheckOutTime,
		&session.Status,
		&session.TotalDuration,
		&session.TotalAmount,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &session, nil
}

func (m ParkingSessionModel) GetAllForUser(userID uuid.UUID, filters Filters) ([]*ParkingSession, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, reservation_id, user_id, vehicle_id, parking_spot_id, check_in_time, check_out_time, status, total_duration, total_amount, created_at, updated_at, version
		FROM parking_sessions
		WHERE user_id = $1
		ORDER BY %s %s, id ASC
		LIMIT $2 OFFSET $3`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{userID, filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	sessions := []*ParkingSession{}

	for rows.Next() {
		var session ParkingSession

		err := rows.Scan(
			&totalRecords,
			&session.ID,
			&session.ReservationID,
			&session.UserID,
			&session.VehicleID,
			&session.ParkingSpotID,
			&session.CheckInTime,
			&session.CheckOutTime,
			&session.Status,
			&session.TotalDuration,
			&session.TotalAmount,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return sessions, metadata, nil
}

func (m ParkingSessionModel) GetActiveBySpot(spotID uuid.UUID) (*ParkingSession, error) {
	query := `
		SELECT id, reservation_id, user_id, vehicle_id, parking_spot_id, check_in_time, check_out_time, status, total_duration, total_amount, created_at, updated_at, version
		FROM parking_sessions
		WHERE parking_spot_id = $1 AND status = $2`

	var session ParkingSession

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, spotID, SessionStatusActive).Scan(
		&session.ID,
		&session.ReservationID,
		&session.UserID,
		&session.VehicleID,
		&session.ParkingSpotID,
		&session.CheckInTime,
		&session.CheckOutTime,
		&session.Status,
		&session.TotalDuration,
		&session.TotalAmount,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &session, nil
}

func (m ParkingSessionModel) GetActiveByUser(userID uuid.UUID) ([]*ParkingSession, error) {
	query := `
		SELECT id, reservation_id, user_id, vehicle_id, parking_spot_id, check_in_time, check_out_time, status, total_duration, total_amount, created_at, updated_at, version
		FROM parking_sessions
		WHERE user_id = $1 AND status = $2
		ORDER BY check_in_time DESC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, SessionStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*ParkingSession

	for rows.Next() {
		var session ParkingSession

		err := rows.Scan(
			&session.ID,
			&session.ReservationID,
			&session.UserID,
			&session.VehicleID,
			&session.ParkingSpotID,
			&session.CheckInTime,
			&session.CheckOutTime,
			&session.Status,
			&session.TotalDuration,
			&session.TotalAmount,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Version,
		)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (m ParkingSessionModel) GetByLot(lotID uuid.UUID, filters Filters) ([]*ParkingSession, Metadata, error) {
	query := `
		SELECT count(*) OVER(), ps.id, ps.reservation_id, ps.user_id, ps.vehicle_id, ps.parking_spot_id, ps.check_in_time, ps.check_out_time, ps.status, ps.total_duration, ps.total_amount, ps.created_at, ps.updated_at, ps.version
		FROM parking_sessions ps
		INNER JOIN parking_spots spot ON ps.parking_spot_id = spot.id
		WHERE spot.parking_lot_id = $1
		ORDER BY %s %s, ps.id ASC
		LIMIT $2 OFFSET $3`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{lotID, filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	sessions := []*ParkingSession{}

	for rows.Next() {
		var session ParkingSession

		err := rows.Scan(
			&totalRecords,
			&session.ID,
			&session.ReservationID,
			&session.UserID,
			&session.VehicleID,
			&session.ParkingSpotID,
			&session.CheckInTime,
			&session.CheckOutTime,
			&session.Status,
			&session.TotalDuration,
			&session.TotalAmount,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return sessions, metadata, nil
}

func (m ParkingSessionModel) Update(session *ParkingSession) error {
	query := `
		UPDATE parking_sessions
		SET check_out_time = $1, status = $2, total_duration = $3, total_amount = $4, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING updated_at, version`

	args := []any{
		session.CheckOutTime,
		session.Status,
		session.TotalDuration,
		session.TotalAmount,
		session.ID,
		session.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&session.UpdatedAt, &session.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m ParkingSessionModel) CheckOut(id uuid.UUID, checkOutTime time.Time, totalAmount float64) error {
	// Calculate duration in minutes
	var durationMinutes int
	durationQuery := `SELECT EXTRACT(EPOCH FROM ($1 - check_in_time))/60 FROM parking_sessions WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, durationQuery, checkOutTime, id).Scan(&durationMinutes)
	if err != nil {
		return err
	}

	query := `
		UPDATE parking_sessions
		SET check_out_time = $1, status = $2, total_duration = $3, total_amount = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 AND status = $6`

	result, err := m.DB.ExecContext(ctx, query, checkOutTime, SessionStatusCompleted, durationMinutes, totalAmount, id, SessionStatusActive)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ParkingSessionModel) MarkAsViolation(id uuid.UUID) error {
	query := `
		UPDATE parking_sessions
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, SessionStatusViolated, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ParkingSessionModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM parking_sessions WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m ParkingSessionModel) GetOvertimeSessions() ([]*ParkingSession, error) {
	query := `
		SELECT ps.id, ps.reservation_id, ps.user_id, ps.vehicle_id, ps.parking_spot_id, ps.check_in_time, ps.check_out_time, ps.status, ps.total_duration, ps.total_amount, ps.created_at, ps.updated_at, ps.version
		FROM parking_sessions ps
		LEFT JOIN reservations r ON ps.reservation_id = r.id
		WHERE ps.status = $1 
		AND (
			(r.id IS NOT NULL AND NOW() > r.end_time) OR
			(r.id IS NULL AND ps.check_in_time < NOW() - INTERVAL '24 hours')
		)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, SessionStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*ParkingSession

	for rows.Next() {
		var session ParkingSession

		err := rows.Scan(
			&session.ID,
			&session.ReservationID,
			&session.UserID,
			&session.VehicleID,
			&session.ParkingSpotID,
			&session.CheckInTime,
			&session.CheckOutTime,
			&session.Status,
			&session.TotalDuration,
			&session.TotalAmount,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.Version,
		)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}
