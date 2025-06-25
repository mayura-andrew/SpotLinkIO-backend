package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

type User struct {
	ID                     uuid.UUID `json:"id" db:"id"`
	Email                  string    `json:"email" db:"email"`
	UserName               string    `json:"username"`
	Password               password  `json:"-" db:"password_hash"`
	FirstName              *string   `json:"first_name" db:"first_name"`
	LastName               *string   `json:"last_name" db:"last_name"`
	MobileNumber           *string   `json:"mobile_number" db:"mobile_number"`
	AvatarURL              *string   `json:"avatar_url" db:"avatar_url"`
	Role                   string    `json:"role" db:"role"`
	AuthType string `json:"authtype" db:"authtype"`
	HasCompletedOnboarding bool      `json:"has_completed_onboarding" db:"has_completed_onboarding"`
	Activated              bool      `json:"activated" db:"activated"`
	Version                int       `json:"version" db:"version"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time `json:"updated_at" db:"updated_at"`
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRx), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	// v.Check(user.UserName != "", "username", "must be provided")
	// v.Check(len(user.UserName) <= 500, "username", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	if user.Password.hash == nil {
		panic("missing password hash for user")
	}

}

type UserModal struct {
	DB *sql.DB
}

func (m UserModal) Insert(user *User) error {
	query := `INSERT INTO users (user_name, email, first_name, last_name, mobile_number, avatar_url, password_hash, user_role, activated, has_completed_onboarding) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) 
			RETURNING id, created_at, version`

	args := []any{user.UserName, user.Email, user.FirstName, user.LastName, user.MobileNumber, user.AvatarURL, user.Password.hash, user.Role, user.Activated, user.HasCompletedOnboarding}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

func (m UserModal) GetByEmail(email string) (*User, error) {
	query := `SELECT id, created_at, user_name, email, first_name, last_name, mobile_number, avatar_url, password_hash, user_role, activated, has_completed_onboarding, version
      		  FROM users
      		  WHERE email = $1`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UserName,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.MobileNumber,
		&user.AvatarURL,
		&user.Password.hash,
		&user.Role,
		&user.Activated,
		&user.HasCompletedOnboarding,
		&user.Version)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

func (m UserModal) Update(user *User) error {
	query := `UPDATE users
			SET user_name = $1, email = $2, password_hash = $3, activated = $4, has_completed_onboarding= $5, version = version + 1
			WHERE id = $6 AND version = $7
			RETURNING version`

	args := []any{
		user.UserName,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.HasCompletedOnboarding,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m UserModal) GetForToken(tokenScope, tokenPlainText string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlainText))

	query := `SELECT users.id, users.created_at, users.user_name, users.email, users.password_hash, users.user_type, users.activated, users.has_completed_onboarding, users.version
	FROM users
	INNER JOIN tokens
	ON users.id = tokens.user_id
	WHERE tokens.hash = $1
	AND tokens.scope = $2
	AND tokens.expiry > $3`

	args := []any{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UserName,
		&user.Email,
		&user.Password.hash,
		&user.Role,
		&user.Activated,
		&user.HasCompletedOnboarding,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (m UserModal) FindOrCreateFromGoogle(googleUser *GoogleUser) (*User, error) {
	// Try to find existing user by email
	user, err := m.GetByEmail(googleUser.Email)
	if err == nil {
		return user, nil
	}

	// If user doesn't exist, create new one
	if errors.Is(err, ErrRecordNotFound) {
		user = &User{
			UserName:  googleUser.Name,
			Email:     googleUser.Email,
			AuthType:  "google",
			Activated: googleUser.VerifiedEmail,
		}

		// Generate random password for Google users
		randomPassword := make([]byte, 32)
		rand.Read(randomPassword)
		user.Password.Set(base64.URLEncoding.EncodeToString(randomPassword))

		err = m.Insert(user)
		if err != nil {
			return nil, err
		}

		return user, nil
	}

	return nil, err
}
