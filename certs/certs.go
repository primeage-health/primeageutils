// Package certs loads the TLS material every PrimeAge gRPC hop uses.
//
// The shape is server-side TLS: a client verifies the server against the CA, and
// the server does not verify the client. Caller identity is established by the
// service token in request metadata instead — see grpcserver.TokenInterceptor.
// TLS here provides the confidentiality that makes sending that token safe, not
// the authentication itself.
package certs

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDir is where the deployment mounts the certificate Secret.
const DefaultDir = "/app/cert"

// File names inside the directory, as cert-gen writes them.
const (
	caCertFile     = "ca-cert.pem"
	serverCertFile = "server-cert.pem"
	serverKeyFile  = "server-key.pem"
)

// Dir is the directory holding the certificate material.
//
// CERT_DIR exists so the services can be run outside a container against a
// locally generated directory; in the cluster nothing sets it and the mount path
// is used.
func Dir() string {
	if dir := strings.TrimSpace(os.Getenv("CERT_DIR")); dir != "" {
		return dir
	}
	return DefaultDir
}

// ClientTLS trusts the PrimeAge CA and nothing else.
//
// Deliberately not the system pool: these are internal names signed by our own
// CA, and a config that also accepted public roots would let any public CA
// impersonate a service to its caller.
func ClientTLS() (*tls.Config, error) {
	path := filepath.Join(Dir(), caCertFile)

	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the CA certificate at %s: %w", path, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no PEM certificate", path)
	}

	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}

// ServerTLS presents this service's certificate and asks for none in return.
func ServerTLS() (*tls.Config, error) {
	dir := Dir()
	certPath := filepath.Join(dir, serverCertFile)
	keyPath := filepath.Join(dir, serverKeyFile)

	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("loading the server key pair from %s: %w", dir, err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		// The caller proves who it is with its service token, not a certificate.
		ClientAuth: tls.NoClientCert,
		MinVersion: tls.VersionTLS12,
	}, nil
}
