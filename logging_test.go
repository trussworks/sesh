package sesh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/trussworks/sesh/pkg/logrecorder"
)

type testUser struct {
	ID               string
	Username         string
	CurrentSessionID string
}

func (u testUser) SeshUserID() string {
	return u.ID
}

func (u testUser) SeshCurrentSessionID() string {
	return u.CurrentSessionID
}

type testUserDelegate struct {
	user *testUser
}

func (d testUserDelegate) FetchUserByID(id string) (SessionUser, error) {
	return nil, nil
}

func (d testUserDelegate) UpdateUser(user SessionUser, currentSessionID string) error {
	if user.SeshUserID() != d.user.ID {
		return errors.New("BAD User ID")
	}

	d.user.CurrentSessionID = currentSessionID
	return nil
}

func TestLogSessionCreated(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{
		&user,
	}

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logrecorder.NewLogRecorder(newDefaultLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "42",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	ctx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged session creation
	line, err := logRecorder.GetOnlyMatchingMessage("New User Session Created")
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged a session id hash
	_, ok := line.Fields["session_id_hash"]
	if !ok {
		t.Fatal("Should have logged a session id hash")
	}

	// TODO: check that we didn't log the real session hash.

}

func TestLogSessionDestroyed(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{
		&user,
	}

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logrecorder.NewLogRecorder(newDefaultLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "42",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	ctx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

	// UserDidLogout depends on the middleware having run.
	userContext := context.WithValue(ctx, userContextKey, user)

	err = userSessions.UserDidLogout(userContext)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged session creation
	line, err := logRecorder.GetOnlyMatchingMessage("User Session Destroyed")
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged a session id hash
	_, ok := line.Fields["session_id_hash"]
	if !ok {
		t.Fatal("Should have logged a session id hash")
	}

	// TODO: Check that we didn't log the real session hash

}

func TestLogConcurrentSession(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{&user}

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logrecorder.NewLogRecorder(newDefaultLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "42",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	firstCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	secondCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(firstCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(secondCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged concurrent session
	line, err := logRecorder.GetOnlyMatchingMessage("User logged in with a concurrent active session")
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged a session id hash
	_, ok := line.Fields["session_id_hash"]
	if !ok {
		t.Fatal("Should have logged a session id hash")
	}

	// TODO: check that we didn't log the real session hash.

}

func TestExpiredSession(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{&user}

	// setup a userSessions
	sessionManager := scs.New()
	sessionManager.IdleTimeout = time.Second / 2

	logRecorder := logrecorder.NewLogRecorder(newDefaultLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "42",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	firstCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	secondCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(firstCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	err = userSessions.UserDidAuthenticate(secondCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged concurrent session
	line, err := logRecorder.GetOnlyMatchingMessage("Previous session expired")
	if err != nil {
		t.Fatal(err)
	}

	// Check that we logged a session id hash
	_, ok := line.Fields["session_id_hash"]
	if !ok {
		t.Fatal("Should have logged a session id hash")
	}

	// TODO: check that we didn't log the real session hash.

}

func TestLoginLogout(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{&user}

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logrecorder.NewLogRecorder(newDefaultLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "42",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	firstCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	secondCtx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(firstCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	// Logout relies on the user being in the context.
	firstUserContext := context.WithValue(firstCtx, userContextKey, user)

	err = userSessions.UserDidLogout(firstUserContext)
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(secondCtx, user)
	if err != nil {
		t.Fatal(err)
	}

	if len(logRecorder.MatchingMessages("User logged in with a concurrent active session")) != 0 {
		t.Log("Should not have logged concurrent message")
		t.Fail()
	}

	if len(logRecorder.MatchingMessages("Session Expired")) != 0 {
		t.Log("Should not have logged expired message")
		t.Fail()
	}

}
