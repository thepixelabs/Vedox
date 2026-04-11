package portcheck_test

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/vedox/vedox/internal/portcheck"
	vdxerr "github.com/vedox/vedox/internal/errors"
)

// findFreePort asks the OS for a free port by binding to :0 and reading
// the assigned port. We then close the listener so CheckPort can bind it.
func findFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("findFreePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestCheckPort_Available(t *testing.T) {
	port := findFreePort(t)
	if err := portcheck.CheckPort(port); err != nil {
		t.Errorf("CheckPort(%d) = %v, want nil (port should be free)", port, err)
	}
}

func TestCheckPort_InUse_ReturnsVDX001(t *testing.T) {
	// Hold the port open so CheckPort finds it in use.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	err = portcheck.CheckPort(port)
	if err == nil {
		t.Fatalf("CheckPort(%d) = nil, want VDX-001 error", port)
	}

	var vdxErr *vdxerr.VedoxError
	if !errors.As(err, &vdxErr) {
		t.Fatalf("expected *vdxerr.VedoxError, got %T: %v", err, err)
	}
	if vdxErr.Code != vdxerr.ErrPortInUse {
		t.Errorf("Code = %q, want %q", vdxErr.Code, vdxerr.ErrPortInUse)
	}
}

func TestListenAddr_Format(t *testing.T) {
	addr := portcheck.ListenAddr(3001)
	if addr != "127.0.0.1:3001" {
		t.Errorf("ListenAddr(3001) = %q, want %q", addr, "127.0.0.1:3001")
	}
}

func TestListenAddr_NeverBindsToAllInterfaces(t *testing.T) {
	// Confirm that ListenAddr never returns 0.0.0.0 (per security policy).
	for _, port := range []int{80, 443, 3001, 8080} {
		addr := portcheck.ListenAddr(port)
		expected := fmt.Sprintf("127.0.0.1:%d", port)
		if addr != expected {
			t.Errorf("ListenAddr(%d) = %q, want %q (must bind to loopback only)", port, addr, expected)
		}
	}
}
