<p align="center">
  <img src="apps/www/static/vedox-logo-01-fossil-record-rembg.png" width="80" alt="Vedox" />
</p>

<h3 align="center"><strong>Vedox</strong></h3>

<p align="center"><em>the documentation operating system for solo devs</em></p>

<p align="center">
  <a href="https://polyformproject.org/licenses/shield/1.0.0"><img src="https://img.shields.io/badge/license-PolyForm%20Shield%201.0.0-blue?style=flat-square" alt="PolyForm Shield 1.0.0 license" /></a>
  <a href="https://github.com/thepixelabs/vedox/actions"><img src="https://img.shields.io/github/actions/workflow/status/thepixelabs/vedox/ci.yml?style=flat-square&label=CI" alt="CI" /></a>
</p>

---

your docs live in Git repos you own. Vedox indexes them in SQLite, runs a local daemon, serves a WYSIWYG editor on localhost, and installs a doc agent into Claude Code, Codex, or Gemini. no server. no account. no telemetry.

---

## what it is

Vedox has three parts that work together:

| part | what it does |
|---|---|
| `vedox server` | Go daemon — indexes your doc repos, serves the HTTP API, runs in the background via launchd or systemd |
| editor | SvelteKit WYSIWYG + raw Markdown editor at `http://127.0.0.1:5151` — doc tree, reference graph, history timeline |
| doc agent | installs into your AI provider — say "vedox document everything" and it writes to the right repo |

**doc repos are separate repos.** your project source and your docs don't share a repo. you create or register doc repos during onboarding and manage them in settings. Vedox tracks which repo is public-facing and which is private.

**Git is the source of truth.** documents are plain `.md` files. SQLite is a cache — drop it and run `vedox reindex`. history is `git log --follow` rendered as a human-readable timeline, not a diff.

**the agent knows where to send things.** public-facing docs go to the project repo. private docs go to the private repo you chose. if you have multiple private repos, the agent asks.

---

## what it isn't

- not Notion or Confluence — no database-backed blocks, no sharing URLs, no org seats
- not Obsidian — docs are organized by project and type, not by a personal PKM graph
- not a code editor — Vedox reads your code via `vedox://` cross-links; it does not replace your IDE
- not a SaaS product — nothing leaves your machine by default; version checks are opt-in

---

## install

**macOS (Homebrew):**

```sh
brew install thepixelabs/tap/vedox
```

**Linux / macOS (curl):**

```sh
curl -fsSL https://vedox.pixelabs.net/install.sh | sh
```

**Docker:**

```sh
docker run -v ~/.vedox:/root/.vedox -v ~/docs:/workspace \
  -p 5150:5150 -p 5151:5151 \
  ghcr.io/thepixelabs/vedox
```

After any install, run `vedox doctor` to confirm your Git identity, keychain access, and daemon status. See [INSTALL.md](INSTALL.md) for platform-specific notes, apt/rpm packages, and source builds.

---

## quickstart

```sh
vedox init                   # 5-step onboarding: detect projects, register doc repos, install agent
vedox server start           # start the daemon (auto-starts on login after onboarding)
open http://127.0.0.1:5151  # open the editor
```

First 10 minutes in detail: [docs/QUICKSTART.md](docs/QUICKSTART.md)

---

## why this exists

solo developers with five or more active projects share a common failure mode: documentation exists in bits — a `README.md` here, a Notion page nobody opens, a comment in the code, a doc the AI generated last month that nobody can find.

Vedox treats docs the same way Git treats code: versioned, typed, searchable, organized. the doc agent removes the activation energy of stopping work to write. the daemon means the index is always fresh. the editor makes the docs worth reading.

this is not a new category. it is the same thing developers already do with Git and Markdown, with the friction removed and an agent standing by.

---

## docs

all documentation lives in `docs/`. Vedox dogfoods itself — the `docs/` tree is authored and served by Vedox.

- [docs/QUICKSTART.md](docs/QUICKSTART.md) — first 10 minutes
- [INSTALL.md](INSTALL.md) — install by platform
- [docs/README.md](docs/README.md) — documentation index
- [docs/contributing.md](docs/contributing.md) — contributor guide
- [docs/adr/001-markdown-as-source-of-truth.md](docs/adr/001-markdown-as-source-of-truth.md) — foundational architecture decision

before writing any documentation, read [docs/WRITING_FRAMEWORK.md](docs/WRITING_FRAMEWORK.md) first.

---

## License

This project is fair-code distributed under the **PolyForm Shield 1.0.0 License**.

You may use, modify, and distribute this software for personal and internal business operations. Commercial use is permitted, provided it does not directly compete with the primary product or services offered by the repository owner.

Please refer to the [`LICENSE`](LICENSE) file for the complete terms and conditions.

---

<p align="center"><em>Built by <a href="https://pixelabs.net">Pixelabs</a></em></p>
