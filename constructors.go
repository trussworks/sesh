package sesh

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
)

// NewUserSessionManager returns a configured UserSessionManager
func NewUserSessionManager(scs *scs.SessionManager, userDelegate UserDelegate, options ...Option) (UserSessionManager, error) {

	sessions := UserSessionManager{
		scs,
		newDefaultLogger(),
		newDefaultErrorHandler(),
		userDelegate,
	}

	for _, option := range options {
		err := option(&sessions)
		if err != nil {
			return UserSessionManager{}, err
		}
	}

	return sessions, nil
}

// Option is an option for constructing a UserSessionManager, they can be passed in to NewUserSessionManager
// The available options are defined below.
type Option func(*UserSessionManager) error

// CustomLogger supplies a custom logger for logging session lifecycle events.
// It must conform to EventLogger
func CustomLogger(logger EventLogger) Option {
	return func(userSeshManager *UserSessionManager) error {
		userSeshManager.logger = logger
		return nil
	}
}

// CustomErrorHandler supplies a custom http.Handler for responding to errors in the ProtectedMiddleware
// Use ErrorFromContext(ctx) to get the error that caused this handler to be called.
func CustomErrorHandler(errorHandler http.Handler) Option {
	return func(userSeshManager *UserSessionManager) error {
		userSeshManager.errorHandler = errorHandler
		return nil
	}
}
