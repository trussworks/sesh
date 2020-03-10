package sesh

import (
	"context"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trussworks/sesh/pkg/dbstore"
	"github.com/trussworks/sesh/pkg/domain"
	"github.com/trussworks/sesh/pkg/seshttp"
	"github.com/trussworks/sesh/pkg/session"
)

// Sessions manages sessions with a db table and logs all significant session events
type Sessions struct {
	session    domain.SessionService
	middleware *seshttp.SessionMiddleware
	cookie     seshttp.SessionCookieService
}

// NewSessions returns a configured Sessions
func NewSessions(db *sqlx.DB, log domain.LogService, timeout time.Duration, useSecureCookie bool) Sessions {
	store := dbstore.NewDBStore(db)
	session := session.NewSessionService(timeout, store, log)
	middleware := seshttp.NewSessionMiddleware(log, session)
	cookie := seshttp.NewSessionCookieService(useSecureCookie)

	return Sessions{
		session,
		middleware,
		cookie,
	}
}

// UserDidAuthenticate creates a new session and writes an HTTPOnly cookie to track that session
// it returns errors
func (s Sessions) UserDidAuthenticate(w http.ResponseWriter, accountID string) (sessionKey string, err error) {
	// call user did authenticate on

	sessionKey, authErr := s.session.UserDidAuthenticate(accountID)
	if authErr != nil {
		return "", authErr
	}

	s.cookie.AddSessionKeyToResponse(w, sessionKey)

	return sessionKey, nil

}

// UserDidLogout destroys the session, writing cookies.
// it returns errors
func (s Sessions) UserDidLogout(w http.ResponseWriter, r *http.Request) error {
	session := seshttp.SessionFromRequestContext(r)

	logoutErr := s.session.UserDidLogout(session.SessionKey)
	if logoutErr != nil {
		return logoutErr
	}

	seshttp.DeleteSessionCookie(w)

	return nil
}

// AuthenticationMiddleware reads the session cookie and verifies that the request is being made by someone with a valid session
// It then stores the current session in the context, which can be retrieved with SessionFromContext(ctx)
// If the session is invalid it responds with an error and does not call any further handlers.
func (s Sessions) AuthenticationMiddleware() func(http.Handler) http.Handler {
	return s.middleware.Middleware
}

// SessionFromContext pulls the current sesh.Session object out of the context
// This function is all that is required in your handlers to get the current session information
func SessionFromContext(ctx context.Context) domain.Session {
	return seshttp.SessionFromContext(ctx)
}
