package portcheck_test

// Tests for SelectPort — the auto-scan port selection path used at daemon
// startup when no explicit --port is given. Previously 0% covered; regressions
// here were startup-blocking.

import (
	"errors"
	"net"
	"testing"

	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/vedox/vedox/internal/portcheck"
)

// TestSelectPort_Explicit_Free returns the explicit port unchanged when it is
// free. This is the "--port 5150 and 5150 is free" happy path.
func TestSelectPort_Explicit_Free(t *testing.T) {
	port := findFreePort(t)

	got, err := portcheck.SelectPort(port)
	if err != nil {
		t.Fatalf("SelectPort(%d) unexpected err: %v", port, err)
	}
	if got != port {
		t.Fatalf("SelectPort(%d) = %d, want %d (explicit request)", port, got, port)
	}
}

// TestSelectPort_Explicit_InUse_NoFallback ensures that when an explicit port
// is taken, SelectPort returns VDX-001 rather than silently falling back to
// the scan range. Spec §11 requires explicit=taken => error.
func TestSelectPort_Explicit_InUse_NoFallback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	got, err := portcheck.SelectPort(port)
	if err == nil {
		t.Fatalf("SelectPort(%d) = %d, want VDX-001 error (port is in use)", port, got)
	}

	var vdxErr *vdxerr.VedoxError
	if !errors.As(err, &vdxErr) {
		t.Fatalf("err type = %T, want *vdxerr.VedoxError: %v", err, err)
	}
	if vdxErr.Code != vdxerr.ErrPortInUse {
		t.Errorf("err code = %q, want %q", vdxErr.Code, vdxerr.ErrPortInUse)
	}
	if got != 0 {
		t.Errorf("SelectPort returned port %d with error; want 0", got)
	}
}

// TestSelectPort_AutoScan_ReturnsDefaultPortWhenFree exercises the scan
// branch: explicit=0 means scan DefaultPort..portRangeEnd. On a typical
// dev machine 5150 is free. We assert the returned port falls inside the
// documented scan range (not identical to DefaultPort because CI hosts may
// have 5150 occupied).
func TestSelectPort_AutoScan_ReturnsInRange(t *testing.T) {
	got, err := portcheck.SelectPort(0)
	if err != nil {
		t.Fatalf("SelectPort(0) unexpected err: %v (all ports 5150-5199 in use?)", err)
	}
	if got < 5150 || got > 5199 {
		t.Fatalf("SelectPort(0) = %d, want port in 5150..5199", got)
	}

	// Sanity: the returned port must actually be free now (we can bind to it).
	ln, err := net.Listen("tcp", portcheck.ListenAddr(got))
	if err != nil {
		t.Fatalf("reported free port %d is not actually free: %v", got, err)
	}
	_ = ln.Close()
}

// TestSelectPort_AutoScan_SkipsOccupiedPort confirms the scan loop is not a
// single-try lookup: when 5150 is held open, SelectPort still returns a port
// inside the range (5151..5199).
func TestSelectPort_AutoScan_SkipsOccupiedPort(t *testing.T) {
	// Try to pin 5150 so the scan must move past it. If 5150 is already in
	// use, skip — the test still makes sense (the scan is still exercised)
	// but we cannot deterministically show "skipped 5150" without owning it.
	ln, err := net.Listen("tcp", "127.0.0.1:5150")
	if err != nil {
		t.Skipf("cannot pin 5150 for test (already bound): %v", err)
	}
	defer ln.Close()

	got, err := portcheck.SelectPort(0)
	if err != nil {
		t.Fatalf("SelectPort(0) unexpected err while 5150 is held: %v", err)
	}
	if got == 5150 {
		t.Fatalf("SelectPort(0) = 5150 but that port is held; scan did not advance")
	}
	if got < 5151 || got > 5199 {
		t.Fatalf("SelectPort(0) = %d, want port in 5151..5199", got)
	}
}
