#!/usr/bin/env bash
# Rebuild the README screenshots (docs/screenshots/) and web walkthrough GIF and
# videos (docs/web-demo.*) from a throwaway, demo-seeded stack.
#
# Needs: docker compose, python3, ffmpeg, and `npm ci` in web/ (for Playwright,
# plus `npx playwright install chromium` once). Set KEEP=1 to leave the stack up.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
compose=(docker compose -f "$here/compose.yml")
video="$(mktemp -d)"
trap 'rm -rf "$video"; [ "${KEEP:-}" = 1 ] || "${compose[@]}" down -v -t 3 >/dev/null 2>&1' EXIT

"${compose[@]}" down -v -t 3 >/dev/null 2>&1 || true
"${compose[@]}" up -d --build
for _ in $(seq 1 60); do
  curl -sf -o /dev/null http://127.0.0.1:19400/api/auth/status && break
  sleep 1
done

python3 -I "$here/seed.py"

mkdir -p "$repo/docs/screenshots"
NODE_PATH="$repo/web/node_modules" node "$here/capture.cjs" "$repo/docs/screenshots" "$video"

webm="$(ls "$video"/*.webm)"
filters="fps=10,scale=1100:-1:flags=lanczos"
ffmpeg -v error -y -ss 1.2 -i "$webm" -vf "$filters,palettegen=max_colors=128:stats_mode=diff" "$video/palette.png"
ffmpeg -v error -y -ss 1.2 -i "$webm" -i "$video/palette.png" \
  -lavfi "$filters [x]; [x][1:v] paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
  "$repo/docs/web-demo.gif"

ffmpeg -v error -y -ss 1.2 -i "$webm" -vf "$filters" -movflags +faststart -pix_fmt yuv420p \
  -c:v libx264 -crf 26 -preset slow -an "$repo/docs/web-demo.mp4"
ffmpeg -v error -y -ss 1.2 -i "$webm" -vf "$filters" -frames:v 1 "$repo/docs/web-demo-poster.png"

echo "wrote docs/screenshots/*.png and docs/web-demo.{gif,mp4} plus poster"
