package sesh

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrValidSessionNotFound is returned when a valid session is not found
	ErrValidSessionNotFound = errors.New("Valid session not found")

	// ErrSessionExpired is returned when the requested session has expired
	ErrSessionExpired = errors.New("Session is expired")
)

// log messages
var (
	SessionExpired                = "Auth failed because of an expired session"
	SessionDoesNotExist           = "Auth failed because of an invalid session"
	SessionUnexpectedError        = "An unexpected error occured while checking the session."
	SessionCreationFailed         = "An unexpected error occured creating a session"
	RequestIsMissingSessionCookie = "Unauthorized: Request is missing a session cookie"

	SessionCreated         = "New Session Created"
	SessionDestroyed       = "Session Was Destroyed"
	SessionRefreshed       = "Session was refreshed with the refresh API"
	SessionConcurrentLogin = "User logged in again with a concurrent active session"
)

// Temporary logging stuff. we should turn this into a callback.
type LogFields map[string]interface{}

type LogService interface {
	Info(message string, fields LogFields)
	WarnError(message string, err error, fields LogFields)
}

type FmtLogger bool

func (l FmtLogger) Info(message string, fields LogFields) {
	fmt.Printf("INFO: %s %v\n", message, fields)
}

func (l FmtLogger) WarnError(message string, err error, fields LogFields) {
	fmt.Printf("WARN: %s %v %v\n", message, err, fields)
}

// end logging stuff

type SessionStorageService interface {
	// Close closes the storage connection
	Close() error

	// CreateSession creates a new session. It errors if a valid session already exists.
	CreateSession(accountID string, sessionKey string, expirationDuration time.Duration) error

	// FetchPossiblyExpiredSession returns a session row by account ID regardless of wether it is expired
	// This is potentially dangerous, it is only intended to be used during the new login flow, never to check
	// on a valid session for authentication purposes.
	FetchPossiblyExpiredSession(accountID string) (Session, error)

	// DeleteSession removes a session record from the db
	DeleteSession(sessionKey string) error

	// ExtendAndFetchSession fetches session data from the db
	// On success it returns the session
	// On failure, it can return ErrValidSessionNotFound, ErrSessionExpired, or an unexpected error
	ExtendAndFetchSession(sessionKey string, expirationDuration time.Duration) (Session, error)
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

// Session contains all the information about a given user session in eApp
type Session struct {
	AccountID      string    `db:"account_id"`
	SessionKey     string    `db:"session_key"`
	ExpirationDate time.Time `db:"expiration_date"`
}
