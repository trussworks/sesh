package sesh

import (
	"context"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/trussworks/sesh/pkg/logger"
)

type testUser struct {
	id               string
	username         string
	currentSessionID string
}

func (u testUser) SeshUserID() string {
	return u.id
}

func (u testUser) SeshCurrentSessionID() string {
	return u.currentSessionID
}

func TestLogSessionCreated(t *testing.T) {

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logger.NewLogRecorder(logger.NewPrintLogger())
	userSessions, err := NewUserSessions(sessionManager, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user := testUser{
		id:               "42",
		username:         "Some Pig",
		currentSessionID: "",
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

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logger.NewLogRecorder(logger.NewPrintLogger())
	userSessions, err := NewUserSessions(sessionManager, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user := testUser{
		id:               "42",
		username:         "Some Pig",
		currentSessionID: "",
	}

	ctx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(ctx, user)
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidLogout(ctx)
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
