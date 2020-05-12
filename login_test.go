package sesh

import (
	"testing"
	"context"

	"github.com/alexedwards/scs/v2"

	"github.com/trussworks/sesh/pkg/logger"
)

func TestEmptyIDErr(t *testing.T) {

	var user testUser
	delegate := testUserDelegate{
		&user,
	}

	// setup a userSessions
	sessionManager := scs.New()
	logRecorder := logger.NewLogRecorder(logger.NewPrintLogger())
	userSessions, err := NewUserSessions(sessionManager, delegate, CustomLogger(&logRecorder))
	if err != nil {
		t.Fatal(err)
	}

	// create a user to authenticate
	user = testUser{
		ID:               "",
		Username:         "Some Pig",
		CurrentSessionID: "",
	}

	ctx, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = userSessions.UserDidAuthenticate(ctx, user)
	if err != ErrEmptySessionID {
		t.Fatal("didn't get the empty ID error.")
	}

}