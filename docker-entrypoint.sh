#!/bin/sh
set -eu

PG_DSN="${ARTEX_PG:?ARTEX_PG must be set}"

# First-run initialization: create an engagement if none exists.
# Retries handle the case where PG is still starting (standalone docker run
# without compose healthchecks).
if [ ! -f /app/data/.artex.json ]; then
    echo "[artex] first run, initializing engagement..."
    i=0
    while [ "$i" -lt 30 ]; do
        if /app/artex-cc init \
            --pg "$PG_DSN" \
            --data /app/data \
            --goal "${ARTEX_GOAL:-Pentest engagement}" 2>/dev/null; then
            break
        fi
        sleep 1
        i=$((i + 1))
    done
    if [ ! -f /app/data/.artex.json ]; then
        echo "[artex] failed to initialize after 30s" >&2
        exit 1
    fi
    echo "[artex] engagement initialized"
fi

exec /app/artex-cc serve \
    --listen :9400 \
    --pg "$PG_DSN" \
    --data /app/data \
    --proxy "${ARTEX_PROXY:-:8788}" \
    --web \
    "$@"
