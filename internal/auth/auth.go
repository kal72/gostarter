package auth

import (
	"strings"
	"time"
)

type AuthRepository interface {
	Authenticate(string, string) (*AdminUser, error)
	CreateSession(int64, time.Duration) (string, error)
	ValidateSession(string) (*AdminUser, error)
	DeleteSession(string) error
	ChangePassword(int64, string, string) error
}
type Auth struct {
	repository AuthRepository
	lifetime   time.Duration
}

func New(repository AuthRepository, lifetime time.Duration) *Auth {
	return &Auth{repository, lifetime}
}
func (s *Auth) Login(username, password string) (*AdminUser, string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, "", ErrRequiredCredentials
	}
	if s.repository == nil {
		return nil, "", ErrUnavailable
	}
	user, err := s.repository.Authenticate(username, password)
	if err != nil {
		return nil, "", ErrCredentials
	}
	token, err := s.repository.CreateSession(user.ID, s.lifetime)
	if err != nil {
		return nil, "", ErrSession
	}
	return user, token, nil
}
func (s *Auth) Session(token string) (*AdminUser, error) {
	if s.repository == nil {
		return nil, ErrUnavailable
	}
	return s.repository.ValidateSession(token)
}
func (s *Auth) Logout(token string) error {
	if s.repository == nil {
		return ErrUnavailable
	}
	return s.repository.DeleteSession(token)
}
func (s *Auth) ChangePassword(id int64, current, next string) error {
	if s.repository == nil {
		return ErrUnavailable
	}
	return s.repository.ChangePassword(id, current, next)
}
