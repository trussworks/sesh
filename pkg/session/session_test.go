package session

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/us-dod-saber/culper/api"
	"github.com/us-dod-saber/culper/api/env"
	"github.com/us-dod-saber/culper/api/log"
	"github.com/us-dod-saber/culper/api/mock"
	"github.com/us-dod-saber/culper/api/simplestore"
)

// randomEmail an example.com email address with 10 random characters
func randomEmail() string {

	rand.Seed(time.Now().UTC().UnixNano())

	len := 10
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		aint := int('a')
		zint := int('z')
		char := aint + rand.Intn(zint-aint)
		bytes[i] = byte(char)
	}

	email := string(bytes) + "@example.com"

	return email

}

func createTestAccount(t *testing.T, store api.StorageService) api.Account {
	t.Helper()

	email := randomEmail()

	account := api.Account{
		Username:    email,
		Email:       api.NonNullString(email),
		FormType:    "SF86",
		FormVersion: "2017-07",
		Status:      api.StatusIncomplete,
		ExternalID:  uuid.New().String(),
	}

	createErr := store.CreateAccount(&account)
	if createErr != nil {
		t.Fatal(createErr)
	}

	return account

}

func getSimpleStore(t *testing.T) api.StorageService {
	env := &env.Native{}
	os.Setenv(api.LogLevel, "info")
	env.Configure()

	logLevel := os.Getenv(api.LogLevel)

	log, logErr := log.NewLogService(logLevel, log.JSONFormat)
	if logErr != nil {
		t.Fatal("Error configuring logger", logErr)
	}

	dbConf := simplestore.DBConfig{
		User:     env.String(api.DatabaseUser),
		Password: env.String(api.DatabasePassword),
		Address:  env.String(api.DatabaseHost),
		DBName:   env.String(api.TestDatabaseName),
		SSLMode:  env.String(api.DatabaseSSLMode),
	}

	connString := simplestore.PostgresConnectURI(dbConf)

	serializer := simplestore.JSONSerializer{}

	store, storeErr := simplestore.NewSimpleStore(connString, log, serializer)
	if storeErr != nil {
		fmt.Println("Unable to configure simple store", storeErr)
		os.Exit(1)
	}
	return store
}

func TestAuthExists(t *testing.T) {

	timeout := 5 * time.Second
	store := getSimpleStore(t)
	defer store.Close()

	sessionLog := &mock.LogService{}
	session := NewSessionService(timeout, store, sessionLog)

	session.UserDidAuthenticate(0, api.NullString())
}

func TestLogSessionCreatedDestroyed(t *testing.T) {

	timeout := 5 * time.Second
	store := getSimpleStore(t)
	defer store.Close()

	sessionLog := &mock.LogRecorder{}
	session := NewSessionService(timeout, store, sessionLog)

	account := createTestAccount(t, store.(simplestore.SimpleStore))

	sessionKey, authErr := session.UserDidAuthenticate(account.ID, api.NullString())
	if authErr != nil {
		t.Fatal(authErr)
	}

	createMsg, logErr := sessionLog.GetOnlyMatchingMessage(api.SessionCreated)
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

	delMsg, delLogErr := sessionLog.GetOnlyMatchingMessage(api.SessionDestroyed)
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

	_, _, getErr := session.GetAccountIfSessionIsValid(sessionKey)
	if getErr != api.ErrValidSessionNotFound {
		t.Fatal(getErr)
	}

	nonExistantMsg, logNonExistantErr := sessionLog.GetOnlyMatchingMessage(api.SessionDoesNotExist)
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
	store := getSimpleStore(t)
	defer store.Close()

	sessionLog := &mock.LogRecorder{}
	session := NewSessionService(timeout, store, sessionLog)

	account := createTestAccount(t, store.(simplestore.SimpleStore))

	sessionKey, authErr := session.UserDidAuthenticate(account.ID, api.NullString())
	if authErr != nil {
		t.Fatal(authErr)
	}

	logCreateMsg, logCreateErr := sessionLog.GetOnlyMatchingMessage(api.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	if logCreateMsg.Level != "INFO" {
		t.Fatal("Wrong Log Level", logCreateMsg.Level)
	}

	_, _, getErr := session.GetAccountIfSessionIsValid(sessionKey)
	if getErr != api.ErrSessionExpired {
		t.Fatal("didn't get the right error back getting the expired session:", getErr)
	}

	expiredMsg, logExpiredErr := sessionLog.GetOnlyMatchingMessage(api.SessionExpired)
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
	_, newAuthErr := session.UserDidAuthenticate(account.ID, api.NullString())
	if newAuthErr != nil {
		t.Fatal(newAuthErr)
	}

}

// TestLogConcurrentSession tests that if you create a session, then create a new session over it, we log something.
func TestLogConcurrentSession(t *testing.T) {

	timeout := 5 * time.Second
	store := getSimpleStore(t)
	defer store.Close()

	sessionLog := &mock.LogRecorder{}
	session := NewSessionService(timeout, store, sessionLog)

	account := createTestAccount(t, store.(simplestore.SimpleStore))

	_, authErr := session.UserDidAuthenticate(account.ID, api.NullString())
	if authErr != nil {
		t.Fatal(authErr)
	}

	_, logCreateErr := sessionLog.GetOnlyMatchingMessage(api.SessionCreated)
	if logCreateErr != nil {
		t.Fatal(logCreateErr)
	}

	// Now login again:
	_, authAgainErr := session.UserDidAuthenticate(account.ID, api.NullString())
	if authAgainErr != nil {
		t.Fatal(authAgainErr)
	}

	createMessages := sessionLog.MatchingMessages(api.SessionCreated)
	if len(createMessages) != 2 {
		t.Fatal("Should have 2 create messages now")
	}

	_, logConcurrentErr := sessionLog.GetOnlyMatchingMessage(api.SessionConcurrentLogin)
	if logConcurrentErr != nil {
		t.Fatal(logConcurrentErr)
	}

}