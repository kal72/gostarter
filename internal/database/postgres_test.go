package database_test

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptPasswordHashing(t *testing.T) {
	password := "admin123"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed hashing password: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("expected password match, got: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrongpassword")); err == nil {
		t.Fatal("expected error on wrong password, got nil")
	}
}

func TestSessionDuration(t *testing.T) {
	lifetime := 24 * time.Hour
	now := time.Now()
	expires := now.Add(lifetime)

	if !expires.After(now) {
		t.Fatalf("expected expiry to be in the future")
	}
}
