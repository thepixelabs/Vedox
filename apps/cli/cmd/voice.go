// Package cmd — voice.go
//
// `vedox voice` parent command and its subcommands.
//
// Subcommands:
//
//	vedox voice status   — hit GET /api/voice/status on the running daemon
//	vedox voice test     — activate PTT for 3 seconds with a stub pipeline,
//	                       print the parsed intent (no daemon required)
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/daemon"
	"github.com/vedox/vedox/internal/portcheck"
	"github.com/vedox/vedox/internal/voice"
)

// voiceCmd is the parent command.  It does nothing on its own.
var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Interact with the Vedox voice pipeline",
	Long: `Interact with the Vedox voice input pipeline.

The voice pipeline is optional (--voice flag on 'vedox server start').
When enabled it listens for push-to-talk input and dispatches
recognised commands to the daemon.

Subcommands:

  vedox voice status  — show current pipeline state
  vedox voice test    — run a 3-second stub PTT test (no daemon required)`,
}

// ---------------------------------------------------------------------------
// vedox voice status
// ---------------------------------------------------------------------------

var voiceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current voice pipeline state",
	Long: `Query the running daemon for voice pipeline status.

Prints the pipeline state (idle/listening/transcribing/dispatching),
whether voice is enabled, the last recognised transcript, and the last
dispatched command.

The daemon must be running with --voice for this to return live data.
Without --voice the endpoint still responds but voice is disabled.`,
	RunE: runVoiceStatus,
}

func runVoiceStatus(_ *cobra.Command, _ []string) error {
	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return err
	}
	p := daemon.NewPaths(vedoxHome)

	rec, err := daemon.ReadPIDFile(p.PIDFile)
	if err != nil || !daemon.IsAlive(rec.PID) {
		return fmt.Errorf("vedox daemon is not running — start it with 'vedox server start'")
	}

	baseURL := fmt.Sprintf("http://%s:%d", portcheck.BindAddr, rec.Port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/voice/status")
	if err != nil {
		return fmt.Errorf("could not reach daemon at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("voice: not enabled on this daemon (start with --voice to enable)")
		return nil
	}

	var status struct {
		Enabled        bool   `json:"enabled"`
		State          string `json:"state"`
		LastTranscript string `json:"lastTranscript"`
		LastCommand    string `json:"lastCommand"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("could not decode voice status: %w", err)
	}

	if !status.Enabled {
		fmt.Println("voice: disabled")
		return nil
	}

	fmt.Printf("voice: enabled\n")
	fmt.Printf("  state:           %s\n", status.State)
	if status.LastTranscript != "" {
		fmt.Printf("  last transcript: %s\n", status.LastTranscript)
	}
	if status.LastCommand != "" {
		fmt.Printf("  last command:    %s\n", status.LastCommand)
	}
	return nil
}

// ---------------------------------------------------------------------------
// vedox voice test
// ---------------------------------------------------------------------------

var voiceTestFlags struct {
	duration int
	text     string
}

var voiceTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Run a 3-second stub PTT test and print the parsed intent",
	Long: `Run a self-contained push-to-talk test using the stub pipeline.

The stub pipeline captures silence (or a custom --text response) for the
given --duration, then transcribes with the stub transcriber and parses
the intent.  No daemon connection is required.

Use --text to supply a canned transcript (simulates what Whisper would return).
Use --duration to control how long PTT is held open.

Examples:

  vedox voice test
  vedox voice test --text "vedox document everything"
  vedox voice test --text "vedox document this folder" --duration 2`,
	RunE: runVoiceTest,
}

func runVoiceTest(_ *cobra.Command, _ []string) error {
	dur := time.Duration(voiceTestFlags.duration) * time.Second
	if dur <= 0 {
		dur = 3 * time.Second
	}

	// Build a canned-response channel if --text was provided.
	var responsesCh chan string
	if voiceTestFlags.text != "" {
		responsesCh = make(chan string, 1)
		responsesCh <- voiceTestFlags.text
	}

	trans := voice.NewStubTranscriber(responsesCh)
	src := voice.NewStubAudioSource("") // silence mode

	// DispatchFunc intercepts the dispatch and prints the result instead of
	// hitting a real daemon URL.
	var dispatchedIntent voice.Intent
	dispatchFn := func(_ context.Context, intent voice.Intent, _ string) error {
		dispatchedIntent = intent
		return nil
	}

	p, err := voice.NewPipeline(voice.PipelineConfig{
		Source:        src,
		Transcriber:   trans,
		DaemonURL:     "http://127.0.0.1:5150", // placeholder — dispatch is overridden
		MinConfidence: 0.5,
		DispatchFunc:  dispatchFn,
	})
	if err != nil {
		return fmt.Errorf("voice test: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur+5*time.Second)
	defer cancel()

	fmt.Printf("voice test: starting stub pipeline (hotkey: %s)\n", voice.DefaultHotkey)
	if err := p.Start(ctx); err != nil {
		return fmt.Errorf("voice test: start pipeline: %w", err)
	}
	defer p.Stop() //nolint:errcheck

	// Simulate press → hold → release.
	fmt.Printf("voice test: PTT active for %s...\n", dur)
	p.SetPTT(true)

	select {
	case <-time.After(dur):
		// Duration elapsed — release PTT.
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "voice test: context cancelled")
		return ctx.Err()
	}

	p.SetPTT(false)

	// Give the pipeline time to transcribe and dispatch.
	time.Sleep(500 * time.Millisecond)

	// Print results.
	if voiceTestFlags.text == "" {
		fmt.Println("voice test: no --text provided; stub returned empty transcript")
		fmt.Printf("  command:    %s\n", voice.CommandUnknown)
		fmt.Println("  confidence: 0.00")
		fmt.Println("  hint: run with --text \"vedox document everything\" to see a real result")
		return nil
	}

	if dispatchedIntent.Command == voice.CommandUnknown {
		fmt.Printf("voice test: transcript %q did not match any command\n", voiceTestFlags.text)
		fmt.Println("  hint: try phrases like \"vedox document everything\" or \"vedox document this folder\"")
		return nil
	}

	fmt.Printf("voice test: success\n")
	fmt.Printf("  transcript: %q\n", dispatchedIntent.RawText)
	fmt.Printf("  command:    %s\n", dispatchedIntent.Command)
	fmt.Printf("  confidence: %.2f\n", dispatchedIntent.Confidence)
	if dispatchedIntent.Target != "" {
		fmt.Printf("  target:     %s\n", dispatchedIntent.Target)
	}
	fmt.Println("  dispatch:   intercepted (stub mode — no daemon call made)")
	return nil
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func init() {
	voiceTestCmd.Flags().IntVar(&voiceTestFlags.duration, "duration", 3,
		"seconds to hold PTT open before auto-releasing")
	voiceTestCmd.Flags().StringVar(&voiceTestFlags.text, "text", "",
		"canned transcript the stub transcriber returns (simulates Whisper output)")

	voiceCmd.AddCommand(voiceStatusCmd)
	voiceCmd.AddCommand(voiceTestCmd)

	rootCmd.AddCommand(voiceCmd)
}
