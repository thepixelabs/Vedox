#!/usr/bin/env bash
# dev.sh — start both Vedox servers (Go backend + SvelteKit frontend)
# Usage: ./dev.sh              start both servers
#        ./dev.sh -fk          force-kill any running instances and exit
#        ./dev.sh --force-kill  same as above
# Stop:  Ctrl+C (kills both processes cleanly)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"

# ── Force-kill mode ───────────────────────────────────────────────────────────
if [[ "${1:-}" == "-fk" || "${1:-}" == "--force-kill" ]]; then
  echo "Force-killing Vedox servers..."
  pkill -9 -f "vedox.*dev --config" 2>/dev/null && echo "  killed: Go backend" || echo "  not running: Go backend"
  pkill -9 -f "vite dev" 2>/dev/null && echo "  killed: SvelteKit frontend" || echo "  not running: SvelteKit frontend"
  exit 0
fi

# ── Start Go backend ──────────────────────────────────────────────────────────
echo "Starting Go backend (port 5150)..."
cd "$ROOT/apps/cli"
go run . dev --config "$ROOT/vedox.config.toml" &
BACKEND_PID=$!

# ── Start SvelteKit frontend ──────────────────────────────────────────────────
echo "Starting SvelteKit frontend (port 5151)..."
cd "$ROOT/apps/editor"
npx pnpm dev &
FRONTEND_PID=$!

# ── Cleanup on exit ───────────────────────────────────────────────────────────
cleanup() {
  echo ""
  echo "Stopping servers..."
  kill "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null || true
  wait "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT INT TERM

echo ""
echo "  Backend:  http://127.0.0.1:5150"
echo "  Frontend: http://127.0.0.1:5151"
echo ""
echo "Press Ctrl+C to stop both."

wait
