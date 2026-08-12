#!/usr/bin/env bash
# Stop backend and frontend dev servers.
# Usage: bash stop.sh
cd "$(dirname "$0")"

PID_FILE=".dev-pids"

if [ ! -f "$PID_FILE" ]; then
  echo "No pid file found — nothing to stop."
  # Fallback: kill by port
  echo "Attempting to kill by port..."
  taskkill //F //FI "WINDOWTITLE eq *cmd/server*" 2>/dev/null || true
  exit 0
fi

read -r BACKEND_PID FRONTEND_PID < "$PID_FILE"

echo "Stopping backend (PID $BACKEND_PID) ..."
kill "$BACKEND_PID" 2>/dev/null && echo "  done" || echo "  already stopped"

echo "Stopping frontend (PID $FRONTEND_PID) ..."
kill "$FRONTEND_PID" 2>/dev/null && echo "  done" || echo "  already stopped"

# Kill child processes (go run spawns a child; npm spawns vite)
pkill -P "$BACKEND_PID" 2>/dev/null || true
pkill -P "$FRONTEND_PID" 2>/dev/null || true

rm -f "$PID_FILE"
echo "All stopped."
