package errors

import "net/http"

// HTTPErrorer defines an interface for errors that include an HTTP status code.
// This interface is implemented by custom error types that map to specific HTTP responses.
type HTTPErrorer interface {
	Error() string   // Error returns the error message as a string.
	StatusCode() int // StatusCode returns the associated HTTP status code for the error.
}

// ErrBadRequest represents an error when the request is not correct (HTTP 400 Bad Request).
type ErrBadRequest struct {
	Message string // Message holds the error message.
}

// Error returns the error message for ErrBadRequest.
func (e *ErrBadRequest) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code for ErrBadRequest (400).
func (e *ErrBadRequest) StatusCode() int {
	return http.StatusBadRequest // 400
}

// ErrNotFound represents an error for when an entity is not found (HTTP 404 Not Found).
type ErrNotFound struct {
	Message string
}

// Error returns the error message for ErrNotFound.
func (e *ErrNotFound) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code for ErrNotFound (404).
func (e *ErrNotFound) StatusCode() int {
	return http.StatusNotFound // 404
}

// ErrUnauthorized represents an error for unauthorized access (HTTP 401 Unauthorized).
type ErrUnauthorized struct {
	Message string
}

// Error returns the error message for ErrUnauthorized.
func (e *ErrUnauthorized) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code for ErrUnauthorized (401).
func (e *ErrUnauthorized) StatusCode() int {
	return http.StatusUnauthorized // 401
}

// ErrInternalServer represents a server error (HTTP 500 Internal Server Error).
type ErrInternalServer struct {
	Message string
}

// Error returns the error message for ErrInternalServer.
func (e *ErrInternalServer) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code for ErrInternalServer (500).
func (e *ErrInternalServer) StatusCode() int {
	return http.StatusInternalServerError // 500
}

// NewBadRequestError creates a new ErrBadRequest with the provided message.
func NewBadRequestError(message string) error {
	return &ErrBadRequest{Message: message}
}

// HandleHTTPError returns the HTTP status code and message for an error.
// If the error implements the HTTPErrorer interface, its StatusCode and Error methods are used.
// Otherwise, it defaults to an HTTP 500 Internal Server Error.
func HandleHTTPError(err error) (int, string) {
	if httpErr, ok := err.(HTTPErrorer); ok {
		// If err is an HTTPErrorer, use the status code and message from the error itself
		return httpErr.StatusCode(), httpErr.Error()
	}
	// Default to 500 internal server error if error does not implement HTTPErrorer
	return http.StatusInternalServerError, "Internal Server Error"
}

// NewNotFoundError creates a new ErrNotFound with the provided message.
func NewNotFoundError(message string) error {
	return &ErrNotFound{Message: message}
}

// NewUnauthorizedError creates a new ErrUnauthorized with the provided message.
func NewUnauthorizedError(message string) error {
	return &ErrUnauthorized{Message: message}
}

// NewInternalServerError creates a new ErrInternalServer with the provided message.
func NewInternalServerError(message string) error {
	return &ErrInternalServer{Message: message}
}
