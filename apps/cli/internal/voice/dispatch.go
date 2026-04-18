package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpClient is the package-level client used by Dispatch.  It has a
// conservative timeout because all calls are to localhost.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// triggerRequest is the JSON body sent to POST /v1/agent/trigger.
type triggerRequest struct {
	Command string `json:"command"`
	Target  string `json:"target,omitempty"`
}

// DispatchError is returned by Dispatch when a request cannot be completed.
type DispatchError struct {
	Command    Command
	StatusCode int    // 0 when the error is not HTTP-level
	Body       string // raw response body, if any
	Wrapped    error
}

func (e *DispatchError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("voice dispatch: command %q returned HTTP %d: %s", e.Command, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("voice dispatch: command %q: %v", e.Command, e.Wrapped)
}

func (e *DispatchError) Unwrap() error { return e.Wrapped }

// Dispatch maps a parsed Intent to a Vedox daemon API call.
//
// Routing:
//
//	CommandDocumentEverything → POST <daemonURL>/v1/agent/trigger {command:"document_everything"}
//	CommandDocumentFolder     → POST <daemonURL>/v1/agent/trigger {command:"document_folder"}
//	CommandDocumentChanges    → POST <daemonURL>/v1/agent/trigger {command:"document_changes"}
//	CommandDocumentFile       → POST <daemonURL>/v1/agent/trigger {command:"document_file", target:<path>}
//	CommandStatus             → GET  <daemonURL>/healthz
//	CommandStop               → POST <daemonURL>/v1/agent/cancel
//	CommandUnknown            → error with suggestion
//
// daemonURL must not have a trailing slash (e.g. "http://127.0.0.1:4711").
//
// On success the function returns nil.  On failure it returns *DispatchError.
func Dispatch(ctx context.Context, intent Intent, daemonURL string) error {
	daemonURL = strings.TrimRight(daemonURL, "/")

	switch intent.Command {
	case CommandDocumentEverything:
		return postTrigger(ctx, daemonURL, triggerRequest{Command: "document_everything"})

	case CommandDocumentFolder:
		return postTrigger(ctx, daemonURL, triggerRequest{
			Command: "document_folder",
			Target:  intent.Target,
		})

	case CommandDocumentChanges:
		return postTrigger(ctx, daemonURL, triggerRequest{Command: "document_changes"})

	case CommandDocumentFile:
		return postTrigger(ctx, daemonURL, triggerRequest{
			Command: "document_file",
			Target:  intent.Target,
		})

	case CommandStatus:
		return getHealthz(ctx, daemonURL)

	case CommandStop:
		return postCancel(ctx, daemonURL)

	case CommandUnknown:
		return &DispatchError{
			Command: CommandUnknown,
			Wrapped: fmt.Errorf(
				"unrecognised command (transcript: %q) — try: \"vedox document everything\", "+
					"\"vedox document this folder\", \"vedox document these changes\", "+
					"\"vedox document <path>\", \"vedox status\", or \"vedox stop\"",
				intent.RawText,
			),
		}

	default:
		return &DispatchError{
			Command: intent.Command,
			Wrapped: fmt.Errorf("voice dispatch: unknown command constant %q", intent.Command),
		}
	}
}

// postTrigger sends POST /v1/agent/trigger with a JSON body.
func postTrigger(ctx context.Context, daemonURL string, body triggerRequest) error {
	endpoint := daemonURL + "/v1/agent/trigger"

	payload, err := json.Marshal(body)
	if err != nil {
		return &DispatchError{
			Command: Command(body.Command),
			Wrapped: fmt.Errorf("marshal trigger request: %w", err),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return &DispatchError{
			Command: Command(body.Command),
			Wrapped: fmt.Errorf("build trigger request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return &DispatchError{
			Command: Command(body.Command),
			Wrapped: fmt.Errorf("POST %s: %w", endpoint, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &DispatchError{
			Command:    Command(body.Command),
			StatusCode: resp.StatusCode,
			Body:       string(rb),
		}
	}
	return nil
}

// getHealthz sends GET /healthz and reports success/failure.
func getHealthz(ctx context.Context, daemonURL string) error {
	endpoint := daemonURL + "/healthz"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &DispatchError{
			Command: CommandStatus,
			Wrapped: fmt.Errorf("build healthz request: %w", err),
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return &DispatchError{
			Command: CommandStatus,
			Wrapped: fmt.Errorf("GET %s: %w", endpoint, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &DispatchError{
			Command:    CommandStatus,
			StatusCode: resp.StatusCode,
			Body:       string(rb),
		}
	}
	return nil
}

// postCancel sends POST /v1/agent/cancel.
func postCancel(ctx context.Context, daemonURL string) error {
	endpoint := daemonURL + "/v1/agent/cancel"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &DispatchError{
			Command: CommandStop,
			Wrapped: fmt.Errorf("build cancel request: %w", err),
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return &DispatchError{
			Command: CommandStop,
			Wrapped: fmt.Errorf("POST %s: %w", endpoint, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &DispatchError{
			Command:    CommandStop,
			StatusCode: resp.StatusCode,
			Body:       string(rb),
		}
	}
	return nil
}
