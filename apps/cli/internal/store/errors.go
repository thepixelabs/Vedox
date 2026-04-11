// Package store — error helpers.
//
// All errors returned by DocStore implementations use the central VDX error
// taxonomy defined in internal/errors. This file provides thin wrappers so
// the store package does not need to repeat the import path at every call site.
package store

import (
	vedoxerrors "github.com/vedox/vedox/internal/errors"
)

// errPathTraversal returns a VDX-005 error for op and path. The path is passed
// to the constructor for logging context but is not included in the user-facing
// message (security: do not reflect attacker-controlled input).
func errPathTraversal(op, path string) *vedoxerrors.VedoxError {
	_ = op // available for debug logging by caller
	return vedoxerrors.PathTraversal(path)
}

// errSecretFile returns a VDX-006 error for op and path. The path is passed
// for logging context but not included in the user-facing message.
func errSecretFile(op, path string) *vedoxerrors.VedoxError {
	_ = op
	return vedoxerrors.SecretFileBlocked(path)
}
