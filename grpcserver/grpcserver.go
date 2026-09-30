// Package grpcserver builds the gRPC server every PrimeAge service serves on.
//
// It carries the service-token check, which is what actually authenticates a
// caller: the transport verifies the server to the client and not the other way
// round, so without this interceptor any process able to reach the port could
// call any method.
package grpcserver

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/primeage-health/primeageutils/certs"
)

// TokenMetadataKey carries the caller's service token. gRPC lowercases metadata
// keys, so this is spelled lowercase to match what a server actually reads.
const TokenMetadataKey = "x-service-token"

// New builds a TLS gRPC server that rejects any call not carrying serviceToken.
//
// The caller registers its service implementations on the returned server and
// then hands it to Serve.
func New(serviceToken string) (*grpc.Server, error) {
	if strings.TrimSpace(serviceToken) == "" {
		return nil, fmt.Errorf("the service token is empty; refusing to serve gRPC with an unauthenticated surface")
	}

	config, err := certs.ServerTLS()
	if err != nil {
		return nil, err
	}

	return grpc.NewServer(
		grpc.Creds(credentials.NewTLS(config)),
		grpc.UnaryInterceptor(TokenInterceptor(serviceToken)),
	), nil
}

// TokenInterceptor rejects any call whose metadata does not carry the expected
// token.
//
// The comparison is constant-time: a byte-by-byte one leaks the token's prefix
// to anything able to time the reply, and this token is enough on its own to
// call every method the service exposes.
func TokenInterceptor(expected string) grpc.UnaryServerInterceptor {
	want := []byte(expected)

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata on the request")
		}

		values := md.Get(TokenMetadataKey)
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "no service token on the request")
		}

		if subtle.ConstantTimeCompare([]byte(values[0]), want) != 1 {
			return nil, status.Error(codes.Unauthenticated, "the service token is not valid")
		}

		return handler(ctx, req)
	}
}

// Serve listens on port and blocks until the server stops.
func Serve(server *grpc.Server, port string) error {
	port = strings.TrimSpace(port)
	if port == "" {
		return fmt.Errorf("GRPC_PORT is not set")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("listening on port %s: %w", port, err)
	}

	return server.Serve(listener)
}

// Port is GRPC_PORT, or 50051 when it is unset.
func Port() string {
	if port := os.Getenv("GRPC_PORT"); port != "" {
		return port
	}
	return "50051"
}

// Run serves server on port until ctx is done, then stops it gracefully. It
// panics if the listener fails: a process serving one transport and silently
// not the other is harder to notice than one that refuses to start.
func Run(ctx context.Context, server *grpc.Server, port string) {
	log.Println("starting grpc server on port: " + port)
	go func() {
		if err := Serve(server, port); err != nil {
			log.Panicf("grpc server closed: %v", err)
		}
	}()

	<-ctx.Done()
	server.GracefulStop()
}

// DrainContext is done five seconds after SIGTERM or an interrupt. Kubernetes
// sends SIGTERM as it pulls the pod from Service endpoints, and the ingress and
// kube-proxy take a few seconds to follow, so the listeners keep accepting
// until they have.
func DrainContext() context.Context {
	signalled, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	context.AfterFunc(signalled, func() {
		time.Sleep(5 * time.Second)
		cancel()
	})
	return ctx
}
