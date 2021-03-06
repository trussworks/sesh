package domain

import "time"

// Session contains all the information about a given user session
type Session struct {
	AccountID      string    `db:"account_id"`
	SessionKey     string    `db:"session_key"`
	ExpirationDate time.Time `db:"expiration_date"`
}

// SessionService backs user authentication -- providing a way to verify & modify session status
type SessionService interface {
	// UserDidAuthenticate creates a session for a newly logged in user
	UserDidAuthenticate(accountID string) (sessionKey string, err error)
	// GetSessionIfValid returns a session if the session is valid, or ErrValidSessionNotFound otherwise
	GetSessionIfValid(sessionKey string) (session Session, err error)
	// UserDidLogout invalidates a session for a newly logged out user
	UserDidLogout(sessionKey string) error
}
