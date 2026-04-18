package testutil

// TestAWSKeyIDFormat shows what an AWS key ID looks like WITHOUT containing
// a real key. The format is AKIA + 16 uppercase alphanumeric chars.
// This comment alone does not trigger the scanner because the pattern
// requires the actual characters: AKIA<16chars> as a token.

// RedactedExampleKeyID is used in test assertions to verify redaction.
// It is intentionally NOT a real key ID — the character after AKIA is
// lowercase which is not valid for real AWS key IDs.
const RedactedExampleKeyID = "AKIAlowercase1234567"

// TestDBURL is a database connection string (not a secret by scanner rules).
const TestDBURL = "postgres://localhost:5432/testdb?sslmode=disable"

// TestHMACKeyLength verifies that our HMAC key generation produces the right length.
const TestHMACKeyLength = 32 // bytes → 64 hex chars
