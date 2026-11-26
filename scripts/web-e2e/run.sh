#!/usr/bin/env bash
# Run the Playwright smoke suite (web/e2e) against a throwaway Docker build of
# this checkout. Extra arguments go to `playwright test`.
#
# Needs: docker compose, curl, and `npm ci` in web/ (plus
# `npx playwright install chromium` once). E2E_PORT picks the host port
# (default 19410). Set KEEP=1 to leave the stack up.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
export E2E_PORT="${E2E_PORT:-19410}"
compose=(docker compose -p "${E2E_PROJECT:-artex-e2e-$$}" -f "$here/compose.yml")
trap '[ "${KEEP:-}" = 1 ] || "${compose[@]}" down -v --rmi local -t 3 >/dev/null 2>&1' EXIT

"${compose[@]}" up -d --build --wait
ready=
for _ in $(seq 1 60); do
  if curl -sf -o /dev/null "http://127.0.0.1:$E2E_PORT/api/auth/status"; then
    ready=1
    break
  fi
  sleep 1
done
if [ -z "$ready" ]; then
  echo "run.sh: server not ready on 127.0.0.1:$E2E_PORT after 60s; last logs:" >&2
  "${compose[@]}" logs --tail 50 >&2 || true
  exit 1
fi

setup_token="$("${compose[@]}" logs artex 2>/dev/null | grep -oE '[0-9a-f]{32}' | tail -1 || true)"

cd "$repo/web"
E2E_BASE_URL="http://127.0.0.1:$E2E_PORT" E2E_PASSWORD=artex-e2e-pass E2E_SETUP_TOKEN="$setup_token" npx playwright test "$@"
