#!/bin/sh
set -e
if ! ls web/dist/assets/*.js >/dev/null 2>&1; then
  echo "ERROR: web/dist/ does not contain built assets."
  echo "Run 'make web' before 'go build'."
  exit 1
fi
echo "OK: web/dist/ contains built assets"
