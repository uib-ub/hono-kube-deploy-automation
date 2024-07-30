package errors

import "net/http"

type HTTPErrorer interface {
	Error() string
	StatusCode() int
}

// ErrBadRequest represents an error when the request is not correct
type ErrBadRequest struct {
	Message string
}

func (e *ErrBadRequest) Error() string {
	return e.Message
}

func (e *ErrBadRequest) StatusCode() int {
	return http.StatusBadRequest // 400
}

// ErrNotFound represents an error for when an entity is not found.
type ErrNotFound struct {
	Message string
}

func (e *ErrNotFound) Error() string {
	return e.Message
}

func (e *ErrNotFound) StatusCode() int {
	return http.StatusNotFound // 404
}

// ErrUnauthorized represents an error for unauthorized access.
type ErrUnauthorized struct {
	Message string
}

func (e *ErrUnauthorized) Error() string {
	return e.Message
}

func (e *ErrUnauthorized) StatusCode() int {
	return http.StatusUnauthorized // 401
}

// ErrInternalServer represents a server error.
type ErrInternalServer struct {
	Message string
}

func (e *ErrInternalServer) Error() string {
	return e.Message
}

func (e *ErrInternalServer) StatusCode() int {
	return http.StatusInternalServerError // 500
}

// NewBadRequest creates a new BadRequest error.
func NewBadRequestError(message string) error {
	return &ErrBadRequest{Message: message}
}

// HandleHTTPError returns the HTTP status code and message for an error.
func HandleHTTPError(err error) (int, string) {
	if httpErr, ok := err.(HTTPErrorer); ok {
		// If err is an HTTPErrorer, use the status code and message from the error itself
		return httpErr.StatusCode(), httpErr.Error()
	}
	// Default to 500 internal server error if error does not implement HTTPErrorer
	return http.StatusInternalServerError, "Internal Server Error"
}

// NewNotFound creates a new NotFound error.
func NewNotFoundError(message string) error {
	return &ErrNotFound{Message: message}
}

// NewUnauthorized creates a new Unauthorized error.
func NewUnauthorizedError(message string) error {
	return &ErrUnauthorized{Message: message}
}

// NewInternalServer creates a new InternalServer error.
func NewInternalServerError(message string) error {
	return &ErrInternalServer{Message: message}
}
