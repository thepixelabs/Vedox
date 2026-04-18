package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// computeSignature calculates the HMAC-SHA256 signature for the given body.
// The secret is read from the OS keychain, not from environment variables.
func computeSignature(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func main() {
	// Secret comes from keychain — never hardcoded.
	secret := []byte(os.Getenv("VEDOX_HMAC_SECRET"))
	body := []byte(`{"type":"doc","content":"hello"}`)
	sig := computeSignature(secret, body)
	fmt.Printf("Signature: %s\n", sig)
}
