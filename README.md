# Sesh

Sesh is a session management library written in Go. It uses a postgres table to track current sessions and their expiration, and it logs all session lifecycle events. It was created to fulfill the following requirements:

1. Sessions can be ended server-side, immediately rejecting all further requests from that session
2. Only one session can be active at a time, if you login while you have a session active the old one will be ended.
3. The browser stores the session in an HttpOnly cookie, minimizing the attack surface area for intercepting the session
4. All session lifecycle events are logged: creation, destruction, reuse, and invalid requests.

## Configuration

1. Run the migration included in ./migrations to set up the `sessions` table in your postgres db.
2. Instantiate the sesh.Sessions struct

```
    dbConnection, err := sqlx.Open("postgres", dbConnectionURL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database using sqlx.Open: %w", err)
	}

    seshLogger := AuthLogger{}
	sessions := sesh.NewSessions(dbConnection, seshLogger, 5*time.Minute, false)
```

There are several options, described below.

3. Pass the Sessions struct to anywhere that needs it. Likely your router and your login/logout handlers.

## Usage

There are 5 places in your code where you need to interact with sesh once it's configured.

1. Login
2. Middleware for protected routes:
3. Extracting the session id inside protected handlers
4. Logout

### Login

When a user successfully authenticates (whether via username/password, OAuth, client certs, etc.), create a new login and set the HttpOnly cookie like so:

```
    // With a valid accountID, we can begin a session.
    _, err := sessions.UserDidAuthenticate(w, accountID.String())
    if err != nil {
        fmt.Println("Error Creating New Session", err)
        http.Error(w, 500, http.StatusInternalServerError)
        return
    }

}
```

This will create a new session associated with that AccountID and set the sesh cookie in the response writer. AccountID can be any string.

### Middleware for protected routes

To protect a route with sesh, add the sesh middleware to it.

```
    router := mux.NewRouter()
    protectedRoutes := router.PathPrefix("/me").Subrouter()
	protectedRoutes.Handle("", fetchCurrentUserHandler)
    protectedRoutes.Handle("/dogs", fetchDogsHandler)

	protectedRoutes.Use(sessions.AuthenticationMiddleware())
```

The middleware will grab the sesh cookie from the request, check that the session with that ID is valid, and add the Session struct to the context. If any part of that fails, it will log, write an error to the response, and not call any further http handlers.

### Extracting the session id inside protected handlers

Inside your protected handlers, you can access the current Session object from the context to get the AccountID that the session belongs to.

```
func (r *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session := sesh.SessionFromContext(ctx)

    user, err := fetchUserForID(session.AccountID)
    ...
```

The session object is inserted into the context by the AuthenticationMiddleware

### Logout

When someone logs out, you can end a session thusly:

```
    logoutErr := sessions.UserDidLogout(w, r)
    if logoutErr != nil {
        fmt.Println("Error logging out user", logoutErr)
        http.Error(w, 500, http.StatusInternalServerError)
        return
    }

```

This will check the sesh cookie in the request, and end the session.

NOTE: while your login handler must _not_ be proteted by the AuthenticationMiddleware, your logout handler _must_ be protected so.

## Lineage

This project was adapted from the session management code written for [Culper](https://github.com/18F/culper).
