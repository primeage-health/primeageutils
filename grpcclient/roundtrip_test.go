package grpcclient_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	notificationpb "github.com/primeage-health/primeageutils/genproto/notification"
	"github.com/primeage-health/primeageutils/grpcclient"
	"github.com/primeage-health/primeageutils/grpcserver"
)

const serviceToken = "token-for-tests"

// stubSender is a registered service, so the round trip exercises a real method
// dispatch rather than only the handshake.
type stubSender struct {
	notificationpb.UnimplementedNotificationServiceServer
}

func (stubSender) SendSMS(_ context.Context, req *notificationpb.SendSMSRequest) (*notificationpb.SendSMSResponse, error) {
	return &notificationpb.SendSMSResponse{Status: "sent", CorrelationId: req.GetTo()}, nil
}

// serveTLS mints a CA and a server certificate into a temp CERT_DIR, then serves
// on an ephemeral loopback port. The certificate's SAN covers what the test
// dials — the same rule the cluster certificates follow, where the SAN is the
// Kubernetes Service name.
//
// It returns a 127.0.0.1 target rather than a "localhost" one deliberately.
// localhost resolves to both ::1 and 127.0.0.1, the listener binds only the
// latter, and a client handed the v6 address first spends the whole RPC deadline
// failing to reach it. That is an artefact of dual-stack loopback, not something
// the cluster reproduces — there a Service name resolves to one ClusterIP.
func serveTLS(t *testing.T, token string) string {
	t.Helper()

	dir := t.TempDir()
	writeCerts(t, dir)
	t.Setenv("CERT_DIR", dir)

	server, err := grpcserver.New(token)
	if err != nil {
		t.Fatalf("grpcserver.New() error = %v", err)
	}
	notificationpb.RegisterNotificationServiceServer(server, stubSender{})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	return "127.0.0.1:" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func writeCerts(t *testing.T, dir string) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "PrimeAge Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("server key: %v", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("server cert: %v", err)
	}

	write := func(name string, block *pem.Block) {
		if err := os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("ca-cert.pem", &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	write("server-cert.pem", &pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	write("server-key.pem", &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
}

func send(t *testing.T, conn *grpc.ClientConn) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := notificationpb.NewNotificationServiceClient(conn).SendSMS(ctx, &notificationpb.SendSMSRequest{
		To:           "+919812345678",
		TemplateName: "login_otp",
	})
	return err
}

// The whole point of the transport: a call carrying the right token over TLS
// reaches the handler.
func TestARoundTripSucceedsWithTheRightToken(t *testing.T) {
	target := serveTLS(t, serviceToken)

	conn, err := grpcclient.Dial(target, serviceToken)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := send(t, conn); err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
}

// The server does not verify client certificates, so this token is the only
// thing standing between the port and every method on it. If this ever passes
// with the wrong value, the surface is unauthenticated.
func TestTheWrongTokenIsRejected(t *testing.T) {
	target := serveTLS(t, serviceToken)

	conn, err := grpcclient.Dial(target, "not-the-token")
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	err = send(t, conn)
	if err == nil {
		t.Fatal("SendSMS() error = nil for a wrong token; the surface is unauthenticated")
	}
	if got := grpcstatus.Code(err); got != codes.Unauthenticated {
		t.Errorf("code = %v; want Unauthenticated", got)
	}
}

// A client that does not trust the CA must not connect. This is what stops
// anything on the pod network from impersonating a service to its caller.
func TestAnUntrustedServerIsRefused(t *testing.T) {
	target := serveTLS(t, serviceToken)

	// Point the client at a different CA than the one that signed the server.
	other := t.TempDir()
	writeCerts(t, other)
	t.Setenv("CERT_DIR", other)

	conn, err := grpcclient.Dial(target, serviceToken)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := send(t, conn); err == nil {
		t.Fatal("SendSMS() error = nil against a server signed by an unknown CA")
	}
}

// A leftover REST value is the mistake most likely to survive the migration, and
// it must not reach the resolver as a host name.
func TestAURLIsRefusedAsATarget(t *testing.T) {
	if _, err := grpcclient.Dial("http://notification-srv:3000", serviceToken); err == nil {
		t.Fatal("Dial() error = nil for a URL; a gRPC target is host:port")
	}
}
