package scsstore

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/jmoiron/sqlx"
)

func hello() {
	manager := scs.New()

	fmt.Println("HI", manager)
}

type SessionUser interface {
	SeshUserID() string //-- fuuuuuu
	SeshCurrentSessionID() string
}

type UserSessions struct {
	db  *sqlx.DB
	scs *scs.SessionManager
}

func (s UserSessions) UserDidAuthenticate(ctx context.Context, user SessionUser) error {
	// got to do a bunch of stuff here.
	s.scs.Put(ctx, "user-id", user.SeshUserID())

	return nil
}

func (s UserSessions) ProtectedMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Checking On Protecting")

		// First, determine if a usersession has been created by looking for UserID.
		userID := s.scs.GetString(r.Context(), "user-id")

		if userID == "" {
			// userID is set by UserDidLogin, it being unset means there is no user session active.
			// In this case, an unauthenticated request has been made.

			// TODO: log it?
			// TODO: call error handler.
			fmt.Println("UNAUTHORIZED REQUEST MADE")
			http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
			return
		}

		// fetch the user with that ID.

		// next, check that the session id is current for the use
		// BLERG. Gotta get the current token somehow.

		next.ServeHTTP(w, r)
	})
}

func (s UserSessions) UserDidLogout(ctx context.Context) error {
	// gotta call the thingamigger

	s.scs.Remove(ctx, "user-id")

	return nil
}
