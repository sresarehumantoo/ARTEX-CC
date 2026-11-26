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
for _ in $(seq 1 60); do
  curl -sf -o /dev/null "http://127.0.0.1:$E2E_PORT/api/auth/status" && break
  sleep 1
done

cd "$repo/web"
E2E_BASE_URL="http://127.0.0.1:$E2E_PORT" E2E_PASSWORD=artex-e2e-pass npx playwright test "$@"
