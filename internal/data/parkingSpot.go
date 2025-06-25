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

type ParkingSpot struct {
	ID           uuid.UUID `json:"id" db:"id"`
	ParkingLotID uuid.UUID `json:"parking_lot_id" db:"parking_lot_id"`
	SpotNumber   string    `json:"spot_number" db:"spot_number"`
	SpotType     string    `json:"spot_type" db:"spot_type"` // regular, handicapped, electric, compact
	IsOccupied   bool      `json:"is_occupied" db:"is_occupied"`
	IsReserved   bool      `json:"is_reserved" db:"is_reserved"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	Version      int       `json:"version" db:"version"`
}

func ValidateParkingSpot(v *validator.Validator, spot *ParkingSpot) {
	v.Check(spot.SpotNumber != "", "spot_number", "must be provided")
	v.Check(len(spot.SpotNumber) <= 20, "spot_number", "must not be more than 20 characters long")

	v.Check(validator.PermittedValue(spot.SpotType, "regular", "handicapped", "electric", "compact"), "spot_type", "must be a valid spot type")
}

type ParkingSpotModel struct {
	DB *sql.DB
}

func (m ParkingSpotModel) Insert(spot *ParkingSpot) error {
	query := `
		INSERT INTO parking_spots (parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		spot.ParkingLotID,
		spot.SpotNumber,
		spot.SpotType,
		spot.IsOccupied,
		spot.IsReserved,
		spot.IsActive,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&spot.ID,
		&spot.CreatedAt,
		&spot.UpdatedAt,
		&spot.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m ParkingSpotModel) Get(id uuid.UUID) (*ParkingSpot, error) {
	query := `
		SELECT id, parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active, created_at, updated_at, version
		FROM parking_spots
		WHERE id = $1`

	var spot ParkingSpot

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&spot.ID,
		&spot.ParkingLotID,
		&spot.SpotNumber,
		&spot.SpotType,
		&spot.IsOccupied,
		&spot.IsReserved,
		&spot.IsActive,
		&spot.CreatedAt,
		&spot.UpdatedAt,
		&spot.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &spot, nil
}

func (m ParkingSpotModel) GetAllByLot(lotID uuid.UUID, filters Filters) ([]*ParkingSpot, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active, created_at, updated_at, version
		FROM parking_spots
		WHERE parking_lot_id = $1
		ORDER BY %s %s, id ASC
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
	spots := []*ParkingSpot{}

	for rows.Next() {
		var spot ParkingSpot

		err := rows.Scan(
			&totalRecords,
			&spot.ID,
			&spot.ParkingLotID,
			&spot.SpotNumber,
			&spot.SpotType,
			&spot.IsOccupied,
			&spot.IsReserved,
			&spot.IsActive,
			&spot.CreatedAt,
			&spot.UpdatedAt,
			&spot.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		spots = append(spots, &spot)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return spots, metadata, nil
}

func (m ParkingSpotModel) GetAvailableByLot(lotID uuid.UUID, spotType string) ([]*ParkingSpot, error) {
	var query string
	var args []any

	if spotType != "" {
		query = `
			SELECT id, parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active, created_at, updated_at, version
			FROM parking_spots
			WHERE parking_lot_id = $1 AND spot_type = $2 AND is_active = true AND is_occupied = false AND is_reserved = false
			ORDER BY spot_number ASC`
		args = []any{lotID, spotType}
	} else {
		query = `
			SELECT id, parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active, created_at, updated_at, version
			FROM parking_spots
			WHERE parking_lot_id = $1 AND is_active = true AND is_occupied = false AND is_reserved = false
			ORDER BY spot_number ASC`
		args = []any{lotID}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spots []*ParkingSpot

	for rows.Next() {
		var spot ParkingSpot

		err := rows.Scan(
			&spot.ID,
			&spot.ParkingLotID,
			&spot.SpotNumber,
			&spot.SpotType,
			&spot.IsOccupied,
			&spot.IsReserved,
			&spot.IsActive,
			&spot.CreatedAt,
			&spot.UpdatedAt,
			&spot.Version,
		)
		if err != nil {
			return nil, err
		}

		spots = append(spots, &spot)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return spots, nil
}

func (m ParkingSpotModel) Update(spot *ParkingSpot) error {
	query := `
		UPDATE parking_spots
		SET spot_number = $1, spot_type = $2, is_occupied = $3, is_reserved = $4, is_active = $5, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $6 AND version = $7
		RETURNING updated_at, version`

	args := []any{
		spot.SpotNumber,
		spot.SpotType,
		spot.IsOccupied,
		spot.IsReserved,
		spot.IsActive,
		spot.ID,
		spot.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&spot.UpdatedAt, &spot.Version)
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

func (m ParkingSpotModel) SetOccupied(spotID uuid.UUID, occupied bool) error {
	query := `
		UPDATE parking_spots
		SET is_occupied = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, occupied, spotID)
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

func (m ParkingSpotModel) SetReserved(spotID uuid.UUID, reserved bool) error {
	query := `
		UPDATE parking_spots
		SET is_reserved = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, reserved, spotID)
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

func (m ParkingSpotModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM parking_spots WHERE id = $1`

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

func (m ParkingSpotModel) BulkCreate(lotID uuid.UUID, spots []ParkingSpot) error {
	query := `
		INSERT INTO parking_spots (parking_lot_id, spot_number, spot_type, is_occupied, is_reserved, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, spot := range spots {
		_, err = stmt.ExecContext(ctx,
			lotID,
			spot.SpotNumber,
			spot.SpotType,
			spot.IsOccupied,
			spot.IsReserved,
			spot.IsActive,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
