// Vedox CLI — local-first, Git-native documentation daemon.
//
// NETWORK POLICY: This binary makes ZERO outbound network calls by design.
// No telemetry, no version-check pings, no analytics, no DNS lookups outside
// of serving localhost. Any change that adds outbound HTTP must be explicitly
// reviewed and requires a user opt-in gate. Do not add http.Get, http.Post,
// or equivalent calls anywhere in this binary without a corresponding config
// flag and this comment updated.
package main

import (
	"os"

	"github.com/vedox/vedox/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
