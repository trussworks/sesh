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

// Actual API
type SeshService interface {
	// UserDidAuthenticate creates a new session and writes an HTTPOnly cookie to track that session
	// it returns errors
	UserDidAuthenticate(w http.ResponseWriter, accountID string) (sessionKey string, err error)
	// UserDidLogout destroys the session, writing cookies.
	// it returns errors
	UserDidLogout(w http.ResponseWriter) error

	// AuthenticationMiddleware reads the session cookie and verifies that the request is being made by someone with a valid session
	// If the session is invalid it responds with an error and does not call any further handlers.
	AuthenticationMiddleware() http.Handler
}

// SessionFromContext pulls the current sesh.Session object out of the context
func SessionFromContext(ctx context.Context) domain.Session {
	panic(99)
}

type seshServiceImpl struct {
	session    domain.SessionService
	middleware *seshttp.SessionMiddleware
	cookie     seshttp.SessionCookieService
}

func NewSeshService(db *sqlx.DB, log domain.LogService, timeout time.Duration, useSecureCookie bool) seshServiceImpl {
	store := dbstore.NewDBStore(db)
	session := session.NewSessionService(timeout, store, log)
	middleware := seshttp.NewSessionMiddleware(log, session)
	cookie := seshttp.NewSessionCookieService(useSecureCookie)

	return seshServiceImpl{
		session,
		middleware,
		cookie,
	}
}
