// Package sesh is an authenticated user session management library
// It provides a ProtectedMiddleware to prevent un-authenticated users from accessing handlers,
// it limits users to a single session, and it logs all session lifecycle events.
package sesh

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

// SessionUser is an interface you can implement on your user that allows Sesh to limit you to a single concurrent session
type SessionUser interface {
	SeshUserID() string
	SeshCurrentSessionID() string
}

// UserDelegate is an implementor provided delegate for managing session IDs on your users
type UserDelegate interface {
	FetchUserByID(id string) (SessionUser, error)
	UpdateUser(user SessionUser, currentSessionID string) error
}

// EventLogger is the interface that is used for logging all session lifecycle events. Supply your own with CustomLogger()
type EventLogger interface {
	LogSeshEvent(message string, metadata map[string]string)
}

// Errors for the error handler
var (
	ErrNoSession         = errors.New("this session is not authenticated")
	ErrNotCurrentSession = errors.New("this session is not the current session")
	ErrEmptySessionID    = errors.New("a user with an empty id cannot login")
)

// You should always make a custom type for context keys
type seshContextKey string

const (
	// userContextKey is the key for storing a user in the context
	userContextKey seshContextKey = "user-context-key"

	// errorHandleKey is the context key for the error that the error handler can fetch
	errorHandleKey seshContextKey = "error-handle-key"
)

const (
	// userIDKey is the key used internally by sesh to track the UserID for the user
	// that is authenticated in this session
	userIDKey = "sesh-user-id"
	// seshIDKey is used to store the session ID in the session because SCS does not expose it
	seshIDKey = "sesh-sesh-id"
)

// Log messages for the logger
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

// UserSessions manage User Sessions. On top of scs for browser sessions
type UserSessions struct {
	scs          *scs.SessionManager
	logger       EventLogger
	errorHandler http.Handler
	userDelegate UserDelegate
}

// UserDidAuthenticate creates a new session and writes an HTTPOnly cookie to track that session
// it returns errors
func (s UserSessions) UserDidAuthenticate(ctx context.Context, user SessionUser) error {
	// got to do a bunch of stuff here.

	userID := user.SeshUserID()
	if userID == "" {
		return ErrEmptySessionID
	}

	// Renew the session token to prevent session fixation attacks on auth change
	err := s.scs.RenewToken(ctx)
	if err != nil {
		return fmt.Errorf("Failed to renew the token for login: %w", err)
	}

	// Put the user ID into the session to track which user authenticated here
	s.scs.Put(ctx, userIDKey, userID)

	// force SCS to commit the session now, this will ensure that the session has been created and give us the session ID.
	sessionID, _, err := s.scs.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Failed to write new user session to store: %w", err)
	}

	// HACKY: We now store the sessionID in the session itself. SCS does not expose
	// the sessionID except when `Commit` is called. We should make a PR to amend that
	// but this will work for now.
	s.scs.Put(ctx, seshIDKey, sessionID)

	// Check to see if sessionID is set on the user, presently
	if user.SeshCurrentSessionID() != "" {

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

func reqWithValue(r *http.Request, key interface{}, value interface{}) *http.Request {
	newCtx := context.WithValue(r.Context(), key, value)
	return r.WithContext(newCtx)
}

// ProtectedMiddleware reads the session cookie and verifies that the request is being made by someone with a valid session
// It then stores the current session in the context, which can be retrieved with SessionFromContext(ctx)
// If the session is invalid it responds with an error and does not call any further handlers.
func (s UserSessions) ProtectedMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// First, determine if a user session has been created by looking for the user ID.
		userID := s.scs.GetString(r.Context(), userIDKey)

		if userID == "" {
			// userID is set by UserDidLogin, it being unset means there is no user session active.
			errReq := reqWithValue(r, errorHandleKey, ErrNoSession)
			s.errorHandler.ServeHTTP(w, errReq)
			return
		}

		// fetch the user with that ID.
		// SO, INTERESTING, maybe we don't need to do this anymore? We are getting this on login instead...

		// IF deletion failed though, we gotta check, right? b/c otherwise that session would just hang out
		// valid when it's explicitly not anymore. This is why we check on every load.
		user, err := s.userDelegate.FetchUserByID(userID)
		if err != nil {
			// We pass the implementor returned error into the context for the handler
			errReq := reqWithValue(r, errorHandleKey, err)
			s.errorHandler.ServeHTTP(w, errReq)
			return
		}

		// next, check that the session id is current for the use
		thisSessionID := s.scs.GetString(r.Context(), seshIDKey)
		if user.SeshCurrentSessionID() != thisSessionID {
			errReq := reqWithValue(r, errorHandleKey, ErrNotCurrentSession)
			s.errorHandler.ServeHTTP(w, errReq)
			return
		}

		userReq := reqWithValue(r, userContextKey, user)

		next.ServeHTTP(w, userReq)
	})
}

// UserFromContext returns the SessionUser that the protected middleware stored in the context.
// It will be the same user that the FetchUserByID delegate method returned, so you can safely cast
// it to your native user type.
func UserFromContext(ctx context.Context) SessionUser {
	return ctx.Value(userContextKey).(SessionUser)
}

// ErrorFromContext returns the error that caused the error handler to be called by the protected middleware.
// It will either be one of the predefined sesh errors: ErrNoSession or ErrNotCurrentSession OR it will be
// an error that wraps whatever error was returned from the FetchUserByID delegate method.
// If this function is called outside of an error handler, it will likely panic because no error has been set.
func ErrorFromContext(ctx context.Context) error {
	return ctx.Value(errorHandleKey).(error)
}

// UserDidLogout destroys the user session and removes the session cookie.
// it returns errors
func (s UserSessions) UserDidLogout(ctx context.Context) error {
	// Renew the session token to prevent session fixation attacks on auth change
	err := s.scs.RenewToken(ctx)
	if err != nil {
		return fmt.Errorf("Failed to renew the token: %w", err)
	}

	// Remove the user id from the session to indicate that the session is unauthenticated.
	s.scs.Remove(ctx, userIDKey)
	currentSessionID := s.scs.PopString(ctx, seshIDKey)

	// Go ahead and commit our changes to the session
	_, _, err = s.scs.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Failed to write new user session to store: %w", err)
	}

	// Update the users's currentSessionID
	user, ok := ctx.Value(userContextKey).(SessionUser)
	if !ok {
		return fmt.Errorf("the User was not in the context, it should have been put there by the protected middleware")
	}

	// Update the user to have no current session id
	err = s.userDelegate.UpdateUser(user, "")
	if err != nil {
		return fmt.Errorf("Failed to reset logged out user's session ID: %w", err)
	}

	// Log the deleted session.
	s.logger.LogSeshEvent(sessionDeletedMessage, map[string]string{"session_id_hash": hashSessionKey(currentSessionID)})

	return nil
}
