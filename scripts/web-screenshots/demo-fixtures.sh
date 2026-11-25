#!/usr/bin/env bash
# Regenerate the static web demo fixtures (web/src/demo/fixtures/) from a
# throwaway, demo-seeded stack. Needs docker compose and python3.
# Set KEEP=1 to leave the stack up.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
compose=(docker compose -f "$here/compose.yml")
trap '[ "${KEEP:-}" = 1 ] || "${compose[@]}" down -v -t 3 >/dev/null 2>&1' EXIT

"${compose[@]}" down -v -t 3 >/dev/null 2>&1 || true
"${compose[@]}" up -d --build
for _ in $(seq 1 60); do
  curl -sf -o /dev/null http://127.0.0.1:19400/api/auth/status && break
  sleep 1
done

python3 -I "$here/seed.py"
python3 -I "$here/demo_fixtures.py"
