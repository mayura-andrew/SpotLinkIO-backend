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

var (
	ErrDuplicateLicensePlate = errors.New("duplicate license plate")
)

type Vehicle struct {
	ID           uuid.UUID `json:"id" db:"id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	LicensePlate string    `json:"license_plate" db:"license_plate"`
	Make         string    `json:"make" db:"make"`
	Model        string    `json:"model" db:"model"`
	Color        string    `json:"color" db:"color"`
	VehicleType  string    `json:"vehicle_type" db:"vehicle_type"` // car, motorcycle, truck, etc.
	IsDefault    bool      `json:"is_default" db:"is_default"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	Version      int       `json:"version" db:"version"`
}

func ValidateVehicle(v *validator.Validator, vehicle *Vehicle) {
	v.Check(vehicle.LicensePlate != "", "license_plate", "must be provided")
	v.Check(len(vehicle.LicensePlate) <= 20, "license_plate", "must not be more than 20 characters long")

	v.Check(vehicle.Make != "", "make", "must be provided")
	v.Check(len(vehicle.Make) <= 50, "make", "must not be more than 50 characters long")

	v.Check(vehicle.Model != "", "model", "must be provided")
	v.Check(len(vehicle.Model) <= 50, "model", "must not be more than 50 characters long")

	v.Check(vehicle.Color != "", "color", "must be provided")
	v.Check(len(vehicle.Color) <= 30, "color", "must not be more than 30 characters long")

	v.Check(validator.PermittedValue(vehicle.VehicleType, "car", "motorcycle", "truck", "suv", "van"), "vehicle_type", "must be a valid vehicle type")
}

type VehicleModel struct {
	DB *sql.DB
}

func (m VehicleModel) Insert(vehicle *Vehicle) error {
	query := `
		INSERT INTO vehicles (user_id, license_plate, make, model, color, vehicle_type, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		vehicle.UserID,
		vehicle.LicensePlate,
		vehicle.Make,
		vehicle.Model,
		vehicle.Color,
		vehicle.VehicleType,
		vehicle.IsDefault,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&vehicle.ID,
		&vehicle.CreatedAt,
		&vehicle.UpdatedAt,
		&vehicle.Version,
	)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "vehicles_license_plate_key"`:
			return ErrDuplicateLicensePlate
		default:
			return err
		}
	}

	// If this is set as default, unset all other vehicles for this user
	if vehicle.IsDefault {
		err = m.UnsetDefaultForUser(vehicle.UserID, vehicle.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m VehicleModel) Get(id uuid.UUID) (*Vehicle, error) {
	query := `
		SELECT id, user_id, license_plate, make, model, color, vehicle_type, is_default, created_at, updated_at, version
		FROM vehicles
		WHERE id = $1`

	var vehicle Vehicle

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&vehicle.ID,
		&vehicle.UserID,
		&vehicle.LicensePlate,
		&vehicle.Make,
		&vehicle.Model,
		&vehicle.Color,
		&vehicle.VehicleType,
		&vehicle.IsDefault,
		&vehicle.CreatedAt,
		&vehicle.UpdatedAt,
		&vehicle.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &vehicle, nil
}

func (m VehicleModel) GetAllForUser(userID uuid.UUID, filters Filters) ([]*Vehicle, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, license_plate, make, model, color, vehicle_type, is_default, created_at, updated_at, version
		FROM vehicles
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
	vehicles := []*Vehicle{}

	for rows.Next() {
		var vehicle Vehicle

		err := rows.Scan(
			&totalRecords,
			&vehicle.ID,
			&vehicle.UserID,
			&vehicle.LicensePlate,
			&vehicle.Make,
			&vehicle.Model,
			&vehicle.Color,
			&vehicle.VehicleType,
			&vehicle.IsDefault,
			&vehicle.CreatedAt,
			&vehicle.UpdatedAt,
			&vehicle.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		vehicles = append(vehicles, &vehicle)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return vehicles, metadata, nil
}

func (m VehicleModel) GetByLicensePlate(licensePlate string) (*Vehicle, error) {
	query := `
		SELECT id, user_id, license_plate, make, model, color, vehicle_type, is_default, created_at, updated_at, version
		FROM vehicles
		WHERE license_plate = $1`

	var vehicle Vehicle

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, licensePlate).Scan(
		&vehicle.ID,
		&vehicle.UserID,
		&vehicle.LicensePlate,
		&vehicle.Make,
		&vehicle.Model,
		&vehicle.Color,
		&vehicle.VehicleType,
		&vehicle.IsDefault,
		&vehicle.CreatedAt,
		&vehicle.UpdatedAt,
		&vehicle.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &vehicle, nil
}

func (m VehicleModel) Update(vehicle *Vehicle) error {
	query := `
		UPDATE vehicles
		SET license_plate = $1, make = $2, model = $3, color = $4, vehicle_type = $5, is_default = $6, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $7 AND version = $8
		RETURNING updated_at, version`

	args := []any{
		vehicle.LicensePlate,
		vehicle.Make,
		vehicle.Model,
		vehicle.Color,
		vehicle.VehicleType,
		vehicle.IsDefault,
		vehicle.ID,
		vehicle.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&vehicle.UpdatedAt, &vehicle.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "vehicles_license_plate_key"`:
			return ErrDuplicateLicensePlate
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	// If this is set as default, unset all other vehicles for this user
	if vehicle.IsDefault {
		err = m.UnsetDefaultForUser(vehicle.UserID, vehicle.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m VehicleModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM vehicles WHERE id = $1`

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

func (m VehicleModel) SetAsDefault(userID, vehicleID uuid.UUID) error {
	// First, unset all defaults for the user
	query1 := `UPDATE vehicles SET is_default = false WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query1, userID)
	if err != nil {
		return err
	}

	// Then set the specified vehicle as default
	query2 := `UPDATE vehicles SET is_default = true WHERE id = $1 AND user_id = $2`

	result, err := m.DB.ExecContext(ctx, query2, vehicleID, userID)
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

func (m VehicleModel) UnsetDefaultForUser(userID, exceptVehicleID uuid.UUID) error {
	query := `UPDATE vehicles SET is_default = false WHERE user_id = $1 AND id != $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, exceptVehicleID)
	return err
}
