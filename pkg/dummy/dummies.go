package dummy

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/alexedwards/scs/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh"
)

type appUser struct {
	ID               string         `db:"id"`
	Username         string         `db:"name"`
	CurrentSessionID sql.NullString `db:"current_session_id"`
}

func (u appUser) SeshUserID() string {
	return u.ID
}

func (u appUser) SeshCurrentSessionID() string {
	if !u.CurrentSessionID.Valid {
		return ""
	}
	return u.CurrentSessionID.String
}

func fetchUserByUsername(db *sqlx.DB, username string) (appUser, error) {
	fetchQuery := `SELECT * FROM users WHERE name=$1`
	var user appUser
	err := db.Get(&user, fetchQuery, username)
	if err != nil {
		return appUser{}, err
	}

	return user, nil
}

func updateUserDelegate(db *sqlx.DB) sesh.UserUpdateDelegate {
	return func(userID string, currentID string) error {
		fmt.Println("SAVING NEW DEALIE")

		updateQuery := `UPDATE users SET current_session_id=$1 WHERE id=$2`

		_, err := db.Exec(updateQuery, currentID, userID)
		if err != nil {
			return err
		}

		return nil
	}
}

func loginEndpoint(db *sqlx.DB, us sesh.UserSessions) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("LOGINGIN")

		usernameb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Body", 400)
			return
		}

		username := string(usernameb)

		// load the user by ID.
		user, err := fetchUserByUsername(db, username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// this is bad auth.
				fmt.Println("User does not exist: ", username)
				http.Error(w, "No Such User", 401)
				return
			}
			fmt.Println("Unexpected error loading user: ", err)
			http.Error(w, "Server Error", 500)
			return
		}

		err = us.UserDidAuthenticate(r.Context(), user)
		if err != nil {
			fmt.Println("Error Authenticating Logged In User: ", err)
			http.Error(w, "Server Error", 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func protectedEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Println("PROTECTED")
}

func logoutEndpoint(us sesh.UserSessions) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Logging OUT")

		err := us.UserDidLogout(r.Context())
		if err != nil {
			fmt.Println("Error Logging Out User: ", err)
			http.Error(w, "Server Error", 500)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	}
}

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

func setupMux(db *sqlx.DB) http.Handler {
	mux := http.NewServeMux()

	updateFn := updateUserDelegate(db)

	sessionManager := scs.New()
	userSessions, err := sesh.NewUserSessions(sessionManager, updateFn)
	if err != nil {
		panic(err)
	}

	protectedMiddleware := userSessions.ProtectedMiddleware

	mux.HandleFunc("/login", loginEndpoint(db, userSessions))
	mux.Handle("/protected", protectedMiddleware(http.HandlerFunc(protectedEndpoint)))
	mux.Handle("/logout", protectedMiddleware(http.HandlerFunc(logoutEndpoint(userSessions))))

	return sessionManager.LoadAndSave(mux)
}
