# Casks/vedox.rb
# DEFERRED: Vedox editor desktop app (.dmg / .app bundle).
# Ships in v2.1 once the editor packaging format is confirmed.
#
# Current status: the editor is served as a web UI by the vedox daemon;
# there is no standalone .app bundle in v2.0. The Casks/ directory and this
# placeholder file are committed on day-0 so the tap structure is correct and
# `brew search vedox` returns both artifacts when the Cask eventually ships.
#
# When this Cask ships (v2.1 target):
#   1. Replace this file with the full Cask definition below.
#   2. Supply a signed + notarized .dmg (Apple Developer ID Application cert
#      + xcrun notarytool submit + xcrun stapler staple). Unsigned .dmg
#      triggers Gatekeeper; users will blame brew.
#   3. Add sha256 for the .dmg to the goreleaser casks: block.
#   4. Update homebrew-tap-seed/README.md with the install command.
#
# Expected definition (v2.1 target):
#
# cask "vedox-editor" do
#   version "__VERSION__"
#   sha256 "__SHA256_DMG__"
#   url "https://github.com/pixelicous/vedox/releases/download/v__VERSION__/Vedox-__VERSION__-universal.dmg"
#   name "Vedox"
#   desc "Documentation operating system for solo developers"
#   homepage "https://vedox.dev"
#
#   livecheck do
#     url :url
#     strategy :github_latest
#   end
#
#   app "Vedox.app"
# end

raise "Vedox editor cask not yet available. Install the CLI: brew install pixelicous/vedox/vedox"
