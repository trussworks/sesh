# Sesh

// INSERT GO DOC

Sesh is a user session management library written in Go. It uses [scs](scs) as a session data store and provides a ProtectedMiddleware to prevent access to resources that require a login. It was created to fulfill the following specific requirements:

1. Sessions can be ended server-side, immediately rejecting all further requests from that session
2. Only one session can be active at a time, if you login while you have a session active the old one will be ended.
3. The browser stores the session in an HttpOnly cookie, minimizing the attack surface area for intercepting the session
4. All session lifecycle events are logged: creation, destruction, reuse, and invalid requests.

## Configuration

1. Create and configure an scs.SessionManager
2. Implement Sesh Interfaces
3. Pass the UserSessionManager anywhere it is needed

For a complete example of configuring sesh to protect routes, see /pkg/dummy/dummy.go

### Create and configure an scs.SessionManager

Sesh uses SCS to store session data backed by a session cookie. In order for Sesh to work you must configure SCS first. From the scs README:

```
	// Initialize a new session manager and configure the session lifetime.
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	mux := http.NewServeMux()
	mux.HandleFunc("/put", putHandler)
	mux.HandleFunc("/get", getHandler)

	// Wrap your handlers with the LoadAndSave() middleware.
	http.ListenAndServe(":4000", sessionManager.LoadAndSave(mux))
}
```

The LoadAndSave() middleware must wrap _all_ your handlers, it provides arbitrary session data storage. Sesh uses that data store to store information about authenticated user sessions.

### Implement Sesh Interfaces

Sesh limits users to a single concurrent session. If a user logs in while they have an active session, that session will be invalidated and all future requests from that session will be rejected in favor of the new session. In order to do that we need to keep track of the current session identifier for every signed in user. Rather than maintain its own table to track this information, necessitating an additional db hit on every request, sesh expects that info to be stored directly on your user table and is accessed via the following interfaces:

#### SeshUserData

SeshUserData is should be implemented by your user type.

You will pass your user into `userSessionManager.UserDidAuthenticate(user)` on login. Sesh will store `SeshUserID()` in the session so that your user can be fetched on every protected request. (using the `FetchUserByID()` delegate method described below)

Then sesh will use the `SeshCurrentSessionID()` method to determine if there is an existing session that needs to be canceled.

#### UserDelegate

UserDelegate is how sesh does two things:

1. Sets the new SessionID as the Current Session ID on every login:
   `UpdateUser(user SessionUser, currentSessionID string) error`
   UpdateUser will be called on your delegate in the middle of the `userSessionManager.UserDidAuthenticate()` call in order to save the new session id on your user.

2. Fetches the logged in user on every protected request:
   `FetchUserByID(id string) (SessionUser, error)` will be called on your delegate by the ProtectedMiddleware on every request to a protected handler. It should return your user, which should conform to SessionUser, and if the session is still valid will store that user in the context, retrievable by calling `sesh.UserFromContext(ctx)` in your protected handlers.

#### Pass the Sessions struct to anywhere that needs it. Likely your router and your login/logout handlers.

### Options

When instantiating sesh you can pass some options to configure its behavior.

1. A custom logger

Sesh logs all session lifecycle events. By default that is done by printing to stdout a message and some metadata. If you would like to hook that into your own (structured) logger, you can pass a logger that implements the `sesh.EventLogger` interface into the constructor.

```
userSessionManager, err := sesh.NewUserSessionManager(scs, delegate, CustomLogger(logger))
```

2. A custom error handler

The protected middleware responds to clients with an error for a few different reasons. By default it prints a line explaining the reason for the error and returns a go-minimalist text response describing the HTTP error code it returns. You can pass your own `http.Handler` to the constructor to be called instead. Inside your handler you can use `sesh.ErrorFromContext(ctx)` to retrieve the error that caused your handler to be called.

```
userSessionManager, err := sesh.NewUserSessionManager(scs, delegate, CustomHandler(errorHandler))
```

## Usage

There are 5 places in your code where you need to interact with sesh once it's configured.

1. Login
2. Middleware for protected routes
3. Extracting the authenticated user inside protected handlers
4. Logout

### Login

When a user successfully authenticates (whether via username/password, OAuth, client certs, etc.), call `UserDidAuthenticate` to initiate a user session. User must implement `sesh.SessionUser`.

```
    // With a valid accountID, we can begin a session.
    _, err := sessions.UserDidAuthenticate(r.Context(), user)
    if err != nil {
        fmt.Println("Error Creating New Session", err)
        http.Error(w, 500, http.StatusInternalServerError)
        return
    }

}
```

This will create a new session associated with that `user.SeshUserID()` and end any concurrent sessions.

### Middleware for protected routes

To protect a route with sesh, add the sesh middleware to it.

```
    router := mux.NewRouter()
    protectedRoutes := router.PathPrefix("/me").Subrouter()
	protectedRoutes.Handle("", fetchCurrentUserHandler)
    protectedRoutes.Handle("/dogs", fetchDogsHandler)

	protectedRoutes.Use(sessions.ProtectedMiddleware)
```

The middleware will validate that a user session exists, call your delegate to fetch the associated user, confirm that users's current session is this session, and add the user to the context. If any part of that fails, it will log, write an error to the response, and not call any further http handlers.

### Extracting the authenticated user inside protected handlers

Inside your protected handlers, you can access your logged in User from the context.

```
func (r *ProtectedActionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := sesh.UserFromContext(ctx).(ACMEUser)

    ... perform action that requires a login ...
```

The user is inserted into the context by the ProtectedMiddleware

### Logout

When someone logs out, you end their session thusly:

```
    logoutErr := sessions.UserDidLogout(r.Context())
    if logoutErr != nil {
        fmt.Println("Error logging out user", logoutErr)
        http.Error(w, 500, http.StatusInternalServerError)
        return
    }

```

This will end the user's session and call your `UpdateUser` delegate method to reset your user's current session to `""`.

NOTE: while your login handler must _not_ be protected by the ProtectedMiddleware, your logout handler _must_ be protected so.

For a complete example of integrating with sesh, check out /pkg/dummy

## Lineage

This project was adapted from the session management code written for [Culper](https://github.com/18F/culper).
