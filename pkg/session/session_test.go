package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh"
	"github.com/trussworks/sesh/pkg/dbstore"
	"github.com/trussworks/sesh/pkg/mock"
)

func getTestStore(t *testing.T) sesh.SessionStorageService {
	t.Helper()

	connStr := "postgres://postgres@localhost:5432/test_sesh?sslmode=disable"

	connection, err := sqlx.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
		return nil
	}

	return dbstore.NewDBStore(connection)

}

func TestAuthExists(t *testing.T) {

	timeout := 5 * time.Second
	store := getTestStore(t)
	defer store.Close()

	sessionLog := sesh.FmtLogger(true)
	session := NewSessionService(timeout, store, sessionLog)

	session.UserDidAuthenticate("foo")
}

func TestLogSessionCreatedDestroyed(t *testing.T) {

	timeout := 5 * time.Second
	store := getTestStore(t)
	defer store.Close()

	sessionLog := mock.NewLogRecorder(sesh.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	sessionKey, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	createMsg, logErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionCreated)
	if logErr != nil {
		t.Fatal(logErr)
	}

	if createMsg.Level != "INFO" {
		t.Fatal("Wrong Log Level", createMsg.Level)
	}

	sessionHash, ok := createMsg.Fields["session_hash"]
	if !ok {
		t.Fatal("Didn't log the hashed session key")
	}

	if sessionHash == sessionKey {
		t.Fatal("We logged the actual session key!")
	}

	delErr := session.UserDidLogout(sessionKey)
	if delErr != nil {
		t.Fatal(delErr)
	}

	delMsg, delLogErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionDestroyed)
	if delLogErr != nil {
		t.Fatal(delLogErr)
	}

	if delMsg.Level != "INFO" {
		t.Fatal("Wrong Log Level", delMsg.Level)
	}

	delSessionHash, ok := delMsg.Fields["session_hash"]
	if !ok {
		t.Fatal("Didn't log the hashed session key")
	}

	if delSessionHash == sessionKey {
		t.Fatal("We logged the actual session key!")
	}

	_, getErr := session.GetSessionIfValid(sessionKey)
	if getErr != sesh.ErrValidSessionNotFound {
		t.Fatal(getErr)
	}

	nonExistantMsg, logNonExistantErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionDoesNotExist)
	if logNonExistantErr != nil {
		t.Fatal(logNonExistantErr)
	}

	nonExistantSessionHash, ok := nonExistantMsg.Fields["session_hash"]
	if !ok {
		t.Fatal("Didn't log the hashed session key")
	}

	if nonExistantSessionHash == sessionKey {
		t.Fatal("We logged the actual session key!")
	}

}

func TestLogSessionExpired(t *testing.T) {

	timeout := -5 * time.Second
	store := getTestStore(t)
	defer store.Close()

	sessionLog := mock.NewLogRecorder(sesh.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	sessionKey, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	logCreateMsg, logCreateErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	if logCreateMsg.Level != "INFO" {
		t.Fatal("Wrong Log Level", logCreateMsg.Level)
	}

	_, getErr := session.GetSessionIfValid(sessionKey)
	if getErr != sesh.ErrSessionExpired {
		t.Fatal("didn't get the right error back getting the expired session:", getErr)
	}

	expiredMsg, logExpiredErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionExpired)
	if logExpiredErr != nil {
		t.Fatal(logExpiredErr)
	}

	expiredSessionHash, ok := expiredMsg.Fields["session_hash"]
	if !ok {
		t.Fatal("Didn't log the hashed session key")
	}

	if expiredSessionHash == sessionKey {
		t.Fatal("We logged the actual session key!")
	}

	// make sure you can re-auth after ending a session
	_, newAuthErr := session.UserDidAuthenticate(accountID)
	if newAuthErr != nil {
		t.Fatal(newAuthErr)
	}

}

// TestLogConcurrentSession tests that if you create a session, then create a new session over it, we log something.
func TestLogConcurrentSession(t *testing.T) {

	timeout := 5 * time.Second
	store := getTestStore(t)
	defer store.Close()

	sessionLog := mock.NewLogRecorder(sesh.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	_, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	_, logCreateErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	// Now login again:
	_, authAgainErr := session.UserDidAuthenticate(accountID)
	if authAgainErr != nil {
		t.Fatal(authAgainErr)
	}

	createMessages := sessionLog.MatchingMessages(sesh.SessionCreated)
	if len(createMessages) != 2 {
		t.Fatal("Should have 2 create messages now")
	}

	_, logConcurrentErr := sessionLog.GetOnlyMatchingMessage(sesh.SessionConcurrentLogin)
	if logConcurrentErr != nil {
		t.Fatal(logConcurrentErr)
	}

}
