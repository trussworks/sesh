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

// SessionUser is an interface you can implement on your user that allows Sesh to limit you to a single concurrent session
type SessionUser interface {
	SeshUserID() string //-- fuuuuuu
	SeshCurrentSessionID() string
}

// UserSessions manage User Sessions. On top of scs for browser sessions
type UserSessions struct {
	scs          *scs.SessionManager
	logger       EventLogger
	userDelegate UserDelegate
}

// UserUpdateDelegate is the function signature that will be called to update an implementors user with the current session ID
type UserUpdateDelegate func(userID string, currentID string) error

// UserDelegate is an implementor provided delegate for managing session IDs on your users
type UserDelegate interface {
	FetchUserByID(id string) (SessionUser, error)
	UpdateUser(user SessionUser, currentSessionID string) error
}

// NewUserSessions returns a configured UserSessions
func NewUserSessions(scs *scs.SessionManager, userDelegate UserDelegate, options ...Option) (UserSessions, error) {

	sessions := UserSessions{
		scs,
		logger.NewPrintLogger(),
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

// You should always make a custom type for context keys
type seshContextKey string

// userContextKey is the key for storing a user in the context
const userContextKey seshContextKey = "user-context-key"

// userIDKey is the key used internally by sesh to track the UserID for the user
// that is authenticated in this session
const userIDKey = "sesh-user-id"

const (
	sessionCreatedMessage = "New User Session Created"
	sessionDeletedMessage = "User Session Destroyed"

	expiredLoginMessage    = "Previous session expired"
	concurrentLoginMessage = "User logged in with a concurrent active session"
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

	// Check to see if sessionID is set on the user, presently
	if user.SeshCurrentSessionID() != "" {
		fmt.Println("DID WE")

		// Lookup the old session that wasn't logged out
		_, exists, err := s.scs.Store.Find(user.SeshCurrentSessionID())
		if err != nil {
			return fmt.Errorf("Error loading previous session: %w", err)
		}

		if !exists {
			s.logger.LogSeshEvent(expiredLoginMessage, map[string]string{"session_id_hash": hashSessionKey(user.SeshCurrentSessionID())})
		} else {
			s.logger.LogSeshEvent(concurrentLoginMessage, map[string]string{"session_id_hash": hashSessionKey(user.SeshCurrentSessionID())})

			// We need to delete the concurrent session.
			err := s.scs.Store.Delete(user.SeshCurrentSessionID())
			if err != nil {
				// TODO, should we delete the new session?
				return fmt.Errorf("Error deleting a previous session on login: %w", err)
			}
		}
	}

	// Save the current session ID on the user
	err = s.userDelegate.UpdateUser(user, sessionID)
	if err != nil {
		// TODO, Should we tear down the scs session for this? probably. It won't work I think.
		return fmt.Errorf("Error in user update delegate: %w", err)
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
		// SO, INTERESTING, maybe we don't need to do this anymore? We are getting this on login instead...

		// IF deletion failed though, we gotta check, right? b/c otherwise that session would just hang out
		// valid when it's explicitly not anymore. This is why we check on every load.
		user, err := s.userDelegate.FetchUserByID(userID)
		if err != nil {
			// TODO: log it?
			// TODO: call error handler.
			fmt.Println("Couldn't find user", err)
			http.Error(w, "SERVER_ERROR", http.StatusInternalServerError)
			return
		}

		// // next, check that the session id is current for the use
		// // BLERG. Gotta get the current token somehow.
		// if user.SeshCurrentSessionID() != "FUUUUU" {
		// 	// TODO: log it?
		// 	// TODO: call error handler.
		// 	fmt.Println("OLD SESSION MADE REQUEST")
		// 	http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
		// 	return
		// }

		userContext := context.WithValue(r.Context(), userContextKey, user)
		userReq := r.WithContext(userContext)

		next.ServeHTTP(w, userReq)
	})
}

// UserFromContext returns the SessionUser that the protected middleware stored in the context.
// It will be the same user that the FetchUserByID delegate method returned, so you can safely cast
// it to your native user type.
func UserFromContext(ctx context.Context) SessionUser {
	return ctx.Value(userContextKey).(SessionUser)
}

// UserDidLogout destroys the user session and removes the session cookie.
// it returns errors
func (s UserSessions) UserDidLogout(ctx context.Context) error {
	// gotta call the thingamigger

	// Renew the session token to prevent session fixation attacks on auth change
	err := s.scs.RenewToken(ctx)
	if err != nil {
		return fmt.Errorf("Failed to renew the token: %w", err)
	}

	// Remove the user id from the session to indicate that the session is unauthenticated.
	s.scs.Remove(ctx, userIDKey)

	// TODO: Probably go ahead and force the deletion to happen now, too. save it?

	// Update the users's currentSessionID
	user, ok := ctx.Value(userContextKey).(SessionUser)
	if !ok {
		return fmt.Errorf("the User was not in the context, it should have been put there by the protected middleware")
	}

	// Update the user to drop currentsessionid
	err = s.userDelegate.UpdateUser(user, "")
	if err != nil {
		return fmt.Errorf("Failed to reset logged out user's session ID: %w", err)
	}

	// Log the deleted session.
	s.logger.LogSeshEvent(sessionDeletedMessage, map[string]string{"session_id_hash": "some_hash_i_think"})

	return nil
}
