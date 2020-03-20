package domain

import (
	"errors"
	"fmt"
)

// errors
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
	SessionUnexpectedError        = "An unexpected error occurred while checking the session."
	SessionCreationFailed         = "An unexpected error occurred creating a session"
	RequestIsMissingSessionCookie = "Unauthorized: Request is missing a session cookie"

	SessionCreated         = "New Session Created"
	SessionDestroyed       = "Session Was Destroyed"
	SessionRefreshed       = "Session was refreshed with the refresh API"
	SessionConcurrentLogin = "User logged in again with a concurrent active session"
)

// Temporary logging stuff. we should turn this into a callback.
type LogFields map[string]string

type LogService interface {
	Info(message string, fields LogFields)
	WarnError(message string, err error, fields LogFields)
}

// put this in mock?

type FmtLogger bool

func (l FmtLogger) Info(message string, fields LogFields) {
	fmt.Printf("INFO: %s %v\n", message, fields)
}

func (l FmtLogger) WarnError(message string, err error, fields LogFields) {
	fmt.Printf("WARN: %s %v %v\n", message, err, fields)
}
