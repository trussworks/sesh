package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh"
	"github.com/trussworks/sesh/pkg/domain"
)

// This is a server for testing the basic auth flow.

//  /login -- redirect to protected
// /logout -- redirect to /
// /protected -- logout button
// / -- login button and protected button

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

type testServer struct {
	sessions sesh.Sessions
}

func newTestServer(sessions sesh.Sessions) testServer {
	return testServer{
		sessions,
	}
}

func (s testServer) homepage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `
	<html>
	<head>
	<title>frontpage</title>
	</head>
	<body>
	<h1>Front Page</h1>
	<p><a href="/login">Login</a></p>
	<p><a href="/protected">Protected</a></p>
	</body>
	</html>
	`)
}

func (s testServer) login(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Logging in user: 1")

	_, err := s.sessions.UserDidAuthenticate(w, "1")
	if err != nil {
		fmt.Println("Error creating session: ", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/protected", http.StatusTemporaryRedirect)

}

func (s testServer) protected(w http.ResponseWriter, r *http.Request) {
	session := sesh.SessionFromContext(r.Context())

	fmt.Fprintf(w, `
	<html>
	<head>
	<title>protected</title>
	</head>
	<body>
	<h1>Protected Page</h1>
	<p>Hello userID: %s!</p>
	<p><a href="/logout">Logout</a></p>
	</body>
	</html>
	`, session.AccountID)
}

func (s testServer) logout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Logging out user 1")

	err := s.sessions.UserDidLogout(w, r)
	if err != nil {
		fmt.Println("Error logging out user: ", err)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func main() {

	connStr := dbURLFromEnv()
	fmt.Println("CON", connStr)
	dbConnection, err := sqlx.Open("postgres", connStr)
	if err != nil {
		fmt.Println("error connecting to database using sqlx.Open:", err)
		os.Exit(1)
	}

	logger := domain.FmtLogger(true)

	sessions := sesh.NewSessions(dbConnection, logger, 5*time.Minute, false)

	server := newTestServer(sessions)

	protectedProtected := (sessions.AuthenticationMiddleware()(http.HandlerFunc(server.protected))).(http.HandlerFunc)
	protectedLogout := (sessions.AuthenticationMiddleware()(http.HandlerFunc(server.logout))).(http.HandlerFunc)

	http.HandleFunc("/", server.homepage)
	http.HandleFunc("/login", server.login)
	http.HandleFunc("/logout", protectedLogout)
	http.HandleFunc("/protected", protectedProtected)
	http.ListenAndServe(":8088", nil)

}
