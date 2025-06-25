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
	ReservationStatusPending   = "pending"
	ReservationStatusConfirmed = "confirmed"
	ReservationStatusActive    = "active"
	ReservationStatusCompleted = "completed"
	ReservationStatusCancelled = "cancelled"
	ReservationStatusExpired   = "expired"
)

type Reservation struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	UserID          uuid.UUID  `json:"user_id" db:"user_id"`
	VehicleID       uuid.UUID  `json:"vehicle_id" db:"vehicle_id"`
	ParkingLotID    uuid.UUID  `json:"parking_lot_id" db:"parking_lot_id"`
	ParkingSpotID   *uuid.UUID `json:"parking_spot_id" db:"parking_spot_id"`
	StartTime       time.Time  `json:"start_time" db:"start_time"`
	EndTime         time.Time  `json:"end_time" db:"end_time"`
	ActualStartTime *time.Time `json:"actual_start_time" db:"actual_start_time"`
	ActualEndTime   *time.Time `json:"actual_end_time" db:"actual_end_time"`
	Status          string     `json:"status" db:"status"`
	TotalAmount     float64    `json:"total_amount" db:"total_amount"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	Version         int        `json:"version" db:"version"`
}

func ValidateReservation(v *validator.Validator, reservation *Reservation) {
	v.Check(!reservation.StartTime.IsZero(), "start_time", "must be provided")
	v.Check(!reservation.EndTime.IsZero(), "end_time", "must be provided")
	v.Check(reservation.EndTime.After(reservation.StartTime), "end_time", "must be after start time")
	v.Check(reservation.StartTime.After(time.Now().Add(-5*time.Minute)), "start_time", "cannot be in the past")

	v.Check(validator.PermittedValue(reservation.Status,
		ReservationStatusPending,
		ReservationStatusConfirmed,
		ReservationStatusActive,
		ReservationStatusCompleted,
		ReservationStatusCancelled,
		ReservationStatusExpired), "status", "must be a valid status")

	v.Check(reservation.TotalAmount >= 0, "total_amount", "must not be negative")
	v.Check(reservation.TotalAmount <= 100000, "total_amount", "must not exceed 100,000")
}

type ReservationModel struct {
	DB *sql.DB
}

func (m ReservationModel) Insert(reservation *Reservation) error {
	query := `
		INSERT INTO reservations (user_id, vehicle_id, parking_lot_id, parking_spot_id, start_time, end_time, status, total_amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		reservation.UserID,
		reservation.VehicleID,
		reservation.ParkingLotID,
		reservation.ParkingSpotID,
		reservation.StartTime,
		reservation.EndTime,
		reservation.Status,
		reservation.TotalAmount,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&reservation.ID,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
		&reservation.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m ReservationModel) Get(id uuid.UUID) (*Reservation, error) {
	query := `
		SELECT id, user_id, vehicle_id, parking_lot_id, parking_spot_id, start_time, end_time, actual_start_time, actual_end_time, status, total_amount, created_at, updated_at, version
		FROM reservations
		WHERE id = $1`

	var reservation Reservation

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.VehicleID,
		&reservation.ParkingLotID,
		&reservation.ParkingSpotID,
		&reservation.StartTime,
		&reservation.EndTime,
		&reservation.ActualStartTime,
		&reservation.ActualEndTime,
		&reservation.Status,
		&reservation.TotalAmount,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
		&reservation.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &reservation, nil
}

func (m ReservationModel) GetAllForUser(userID uuid.UUID, filters Filters) ([]*Reservation, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, vehicle_id, parking_lot_id, parking_spot_id, start_time, end_time, actual_start_time, actual_end_time, status, total_amount, created_at, updated_at, version
		FROM reservations
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
	reservations := []*Reservation{}

	for rows.Next() {
		var reservation Reservation

		err := rows.Scan(
			&totalRecords,
			&reservation.ID,
			&reservation.UserID,
			&reservation.VehicleID,
			&reservation.ParkingLotID,
			&reservation.ParkingSpotID,
			&reservation.StartTime,
			&reservation.EndTime,
			&reservation.ActualStartTime,
			&reservation.ActualEndTime,
			&reservation.Status,
			&reservation.TotalAmount,
			&reservation.CreatedAt,
			&reservation.UpdatedAt,
			&reservation.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reservations = append(reservations, &reservation)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reservations, metadata, nil
}

func (m ReservationModel) GetByStatus(status string, filters Filters) ([]*Reservation, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, vehicle_id, parking_lot_id, parking_spot_id, start_time, end_time, actual_start_time, actual_end_time, status, total_amount, created_at, updated_at, version
		FROM reservations
		WHERE status = $1
		ORDER BY %s %s, id ASC
		LIMIT $2 OFFSET $3`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{status, filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	reservations := []*Reservation{}

	for rows.Next() {
		var reservation Reservation

		err := rows.Scan(
			&totalRecords,
			&reservation.ID,
			&reservation.UserID,
			&reservation.VehicleID,
			&reservation.ParkingLotID,
			&reservation.ParkingSpotID,
			&reservation.StartTime,
			&reservation.EndTime,
			&reservation.ActualStartTime,
			&reservation.ActualEndTime,
			&reservation.Status,
			&reservation.TotalAmount,
			&reservation.CreatedAt,
			&reservation.UpdatedAt,
			&reservation.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reservations = append(reservations, &reservation)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reservations, metadata, nil
}

func (m ReservationModel) GetActiveByLot(lotID uuid.UUID) ([]*Reservation, error) {
	query := `
		SELECT id, user_id, vehicle_id, parking_lot_id, parking_spot_id, start_time, end_time, actual_start_time, actual_end_time, status, total_amount, created_at, updated_at, version
		FROM reservations
		WHERE parking_lot_id = $1 AND status IN ($2, $3) AND start_time <= NOW() AND end_time >= NOW()
		ORDER BY start_time ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, lotID, ReservationStatusConfirmed, ReservationStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []*Reservation

	for rows.Next() {
		var reservation Reservation

		err := rows.Scan(
			&reservation.ID,
			&reservation.UserID,
			&reservation.VehicleID,
			&reservation.ParkingLotID,
			&reservation.ParkingSpotID,
			&reservation.StartTime,
			&reservation.EndTime,
			&reservation.ActualStartTime,
			&reservation.ActualEndTime,
			&reservation.Status,
			&reservation.TotalAmount,
			&reservation.CreatedAt,
			&reservation.UpdatedAt,
			&reservation.Version,
		)
		if err != nil {
			return nil, err
		}

		reservations = append(reservations, &reservation)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return reservations, nil
}

func (m ReservationModel) Update(reservation *Reservation) error {
	query := `
		UPDATE reservations
		SET parking_spot_id = $1, start_time = $2, end_time = $3, actual_start_time = $4, actual_end_time = $5, status = $6, total_amount = $7, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $8 AND version = $9
		RETURNING updated_at, version`

	args := []any{
		reservation.ParkingSpotID,
		reservation.StartTime,
		reservation.EndTime,
		reservation.ActualStartTime,
		reservation.ActualEndTime,
		reservation.Status,
		reservation.TotalAmount,
		reservation.ID,
		reservation.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&reservation.UpdatedAt, &reservation.Version)
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

func (m ReservationModel) UpdateStatus(id uuid.UUID, status string) error {
	query := `
		UPDATE reservations
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, status, id)
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

func (m ReservationModel) CheckIn(id uuid.UUID, actualStartTime time.Time) error {
	query := `
		UPDATE reservations
		SET actual_start_time = $1, status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND status = $4`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, actualStartTime, ReservationStatusActive, id, ReservationStatusConfirmed)
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

func (m ReservationModel) CheckOut(id uuid.UUID, actualEndTime time.Time) error {
	query := `
		UPDATE reservations
		SET actual_end_time = $1, status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND status = $4`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, actualEndTime, ReservationStatusCompleted, id, ReservationStatusActive)
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

func (m ReservationModel) Cancel(id uuid.UUID) error {
	query := `
		UPDATE reservations
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status IN ($3, $4)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, ReservationStatusCancelled, id, ReservationStatusPending, ReservationStatusConfirmed)
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

func (m ReservationModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM reservations WHERE id = $1`

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

func (m ReservationModel) ExpireOverdue() error {
	query := `
		UPDATE reservations
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE status = $2 AND end_time < NOW()`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, ReservationStatusExpired, ReservationStatusConfirmed)
	return err
}
