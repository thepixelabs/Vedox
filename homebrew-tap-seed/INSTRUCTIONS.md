# Bootstrap instructions: pixelicous/homebrew-vedox

Run these steps once, before the first Vedox release. Steps 1-3 are
one-time setup. Step 4 is the ongoing verification after any release.

---

## Step 1 — create the GitHub repo

Create a public GitHub repo named exactly `homebrew-vedox` under the
`pixelicous` organization. The `homebrew-` prefix is required — Homebrew
discovers taps by this naming convention.

```sh
gh repo create pixelicous/homebrew-vedox \
  --public \
  --description "Homebrew tap for Vedox — local-first, Git-native documentation operating system" \
  --clone
cd homebrew-vedox
```

If you do not have the GitHub CLI installed: go to
https://github.com/new, owner = pixelicous, name = homebrew-vedox,
visibility = Public.

---

## Step 2 — copy the seed files and push

From the pixelicous/vedox repo root, copy the seed directory contents
into the freshly cloned homebrew-vedox repo:

```sh
# Assuming you cloned homebrew-vedox alongside vedox:
cp -r vedox/homebrew-tap-seed/. homebrew-vedox/

cd homebrew-vedox
git add .
git commit -m "feat: bootstrap homebrew tap for vedox"
git push origin main
```

The repo will contain:
  Formula/vedox.rb             — formula with __VERSION__ placeholders
  Casks/vedox.rb               — deferred cask placeholder
  .github/workflows/test.yml   — brew test-bot CI on PRs
  .github/workflows/accept-bump.yml — auto-merge for goreleaser bumps
  README.md                    — tap install instructions

---

## Step 3 — add HOMEBREW_TAP_TOKEN to the vedox repo

goreleaser needs write access to homebrew-vedox to open version-bump PRs.

### Option A: fine-grained PAT (short-term, simpler)

1. Go to https://github.com/settings/tokens?type=beta
2. Click "Generate new token"
3. Token name: vedox-homebrew-tap
4. Expiration: 90 days (set a calendar reminder to rotate)
5. Repository access: Only selected repositories → pixelicous/homebrew-vedox
6. Permissions:
     Contents: Read and write
     Pull requests: Read and write
7. Click "Generate token" and copy the value

Add it to the vedox repo's release environment:

```sh
gh secret set HOMEBREW_TAP_TOKEN \
  --repo pixelicous/vedox \
  --env release \
  --body "ghp_PASTE_TOKEN_HERE"
```

Or via the web UI:
  https://github.com/pixelicous/vedox/settings/environments → release → Secrets

### Option B: GitHub App (recommended for long-term)

Recommended once you have more than one distribution target. A GitHub App
installation token is scoped to homebrew-vedox only, expires in 1 hour
per run, and survives maintainer turnover.

1. Go to https://github.com/organizations/pixelicous/settings/apps/new
   (or https://github.com/settings/apps/new for personal account)
2. App name: vedox-release-bot
3. Permissions (Repository permissions):
     Contents: Read and write
     Pull requests: Read and write
4. Install the app on pixelicous/homebrew-vedox only (not the whole org)
5. Generate a private key and base64-encode it:
     base64 -i vedox-release-bot.private-key.pem | tr -d '\n'
6. Add two secrets to pixelicous/vedox release environment:
     VEDOX_RELEASE_APP_ID          — the numeric App ID from the app page
     VEDOX_RELEASE_APP_PRIVATE_KEY — the base64-encoded PEM from step 5

Add the token-generation step to .github/workflows/release.yml before
the goreleaser step:

  - name: generate tap token
    id: tap-token
    uses: tibdex/github-app-token@v2
    with:
      app_id: ${{ secrets.VEDOX_RELEASE_APP_ID }}
      private_key: ${{ secrets.VEDOX_RELEASE_APP_PRIVATE_KEY }}
      repository: pixelicous/homebrew-vedox

  - name: run goreleaser
    uses: goreleaser/goreleaser-action@v6
    env:
      HOMEBREW_TAP_TOKEN: ${{ steps.tap-token.outputs.token }}
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    with:
      args: release --clean

---

## Step 4 — verify the tap works

After the first real release tag is pushed and goreleaser has opened and
merged the version-bump PR into homebrew-vedox/main:

```sh
# On a clean machine (or a fresh GitHub Actions macOS runner):
brew tap pixelicous/vedox
brew install vedox
vedox --version
brew test vedox
```

Expected output of `brew tap pixelicous/vedox`:
  ==> Tapping pixelicous/vedox
  Cloning into '/opt/homebrew/Library/Taps/pixelicous/homebrew-vedox'...
  Tapped 1 formula and 1 cask (N files, N KB).

Expected output of `vedox --version`:
  vedox version 2.0.0

If `brew install` fails with a 404 on the URL, the goreleaser
name_template or universal_binaries block is not producing the expected
filename. Run `goreleaser release --snapshot --clean` locally and inspect
the `dist/` directory to confirm the exact archive names.

---

## Secret rotation schedule

| Secret                       | Rotation interval | Where to rotate                                              |
|------------------------------|-------------------|--------------------------------------------------------------|
| HOMEBREW_TAP_TOKEN (PAT)     | Every 90 days     | github.com/settings/tokens then re-set the Actions secret    |
| VEDOX_RELEASE_APP_PRIVATE_KEY | Yearly           | github.com/settings/apps → vedox-release-bot → Keys         |
| APPLE_CERT_P12               | Yearly (cert exp) | Apple Developer portal → Certificates                        |
| APPLE_APP_SPECIFIC_PASSWORD  | On demand         | appleid.apple.com → App-Specific Passwords                   |

---

## Troubleshooting

**`brew tap pixelicous/vedox` returns "not a valid tap"**
The repo must be named `homebrew-vedox` exactly. Check for typos.

**auto-merge PR is left open**
One of the three guards failed. Look at the accept-bump.yml run log:
  - More than one file changed → goreleaser changed something unexpected
  - Diff touches non-version lines → formula has drifted from the template
  - `brew audit --strict --online` failed → formula syntax error

**goreleaser fails with "bad credentials" on the brews block**
HOMEBREW_TAP_TOKEN secret is missing from the release environment or the
token has expired. Re-generate and re-set the secret.
