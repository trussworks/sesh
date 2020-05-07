// +build skip

package dbstore

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh/pkg/domain"
)

func dbURLFromEnv() string {
	host := os.Getenv("DATABASE_HOST")
	port := os.Getenv("DATABASE_PORT")
	name := os.Getenv("DATABASE_NAME")
	user := os.Getenv("DATABASE_USER")
	// password := os.Getenv("DATABASE_PASSWORD")
	sslmode := os.Getenv("DATABASE_SSL_MODE")

	connStr := fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s", user, host, port, name, sslmode)
	return connStr
}

func getTestStore() (DBStore, error) {
	connStr := dbURLFromEnv()

	connection, err := sqlx.Open("postgres", connStr)
	if err != nil {
		return DBStore{}, fmt.Errorf("error connecting to database using sqlx.Open: %w", err)
	}

	return NewDBStore(connection), nil

}

// getTestObjects gives you a Store, and two random UUIDs
func getTestObjects(t *testing.T) (DBStore, string, string) {
	t.Helper()

	store, storeErr := getTestStore()
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	accountID := uuid.New().String()
	sessionKey := uuid.New().String()
	return store, accountID, sessionKey
}

func timeIsCloseToTime(test time.Time, expected time.Time, diff time.Duration) bool {
	lowerBound := expected.Add(-diff)
	upperBound := expected.Add(diff)

	if !(test.After(lowerBound) && test.Before(upperBound)) {
		return false
	}
	return true
}

func TestFetchExistingSessionToOverwrite(t *testing.T) {
	store, accountID, firstSessionKey := getTestObjects(t)
	expirationDuration := 5 * time.Minute

	firstCreateErr := store.CreateSession(accountID, firstSessionKey, expirationDuration)
	if firstCreateErr != nil {
		t.Fatal(firstCreateErr)
	}

	firstSession, fetchErr := store.ExtendAndFetchSession(firstSessionKey, expirationDuration)
	if fetchErr != nil {
		t.Fatal(fetchErr)
	}

	// Duplicate what we do in Sessions.UserDidAuth
	fetchedSession, fetchErr := store.FetchPossiblyExpiredSession(accountID)
	if fetchErr != nil {
		t.Fatal(fetchErr)
	}

	if fetchedSession.SessionKey != firstSessionKey {
		t.Fatal("Didn't get the same session back!")
	}

	delErr := store.DeleteSession(firstSessionKey)
	if delErr != nil {
		t.Fatal(delErr)
	}

	secondSessionKey := uuid.New().String()
	secondCreateErr := store.CreateSession(accountID, secondSessionKey, expirationDuration)
	if secondCreateErr != nil {
		t.Fatal(secondCreateErr)
	}

	secondSession, fetchErr := store.ExtendAndFetchSession(secondSessionKey, expirationDuration)
	if fetchErr != nil {
		t.Fatal(fetchErr)
	}

	if firstSession.AccountID != secondSession.AccountID {
		t.Fatal("both fetches should return the same account")
	}

	_, expectedFetchErr := store.ExtendAndFetchSession(firstSessionKey, expirationDuration)
	if expectedFetchErr != domain.ErrValidSessionNotFound {
		t.Fatal("using the first session key should cause an error to be thrown, since it has been overwritten, got", expectedFetchErr)
	}
}

func TestFetchSessionReturnsAccountAndSessionOnValidSession(t *testing.T) {
	store, accountID, sessionKey := getTestObjects(t)
	expirationDuration := 5 * time.Minute

	createErr := store.CreateSession(accountID, sessionKey, expirationDuration)
	if createErr != nil {
		t.Fatal(createErr)
	}

	actualSession, err := store.ExtendAndFetchSession(sessionKey, expirationDuration)
	if err != nil {
		t.Fatal(err)
	}

	// if actualSession.AccountID != accountID {
	// 	t.Fatal(fmt.Sprintf("actual returned accountID does not match expected returned accountID:\n%v\n%v", actualSession.AccountID, accountID))
	// }

	if !(actualSession.AccountID == accountID && actualSession.SessionKey == sessionKey) {
		t.Fatal("Didn't get the expected session values back", actualSession)
	}

	expectedExpiration := time.Now().UTC().Add(expirationDuration)
	if !timeIsCloseToTime(actualSession.ExpirationDate, expectedExpiration, time.Second) {
		t.Fatal("The returned expiration date is different from the expected", actualSession.ExpirationDate, expectedExpiration)
	}

}

func TestFetchSessionExtendsValidSession(t *testing.T) {
	store, accountID, sessionKey := getTestObjects(t)

	shortInitialDuration := 5 * time.Minute

	createErr := store.CreateSession(accountID, sessionKey, shortInitialDuration)
	if createErr != nil {
		t.Fatal(createErr)
	}

	session, err := store.ExtendAndFetchSession(sessionKey, shortInitialDuration)
	if err != nil {
		t.Fatal(err)
	}

	expectedExpiration := time.Now().UTC().Add(shortInitialDuration)
	if !timeIsCloseToTime(session.ExpirationDate, expectedExpiration, time.Second) {
		t.Fatal("The returned expiration date is different from the expected", session.ExpirationDate, expectedExpiration)
	}

	longDuration := 5 * time.Hour
	secondSession, err := store.ExtendAndFetchSession(sessionKey, longDuration)
	if err != nil {
		t.Fatal(err)
	}

	expectedLongExpiration := time.Now().UTC().Add(longDuration)
	if !timeIsCloseToTime(secondSession.ExpirationDate, expectedLongExpiration, time.Second) {
		t.Fatal("The returned expiration date is different from the expected", secondSession.ExpirationDate, expectedLongExpiration)
	}

}

func TestFetchSessionReturnsErrorOnExpiredSession(t *testing.T) {
	store, accountID, sessionKey := getTestObjects(t)
	expirationDuration := -10 * time.Minute

	createErr := store.CreateSession(accountID, sessionKey, expirationDuration)
	if createErr != nil {
		t.Fatal(createErr)
	}

	_, err := store.ExtendAndFetchSession(sessionKey, expirationDuration)
	if err != domain.ErrSessionExpired {
		t.Fatal(err)
	}
}

func TestDeleteSessionRemovesRecord(t *testing.T) {
	store, accountID, sessionKey := getTestObjects(t)
	expirationDuration := 5 * time.Minute
	store.CreateSession(accountID, sessionKey, expirationDuration)

	fetchQuery := `SELECT * FROM sessions WHERE session_key = $1`
	row := domain.Session{}
	store.db.Get(&row, fetchQuery, sessionKey)
	if row.SessionKey != sessionKey {
		t.Fatal("new session should have been created")
	}

	err := store.DeleteSession(sessionKey)
	if err != nil {
		t.Fatal("encountered issue when trinyg to delete session")
	}

	row = domain.Session{}
	expectedErr := store.db.Get(&row, fetchQuery, sessionKey)
	if expectedErr != sql.ErrNoRows {
		t.Fatal("session should not exist")
	}
}

func TestDeleteSessionReturnsErrIfSessionNotFound(t *testing.T) {
	store, storeErr := getTestStore()
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	defer store.Close()
	sessionKeyWithNoAssociatedRecord := uuid.New().String()

	err := store.DeleteSession(sessionKeyWithNoAssociatedRecord)
	if err != domain.ErrValidSessionNotFound {
		t.Fatal("session should not exist")
	}
}

func TestSessionDBConstraints(t *testing.T) {
	s, accountID, sessionKey := getTestObjects(t)
	expirationDuration := 5 * time.Minute
	expirationDate := time.Now().UTC().Add(expirationDuration)

	justCreateQuery := `INSERT INTO Sessions (session_key, account_id, expiration_date) VALUES ($1, $2, $3)`

	// // bogus account ID
	// _, createErr := s.db.Exec(justCreateQuery, sessionKey, "200", expirationDate)
	// if createErr == nil {
	// 	t.Log("Should not have created a bogus session: bogus account id")
	// 	t.Fail()

	// 	s.DeleteSession(sessionKey)
	// }

	// missing account.ID
	_, createErr := s.db.Exec(justCreateQuery, sessionKey, sql.NullString{}, expirationDate)
	if createErr == nil {
		t.Log("Should not have created a bogus session: missing account id")
		t.Fail()

		s.DeleteSession(sessionKey)
	}

	// nil sessionkey
	_, createErr = s.db.Exec(justCreateQuery, sql.NullString{}, accountID, expirationDate)
	if createErr == nil {
		t.Log("Should not have created a bogus session: missing sessionkey")
		t.Fail()
	}

	noDateQuery := `INSERT INTO Sessions (session_key, account_id) VALUES ($1, $2)`

	_, createErr = s.db.Exec(noDateQuery, sessionKey, accountID)
	if createErr == nil {
		t.Log("Should not have created a bogus session: missing date")
		t.Fail()
	}

	// this creates a record we can check UNIQE against
	_, createErr = s.db.Exec(justCreateQuery, sessionKey, accountID, expirationDate)
	if createErr != nil {
		t.Log("Should have created a valid session")
		t.Fail()
	}

	// duplicate accountid
	differentSessionKey := uuid.New().String()
	_, createErr = s.db.Exec(justCreateQuery, differentSessionKey, accountID, expirationDate)
	if createErr == nil {
		t.Log("Should not have created a session with a duplicate account ID")
		t.Fail()
	}

	// duplicate sessionkey
	differentAccountID := uuid.New().String()
	_, createErr = s.db.Exec(justCreateQuery, sessionKey, differentAccountID, expirationDate)
	if createErr == nil {
		t.Log("Should not have created a session with a duplicate SessionKey")
		t.Fail()
	}

}

// func TestDeleteAccountDeletesSession(t *testing.T) {
// 	store, account, sessionKey := getTestObjects(t)
// 	expirationDuration := -5 * time.Minute

// 	firstCreateErr := store.CreateSession(account.ID, sessionKey, api.NullString(), expirationDuration)
// 	if firstCreateErr != nil {
// 		t.Fatal(firstCreateErr)
// 	}

// 	deleteAccountQuery := `DELETE FROM accounts WHERE id = $1`
// 	store.db.MustExec(deleteAccountQuery, account.ID)

// 	_, fetchErr := store.FetchPossiblyExpiredSession(account.ID)
// 	if fetchErr == nil {
// 		t.Fatal("Should have failed to find a session matching this account.")
// 	}
// }
