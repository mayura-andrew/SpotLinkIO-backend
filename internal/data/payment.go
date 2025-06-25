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
	PaymentStatusPending   = "pending"
	PaymentStatusCompleted = "completed"
	PaymentStatusFailed    = "failed"
	PaymentStatusRefunded  = "refunded"
)

const (
	PaymentMethodCard          = "card"
	PaymentMethodCash          = "cash"
	PaymentMethodDigitalWallet = "digital_wallet"
)

type Payment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	ReservationID uuid.UUID `json:"reservation_id" db:"reservation_id"`
	UserID        uuid.UUID `json:"user_id" db:"user_id"`
	Amount        float64   `json:"amount" db:"amount"`
	Currency      string    `json:"currency" db:"currency"`
	PaymentMethod string    `json:"payment_method" db:"payment_method"`
	Status        string    `json:"status" db:"status"`
	TransactionID *string   `json:"transaction_id" db:"transaction_id"`
	PaymentDate   time.Time `json:"payment_date" db:"payment_date"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
	Version       int       `json:"version" db:"version"`
}

func ValidatePayment(v *validator.Validator, payment *Payment) {
	v.Check(payment.Amount > 0, "amount", "must be greater than zero")
	v.Check(payment.Amount <= 100000, "amount", "must not exceed 100,000")

	v.Check(payment.Currency != "", "currency", "must be provided")
	v.Check(len(payment.Currency) == 3, "currency", "must be a valid 3-letter currency code")

	v.Check(validator.PermittedValue(payment.PaymentMethod,
		PaymentMethodCard,
		PaymentMethodCash,
		PaymentMethodDigitalWallet), "payment_method", "must be a valid payment method")

	v.Check(validator.PermittedValue(payment.Status,
		PaymentStatusPending,
		PaymentStatusCompleted,
		PaymentStatusFailed,
		PaymentStatusRefunded), "status", "must be a valid status")
}

type PaymentModel struct {
	DB *sql.DB
}

func (m PaymentModel) Insert(payment *Payment) error {
	query := `
		INSERT INTO payments (reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		payment.ReservationID,
		payment.UserID,
		payment.Amount,
		payment.Currency,
		payment.PaymentMethod,
		payment.Status,
		payment.TransactionID,
		payment.PaymentDate,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&payment.ID,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&payment.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m PaymentModel) Get(id uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date, created_at, updated_at, version
		FROM payments
		WHERE id = $1`

	var payment Payment

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&payment.ID,
		&payment.ReservationID,
		&payment.UserID,
		&payment.Amount,
		&payment.Currency,
		&payment.PaymentMethod,
		&payment.Status,
		&payment.TransactionID,
		&payment.PaymentDate,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&payment.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &payment, nil
}

func (m PaymentModel) GetByReservation(reservationID uuid.UUID) (*Payment, error) {
	query := `
		SELECT id, reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date, created_at, updated_at, version
		FROM payments
		WHERE reservation_id = $1`

	var payment Payment

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, reservationID).Scan(
		&payment.ID,
		&payment.ReservationID,
		&payment.UserID,
		&payment.Amount,
		&payment.Currency,
		&payment.PaymentMethod,
		&payment.Status,
		&payment.TransactionID,
		&payment.PaymentDate,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&payment.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &payment, nil
}

func (m PaymentModel) GetAllForUser(userID uuid.UUID, filters Filters) ([]*Payment, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date, created_at, updated_at, version
		FROM payments
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
	payments := []*Payment{}

	for rows.Next() {
		var payment Payment

		err := rows.Scan(
			&totalRecords,
			&payment.ID,
			&payment.ReservationID,
			&payment.UserID,
			&payment.Amount,
			&payment.Currency,
			&payment.PaymentMethod,
			&payment.Status,
			&payment.TransactionID,
			&payment.PaymentDate,
			&payment.CreatedAt,
			&payment.UpdatedAt,
			&payment.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		payments = append(payments, &payment)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return payments, metadata, nil
}

func (m PaymentModel) GetByStatus(status string, filters Filters) ([]*Payment, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date, created_at, updated_at, version
		FROM payments
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
	payments := []*Payment{}

	for rows.Next() {
		var payment Payment

		err := rows.Scan(
			&totalRecords,
			&payment.ID,
			&payment.ReservationID,
			&payment.UserID,
			&payment.Amount,
			&payment.Currency,
			&payment.PaymentMethod,
			&payment.Status,
			&payment.TransactionID,
			&payment.PaymentDate,
			&payment.CreatedAt,
			&payment.UpdatedAt,
			&payment.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		payments = append(payments, &payment)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return payments, metadata, nil
}

func (m PaymentModel) GetByTransactionID(transactionID string) (*Payment, error) {
	query := `
		SELECT id, reservation_id, user_id, amount, currency, payment_method, status, transaction_id, payment_date, created_at, updated_at, version
		FROM payments
		WHERE transaction_id = $1`

	var payment Payment

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, transactionID).Scan(
		&payment.ID,
		&payment.ReservationID,
		&payment.UserID,
		&payment.Amount,
		&payment.Currency,
		&payment.PaymentMethod,
		&payment.Status,
		&payment.TransactionID,
		&payment.PaymentDate,
		&payment.CreatedAt,
		&payment.UpdatedAt,
		&payment.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &payment, nil
}

func (m PaymentModel) Update(payment *Payment) error {
	query := `
		UPDATE payments
		SET amount = $1, currency = $2, payment_method = $3, status = $4, transaction_id = $5, payment_date = $6, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $7 AND version = $8
		RETURNING updated_at, version`

	args := []any{
		payment.Amount,
		payment.Currency,
		payment.PaymentMethod,
		payment.Status,
		payment.TransactionID,
		payment.PaymentDate,
		payment.ID,
		payment.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&payment.UpdatedAt, &payment.Version)
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

func (m PaymentModel) UpdateStatus(id uuid.UUID, status string, transactionID *string) error {
	query := `
		UPDATE payments
		SET status = $1, transaction_id = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, status, transactionID, id)
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

func (m PaymentModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM payments WHERE id = $1`

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

func (m PaymentModel) GetTotalRevenue(startDate, endDate time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM payments
		WHERE status = $1 AND payment_date BETWEEN $2 AND $3`

	var totalRevenue float64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, PaymentStatusCompleted, startDate, endDate).Scan(&totalRevenue)
	if err != nil {
		return 0, err
	}

	return totalRevenue, nil
}

func (m PaymentModel) GetRevenueByLot(lotID uuid.UUID, startDate, endDate time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(p.amount), 0)
		FROM payments p
		INNER JOIN reservations r ON p.reservation_id = r.id
		WHERE p.status = $1 AND r.parking_lot_id = $2 AND p.payment_date BETWEEN $3 AND $4`

	var totalRevenue float64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, PaymentStatusCompleted, lotID, startDate, endDate).Scan(&totalRevenue)
	if err != nil {
		return 0, err
	}

	return totalRevenue, nil
}
