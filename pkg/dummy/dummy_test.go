package dummy

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// returns username
func newTestUserName(t *testing.T, db *sqlx.DB) string {

	randID, err := rand.Int(rand.Reader, big.NewInt(100000))
	if err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	username := "dummy" + randID.String()

	createQuery := `INSERT INTO users VALUES ($1, $2)`

	_, err = db.Exec(createQuery, id, username)
	if err != nil {
		t.Fatal(err)
	}

	return username
}

func TestFlow(t *testing.T) {

	connStr := dbURLFromEnv()
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}

	testUsername := newTestUserName(t, db)

	testServer := httptest.NewServer(setupMux(db))
	defer testServer.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	// We shouldn't be able to hit the protected URL while logged in.
	blockedR, err := client.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if blockedR.StatusCode != http.StatusUnauthorized {
		t.Fatal("first request should have failed!")
	}

	// Login
	loginResp, err := client.Post(testServer.URL+"/login", "http/txt", strings.NewReader(testUsername))
	if err != nil {
		t.Fatal(err)
	}

	if loginResp.StatusCode != 201 {
		t.Fatal("LoginFailed")
	}

	// Make the protected request again
	allowedR, err := client.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR.StatusCode != 200 {
		t.Fatal("second request should have succeeded!")
	}

	// logout
	logoutResp, err := client.Post(testServer.URL+"/logout", "http/txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	if logoutResp.StatusCode != 204 {
		t.Fatal("Logout Failed.", logoutResp.StatusCode)
	}

	// Make the protected request a third time, it again should be rejected.
	blockedAgainResp, err := client.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if blockedAgainResp.StatusCode != http.StatusUnauthorized {
		t.Fatal("Final request should have failed!")
	}

}

func TestConcurrentLogin(t *testing.T) {

	connStr := dbURLFromEnv()
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}

	testUsername := newTestUserName(t, db)

	testServer := httptest.NewServer(setupMux(db))
	defer testServer.Close()

	firstJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	firstClient := &http.Client{Jar: firstJar}

	secondJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	secondClient := &http.Client{Jar: secondJar}

	// First Login
	loginResp, err := firstClient.Post(testServer.URL+"/login", "http/txt", strings.NewReader(testUsername))
	if err != nil {
		t.Fatal(err)
	}

	if loginResp.StatusCode != 201 {
		t.Fatal("LoginFailed")
	}

	// Make the protected request
	allowedR, err := firstClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR.StatusCode != 200 {
		t.Fatal("First request should have succeeded!")
	}

	// Second Login
	loginResp2, err := secondClient.Post(testServer.URL+"/login", "http/txt", strings.NewReader(testUsername))
	if err != nil {
		t.Fatal(err)
	}

	if loginResp2.StatusCode != 201 {
		t.Fatal("LoginFailed")
	}

	// Make the protected request with the second session
	allowedR2, err := secondClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR2.StatusCode != 200 {
		t.Fatal("Second request should have succeeded!")
	}

	// The first session should no longer be valid.
	disallowedR, err := firstClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if disallowedR.StatusCode != 401 {
		t.Log("The first session should no longer be valid!")
		t.Fail()
	}

	// Make the protected request with the second session, again. Should still be valid.
	allowedR3, err := secondClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR3.StatusCode != 200 {
		t.Fatal("Second request should have succeeded!")
	}

}

type silentFailDeleteStore struct {
	scs.Store
}

// Delete silently fails so that we can make a request that still has a valid session with
// the old sessionID.
func (s silentFailDeleteStore) Delete(token string) (err error) {
	return nil
}

func TestConcurrentLoginWithFailedDelete(t *testing.T) {

	connStr := dbURLFromEnv()
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err)
	}

	testUsername := newTestUserName(t, db)

	failStore := silentFailDeleteStore{memstore.New()}

	testServer := httptest.NewServer(setupMuxWithStore(db, failStore))
	defer testServer.Close()

	firstJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	firstClient := &http.Client{Jar: firstJar}

	secondJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	secondClient := &http.Client{Jar: secondJar}

	// First Login
	loginResp, err := firstClient.Post(testServer.URL+"/login", "http/txt", strings.NewReader(testUsername))
	if err != nil {
		t.Fatal(err)
	}

	if loginResp.StatusCode != 201 {
		t.Fatal("LoginFailed")
	}

	// Make the protected request
	allowedR, err := firstClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR.StatusCode != 200 {
		t.Fatal("First request should have succeeded!")
	}

	// Second Login
	loginResp2, err := secondClient.Post(testServer.URL+"/login", "http/txt", strings.NewReader(testUsername))
	if err != nil {
		t.Fatal(err)
	}

	if loginResp2.StatusCode != 201 {
		t.Fatal("LoginFailed")
	}

	// Make the protected request with the second session
	allowedR2, err := secondClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR2.StatusCode != 200 {
		t.Fatal("Second request should have succeeded!")
	}

	// The first session should no longer be valid, even though we didn't delete the session
	disallowedR, err := firstClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if disallowedR.StatusCode != 401 {
		t.Log("The first session should no longer be valid!")
		t.Fail()
	}

	// Make the protected request with the second session, again. Should still be valid.
	allowedR3, err := secondClient.Get(testServer.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}

	if allowedR3.StatusCode != 200 {
		t.Fatal("Second request should have succeeded!")
	}

}
