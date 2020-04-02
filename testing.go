package sesh

// Everything exported in this file is intended to be used to make testing code that is protected by sesh easier.

import (
	"context"
	"net/http"

	"github.com/trussworks/sesh/pkg/domain"
	"github.com/trussworks/sesh/pkg/seshttp"
)

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

// AuthenticateUserAndAddToTestRequest is not used in the operation of sesh. It is intended to
// be used in your tests to create a valid session for a request, alleviating you from having to make a login request
// as part of the test.
func (s Sessions) AuthenticateUserAndAddToTestRequest(r *http.Request, accountID string) error {
	sessionKey, authErr := s.session.UserDidAuthenticate(accountID)
	if authErr != nil {
		return authErr
	}

	s.cookie.AddSessionKeyToRequest(r, sessionKey)

	return nil
}
