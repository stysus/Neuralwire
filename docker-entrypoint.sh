#!/bin/sh
set -e

# Railway attaches volumes with root ownership. The app runs as the
# non-root 'neuralwire' user, so ensure the data directory (SQLite DB) is
# writable before starting. Idempotent and safe on both plain Docker and
# Railway-attached volumes.
if [ -d /app/data ]; then
    chown -R neuralwire:neuralwire /app/data 2>/dev/null || true
fi

# Switch to the non-root user and exec the server (replaces the shell).
exec su-exec neuralwire /app/neuralwire-server "$@"
