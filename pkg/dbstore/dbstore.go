package dbstore

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/trussworks/sesh"
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
func (s DBStore) FetchPossiblyExpiredSession(accountID string) (sesh.Session, error) {
	fetchQuery := `SELECT * FROM sessions WHERE account_id = $1`

	session := sesh.Session{}
	selectErr := s.db.Get(&session, fetchQuery, accountID)
	if selectErr != nil {
		if selectErr == sql.ErrNoRows {
			return sesh.Session{}, sql.ErrNoRows
		}
		return sesh.Session{}, fmt.Errorf("Failed to fetch a session row: %w", selectErr)
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
		return sesh.ErrValidSessionNotFound
	}

	return nil
}

// type sessionAccountRow struct {
// 	sesh.Session
// 	sesh.Account
// }

// // ExtendAndFetchSessionAccount fetches an account and session data from the db
// // On success it returns the account and the session
// // On failure, it can return ErrValidSessionNotFound, ErrSessionExpired, or an unexpected error
// func (s DBStore) ExtendAndFetchSessionAccount(sessionKey string, expirationDuration time.Duration) (sesh.Account, sesh.Session, error) {

// 	expirationDate := time.Now().UTC().Add(expirationDuration)

// 	// We update the session expiration date to be $DURATION from now and fetch the account and the session.
// 	fetchQuery := `UPDATE sessions
// 					SET expiration_date = $1
// 				FROM accounts
// 				WHERE
// 					sessions.account_id = accounts.id
// 					AND sessions.session_key = $2
// 					AND sessions.expiration_date > $3
// 				RETURNING
// 					sessions.session_key, sessions.account_id, sessions.expiration_date, sessions.session_index,
// 					accounts.id, accounts.form_version, accounts.form_type, accounts.username,
// 					accounts.email, accounts.external_id, accounts.status`

// 	row := sessionAccountRow{}
// 	selectErr := s.db.Get(&row, fetchQuery, expirationDate, sessionKey, time.Now().UTC())
// 	if selectErr != nil {
// 		if selectErr != sql.ErrNoRows {
// 			return sesh.Account{}, sesh.Session{}, fmt.Errorf("Unexpected error looking for valid session: %w", selectErr)
// 		}

// 		// If the above query returns no rows, either the session is expired, or it does not exist.
// 		// To determine which and return an appropriate error, we do a second query to see if it exists
// 		existsQuery := `SELECT sessions.* FROM sessions, accounts WHERE sessions.account_id = accounts.id AND sessions.session_key = $1`

// 		session := sesh.Session{}
// 		selectAgainErr := s.db.Get(&session, existsQuery, sessionKey)
// 		if selectAgainErr != nil {
// 			if selectAgainErr == sql.ErrNoRows {
// 				return sesh.Account{}, sesh.Session{}, sesh.ErrValidSessionNotFound
// 			}
// 			return sesh.Account{}, sesh.Session{}, fmt.Errorf("Unexpected error fetching single invalid session: %w", selectAgainErr)
// 		}

// 		// quick sanity check:
// 		if session.ExpirationDate.After(time.Now()) {
// 			errors.New(fmt.Sprintf("For some reason, this session we could not find was not actually expired: %s", session.SessionKey))
// 		}
// 		// The session must have been expired, not deleted.
// 		return sesh.Account{}, sesh.Session{}, sesh.ErrSessionExpired
// 	}

// 	// time.Times come back from the db with no tz info, so let's set it to UTC to be safe and consistent.
// 	row.Session.ExpirationDate = row.Session.ExpirationDate.UTC()

// 	return row.Account, row.Session, nil
// }

// ExtendAndFetchSession fetches session data from the db
// On success it returns the session
// On failure, it can return ErrValidSessionNotFound, ErrSessionExpired, or an unexpected error
func (s DBStore) ExtendAndFetchSession(sessionKey string, expirationDuration time.Duration) (sesh.Session, error) {
	fmt.Println("Fetching", sessionKey)

	expirationDate := time.Now().UTC().Add(expirationDuration)

	// We update the session expiration date to be $DURATION from now and fetch the account and the session.
	fetchQuery := `UPDATE sessions
					SET expiration_date = $1
				WHERE
					session_key = $2
					AND expiration_date > $3
				RETURNING
					session_key, account_id, expiration_date`

	session := sesh.Session{}
	selectErr := s.db.Get(&session, fetchQuery, expirationDate, sessionKey, time.Now().UTC())
	if selectErr != nil {
		if selectErr != sql.ErrNoRows {
			return sesh.Session{}, fmt.Errorf("Unexpected error looking for valid session: %w", selectErr)
		}

		// If the above query returns no rows, either the session is expired, or it does not exist.
		// To determine which and return an appropriate error, we do a second query to see if it exists
		existsQuery := `SELECT * FROM sessions WHERE session_key = $1`

		session := sesh.Session{}
		selectAgainErr := s.db.Get(&session, existsQuery, sessionKey)
		if selectAgainErr != nil {
			if selectAgainErr == sql.ErrNoRows {
				return sesh.Session{}, sesh.ErrValidSessionNotFound
			}
			return sesh.Session{}, fmt.Errorf("Unexpected error fetching single invalid session: %w", selectAgainErr)
		}

		// quick sanity check:
		if session.ExpirationDate.After(time.Now()) {
			errors.New(fmt.Sprintf("For some reason, this session we could not find was not actually expired: %s", session.SessionKey))
		}
		// The session must have been expired, not deleted.
		return sesh.Session{}, sesh.ErrSessionExpired
	}

	// time.Times come back from the db with no tz info, so let's set it to UTC to be safe and consistent.
	session.ExpirationDate = session.ExpirationDate.UTC()

	fmt.Printf("NO ERROR %+v\n", session)

	return session, nil
}
