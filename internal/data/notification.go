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
	NotificationTypeReservationReminder  = "reservation_reminder"
	NotificationTypePaymentDue           = "payment_due"
	NotificationTypeSessionExpiring      = "session_expiring"
	NotificationTypeReservationConfirmed = "reservation_confirmed"
	NotificationTypeReservationCancelled = "reservation_cancelled"
	NotificationTypePaymentCompleted     = "payment_completed"
	NotificationTypeViolationAlert       = "violation_alert"
)

type Notification struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Type      string    `json:"type" db:"type"`
	Title     string    `json:"title" db:"title"`
	Message   string    `json:"message" db:"message"`
	IsRead    bool      `json:"is_read" db:"is_read"`
	Data      *string   `json:"data" db:"data"` // JSON data for additional context
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func ValidateNotification(v *validator.Validator, notification *Notification) {
	v.Check(notification.Title != "", "title", "must be provided")
	v.Check(len(notification.Title) <= 100, "title", "must not be more than 100 characters long")

	v.Check(notification.Message != "", "message", "must be provided")
	v.Check(len(notification.Message) <= 500, "message", "must not be more than 500 characters long")

	v.Check(validator.PermittedValue(notification.Type,
		NotificationTypeReservationReminder,
		NotificationTypePaymentDue,
		NotificationTypeSessionExpiring,
		NotificationTypeReservationConfirmed,
		NotificationTypeReservationCancelled,
		NotificationTypePaymentCompleted,
		NotificationTypeViolationAlert), "type", "must be a valid notification type")
}

type NotificationModel struct {
	DB *sql.DB
}

func (m NotificationModel) Insert(notification *Notification) error {
	query := `
		INSERT INTO notifications (user_id, type, title, message, is_read, data)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	args := []any{
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Message,
		notification.IsRead,
		notification.Data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&notification.ID,
		&notification.CreatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m NotificationModel) Get(id uuid.UUID) (*Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, is_read, data, created_at
		FROM notifications
		WHERE id = $1`

	var notification Notification

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.IsRead,
		&notification.Data,
		&notification.CreatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &notification, nil
}

func (m NotificationModel) GetAllForUser(userID uuid.UUID, filters Filters) ([]*Notification, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, type, title, message, is_read, data, created_at
		FROM notifications
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
	notifications := []*Notification{}

	for rows.Next() {
		var notification Notification

		err := rows.Scan(
			&totalRecords,
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&notification.IsRead,
			&notification.Data,
			&notification.CreatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		notifications = append(notifications, &notification)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return notifications, metadata, nil
}

func (m NotificationModel) GetUnreadForUser(userID uuid.UUID, limit int) ([]*Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, is_read, data, created_at
		FROM notifications
		WHERE user_id = $1 AND is_read = false
		ORDER BY created_at DESC
		LIMIT $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*Notification

	for rows.Next() {
		var notification Notification

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&notification.IsRead,
			&notification.Data,
			&notification.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		notifications = append(notifications, &notification)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

func (m NotificationModel) GetUnreadCountForUser(userID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`

	var count int

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (m NotificationModel) MarkAsRead(id uuid.UUID) error {
	query := `UPDATE notifications SET is_read = true WHERE id = $1`

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

func (m NotificationModel) MarkAllAsReadForUser(userID uuid.UUID) error {
	query := `UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID)
	return err
}

func (m NotificationModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM notifications WHERE id = $1`

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

func (m NotificationModel) DeleteAllForUser(userID uuid.UUID) error {
	query := `DELETE FROM notifications WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID)
	return err
}

func (m NotificationModel) DeleteOldNotifications(olderThan time.Time) error {
	query := `DELETE FROM notifications WHERE created_at < $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, olderThan)
	return err
}

func (m NotificationModel) BulkInsert(notifications []*Notification) error {
	query := `
		INSERT INTO notifications (user_id, type, title, message, is_read, data)
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

	for _, notification := range notifications {
		_, err = stmt.ExecContext(ctx,
			notification.UserID,
			notification.Type,
			notification.Title,
			notification.Message,
			notification.IsRead,
			notification.Data,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
