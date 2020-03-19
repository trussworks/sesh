// Package sesh is a session management library written in Go.
// It uses a postgres table to track current sessions and their expiration,
// and it logs all session lifecycle events.
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

// Sessions manage browser sessions with a db table and logs all significant lifecycle events
type Sessions struct {
	session    domain.SessionService
	middleware *seshttp.SessionMiddleware
	cookie     seshttp.SessionCookieService
}

// NewSessions returns a configured Sessions, taking an existing sqlx.DB as the first argument.
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

// Session contains all the information about a given user session
// This type is inserted into the context by the AuthenticationMiddleware
type Session struct {
	AccountID      string
	SessionKey     string
	ExpirationDate time.Time
}

// UserDidAuthenticate creates a new session and writes an HTTPOnly cookie to track that session
// it returns errors
func (s Sessions) UserDidAuthenticate(w http.ResponseWriter, accountID string) (sessionKey string, err error) {
	sessionKey, authErr := s.session.UserDidAuthenticate(accountID)
	if authErr != nil {
		return "", authErr
	}

	s.cookie.AddSessionKeyToResponse(w, sessionKey)

	return sessionKey, nil
}

// UserDidLogout destroys the session and removes the session cookie.
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
func SessionFromContext(ctx context.Context) Session {
	domainSession := seshttp.SessionFromContext(ctx)
	session := Session{
		AccountID:      domainSession.AccountID,
		SessionKey:     domainSession.SessionKey,
		ExpirationDate: domainSession.ExpirationDate,
	}
	return session
}

// ContextWithTestSession is not used in the operation of sesh. It is intended to
// be used in your tests, to mimic what AuthenticatedMiddleware does for authenticated requests.
func ContextWithTestSession(ctx context.Context, session Session) context.Context {
	domainSession := domain.Session{
		AccountID:      session.AccountID,
		SessionKey:     session.SessionKey,
		ExpirationDate: session.ExpirationDate,
	}

	return seshttp.SetSessionInContext(ctx, domainSession)
}
