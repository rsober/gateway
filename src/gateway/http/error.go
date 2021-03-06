package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Error is an interface that describes an error case.
type Error interface {
	Error() error
	String() string
	Body() string
	Code() int
}

type httpError struct {
	err  error
	code int
}

// Error returns the underlying error.
func (h *httpError) Error() error {
	return h.err
}

// String returns the string representation of the underlying error.
func (h *httpError) String() string {
	return h.err.Error()
}

// Body returns a ready to use HTTP body for the error response, if present.
func (h *httpError) Body() string {
	return ""
}

// Code returns the HTTP status code of the error. Defaults to 500.
func (h *httpError) Code() int {
	if h.code == 0 {
		return http.StatusInternalServerError
	}
	return h.code
}

// NewError returns a new error to use with this library.
func NewError(err error, code int) Error {
	return &httpError{err: err, code: code}
}

// NewServerError returns a new error with standard code.
func NewServerError(err error) Error {
	return NewError(err, 0)
}

// DefaultServerError returns a standard error message with standard code.
func DefaultServerError() Error {
	return NewError(errors.New("Server error"), 0)
}

// ErrorReturningHandler is an http.Handler that can return an error
type ErrorReturningHandler func(w http.ResponseWriter, r *http.Request) Error

// ErrorCatchingHandler catches an error a handler throws and responds with it.
func ErrorCatchingHandler(handler ErrorReturningHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			body := err.Body()
			if body == "" {
				body = fmt.Sprintf("%s\n", err.String())
			}
			http.Error(w, body, err.Code())
		}
	})
}

// JSONErrorCatchingHandler catches an error a handler throws and responds with
// it in JSON format.
func JSONErrorCatchingHandler(handler ErrorReturningHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpErr := handler(w, r); httpErr != nil {
			body := httpErr.Body()
			if body == "" {
				data, err := json.MarshalIndent(struct {
					Error string `json:"error"`
				}{httpErr.String()}, "", "    ")
				if err == nil {
					body = string(data)
				} else {
					// Fall back to non-JSON body
					body = fmt.Sprintf("%s\n", httpErr.String())
				}
			}
			http.Error(w, body, httpErr.Code())
		}
	})
}
