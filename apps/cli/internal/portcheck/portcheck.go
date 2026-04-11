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

const bindAddr = "127.0.0.1"

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
