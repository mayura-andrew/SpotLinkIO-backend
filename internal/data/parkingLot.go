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

type ParkingLot struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Address     string    `json:"address" db:"address"`
	Latitude    float64   `json:"latitude" db:"latitude"`
	Longitude   float64   `json:"longitude" db:"longitude"`
	TotalSpots  int       `json:"total_spots" db:"total_spots"`
	HourlyRate  float64   `json:"hourly_rate" db:"hourly_rate"`
	DailyRate   *float64  `json:"daily_rate" db:"daily_rate"`
	MonthlyRate *float64  `json:"monthly_rate" db:"monthly_rate"`
	OpenTime    string    `json:"open_time" db:"open_time"`
	CloseTime   string    `json:"close_time" db:"close_time"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	OwnerID     uuid.UUID `json:"owner_id" db:"owner_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Version     int       `json:"version" db:"version"`
}

func ValidateParkingLot(v *validator.Validator, lot *ParkingLot) {
	v.Check(lot.Name != "", "name", "must be provided")
	v.Check(len(lot.Name) <= 100, "name", "must not be more than 100 characters long")

	v.Check(lot.Address != "", "address", "must be provided")
	v.Check(len(lot.Address) <= 255, "address", "must not be more than 255 characters long")

	v.Check(lot.Latitude >= -90 && lot.Latitude <= 90, "latitude", "must be between -90 and 90")
	v.Check(lot.Longitude >= -180 && lot.Longitude <= 180, "longitude", "must be between -180 and 180")

	v.Check(lot.TotalSpots > 0, "total_spots", "must be greater than zero")
	v.Check(lot.TotalSpots <= 10000, "total_spots", "must not exceed 10,000")

	v.Check(lot.HourlyRate >= 0, "hourly_rate", "must not be negative")
	v.Check(lot.HourlyRate <= 1000, "hourly_rate", "must not exceed 1000")

	if lot.DailyRate != nil {
		v.Check(*lot.DailyRate >= 0, "daily_rate", "must not be negative")
		v.Check(*lot.DailyRate <= 10000, "daily_rate", "must not exceed 10,000")
	}

	if lot.MonthlyRate != nil {
		v.Check(*lot.MonthlyRate >= 0, "monthly_rate", "must not be negative")
		v.Check(*lot.MonthlyRate <= 100000, "monthly_rate", "must not exceed 100,000")
	}

	v.Check(lot.OpenTime != "", "open_time", "must be provided")
	v.Check(lot.CloseTime != "", "close_time", "must be provided")
}

type ParkingLotModel struct {
	DB *sql.DB
}

func (m ParkingLotModel) Insert(lot *ParkingLot) error {
	query := `
		INSERT INTO parking_lots (name, address, latitude, longitude, total_spots, hourly_rate, daily_rate, monthly_rate, open_time, close_time, is_active, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		lot.Name,
		lot.Address,
		lot.Latitude,
		lot.Longitude,
		lot.TotalSpots,
		lot.HourlyRate,
		lot.DailyRate,
		lot.MonthlyRate,
		lot.OpenTime,
		lot.CloseTime,
		lot.IsActive,
		lot.OwnerID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&lot.ID,
		&lot.CreatedAt,
		&lot.UpdatedAt,
		&lot.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m ParkingLotModel) Get(id uuid.UUID) (*ParkingLot, error) {
	query := `
		SELECT id, name, address, latitude, longitude, total_spots, hourly_rate, daily_rate, monthly_rate, open_time, close_time, is_active, owner_id, created_at, updated_at, version
		FROM parking_lots
		WHERE id = $1`

	var lot ParkingLot

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&lot.ID,
		&lot.Name,
		&lot.Address,
		&lot.Latitude,
		&lot.Longitude,
		&lot.TotalSpots,
		&lot.HourlyRate,
		&lot.DailyRate,
		&lot.MonthlyRate,
		&lot.OpenTime,
		&lot.CloseTime,
		&lot.IsActive,
		&lot.OwnerID,
		&lot.CreatedAt,
		&lot.UpdatedAt,
		&lot.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &lot, nil
}

func (m ParkingLotModel) GetAll(filters Filters) ([]*ParkingLot, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, name, address, latitude, longitude, total_spots, hourly_rate, daily_rate, monthly_rate, open_time, close_time, is_active, owner_id, created_at, updated_at, version
		FROM parking_lots
		WHERE is_active = true
		ORDER BY %s %s, id ASC
		LIMIT $1 OFFSET $2`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	lots := []*ParkingLot{}

	for rows.Next() {
		var lot ParkingLot

		err := rows.Scan(
			&totalRecords,
			&lot.ID,
			&lot.Name,
			&lot.Address,
			&lot.Latitude,
			&lot.Longitude,
			&lot.TotalSpots,
			&lot.HourlyRate,
			&lot.DailyRate,
			&lot.MonthlyRate,
			&lot.OpenTime,
			&lot.CloseTime,
			&lot.IsActive,
			&lot.OwnerID,
			&lot.CreatedAt,
			&lot.UpdatedAt,
			&lot.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		lots = append(lots, &lot)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return lots, metadata, nil
}

func (m ParkingLotModel) GetByOwner(ownerID uuid.UUID, filters Filters) ([]*ParkingLot, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, name, address, latitude, longitude, total_spots, hourly_rate, daily_rate, monthly_rate, open_time, close_time, is_active, owner_id, created_at, updated_at, version
		FROM parking_lots
		WHERE owner_id = $1
		ORDER BY %s %s, id ASC
		LIMIT $2 OFFSET $3`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{ownerID, filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	lots := []*ParkingLot{}

	for rows.Next() {
		var lot ParkingLot

		err := rows.Scan(
			&totalRecords,
			&lot.ID,
			&lot.Name,
			&lot.Address,
			&lot.Latitude,
			&lot.Longitude,
			&lot.TotalSpots,
			&lot.HourlyRate,
			&lot.DailyRate,
			&lot.MonthlyRate,
			&lot.OpenTime,
			&lot.CloseTime,
			&lot.IsActive,
			&lot.OwnerID,
			&lot.CreatedAt,
			&lot.UpdatedAt,
			&lot.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		lots = append(lots, &lot)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return lots, metadata, nil
}

func (m ParkingLotModel) SearchByLocation(lat, lng, radiusKm float64, filters Filters) ([]*ParkingLot, Metadata, error) {
	// Using Haversine formula for distance calculation
	query := `
		SELECT count(*) OVER(), id, name, address, latitude, longitude, total_spots, hourly_rate, daily_rate, monthly_rate, open_time, close_time, is_active, owner_id, created_at, updated_at, version,
		(6371 * acos(cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2)) + sin(radians($1)) * sin(radians(latitude)))) AS distance
		FROM parking_lots
		WHERE is_active = true
		HAVING distance <= $3
		ORDER BY distance ASC, %s %s
		LIMIT $4 OFFSET $5`

	query = fmt.Sprintf(query, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{lat, lng, radiusKm, filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	lots := []*ParkingLot{}

	for rows.Next() {
		var lot ParkingLot
		var distance float64

		err := rows.Scan(
			&totalRecords,
			&lot.ID,
			&lot.Name,
			&lot.Address,
			&lot.Latitude,
			&lot.Longitude,
			&lot.TotalSpots,
			&lot.HourlyRate,
			&lot.DailyRate,
			&lot.MonthlyRate,
			&lot.OpenTime,
			&lot.CloseTime,
			&lot.IsActive,
			&lot.OwnerID,
			&lot.CreatedAt,
			&lot.UpdatedAt,
			&lot.Version,
			&distance,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		lots = append(lots, &lot)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return lots, metadata, nil
}

func (m ParkingLotModel) Update(lot *ParkingLot) error {
	query := `
		UPDATE parking_lots
		SET name = $1, address = $2, latitude = $3, longitude = $4, total_spots = $5, hourly_rate = $6, daily_rate = $7, monthly_rate = $8, open_time = $9, close_time = $10, is_active = $11, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $12 AND version = $13
		RETURNING updated_at, version`

	args := []any{
		lot.Name,
		lot.Address,
		lot.Latitude,
		lot.Longitude,
		lot.TotalSpots,
		lot.HourlyRate,
		lot.DailyRate,
		lot.MonthlyRate,
		lot.OpenTime,
		lot.CloseTime,
		lot.IsActive,
		lot.ID,
		lot.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&lot.UpdatedAt, &lot.Version)
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

func (m ParkingLotModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM parking_lots WHERE id = $1`

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

func (m ParkingLotModel) GetAvailableSpots(lotID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM parking_spots
		WHERE parking_lot_id = $1 AND is_active = true AND is_occupied = false AND is_reserved = false`

	var availableSpots int

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, lotID).Scan(&availableSpots)
	if err != nil {
		return 0, err
	}

	return availableSpots, nil
}
