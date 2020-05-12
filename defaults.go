package sesh

import (
	"errors"
	"fmt"
	"net/http"
)

type defaultErrorHandler int

func (h defaultErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("BAD GO")

	err := ErrorFromContext(r.Context())
	fmt.Println("ERR", err)

	if errors.Is(err, ErrNoSession) {
		fmt.Println("UNAUTHORIZED REQUEST MADE", err)
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
	} else if errors.Is(err, ErrNotCurrentSession) {
		fmt.Println("REQUEST MADE FROM OLD SESSION", err)
		http.Error(w, "UNAUTHORIZED", http.StatusUnauthorized)
	} else {
		fmt.Println("Error Fetching User", err)
		http.Error(w, "SERVER_ERROR", http.StatusInternalServerError)
	}

}

func newDefaultErrorHandler() defaultErrorHandler {
	return 0
}
