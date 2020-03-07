package http

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/trussworks/sesh"
	"github.com/trussworks/sesh/pkg/dbstore"
	"github.com/trussworks/sesh/pkg/session"
)

func makeAuthenticatedFormRequest(logger sesh.LogService, sessionService *session.Service, sessionKey string) *http.Response {
	sessionMiddleware := NewSessionMiddleware(logger, sessionService)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Before")
		log.Println("After")
	})

	wrappedHandler := sessionMiddleware.Middleware(testHandler)

	responseWriter := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me/save", nil)

	if sessionKey != "" {
		sessionCookie := &http.Cookie{
			Name:     SessionCookieName,
			Value:    sessionKey,
			HttpOnly: true,
		}

		req.AddCookie(sessionCookie)
	}

	// make a request to some endpoint wrapped in middleware
	wrappedHandler.ServeHTTP(responseWriter, req)

	// confirm response follows unauthorized path
	response := responseWriter.Result()
	return response
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

func getTestStore(t *testing.T) sesh.SessionStorageService {
	t.Helper()

	connStr := dbURLFromEnv()

	connection, err := sqlx.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
		return nil
	}

	return dbstore.NewDBStore(connection)
}

func TestFullSessionHTTPFlow_Unauthenticated(t *testing.T) {
	store := getTestStore(t)
	logger := sesh.FmtLogger(true)
	defer store.Close()
	sessionService := session.NewSessionService(5*time.Minute, store, logger)

	response := makeAuthenticatedFormRequest(logger, sessionService, "")

	if response.StatusCode != 401 {
		t.Fatal("Session middleware should have returned 401 unauthorized response")
	}
}

func TestFullSessionHTTPFlow_BadAuthentication(t *testing.T) {
	store := getTestStore(t)
	logger := sesh.FmtLogger(true)
	defer store.Close()
	sessionService := session.NewSessionService(5*time.Minute, store, logger)

	response := makeAuthenticatedFormRequest(logger, sessionService, "GARBAGE")

	// confirm response follows unauthorized path
	if response.StatusCode != 401 {
		t.Fatal("Session middleware should have returned 401 unauthorized response")
	}
}

type testLoginHandler struct {
	session sesh.SessionService
	cookie  SessionCookieService
}

func (h testLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Login Handler!")

	accountID := "FOO"

	sessionKey, authErr := h.session.UserDidAuthenticate(accountID)
	if authErr != nil {
		RespondWithStructuredError(w, "bad session get", http.StatusInternalServerError)
		return
	}

	h.cookie.AddSessionKeyToResponse(w, sessionKey)
}

type testAuthenticatedHandler struct{}

func (h testAuthenticatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// return a 500 if we don't have a session in the context.
	session := SessionFromRequestContext(r)

	fmt.Println("We have a valid session!", session)

}

type testLogoutHandler struct {
	session sesh.SessionService
}

func (h testLogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Logout Handler!")

	session := SessionFromRequestContext(r)

	logoutErr := h.session.UserDidLogout(session.SessionKey)
	if logoutErr != nil {
		RespondWithStructuredError(w, "Logout Failed", http.StatusInternalServerError)
		return
	}

	DeleteSessionCookie(w)
}

func TestFullSessionHTTPFlow(t *testing.T) {
	store := getTestStore(t)
	logger := sesh.FmtLogger(true)
	defer store.Close()
	sessionService := session.NewSessionService(5*time.Minute, store, logger)
	cookieService := NewSessionCookieService(false)

	loginRequestHandler := testLoginHandler{
		sessionService,
		cookieService,
	}
	responseWriter := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/basic", nil)

	loginRequestHandler.ServeHTTP(responseWriter, req)

	// confirm response succeeds
	response := responseWriter.Result()

	body, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != 200 {
		fmt.Println("GOT", response.StatusCode, string(body))
		t.Fatal("should be a valid login")
	}

	// Check that one of the cookies is the session cookie
	cookies := response.Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == SessionCookieName {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("The cookie was not set on the response")
	}

	if sessionCookie.HttpOnly != true {
		t.Fatal("The cookie was not set HTTP_ONLY = true")
	}

	// Make an authenticated request by passing that cookie back in the next request
	authenticatedHandler := testAuthenticatedHandler{}

	sessionMiddleware := NewSessionMiddleware(logger, sessionService)
	wrappedHandler := sessionMiddleware.Middleware(authenticatedHandler)

	authedW := httptest.NewRecorder()
	authedR := httptest.NewRequest("GET", "/me/save", nil)
	authedR.AddCookie(sessionCookie)

	wrappedHandler.ServeHTTP(authedW, authedR)

	authedResponse := authedW.Result()

	authedBody, readErr := ioutil.ReadAll(authedResponse.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if authedResponse.StatusCode != 200 {
		fmt.Println("GOT", authedResponse.StatusCode, string(authedBody))
		t.Fatal("should be a valid login")
	}

	// logout
	logoutHandler := testLogoutHandler{sessionService}
	wrappedLogoutHandler := sessionMiddleware.Middleware(logoutHandler)

	logoutW := httptest.NewRecorder()
	logoutR := httptest.NewRequest("POST", "/me/logout", nil)
	logoutR.AddCookie(sessionCookie)

	wrappedLogoutHandler.ServeHTTP(logoutW, logoutR)

	logoutResponse := logoutW.Result()

	logoutBody, readErr := ioutil.ReadAll(logoutResponse.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if logoutResponse.StatusCode != 200 {
		fmt.Println("GOT", logoutResponse.StatusCode, string(logoutBody))
		t.Fatal("should be a valid logout")
	}

	// Check that the session cookie has max age -1, this tells the browser to lose it.
	logoutCookies := logoutResponse.Cookies()
	for _, cookie := range logoutCookies {
		if cookie.Name == SessionCookieName {
			if cookie.MaxAge != -1 {
				t.Fatal("Should have a negative MaxAge.", cookie.MaxAge)
			}
			if cookie.Value != "" {
				t.Fatal("Deleted Cookie should have no value")
			}
			break
		}
	}

	// make another request with that cookie:

	authedAgainW := httptest.NewRecorder()
	authedAgainR := httptest.NewRequest("GET", "/me/save", nil)
	authedAgainR.AddCookie(sessionCookie)

	wrappedHandler.ServeHTTP(authedAgainW, authedAgainR)

	authedAgainResponse := authedAgainW.Result()

	authedAgainBody, readErr := ioutil.ReadAll(authedAgainResponse.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if authedAgainResponse.StatusCode != 401 {
		fmt.Println("GOT", authedAgainResponse.StatusCode, string(authedAgainBody))
		t.Fatal("should be invalid, now")
	}
}
