package sesh

import (
	"errors"
	"fmt"
	"net/http"
)

// defaultErrorHandler is the error handler used if no optional one is provided
type defaultErrorHandler int

func (h defaultErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := ErrorFromContext(r.Context())

	if errors.Is(err, ErrNoSession) {
		fmt.Println("Unauthorized Request Made.")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	} else if errors.Is(err, ErrNotCurrentSession) {
		fmt.Println("Request made from expired session.")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	} else {
		fmt.Println("Error Fetching User: ", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

}

func newDefaultErrorHandler() defaultErrorHandler {
	return 0
}

// defaultLogger is the logger that is used if no optional one is provided.
type defaultLogger int

func (l defaultLogger) LogSeshEvent(message string, metadata map[string]string) {
	var metaString string
	for key, value := range metadata {
		metaString = metaString + fmt.Sprintf(" %s=%s", key, value)
	}

	fmt.Printf("SeshEvent: %s, %s\n", message, metaString)
}

func newDefaultLogger() defaultLogger {
	return 0
}
