package grpcserver

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error turns a *errs.RestError's HTTP status into a gRPC status.
//
// The service layer of every PrimeAge service reports failure as an HTTP status,
// because REST was the only transport it had. Rather than rewrite that layer to
// speak gRPC, the gRPC adapters translate at the edge — the same job the REST
// adapters do when they write the status onto a response.
//
// The distinction that matters to callers is NOT_FOUND: three identity methods
// treat it as an answer rather than a fault, and flattening it into INTERNAL
// would turn "no such tenant" into "the identity service is broken".
func Error(httpStatus int, message string) error {
	return status.Error(codeFor(httpStatus), message)
}

func codeFor(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	default:
		// Including 500. An unmapped status is a fault of the callee, and
		// INTERNAL is the honest reading of one.
		return codes.Internal
	}
}
