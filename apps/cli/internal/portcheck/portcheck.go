// Package portcheck tests whether a TCP port is available before the server
// attempts to bind it, producing a clean VDX-001 error instead of a raw
// "bind: address already in use" kernel message.
//
// We always bind to 127.0.0.1 per the security architecture. 0.0.0.0 binding
// is not supported in Phase 1.
package portcheck

import (
	"fmt"
	"net"
	"strings"

	vdxerr "github.com/vedox/vedox/internal/errors"
)

// BindAddr is the canonical loopback address the daemon binds to.
// This is the SOLE source of the bind string across the codebase — do not
// duplicate this value elsewhere. Per spec §6.2 binding-guard invariant.
const BindAddr = "127.0.0.1"

// bindAddr retains the unexported alias for backward compatibility within this package.
const bindAddr = BindAddr

// CheckPort tries to open a TCP listener on 127.0.0.1:<port>.
// If the port is already in use it returns a *vdxerr.VedoxError with code VDX-001.
// Any other error (e.g., permission denied on port < 1024) is returned as-is.
func CheckPort(port int) error {
	addr := fmt.Sprintf("%s:%d", bindAddr, port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Distinguish "address already in use" from other OS errors.
		// We check the error message string rather than importing syscall because
		// EADDRINUSE is not exported identically across all supported platforms.
		if isAddrInUse(err) {
			return vdxerr.PortInUse(port)
		}
		return fmt.Errorf("port check failed for %s: %w", addr, err)
	}

	// Port is available — close the test listener immediately.
	_ = ln.Close()
	return nil
}

// isAddrInUse returns true when err carries the "address already in use"
// indicator from the OS. Covers Linux/macOS (POSIX) and Windows.
func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "Only one usage of each socket")
}

// ListenAddr returns the canonical bind address string for the given port.
// All callers constructing a server listen address must use this function to
// ensure 127.0.0.1 is always used — never 0.0.0.0.
func ListenAddr(port int) string {
	return fmt.Sprintf("%s:%d", bindAddr, port)
}

// DefaultPort is the preferred port when no --port flag is provided.
const DefaultPort = 5150

// portRangeEnd is the inclusive upper bound of the automatic scan range.
const portRangeEnd = 5199

// SelectPort selects an available port per spec §11:
//
//   - If explicit > 0, try that port only. If taken, return VDX-001 (no fallback).
//   - Otherwise, start at DefaultPort (5150) and scan up to portRangeEnd (5199),
//     returning the first free port.
//   - If the entire range is occupied, return VDX-001.
func SelectPort(explicit int) (int, error) {
	if explicit > 0 {
		if err := CheckPort(explicit); err != nil {
			return 0, err
		}
		return explicit, nil
	}
	for p := DefaultPort; p <= portRangeEnd; p++ {
		if err := CheckPort(p); err == nil {
			return p, nil
		}
	}
	return 0, fmt.Errorf("[VDX-001] all ports in range %d–%d are in use; pass --port to choose another",
		DefaultPort, portRangeEnd)
}
