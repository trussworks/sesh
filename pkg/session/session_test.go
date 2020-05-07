// +build skip

package session

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh/pkg/dbstore"
	"github.com/trussworks/sesh/pkg/domain"
	"github.com/trussworks/sesh/pkg/mock"
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

func getTestStore(t *testing.T) domain.SessionStorageService {
	t.Helper()

	connStr := dbURLFromEnv()

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

	sessionLog := domain.FmtLogger(true)
	session := NewSessionService(timeout, store, sessionLog)

	session.UserDidAuthenticate("foo")
}

func TestLogSessionCreatedDestroyed(t *testing.T) {

	timeout := 5 * time.Second
	store := getTestStore(t)
	defer store.Close()

	sessionLog := mock.NewLogRecorder(domain.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	sessionKey, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	createMsg, logErr := sessionLog.GetOnlyMatchingMessage(domain.SessionCreated)
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

	delMsg, delLogErr := sessionLog.GetOnlyMatchingMessage(domain.SessionDestroyed)
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
	if getErr != domain.ErrValidSessionNotFound {
		t.Fatal(getErr)
	}

	nonExistantMsg, logNonExistantErr := sessionLog.GetOnlyMatchingMessage(domain.SessionDoesNotExist)
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

	sessionLog := mock.NewLogRecorder(domain.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	sessionKey, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	logCreateMsg, logCreateErr := sessionLog.GetOnlyMatchingMessage(domain.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	if logCreateMsg.Level != "INFO" {
		t.Fatal("Wrong Log Level", logCreateMsg.Level)
	}

	_, getErr := session.GetSessionIfValid(sessionKey)
	if getErr != domain.ErrSessionExpired {
		t.Fatal("didn't get the right error back getting the expired session:", getErr)
	}

	expiredMsg, logExpiredErr := sessionLog.GetOnlyMatchingMessage(domain.SessionExpired)
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

	sessionLog := mock.NewLogRecorder(domain.FmtLogger(true))
	session := NewSessionService(timeout, store, &sessionLog)

	accountID := uuid.New().String()

	_, authErr := session.UserDidAuthenticate(accountID)
	if authErr != nil {
		t.Fatal(authErr)
	}

	_, logCreateErr := sessionLog.GetOnlyMatchingMessage(domain.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	// Now login again:
	_, authAgainErr := session.UserDidAuthenticate(accountID)
	if authAgainErr != nil {
		t.Fatal(authAgainErr)
	}

	createMessages := sessionLog.MatchingMessages(domain.SessionCreated)
	if len(createMessages) != 2 {
		t.Fatal("Should have 2 create messages now")
	}

	_, logConcurrentErr := sessionLog.GetOnlyMatchingMessage(domain.SessionConcurrentLogin)
	if logConcurrentErr != nil {
		t.Fatal(logConcurrentErr)
	}

}
