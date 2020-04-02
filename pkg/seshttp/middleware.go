package seshttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/trussworks/sesh/pkg/domain"
)

// SessionCookieName is the name of the cookie that is used to store the session
const SessionCookieName = "sesh-session-key"

// SessionMiddleware is the session handler.
type SessionMiddleware struct {
	log     domain.LogService
	session domain.SessionService
}

// NewSessionMiddleware returns a configured SessionMiddleware
func NewSessionMiddleware(log domain.LogService, session domain.SessionService) *SessionMiddleware {
	return &SessionMiddleware{
		log,
		session,
	}
}

// Middleware for verifying session
func (service SessionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		sessionCookie, cookieErr := r.Cookie(SessionCookieName)
		if cookieErr != nil {
			service.log.WarnError(domain.RequestIsMissingSessionCookie, cookieErr, domain.LogFields{})
			RespondWithStructuredError(w, domain.RequestIsMissingSessionCookie, http.StatusUnauthorized)
			return
		}

		sessionKey := sessionCookie.Value
		session, err := service.session.GetSessionIfValid(sessionKey)
		if err != nil {
			if err == domain.ErrValidSessionNotFound {
				service.log.WarnError(domain.SessionDoesNotExist, err, domain.LogFields{})
				RespondWithStructuredError(w, domain.SessionDoesNotExist, http.StatusUnauthorized)
				return
			}
			if err == domain.ErrSessionExpired {
				service.log.WarnError(domain.SessionExpired, err, domain.LogFields{})
				RespondWithStructuredError(w, domain.SessionExpired, http.StatusUnauthorized)
				return
			}
			service.log.WarnError(domain.SessionUnexpectedError, err, domain.LogFields{})
			RespondWithStructuredError(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		newContext := SetSessionInRequestContext(r, session)
		next.ServeHTTP(w, r.WithContext(newContext))
	})
}

// SessionCookieService writes session cookies to a response
type SessionCookieService struct {
	secure bool
}

// NewSessionCookieService returns a SessionCookieService
func NewSessionCookieService(secure bool) SessionCookieService {
	return SessionCookieService{
		secure,
	}
}

func sessionCookie(sessionKey string, secure bool) *http.Cookie {
	// LESSONS:
	// The domain must be "" for localhost to work
	// Safari will fuck up cookies if you have a .local hostname, chrome does fine
	// Secure must be false for http to work

	return &http.Cookie{
		Secure:   secure,
		Name:     SessionCookieName,
		Value:    sessionKey,
		HttpOnly: true,
		Path:     "/",
		// Omit MaxAge and Expires to make this a session cookie.
		// Omit domain to default to the full domain
	}
}

// AddSessionKeyToResponse adds the session cookie to a response given a valid sessionKey
func (s SessionCookieService) AddSessionKeyToResponse(w http.ResponseWriter, sessionKey string) {

	cookie := sessionCookie(sessionKey, s.secure)

	http.SetCookie(w, cookie)
}

// AddSessionKeyToRequest adds the session cookie to a request given a valid sessionKey
func (s SessionCookieService) AddSessionKeyToRequest(r *http.Request, sessionKey string) {

	cookie := sessionCookie(sessionKey, s.secure)

	r.AddCookie(cookie)
}

// DeleteSessionCookie removes the session cookie
func DeleteSessionCookie(w http.ResponseWriter) {
	fmt.Println("DELETING COOK!")
	cookie := &http.Cookie{
		Name:   SessionCookieName,
		MaxAge: -1,
	}
	http.SetCookie(w, cookie)
}

// -- Context Storage
type authContextKey string

const sessionKey authContextKey = "SESSION"

// SetSessionInRequestContext modifies the request's Context() to add the Session
func SetSessionInRequestContext(r *http.Request, session domain.Session) context.Context {
	return SetSessionInContext(r.Context(), session)
}

// SetSessionInContext modifies the given context to add the Session
func SetSessionInContext(ctx context.Context, session domain.Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// SessionFromRequestContext gets the reference to the Session stored in the request.Context()
func SessionFromRequestContext(r *http.Request) domain.Session {
	// This will panic if it is not set or if it's not a Session. That will always be a programmer
	// error so I think that it's worth the tradeoff for the simpler method signature.
	return SessionFromContext(r.Context())
}

// SessionFromContext gets the reference to the Session stored in the request.Context()
func SessionFromContext(ctx context.Context) domain.Session {
	// This will panic if it is not set or if it's not a Session. That will always be a programmer
	// error so I think that it's worth the tradeoff for the simpler method signature.
	session := ctx.Value(sessionKey).(domain.Session)
	return session
}

// RespondWithStructuredError writes an error code and a json error response
func RespondWithStructuredError(w http.ResponseWriter, errorMessage string, code int) {
	errorStruct := newStructuredErrors(newStructuredError(errorMessage))
	// It's a little ugly to not just have json write directly to the the Writer, but I don't see another way
	// to return 500 correctly in the case of an error.
	jsonString, err := json.Marshal(errorStruct)
	if err != nil {
		// Log error
		http.Error(w, "Internal Server Error: failed to encode error json", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Println("ENCODING")
	http.Error(w, string(jsonString), code)
}

type structuredError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type structuredErrors struct {
	Errors []structuredError `json:"errors"`
}

func newStructuredError(message string) structuredError {
	return structuredError{
		Message: message,
	}
}

func newStructuredErrors(errors ...structuredError) structuredErrors {
	return structuredErrors{
		Errors: errors,
	}
}
