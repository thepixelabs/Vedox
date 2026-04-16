//go:build !whisper

// transcriber_stub.go — default (non-whisper) build.
// newTranscriber returns a StubTranscriber so the package compiles and runs
// without any C dependencies.
package voice

// newTranscriber is the build-tag-specific constructor called by NewTranscriber.
// This file is compiled when the "whisper" build tag is NOT present.
func newTranscriber(_ string) (Transcriber, error) {
	return NewStubTranscriber(nil), nil
}
