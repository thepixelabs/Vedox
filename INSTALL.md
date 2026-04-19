# install

Vedox ships as a single binary (`vedox`) that includes both the daemon and the CLI. the editor is served as static assets bundled with the binary.

after any install, run `vedox doctor` to verify your environment. it checks Git identity, system keychain access, daemon status, and port availability.

---

## macOS — Homebrew

```sh
brew install thepixelabs/tap/vedox
```

to auto-start the daemon on login:

```sh
vedox server enable   # registers a launchd agent in ~/Library/LaunchAgents
vedox server start    # starts it now without waiting for a reboot
```

to stop and unregister:

```sh
vedox server stop
vedox server disable
```

---

## Linux — curl installer

```sh
curl -fsSL https://vedox.pixelabs.net/install.sh | sh
```

the script installs the binary to `/usr/local/bin/vedox` and detects your init system. on systemd hosts it registers a user service:

```sh
vedox server enable   # runs: systemctl --user enable vedox
vedox server start    # runs: systemctl --user start vedox
```

**apt (Debian / Ubuntu)** — coming in v2.1:

```sh
# placeholder — package not yet published
sudo apt install vedox
```

**rpm (Fedora / RHEL)** — coming in v2.1:

```sh
# placeholder — package not yet published
sudo dnf install vedox
```

---

## Docker

```sh
docker run \
  -v ~/.vedox:/root/.vedox \
  -v ~/docs:/workspace \
  -p 5150:5150 \
  -p 5151:5151 \
  ghcr.io/thepixelabs/vedox
```

- `~/.vedox` holds your global config, registered repos, and user preferences
- `/workspace` is the default doc repo path inside the container; mount each doc repo you want accessible
- port 5150 is the API; port 5151 is the editor

the Docker image does not run launchd or systemd. use your container runtime's restart policy (`--restart unless-stopped`) for persistence.

---

## source build

**prerequisites:** Go 1.23+, Node 20+, pnpm 9+, Git with `user.name` and `user.email` set.

```sh
git clone https://github.com/thepixelabs/vedox.git
cd vedox
pnpm install
cd apps/cli && make build   # → apps/cli/bin/vedox
```

add `apps/cli/bin/` to your `$PATH`, then confirm:

```sh
vedox version
vedox doctor
```

to build the editor for production and embed it in the binary:

```sh
pnpm build
cd apps/cli && make build-prod
```

---

## first-run checklist

`vedox doctor` reports on each of these. fix any failures before running `vedox server start`.

| check | what it verifies |
|---|---|
| Git identity | `git config user.name` and `user.email` are set |
| Keychain | system keychain accessible (used for HMAC secret storage) |
| Port 5150 | not in use by another process |
| Port 5151 | not in use by another process |
| `gh` CLI | optional — enables repo creation from onboarding |

if `vedox doctor` exits cleanly, run `vedox init` to start onboarding.

---

## uninstall

```sh
vedox server stop
vedox server disable
rm $(which vedox)
rm -rf ~/.vedox          # removes global config and preferences
```

doc repos you registered are not deleted — they are ordinary Git repos on disk or on GitHub.
