package domain

import "time"

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
