// Package grpcclient dials another PrimeAge service over TLS.
//
// There is deliberately no package-level singleton and no GetXClient() accessor.
// Dial returns a connection the caller owns and closes, so an adapter can be
// constructed twice, pointed somewhere else in a test, and shut down
// deterministically — none of which a shared global allows.
package grpcclient

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/primeage-health/primeageutils/certs"
	"github.com/primeage-health/primeageutils/grpcserver"
)

// Dial opens a connection to target — host:port, e.g. primeage-auth-srv:50051 —
// attaching serviceToken to every call made over it.
//
// The connection is lazy: grpc.NewClient does not block on a handshake, so a
// callee that is not up yet fails the first RPC rather than the process's boot.
// That is deliberate for a callee that may restart independently.
func Dial(target, serviceToken string) (*grpc.ClientConn, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("no target address")
	}
	if strings.TrimSpace(serviceToken) == "" {
		return nil, fmt.Errorf("no service token; the callee will reject every call")
	}

	// A leftover http:// from the REST era would be parsed as a scheme and sent
	// the resolver somewhere unhelpful. Catching it here names the real problem.
	if strings.Contains(target, "://") {
		return nil, fmt.Errorf("target %q carries a URL scheme; a gRPC target is host:port", target)
	}

	config, err := certs.ClientTLS()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithUnaryInterceptor(tokenInterceptor(serviceToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", target, err)
	}

	return conn, nil
}

// tokenInterceptor puts the service token on every outgoing call.
func tokenInterceptor(serviceToken string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, grpcserver.TokenMetadataKey, serviceToken)
		return invoke(ctx, method, req, reply, conn, opts...)
	}
}
