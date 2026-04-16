# System Architecture

## Authentication Flow

The Vedox daemon uses HMAC-SHA256 for agent authentication. The flow is:

1. Agent generates a timestamp (Unix seconds, UTC).
2. Agent computes HMAC-SHA256 over the request body using the shared secret.
3. Agent sends X-Vedox-Key-Id, X-Vedox-Signature, X-Vedox-Timestamp headers.
4. Daemon validates the signature within a 5-minute clock skew window.

## Key Storage

Secrets are stored in the OS keychain:
- macOS: Keychain Services (Secure Enclave on Apple Silicon)
- Linux: libsecret / Secret Service API
- Windows: Windows Credential Manager

No secrets are written to disk or included in log output.

## Token Scopes

GitHub fine-grained tokens require `contents:write` on the target repo only.
No admin, org, or workflow permissions are granted.
