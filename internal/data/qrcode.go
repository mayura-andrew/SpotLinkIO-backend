package data

import (
    "context"
    "database/sql"
    "time"

    "github.com/google/uuid"
)

type QRCode struct {
    ID        uuid.UUID `json:"id" db:"id"`
    UserID    uuid.UUID `json:"user_id" db:"user_id"`
    VehicleID uuid.UUID `json:"vehicle_id" db:"vehicle_id"`
    Code      string    `json:"code" db:"code"`
    Data      string    `json:"data" db:"data"` // JSON string of embedded data
    ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
    IsActive  bool      `json:"is_active" db:"is_active"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    Version   int       `json:"version" db:"version"`
}

type QRCodeData struct {
    UserProfile UserProfile     `json:"user_profile"`
    Vehicle     VehicleData     `json:"vehicle"`
    QRInfo      QRCodeInfo      `json:"qr_info"`
}

type UserProfile struct {
    ID           uuid.UUID `json:"id"`
    UserName     string    `json:"username"`
    FirstName    *string   `json:"first_name"`
    LastName     *string   `json:"last_name"`
    MobileNumber *string   `json:"mobile_number"`
    Email        string    `json:"email"`
}

type VehicleData struct {
    ID           uuid.UUID `json:"id"`
    LicensePlate string    `json:"license_plate"`
    Make         string    `json:"make"`
    Model        string    `json:"model"`
    Color        string    `json:"color"`
    VehicleType  string    `json:"vehicle_type"`
}

type QRCodeInfo struct {
    Code        string    `json:"code"`
    GeneratedAt time.Time `json:"generated_at"`
    ExpiresAt   time.Time `json:"expires_at"`
    Purpose     string    `json:"purpose"` // "parking", "identification", etc.
}

type QRCodeModel struct {
    DB *sql.DB
}

func (m QRCodeModel) Insert(qrCode *QRCode) error {
    query := `
        INSERT INTO qr_codes (user_id, vehicle_id, code, data, expires_at, is_active)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at, version`

    args := []any{
        qrCode.UserID,
        qrCode.VehicleID,
        qrCode.Code,
        qrCode.Data,
        qrCode.ExpiresAt,
        qrCode.IsActive,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.QueryRowContext(ctx, query, args...).Scan(
        &qrCode.ID,
        &qrCode.CreatedAt,
        &qrCode.Version,
    )

    return err
}

func (m QRCodeModel) GetByCode(code string) (*QRCode, error) {
    query := `
        SELECT id, user_id, vehicle_id, code, data, expires_at, is_active, created_at, version
        FROM qr_codes
        WHERE code = $1 AND is_active = true AND expires_at > CURRENT_TIMESTAMP`

    var qrCode QRCode

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.QueryRowContext(ctx, query, code).Scan(
        &qrCode.ID,
        &qrCode.UserID,
        &qrCode.VehicleID,
        &qrCode.Code,
        &qrCode.Data,
        &qrCode.ExpiresAt,
        &qrCode.IsActive,
        &qrCode.CreatedAt,
        &qrCode.Version,
    )

    if err != nil {
        switch {
        case err == sql.ErrNoRows:
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &qrCode, nil
}

func (m QRCodeModel) DeactivateAllForUser(userID uuid.UUID) error {
    query := `UPDATE qr_codes SET is_active = false WHERE user_id = $1`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.ExecContext(ctx, query, userID)
    return err
}

func (m QRCodeModel) GetActiveForUser(userID uuid.UUID) ([]*QRCode, error) {
    query := `
        SELECT id, user_id, vehicle_id, code, data, expires_at, is_active, created_at, version
        FROM qr_codes
        WHERE user_id = $1 AND is_active = true AND expires_at > CURRENT_TIMESTAMP
        ORDER BY created_at DESC`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    rows, err := m.DB.QueryContext(ctx, query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var qrCodes []*QRCode

    for rows.Next() {
        var qrCode QRCode
        err := rows.Scan(
            &qrCode.ID,
            &qrCode.UserID,
            &qrCode.VehicleID,
            &qrCode.Code,
            &qrCode.Data,
            &qrCode.ExpiresAt,
            &qrCode.IsActive,
            &qrCode.CreatedAt,
            &qrCode.Version,
        )
        if err != nil {
            return nil, err
        }
        qrCodes = append(qrCodes, &qrCode)
    }

    return qrCodes, rows.Err()
}

func (m QRCodeModel) CleanupExpired() error {
    query := `UPDATE qr_codes SET is_active = false WHERE expires_at <= CURRENT_TIMESTAMP`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.ExecContext(ctx, query)
    return err
}