package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// ─── Authentication & Session Methods ─────────────────────────────────────────

// Authenticate verifies username and password using bcrypt
func (d *DB) Authenticate(username, password string) (*AdminUser, error) {
	if d.conn == nil {
		return nil, fmt.Errorf("database not connected")
	}

	query := `SELECT id, username, password_hash FROM admin_users WHERE username = $1`
	var user AdminUser
	err := d.conn.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return &user, nil
}

// CreateSession generates a random token and stores session in PostgreSQL
func (d *DB) CreateSession(userID int64, lifetime time.Duration) (string, error) {
	if d.conn == nil {
		return "", fmt.Errorf("database not connected")
	}
	if lifetime <= 0 {
		lifetime = 24 * time.Hour
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed generating random token: %w", err)
	}
	token := hex.EncodeToString(b)

	query := `INSERT INTO admin_sessions (token, user_id, expires_at) VALUES ($1, $2, CURRENT_TIMESTAMP + $3 * interval '1 second')`
	_, err := d.conn.Exec(query, token, userID, lifetime.Seconds())
	if err != nil {
		return "", fmt.Errorf("failed creating session: %w", err)
	}

	return token, nil
}

// ValidateSession verifies token against PostgreSQL admin_sessions
func (d *DB) ValidateSession(token string) (*AdminUser, error) {
	if d.conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	if token == "" {
		return nil, errors.New("missing session token")
	}

	query := `SELECT u.id, u.username FROM admin_sessions s JOIN admin_users u ON s.user_id = u.id WHERE s.token = $1 AND s.expires_at > CURRENT_TIMESTAMP`
	var user AdminUser
	err := d.conn.QueryRow(query, token).Scan(&user.ID, &user.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid or expired session")
		}
		return nil, err
	}

	return &user, nil
}

// DeleteSession removes a session from PostgreSQL
func (d *DB) DeleteSession(token string) error {
	if d.conn == nil {
		return nil
	}
	_, err := d.conn.Exec(`DELETE FROM admin_sessions WHERE token = $1`, token)
	return err
}

// ChangePassword verifies existing password and stores new bcrypt hash
func (d *DB) ChangePassword(userID int64, currentPassword, newPassword string) error {
	if d.conn == nil {
		return fmt.Errorf("database not connected")
	}
	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	var currentHash string
	err := d.conn.QueryRow(`SELECT password_hash FROM admin_users WHERE id = $1`, userID).Scan(&currentHash)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return errors.New("incorrect current password")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed hashing password: %w", err)
	}

	_, err = d.conn.Exec(`UPDATE admin_users SET password_hash = $1 WHERE id = $2`, string(newHash), userID)
	return err
}
