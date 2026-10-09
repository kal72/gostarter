package auth

import (
	"errors"
	"testing"
	"time"
)

type authRecorder struct {
	username            string
	authErr, sessionErr error
	sessions            int
}

func (r *authRecorder) Authenticate(username, password string) (*AdminUser, error) {
	r.username = username
	return &AdminUser{ID: 7, Username: username}, r.authErr
}
func (r *authRecorder) CreateSession(id int64, lifetime time.Duration) (string, error) {
	r.sessions++
	return "token", r.sessionErr
}
func (r *authRecorder) ValidateSession(string) (*AdminUser, error) { return nil, nil }
func (r *authRecorder) DeleteSession(string) error                 { return nil }
func (r *authRecorder) ChangePassword(int64, string, string) error { return nil }
func TestLoginNeverCreatesSessionForRejectedCredentials(t *testing.T) {
	for _, tc := range []struct {
		username      string
		authErr, want error
	}{{" ", nil, ErrRequiredCredentials}, {"admin", errors.New("wrong password"), ErrCredentials}} {
		repository := &authRecorder{authErr: tc.authErr}
		_, _, err := New(repository, time.Hour).Login(tc.username, "password")
		if !errors.Is(err, tc.want) || repository.sessions != 0 {
			t.Fatalf("err=%v sessions=%d", err, repository.sessions)
		}
	}
}
func TestLoginNormalizesUsernameAndPropagatesSessionFailure(t *testing.T) {
	repository := &authRecorder{sessionErr: errors.New("storage failure")}
	user, token, err := New(repository, time.Hour).Login(" admin ", "password")
	if !errors.Is(err, ErrSession) || user != nil || token != "" || repository.username != "admin" || repository.sessions != 1 {
		t.Fatalf("unexpected login result: user=%v token=%q err=%v", user, token, err)
	}
	repository.sessionErr = nil
	user, token, err = New(repository, time.Hour).Login(" admin ", "password")
	if err != nil || user.ID != 7 || token != "token" {
		t.Fatalf("login failed: %v", err)
	}
}
