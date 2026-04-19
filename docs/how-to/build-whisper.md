# build vedox with real whisper transcription

by default `vedox` ships with a stub transcriber that returns empty text. this
is intentional — it keeps the binary dependency-free and CI fast. when you want
real speech-to-text, rebuild with `-tags whisper`.

## prerequisites

- go 1.23+, gcc or clang, cmake 3.21+
- macOS (accelerate + coreml) or linux (openmp or pthreads)
- at least 500 mb free disk for the model

## 1 — build whisper.cpp as a static library

```sh
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp

# cmake: static lib, no shared objects
cmake -B build \
  -DBUILD_SHARED_LIBS=OFF \
  -DWHISPER_BUILD_TESTS=OFF \
  -DWHISPER_BUILD_EXAMPLES=OFF
cmake --build build --config Release -j$(nproc 2>/dev/null || sysctl -n hw.logicalcpu)
```

after the build you should have:

```
whisper.cpp/
  build/
    src/libwhisper.a
    ggml/src/libggml.a
    ggml/src/libggml-base.a
    ggml/src/libggml-cpu.a
  include/
    whisper.h
    ggml.h
```

## 2 — download a model

```sh
# inside the whisper.cpp repo
bash models/download-ggml-model.sh base.en
# downloads: models/ggml-base.en.bin  (~148 mb)
```

other sizes: `tiny.en`, `small.en`, `medium.en`, `large-v3`. larger = more
accurate and slower.

## 3 — set CGO environment variables

### macOS (apple silicon or intel)

```sh
export WHISPER_ROOT=/path/to/whisper.cpp
export CGO_CPPFLAGS="-I${WHISPER_ROOT}/include"
export CGO_LDFLAGS="-L${WHISPER_ROOT}/build/src \
                    -L${WHISPER_ROOT}/build/ggml/src \
                    -lwhisper -lggml -lggml-base -lggml-cpu \
                    -framework Accelerate -framework CoreML \
                    -framework Foundation"
```

### linux (x86-64, openmp)

```sh
export WHISPER_ROOT=/path/to/whisper.cpp
export CGO_CPPFLAGS="-I${WHISPER_ROOT}/include"
export CGO_LDFLAGS="-L${WHISPER_ROOT}/build/src \
                    -L${WHISPER_ROOT}/build/ggml/src \
                    -lwhisper -lggml -lggml-base -lggml-cpu \
                    -lstdc++ -lm -lgomp"
```

if openmp is not installed, replace `-lgomp` with `-lpthread`.

## 4 — build vedox with the whisper tag

```sh
cd /path/to/vedox/apps/cli

# via make (recommended)
WHISPER_ROOT=/path/to/whisper.cpp make build-whisper
# produces: bin/vedox-whisper

# or directly
CGO_ENABLED=1 go build -tags whisper -o bin/vedox-whisper .
```

## 5 — run with a model

```sh
bin/vedox-whisper server start --voice \
  --model /path/to/whisper.cpp/models/ggml-base.en.bin
```

the `--model` flag accepts any ggml-format whisper model file. the path is
passed to `NewTranscriber` which calls `whisper.New` from the official go
binding.

## ci and default builds

the standard ci pipeline runs `go build ./...` and `go test -race ./...`
without the `whisper` tag. this is intentional: no C library or model file is
required. the stub transcriber is used and all tests pass normally.

if you run `go mod tidy` in a non-whisper environment the binding will remain
in go.mod because go's module graph tracks build-tag-conditional imports. to
update or remove the binding run `GOFLAGS=-tags=whisper go mod tidy`.

## troubleshooting

| symptom | likely cause |
|---------|--------------|
| `ld: library not found for -lwhisper` | `WHISPER_ROOT` points at the wrong directory, or the cmake build did not complete |
| `whisper: failed to load model` | model path is wrong or the file is not a valid ggml model |
| `cgo: C compiler not found` | install xcode command-line tools (macOS) or `build-essential` (ubuntu) |
| poor transcription quality | try a larger model (`small.en` or `medium.en`) |
