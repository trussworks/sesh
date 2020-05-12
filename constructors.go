package sesh

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/trussworks/sesh/pkg/logger"
)

// NewUserSessions returns a configured UserSessions
func NewUserSessions(scs *scs.SessionManager, userDelegate UserDelegate, options ...Option) (UserSessions, error) {

	sessions := UserSessions{
		scs,
		logger.NewPrintLogger(),
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

type Option func(*UserSessions) error

func CustomLogger(logger EventLogger) Option {
	return func(userSessions *UserSessions) error {
		userSessions.logger = logger
		return nil
	}
}

func CustomErrorHandler(errorHandler http.Handler) Option {
	return func(userSessions *UserSessions) error {
		userSessions.errorHandler = errorHandler
		return nil
	}
}
