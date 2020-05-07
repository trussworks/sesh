// Package sesh is a session management library written in Go.
// It uses a postgres table to track current sessions and their expiration,
// and it logs all session lifecycle events.
package sesh

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/trussworks/sesh/pkg/logger"
)

func hello() {
	manager := scs.New()

	fmt.Println("HI", manager)
}

type SessionUser interface {
	SeshUserID() string //-- fuuuuuu
	SeshCurrentSessionID() string
}

// UserSessions manage User Sessions. On top of scs for browser sessions
type UserSessions struct {
	scs    *scs.SessionManager
	logger EventLogger
}

// NewUserSessions returns a configured UserSessions
func NewUserSessions(scs *scs.SessionManager, options ...Option) (UserSessions, error) {

	sessions := UserSessions{
		scs,
		logger.NewPrintLogger(),
	}

	for _, option := range options {
		err := option(&sessions)
		if err != nil {
			return UserSessions{}, err
		}
	}

	return sessions, nil
}

type EventLogger interface {
	LogSeshEvent(message string, metadata map[string]string)
}

type Option func(*UserSessions) error

func CustomLogger(logger EventLogger) Option {
	return func(userSessions *UserSessions) error {
		userSessions.logger = logger
		return nil
	}
}

// userIDKey is the key used internally by sesh to track the UserID for the user
// that is authenticated in this session
const userIDKey = "sesh-user-id"

const (
	sessionCreatedMessage = "New User Session Created"
	sessionDeletedMessage = "User Session Destroyed"
)

func hashSessionKey(sessionKey string) string {
	hashed := sha512.Sum512([]byte(sessionKey))
	hexEncoded := hex.EncodeToString(hashed[:])
	return hexEncoded[:12]
}

// UserDidAuthenticate creates a new session and writes an HTTPOnly cookie to track that session
// it returns errors
func (s UserSessions) UserDidAuthenticate(ctx context.Context, user SessionUser) error {
	// got to do a bunch of stuff here.

	// Renew the session token to prevent session fixation attacks on auth change
	err := s.scs.RenewToken(ctx)
	if err != nil {
		return fmt.Errorf("Failed to renew the token for login: %w", err)
	}

	// Put the user ID into the session to track which user authenticated here
	s.scs.Put(ctx, userIDKey, user.SeshUserID())

	// force SCS to commit the session now, this will ensure that the session has been created and give us the session ID.
	sessionID, _, err := s.scs.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Failed to write new user session to store: %w", err)
	}

	// Log the created session.
	s.logger.LogSeshEvent(sessionCreatedMessage, map[string]string{"session_id_hash": hashSessionKey(sessionID)})

	return nil
}

// ProtectedMiddleware reads the session cookie and verifies that the request is being made by someone with a valid session
// It then stores the current session in the context, which can be retrieved with SessionFromContext(ctx)
// If the session is invalid it responds with an error and does not call any further handlers.
func (s UserSessions) ProtectedMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Checking On Protecting")

		// First, determine if a user session has been created by looking for the user ID.
		userID := s.scs.GetString(r.Context(), userIDKey)

		if userID == "" {
			// userID is set by UserDidLogin, it being unset means there is no user session active.
			// In this case, an unauthenticated request has been made.

			// TODO: log it?
			// TODO: call error handler.
			fmt.Println("UNAUTHORIZED REQUEST MADE")
			http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
			return
		}

		// fetch the user with that ID.

		// next, check that the session id is current for the use
		// BLERG. Gotta get the current token somehow.

		next.ServeHTTP(w, r)
	})
}

// UserDidLogout destroys the user session and removes the session cookie.
// it returns errors
func (s UserSessions) UserDidLogout(ctx context.Context) error {
	// gotta call the thingamigger

	// Renew the session token to prevent session fixation attacks on auth change
	err := s.scs.RenewToken(ctx)
	if err != nil {
		return err
	}

	// Remove the user id from the session to indicate that the session is unauthenticated.
	s.scs.Remove(ctx, userIDKey)

	// Probably go ahead and force the deletion to happen now, too.

	// Log the created session.
	s.logger.LogSeshEvent(sessionDeletedMessage, map[string]string{"session_id_hash": "some_hash_i_think"})

	return nil
}
