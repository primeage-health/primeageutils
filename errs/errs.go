// Package errs is the error envelope that crosses a layer boundary.
//
// A service method reports failure as a *RestError carrying an HTTP status. The
// REST adapters write that status onto the response; the gRPC adapters translate
// it with grpcserver.Error. Keeping the status in the service layer is what lets
// one piece of business logic answer on both transports without knowing either.
package errs

import "net/http"

// RestError is a failure and the status it should be reported with.
//
// Code carries no json tag on purpose: it is the transport's business, not the
// client's, and a response body that repeated the HTTP status would be the only
// place the two could disagree.
type RestError struct {
	Code    int    `json:"-"`
	Message string `json:"message,omitempty"`
}

// Error makes RestError an error, so a caller can wrap or return one where the
// standard interface is expected.
//
// Most existing call sites compare against nil rather than using errors.As, for
// historical reasons. Both work.
func (e *RestError) Error() string { return e.Message }

// AsMessage strips the status, leaving what is safe to send to a client.
//
// The status travels in the response's status line or the gRPC code; repeating
// it in the body would let the two drift apart.
func (e RestError) AsMessage() *RestError {
	return &RestError{Message: e.Message}
}

// NewBadRequestError reports a malformed request — 400.
func NewBadRequestError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusBadRequest}
}

// NewValidationError reports a well-formed request that cannot be acted on — 422.
func NewValidationError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusUnprocessableEntity}
}

// NewAuthenticationError reports an unproven caller — 401.
func NewAuthenticationError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusUnauthorized}
}

// NewAuthorizationError reports a proven caller who may not do this — 403.
func NewAuthorizationError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusForbidden}
}

// NewNotFoundError reports something absent — 404.
//
// On the identity surface this is an answer rather than a fault: an identifier
// nobody holds, and a person with several memberships, both resolve to it.
func NewNotFoundError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusNotFound}
}

// NewConflictError reports a collision with something that already exists — 409.
func NewConflictError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusConflict}
}

// NewTooManyRequestsError reports a caller who has spent a rate limit — 429.
//
// It is distinct from a 400 on purpose: a refused send is not a malformed
// request, and a client that retried a 400 forever would be right to. A 429 is
// what tells it to wait.
func NewTooManyRequestsError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusTooManyRequests}
}

// NewUnexpectedError reports a fault of ours — 500.
//
// The message reaches the client, so it must not name the internal cause. Log
// the detail and hand this a sentence a caregiver could read.
func NewUnexpectedError(message string) *RestError {
	return &RestError{Message: message, Code: http.StatusInternalServerError}
}
