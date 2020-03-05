package sesh

import (
	"errors"
	"time"
)

var (
	// ErrValidSessionNotFound is returned when a valid session is not found
	ErrValidSessionNotFound = errors.New("Valid session not found")

	// ErrSessionExpired is returned when the requested session has expired
	ErrSessionExpired = errors.New("Session is expired")
)

// SessionService backs user authentication -- providing a way to verify & modify session status
type SessionService interface {
	// UserDidAuthenticate creates a session for a newly logged in user
	UserDidAuthenticate(accountID int) (sessionKey string, err error)
	// GetSessionIfValid returns an account if the session is valid, or ErrValidSessionNotFound otherwise
	GetSessionIfValid(sessionKey string) (session Session, err error)
	// UserDidLogout invalidates a session for a newly logged out user
	UserDidLogout(sessionKey string) error
}

// Session contains all the information about a given user session in eApp
type Session struct {
	AccountID      string    `db:"account_id"`
	SessionKey     string    `db:"session_key"`
	ExpirationDate time.Time `db:"expiration_date"`
}
