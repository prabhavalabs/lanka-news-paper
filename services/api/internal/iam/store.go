package iam

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Role        string
	MFAEnabled  bool
	HasTOTP     bool
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) Bootstrap(ctx context.Context, email string, password string) error {
	if email == "" || password == "" {
		return nil
	}
	var exists bool
	if err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_users)`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = store.pool.Exec(ctx, `
		INSERT INTO admin_users (email, display_name, role, password_hash, mfa_enabled, status)
		VALUES ($1, 'Editor', 'administrator', $2, true, 'active')
	`, email, string(hash))
	return err
}

func (store *Store) Authenticate(ctx context.Context, email string, password string) (User, error) {
	var user User
	var hash string
	var secret *string
	err := store.pool.QueryRow(ctx, `
		SELECT id, email, display_name, role, mfa_enabled, password_hash, totp_secret
		FROM admin_users
		WHERE email = $1 AND status = 'active'
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.MFAEnabled, &hash, &secret)
	if err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}
	user.HasTOTP = secret != nil && *secret != ""
	return user, nil
}

func (store *Store) CreateTOTP(email string) (secret string, url string, qr string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "SNAP",
		AccountName: email,
	})
	if err != nil {
		return "", "", "", err
	}
	img, err := key.Image(192, 192)
	if err != nil {
		return "", "", "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", "", err
	}
	qr = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return key.Secret(), key.URL(), qr, nil
}

func (store *Store) ConfirmTOTP(ctx context.Context, userID uuid.UUID, secret string, code string) error {
	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid code")
	}
	_, err := store.pool.Exec(ctx, `UPDATE admin_users SET totp_secret = $2, mfa_enabled = true WHERE id = $1`, userID, secret)
	return err
}

func (store *Store) VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	var secret string
	if err := store.pool.QueryRow(ctx, `SELECT totp_secret FROM admin_users WHERE id = $1`, userID).Scan(&secret); err != nil {
		return err
	}
	if !totp.Validate(code, secret) {
		return fmt.Errorf("invalid code")
	}
	return nil
}

func (store *Store) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	_, err := store.pool.Exec(ctx, `
		INSERT INTO admin_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hex.EncodeToString(sum[:]), time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (store *Store) LookupSession(ctx context.Context, token string) (User, error) {
	sum := sha256.Sum256([]byte(token))
	var user User
	var secret *string
	err := store.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.role, u.mfa_enabled, u.totp_secret
		FROM admin_sessions s
		JOIN admin_users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > clock_timestamp() AND u.status = 'active'
	`, hex.EncodeToString(sum[:])).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.MFAEnabled, &secret)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, fmt.Errorf("unauthorized")
		}
		return User{}, err
	}
	user.HasTOTP = secret != nil && *secret != ""
	return user, nil
}

func (store *Store) RevokeSession(ctx context.Context, token string) {
	sum := sha256.Sum256([]byte(token))
	_, _ = store.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, hex.EncodeToString(sum[:]))
}
