package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Role        string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) Bootstrap(ctx context.Context, email string, passwordHash string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || passwordHash == "" {
		return nil
	}
	if _, err := bcrypt.Cost([]byte(passwordHash)); err != nil {
		return fmt.Errorf("invalid bootstrap admin password hash: %w", err)
	}
	_, err := store.pool.Exec(ctx, `
		INSERT INTO admin_users (email, display_name, role, password_hash, mfa_enabled, status)
		VALUES ($1, 'Admin User', 'administrator', $2, false, 'active')
		ON CONFLICT (email) DO NOTHING
	`, email, passwordHash)
	return err
}

func (store *Store) Authenticate(ctx context.Context, email string, password string) (User, error) {
	var user User
	var hash string
	err := store.pool.QueryRow(ctx, `
		SELECT id, email, display_name, role, password_hash
		FROM admin_users
		WHERE email = $1 AND status = 'active'
	`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &hash)
	if err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return User{}, fmt.Errorf("invalid credentials")
	}
	_, _ = store.pool.Exec(ctx, `UPDATE admin_users SET last_login_at = clock_timestamp() WHERE id = $1`, user.ID)
	return user, nil
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
	err := store.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.role
		FROM admin_sessions s
		JOIN admin_users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > clock_timestamp() AND u.status = 'active'
	`, hex.EncodeToString(sum[:])).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, fmt.Errorf("unauthorized")
		}
		return User{}, err
	}
	return user, nil
}

func (store *Store) RevokeSession(ctx context.Context, token string) {
	sum := sha256.Sum256([]byte(token))
	_, _ = store.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, hex.EncodeToString(sum[:]))
}
