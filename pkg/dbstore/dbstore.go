package dbstore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trussworks/sesh/pkg/domain"
)

type DBStore struct {
	db *sqlx.DB
}

func NewDBStore(db *sqlx.DB) DBStore {
	return DBStore{
		db,
	}
}

func (s DBStore) Close() error {
	return s.db.Close()
}

// CreateSession creates a new session. It errors if a valid session already exists.
func (s DBStore) CreateSession(accountID string, sessionKey string, expirationDuration time.Duration) error {
	expirationDate := time.Now().UTC().Add(expirationDuration)

	createQuery := `INSERT INTO sessions (session_key, account_id, expiration_date)
		VALUES ($1, $2, $3)`

	_, createErr := s.db.Exec(createQuery, sessionKey, accountID, expirationDate)
	if createErr != nil {

		return fmt.Errorf("Unexpectedly failed to create a session: %w", createErr)
	}

	return nil
}

// FetchPossiblyExpiredSession returns a session row by account ID regardless of wether it is expired
// This is potentially dangerous, it is only intended to be used during the new login flow, never to check
// on a valid session for authentication purposes.
func (s DBStore) FetchPossiblyExpiredSession(accountID string) (domain.Session, error) {
	fetchQuery := `SELECT * FROM sessions WHERE account_id = $1`

	session := domain.Session{}
	selectErr := s.db.Get(&session, fetchQuery, accountID)
	if selectErr != nil {
		if selectErr == sql.ErrNoRows {
			return domain.Session{}, sql.ErrNoRows
		}
		return domain.Session{}, fmt.Errorf("Failed to fetch a session row: %w", selectErr)
	}

	return session, nil

}

// DeleteSession removes a session record from the db
func (s DBStore) DeleteSession(sessionKey string) error {
	deleteQuery := "DELETE FROM sessions WHERE session_key = $1"

	sqlResult, deleteErr := s.db.Exec(deleteQuery, sessionKey)
	if deleteErr != nil {
		return fmt.Errorf("Failed to delete session: %w", deleteErr)
	}

	rowsAffected, _ := sqlResult.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrValidSessionNotFound
	}

	return nil
}

// ExtendAndFetchSession fetches session data from the db
// On success it returns the session
// On failure, it can return ErrValidSessionNotFound, ErrSessionExpired, or an unexpected error
func (s DBStore) ExtendAndFetchSession(sessionKey string, expirationDuration time.Duration) (domain.Session, error) {
	expirationDate := time.Now().UTC().Add(expirationDuration)

	// We update the session expiration date to be $DURATION from now and fetch the account and the session.
	fetchQuery := `UPDATE sessions
					SET expiration_date = $1
				WHERE
					session_key = $2
					AND expiration_date > $3
				RETURNING
					session_key, account_id, expiration_date`

	session := domain.Session{}
	selectErr := s.db.Get(&session, fetchQuery, expirationDate, sessionKey, time.Now().UTC())
	if selectErr != nil {
		if selectErr != sql.ErrNoRows {
			return domain.Session{}, fmt.Errorf("Unexpected error looking for valid session: %w", selectErr)
		}

		// If the above query returns no rows, either the session is expired, or it does not exist.
		// To determine which and return an appropriate error, we do a second query to see if it exists
		existsQuery := `SELECT * FROM sessions WHERE session_key = $1`

		session := domain.Session{}
		selectAgainErr := s.db.Get(&session, existsQuery, sessionKey)
		if selectAgainErr != nil {
			if selectAgainErr == sql.ErrNoRows {
				return domain.Session{}, domain.ErrValidSessionNotFound
			}
			return domain.Session{}, fmt.Errorf("Unexpected error fetching single invalid session: %w", selectAgainErr)
		}

		// quick sanity check:
		if session.ExpirationDate.After(time.Now().UTC()) {
			errors.New(fmt.Sprintf("For some reason, this session we could not find was not actually expired: %s", session.SessionKey))
		}
		// The session must have been expired, not deleted.
		return domain.Session{}, domain.ErrSessionExpired
	}

	// time.Times come back from the db with no tz info, so let's set it to UTC to be safe and consistent.
	session.ExpirationDate = session.ExpirationDate.UTC()

	fmt.Printf("NO ERROR %+v\n", session)

	return session, nil
}
