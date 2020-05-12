package sesh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
)

func TestCustomFailureHandler(t *testing.T) {

	var customCalled bool
	var passedErr error
	failureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customCalled = true
		passedErr = r.Context().Value(errorHandleKey).(error)
		fmt.Println("WE WERE CALLED")
	})

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("should never be called, there is no one logged in.")
	})

	sessionManager := scs.New()
	userSessions, err := NewUserSessions(sessionManager, nil, CustomErrorHandler(failureHandler))
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/something/protected", nil)
	scsContext, err := sessionManager.LoadNew(r.Context())
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithContext(scsContext)

	wrappedHandler := userSessions.ProtectedMiddleware(protectedHandler)

	wrappedHandler.ServeHTTP(w, r)

	if !customCalled {
		t.Log("Our custom logger wasn't even called.")
		t.Fail()
	}

	if !errors.Is(passedErr, ErrNoSession) {
		t.Log("Didn't get the right error out: ", passedErr)
		t.Fail()
	}
}

type failUserFetchDelegate struct {
}

func (d failUserFetchDelegate) FetchUserByID(id string) (SessionUser, error) {
	return nil, errors.New("Fetch Failure")
}

func (d failUserFetchDelegate) UpdateUser(user SessionUser, currentSessionID string) error {
	return nil
}

func TestFetchFailure(t *testing.T) {

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("should never be called, there will be an error.")
	})

	sessionManager := scs.New()
	userSessions, err := NewUserSessions(sessionManager, failUserFetchDelegate{})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/something/protected", nil)

	// create a scs context
	scsContext, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// log a user in
	err = userSessions.UserDidAuthenticate(scsContext, testUser{
		ID: "one",
	})
	if err != nil {
		t.Fatal(err)
	}

	r = r.WithContext(scsContext)

	wrappedHandler := userSessions.ProtectedMiddleware(protectedHandler)

	wrappedHandler.ServeHTTP(w, r)

	resp := w.Result()

	if resp.StatusCode != 500 {
		t.Fatal("We should have gotten server error", resp.StatusCode)
	}

}

func TestCustomFetchFailure(t *testing.T) {

	var customCalled bool
	var passedErr error
	failureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customCalled = true
		passedErr = r.Context().Value(errorHandleKey).(error)
		fmt.Println("WE WERE CALLED")
	})

	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("should never be called, there will be an error.")
	})

	sessionManager := scs.New()
	userSessions, err := NewUserSessions(sessionManager, failUserFetchDelegate{}, CustomErrorHandler(failureHandler))
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/something/protected", nil)

	// create a scs context
	scsContext, err := sessionManager.LoadNew(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// log a user in
	err = userSessions.UserDidAuthenticate(scsContext, testUser{
		ID: "one",
	})
	if err != nil {
		t.Fatal(err)
	}

	r = r.WithContext(scsContext)

	wrappedHandler := userSessions.ProtectedMiddleware(protectedHandler)

	wrappedHandler.ServeHTTP(w, r)

	if !customCalled {
		t.Log("Our custom logger wasn't even called.")
		t.Fail()
	}

	if passedErr.Error() != "Fetch Failure" {
		t.Log("Didn't get the right error out: ", passedErr)
		t.Fail()
	}

}
