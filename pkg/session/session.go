package session

import (
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/pkg/errors"

	"github.com/trussworks/sesh"
)

// Service represents a StorageService internally
type Service struct {
	timeout time.Duration
	store   sesh.SessionStorageService
	log     sesh.LogService
}

// NewSessionService returns a SessionService
func NewSessionService(timeout time.Duration, store sesh.SessionStorageService, log sesh.LogService) *Service {
	return &Service{
		timeout,
		store,
		log,
	}
}

// generateSessionKey generates a cryptographically random session key
func generateSessionKey() (string, error) {
	secureBytes := securecookie.GenerateRandomKey(32)
	if secureBytes == nil {
		return "", errors.New("Failed to generate random data for a key")
	}

	secureString := hex.EncodeToString(secureBytes)

	return secureString, nil

}

func hashSessionKey(sessionKey string) string {
	hashed := sha512.Sum512([]byte(sessionKey))
	hexEncoded := hex.EncodeToString(hashed[:])
	return hexEncoded[:12]
}

// UserDidAuthenticate returns a session key and an error if applicable
func (s Service) UserDidAuthenticate(accountID string) (string, error) {
	sessionKey, keyErr := generateSessionKey()
	if keyErr != nil {
		return "", keyErr
	}

	// First, check to see if there is an extant session in the DB, expired or otherwise
	extantSession, fetchErr := s.store.FetchPossiblyExpiredSession(accountID)
	// Little uncommon Go logic here. First, return the error if it wasn't a Not Found error
	if fetchErr != nil && fetchErr != sql.ErrNoRows {
		return "", fetchErr
	}
	// Then, if we sucessfully got a session back, deal with that. If we got ErrNoRows, we just want to skip this part.
	if fetchErr == nil {
		if extantSession.ExpirationDate.Before(time.Now().UTC()) {
			// If the session is expired, delete it.
			s.log.Info(fmt.Sprintf("Creating new Session: Previous session expired at %s", extantSession.ExpirationDate), sesh.LogFields{"account_id": accountID})
			delErr := s.store.DeleteSession(extantSession.SessionKey)
			if delErr != nil {
				s.log.WarnError("Unexpectedly failed to delete an expired session during authentication", delErr, sesh.LogFields{"account_id": accountID})
				// We will continue and attempt to create the new session here, anyway.
			}
		} else {
			// If the session is valid, log that this is a concurrent login, and then delete it.
			s.log.Info(sesh.SessionConcurrentLogin, sesh.LogFields{"prev_session_hash": hashSessionKey(extantSession.SessionKey)})
			delErr := s.store.DeleteSession(extantSession.SessionKey)
			if delErr != nil {
				s.log.WarnError("Unexpectedly failed to delete an expired session during authentication", delErr, sesh.LogFields{"account_id": accountID})
				// We will continue and attempt to create the new session here, anyway.
			}
		}
	}

	createErr := s.store.CreateSession(accountID, sessionKey, s.timeout)
	if createErr != nil {
		return "", createErr
	}
	s.log.Info(sesh.SessionCreated, sesh.LogFields{"session_hash": hashSessionKey(sessionKey)})

	return sessionKey, createErr
}

// GetSessionIfValid returns a session if the session key is valid and an error otherwise
func (s Service) GetSessionIfValid(sessionKey string) (sesh.Session, error) {
	session, fetchErr := s.store.ExtendAndFetchSession(sessionKey, s.timeout)
	if fetchErr != nil {
		if fetchErr == sesh.ErrSessionExpired {
			s.log.Info(sesh.SessionExpired, sesh.LogFields{"session_hash": hashSessionKey(sessionKey)})
		} else if fetchErr == sesh.ErrValidSessionNotFound {
			s.log.Info(sesh.SessionDoesNotExist, sesh.LogFields{"session_hash": hashSessionKey(sessionKey)})
		}

		return sesh.Session{}, fetchErr
	}

	return session, nil
}

// UserDidLogout attempts to end the session and returns an error on failure
func (s Service) UserDidLogout(sessionKey string) error {
	delErr := s.store.DeleteSession(sessionKey)
	if delErr != nil {
		return delErr
	}

	s.log.Info(sesh.SessionDestroyed, sesh.LogFields{"session_hash": hashSessionKey(sessionKey)})

	return nil
}
