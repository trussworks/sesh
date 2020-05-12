package sesh

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
)

// NewUserSessions returns a configured UserSessions
func NewUserSessions(scs *scs.SessionManager, userDelegate UserDelegate, options ...Option) (UserSessions, error) {

	sessions := UserSessions{
		scs,
		newDefaultLogger(),
		newDefaultErrorHandler(),
		userDelegate,
	}

	for _, option := range options {
		err := option(&sessions)
		if err != nil {
			return UserSessions{}, err
		}
	}

	return sessions, nil
}

// Option is an option for constructing a UserSessionManager, they can be passed in to NewUserSessions
// The available options are defined below.
type Option func(*UserSessions) error

// CustomLogger supplies a custom logger for logging session lifecycle events.
// It must conform to EventLogger
func CustomLogger(logger EventLogger) Option {
	return func(userSessions *UserSessions) error {
		userSessions.logger = logger
		return nil
	}
}

// CustomErrorHandler supplies a custom http.Handler for responding to errors in the ProtectedMiddleware
// Use ErrorFromContext(ctx) to get the error that caused this handler to be called.
func CustomErrorHandler(errorHandler http.Handler) Option {
	return func(userSessions *UserSessions) error {
		userSessions.errorHandler = errorHandler
		return nil
	}
}
