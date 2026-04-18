package secrets

import "testing"

// LowerScryptWorkFactorForTests drops the age scrypt work factor from the
// production default (18, ~0.3 s per encrypt/decrypt) to a test-friendly value
// for the lifetime of t. The factor is restored with t.Cleanup so the package
// state is left untouched for other tests that ran before this one.
//
// This function is exported so test binaries in sibling packages (e.g.
// secrets_integration_test.go, agentauth_test.go that constructs an AgeStore)
// can share the speed-up. Production code must never call it — the
// minWorkFactor guard in encryptAge ensures a stray call still refuses a
// critically low factor, and this function accepts a *testing.T so a non-test
// build cannot call it.
//
// Typical usage inside a test:
//
//	func TestSomething(t *testing.T) {
//	    secrets.LowerScryptWorkFactorForTests(t, 10)
//	    // ... fast AgeStore round-trips here ...
//	}
func LowerScryptWorkFactorForTests(t *testing.T, logN int) {
	t.Helper()
	if logN < minWorkFactor {
		t.Fatalf("LowerScryptWorkFactorForTests: logN=%d below minWorkFactor=%d", logN, minWorkFactor)
	}
	prev := scryptWorkFactor
	scryptWorkFactor = logN
	t.Cleanup(func() {
		scryptWorkFactor = prev
	})
}
