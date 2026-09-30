package grpcserver

import "testing"

func TestPort(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	if got := Port(); got != "50051" {
		t.Errorf("Port() = %q; want the default", got)
	}
	t.Setenv("GRPC_PORT", "6000")
	if got := Port(); got != "6000" {
		t.Errorf("Port() = %q; want GRPC_PORT", got)
	}
}
