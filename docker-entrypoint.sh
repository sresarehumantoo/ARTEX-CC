#!/bin/sh
set -eu

PG_DSN="${CC_ARTEX_PG:?CC_ARTEX_PG must be set}"

# First-run initialization: create an engagement if none exists.
# Retries handle the case where PG is still starting (standalone docker run
# without compose healthchecks).
if [ ! -f /app/data/.cc-artex.json ]; then
    echo "[cc-artex] first run, initializing engagement..."
    i=0
    while [ "$i" -lt 30 ]; do
        if /app/cc-artex init \
            --pg "$PG_DSN" \
            --data /app/data \
            --goal "${CC_ARTEX_GOAL:-Pentest engagement}" 2>/dev/null; then
            break
        fi
        sleep 1
        i=$((i + 1))
    done
    if [ ! -f /app/data/.cc-artex.json ]; then
        echo "[cc-artex] failed to initialize after 30s" >&2
        exit 1
    fi
    echo "[cc-artex] engagement initialized"
fi

exec /app/cc-artex serve \
    --listen :9400 \
    --pg "$PG_DSN" \
    --data /app/data \
    --proxy "${CC_ARTEX_PROXY:-:8788}" \
    "$@"
