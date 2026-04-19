# Formula/vedox.rb
#
# Distributed as a pre-built binary — no source compilation in this formula.
# Placeholders replaced automatically by goreleaser on every release:
#   __VERSION__               → semver tag (e.g. 2.0.0)
#   __SHA256_DARWIN_UNIVERSAL__ → sha256 of the darwin-all.zip universal binary
#   __SHA256_LINUX_ARM64__    → sha256 of the linux-arm64.tar.gz
#   __SHA256_LINUX_AMD64__    → sha256 of the linux-amd64.tar.gz
#
# goreleaser brews: block writes a new commit to this file via the
# HOMEBREW_TAP_TOKEN secret in the pixelicous/vedox release environment.
# The accept-bump.yml workflow in this repo auto-merges PRs that only
# touch version / url / sha256 lines and pass `brew audit --strict`.

class Vedox < Formula
  desc "Local-first, Git-native documentation operating system for solo developers"
  homepage "https://vedox.dev"
  license :cannot_represent
  version "__VERSION__"

  # No depends_on: statically compiled Go binary, zero runtime dependencies.
  # modernc.org/sqlite (pure Go) is linked in — no system libsqlite3 required.

  on_macos do
    # goreleaser produces a universal binary (arm64 + amd64 merged via lipo).
    # A single tarball covers both Apple Silicon and Intel Mac.
    # Archive name produced by goreleaser universal_binaries + name_template:
    #   vedox-{{ .Version }}-{{ .Os }}-{{ .Arch }}
    # For universal binaries goreleaser sets .Arch to "all".
    url "https://github.com/pixelicous/vedox/releases/download/v__VERSION__/vedox-__VERSION__-darwin-all.zip"
    sha256 "__SHA256_DARWIN_UNIVERSAL__"
  end

  on_linux do
    on_arm do
      url "https://github.com/pixelicous/vedox/releases/download/v__VERSION__/vedox-__VERSION__-linux-arm64.tar.gz"
      sha256 "__SHA256_LINUX_ARM64__"
    end
    on_intel do
      url "https://github.com/pixelicous/vedox/releases/download/v__VERSION__/vedox-__VERSION__-linux-amd64.tar.gz"
      sha256 "__SHA256_LINUX_AMD64__"
    end
  end

  # livecheck: used by `brew livecheck` and `brew bump-formula-pr`.
  livecheck do
    url :stable
    strategy :github_latest
  end

  def install
    bin.install "vedox"
    # Shell completions: vedox completion <shell> writes to stdout.
    # Confirm with CLI engineering that `vedox completion bash/zsh/fish` works
    # before the first release. If the subcommand does not exist, remove this line.
    generate_completions_from_executable(bin/"vedox", "completion")
  end

  # brew services integration: wraps launchd so users can run:
  #   brew services start vedox
  # instead of manually registering a launchd plist.
  # IMPORTANT: the CLI must check for the Homebrew service plist at
  # ~/Library/LaunchAgents/homebrew.mxcl.vedox.plist and skip manual plist
  # installation if found, to avoid double-registration conflicts.
  service do
    run [opt_bin/"vedox", "server"]
    keep_alive true
    log_path var/"log/vedox.log"
    error_log_path var/"log/vedox.error.log"
    # The daemon reads its port/socket config from ~/.vedox/config.json.
    # No port is hardcoded here.
  end

  test do
    # Smoke test: version string is present in output.
    assert_match "vedox", shell_output("#{bin}/vedox --version")
    # Extended smoke test (preferred if --brief flag exists at v2.0.0 ship):
    # system "#{bin}/vedox", "doctor", "--brief"
    # doctor must not require a running daemon or ~/.vedox/ to exist.
    # Confirm with CLI engineering before enabling.
  end

  # head: for contributors who want to build from main.
  # Requires Go 1.22+ and pnpm on PATH (SvelteKit build step).
  # NOT intended for end users.
  head do
    url "https://github.com/pixelicous/vedox.git", branch: "main"
    depends_on "go" => :build
    depends_on "node" => :build
    depends_on "pnpm" => :build
  end
end
