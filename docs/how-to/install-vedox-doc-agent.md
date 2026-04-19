# install the vedox doc agent

install the vedox doc agent into your ai coding tool so the trigger phrase
"vedox document everything" routes documentation to your registered repos.

**prerequisites:** `vedox server start` must be running before you install.
the installer probes the daemon to provision an hmac auth key.

**verification (all providers):**

```sh
vedox agent list
```

output lists every provider where the agent is installed, the version, the
key id, and the install timestamp.

---

## claude code

```sh
vedox agent install --provider claude
```

**what it writes:**

| file | what changes |
|---|---|
| `~/.claude/agents/vedox-doc.md` | new subagent file — yaml frontmatter + instruction body with `{{HMAC_KEY_ID}}` |
| `~/.claude/CLAUDE.md` | appends a fenced `<!-- vedox-agent:start -->` block if absent |
| `~/.vedox/install-receipts/claude.json` | install receipt (key id, version, file hashes) |

the hmac secret never touches disk — it lives in the os keychain under the key
id written into the subagent file.

**verify:**

```sh
vedox agent list
# PROVIDER   VERSION  KEY ID    INSTALLED AT
# claude     2.0      <uuid>    2026-04-17 ...
```

in claude code, say "vedox document everything" to confirm the agent responds.

**if already installed:**

```sh
vedox agent repair --provider claude
```

---

## openai codex

```sh
vedox agent install --provider codex
```

**what it writes:**

| file | what changes |
|---|---|
| `~/.codex/config.toml` | adds `[mcp_servers.vedox]` entry with daemon url and key id |
| `~/.codex/AGENTS.md` | appends a fenced `<!-- vedox-agent:start -->` block |
| `~/.vedox/install-receipts/codex.json` | install receipt |

if `~/.codex/config.toml` does not exist, the installer creates it. the
fallback path `~/.config/codex/config.toml` is tried first on systems that
follow xdg conventions.

**verify:**

```sh
vedox agent list
# PROVIDER   VERSION  KEY ID    INSTALLED AT
# codex      2.0      <uuid>    2026-04-17 ...
```

**if already installed:**

```sh
vedox agent repair --provider codex
```

---

## github copilot

```sh
vedox agent install --provider copilot
```

copilot does not yet have an mcp tool surface, so the agent runs in **degraded
mode**: routing rules are installed as prose that copilot can read and follow,
but copilot cannot make hmac-signed requests to the daemon. tool-call support
will be enabled automatically when copilot adds mcp.

**what it writes:**

| file | what changes |
|---|---|
| `<project-root>/.github/copilot-instructions.md` | appends a `## Vedox Documentation Agent` section inside `<!-- vedox-copilot:start -->` markers |
| `~/.vedox/install-receipts/copilot.json` | install receipt (carries `version: 2.0+degraded`) |

run the command from inside the project where you want copilot guidance to
apply. the `.github/copilot-instructions.md` file is project-scoped.

**verify:**

```sh
vedox agent list
# PROVIDER   VERSION        KEY ID    INSTALLED AT
# copilot    2.0+degraded   <uuid>    2026-04-17 ...
```

**if already installed:**

```sh
vedox agent repair --provider copilot
```

---

## google gemini

```sh
vedox agent install --provider gemini
```

**what it writes:**

| file | what changes |
|---|---|
| `~/.gemini/extensions/vedox/vedox-agent.json` | extension manifest (name, version, commands, daemon url, key id, instruction body) |
| `~/.gemini/config.yaml` | appends a fenced block registering the `vedox` extension in the extensions list |
| `~/.vedox/install-receipts/gemini.json` | install receipt |

the extension path (`~/.gemini/extensions/`) matches the gemini cli extension
spec. if your gemini cli version uses a different directory, run
`vedox agent repair --provider gemini` after upgrading to re-apply.

**verify:**

```sh
vedox agent list
# PROVIDER   VERSION  KEY ID    INSTALLED AT
# gemini     2.0      <uuid>    2026-04-17 ...
```

**if already installed:**

```sh
vedox agent repair --provider gemini
```

---

## uninstall

```sh
vedox agent uninstall --provider <name>
```

removes the vedox-managed blocks from config files and deletes the install
receipt. files outside the vedox-managed fences are not touched.
