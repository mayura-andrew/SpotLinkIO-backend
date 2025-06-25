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

type Review struct {
	ID           uuid.UUID `json:"id" db:"id"`
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	ParkingLotID uuid.UUID `json:"parking_lot_id" db:"parking_lot_id"`
	Rating       int       `json:"rating" db:"rating"` // 1-5 stars
	Comment      *string   `json:"comment" db:"comment"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	Version      int       `json:"version" db:"version"`
}

func ValidateReview(v *validator.Validator, review *Review) {
	v.Check(review.Rating >= 1, "rating", "must be at least 1")
	v.Check(review.Rating <= 5, "rating", "must not be more than 5")

	if review.Comment != nil {
		v.Check(len(*review.Comment) <= 1000, "comment", "must not be more than 1000 characters long")
	}
}

type ReviewModel struct {
	DB *sql.DB
}

func (m ReviewModel) Insert(review *Review) error {
	query := `
		INSERT INTO reviews (user_id, parking_lot_id, rating, comment)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at, version`

	args := []any{
		review.UserID,
		review.ParkingLotID,
		review.Rating,
		review.Comment,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&review.ID,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.Version,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m ReviewModel) Get(id uuid.UUID) (*Review, error) {
	query := `
		SELECT id, user_id, parking_lot_id, rating, comment, created_at, updated_at, version
		FROM reviews
		WHERE id = $1`

	var review Review

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&review.ID,
		&review.UserID,
		&review.ParkingLotID,
		&review.Rating,
		&review.Comment,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &review, nil
}

func (m ReviewModel) GetByLot(lotID uuid.UUID, filters Filters) ([]*Review, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, parking_lot_id, rating, comment, created_at, updated_at, version
		FROM reviews
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
	reviews := []*Review{}

	for rows.Next() {
		var review Review

		err := rows.Scan(
			&totalRecords,
			&review.ID,
			&review.UserID,
			&review.ParkingLotID,
			&review.Rating,
			&review.Comment,
			&review.CreatedAt,
			&review.UpdatedAt,
			&review.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reviews = append(reviews, &review)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}

func (m ReviewModel) GetByUser(userID uuid.UUID, filters Filters) ([]*Review, Metadata, error) {
	query := `
		SELECT count(*) OVER(), id, user_id, parking_lot_id, rating, comment, created_at, updated_at, version
		FROM reviews
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
	reviews := []*Review{}

	for rows.Next() {
		var review Review

		err := rows.Scan(
			&totalRecords,
			&review.ID,
			&review.UserID,
			&review.ParkingLotID,
			&review.Rating,
			&review.Comment,
			&review.CreatedAt,
			&review.UpdatedAt,
			&review.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		reviews = append(reviews, &review)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}

func (m ReviewModel) GetUserReviewForLot(userID, lotID uuid.UUID) (*Review, error) {
	query := `
		SELECT id, user_id, parking_lot_id, rating, comment, created_at, updated_at, version
		FROM reviews
		WHERE user_id = $1 AND parking_lot_id = $2`

	var review Review

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, userID, lotID).Scan(
		&review.ID,
		&review.UserID,
		&review.ParkingLotID,
		&review.Rating,
		&review.Comment,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &review, nil
}

func (m ReviewModel) Update(review *Review) error {
	query := `
		UPDATE reviews
		SET rating = $1, comment = $2, updated_at = CURRENT_TIMESTAMP, version = version + 1
		WHERE id = $3 AND version = $4
		RETURNING updated_at, version`

	args := []any{
		review.Rating,
		review.Comment,
		review.ID,
		review.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&review.UpdatedAt, &review.Version)
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

func (m ReviewModel) Delete(id uuid.UUID) error {
	query := `DELETE FROM reviews WHERE id = $1`

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

func (m ReviewModel) GetAverageRatingForLot(lotID uuid.UUID) (float64, error) {
	query := `SELECT COALESCE(AVG(rating), 0) FROM reviews WHERE parking_lot_id = $1`

	var avgRating float64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, lotID).Scan(&avgRating)
	if err != nil {
		return 0, err
	}

	return avgRating, nil
}

func (m ReviewModel) GetRatingDistributionForLot(lotID uuid.UUID) (map[int]int, error) {
	query := `
		SELECT rating, COUNT(*) as count
		FROM reviews
		WHERE parking_lot_id = $1
		GROUP BY rating
		ORDER BY rating`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[int]int)

	// Initialize all ratings to 0
	for i := 1; i <= 5; i++ {
		distribution[i] = 0
	}

	for rows.Next() {
		var rating, count int
		err := rows.Scan(&rating, &count)
		if err != nil {
			return nil, err
		}
		distribution[rating] = count
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return distribution, nil
}

func (m ReviewModel) GetTotalReviewsForLot(lotID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM reviews WHERE parking_lot_id = $1`

	var totalReviews int

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, lotID).Scan(&totalReviews)
	if err != nil {
		return 0, err
	}

	return totalReviews, nil
}
