#!/usr/bin/env bash
# Quick-start both backend and frontend for development.
# Usage: bash start.sh
set -e
cd "$(dirname "$0")"

PID_FILE=".dev-pids"

if [ -f "$PID_FILE" ]; then
  echo "Already running (pid file exists). Run stop.sh first."
  exit 1
fi

echo "Starting backend (go run ./cmd/server) on :3000 ..."
go run ./cmd/server > server.log 2> server.err.log &
BACKEND_PID=$!

echo "Starting frontend (npm run dev) on :5174 ..."
cd web
npm run dev > ../web-dev.log 2>&1 &
FRONTEND_PID=$!
cd ..

echo "$BACKEND_PID $FRONTEND_PID" > "$PID_FILE"

echo ""
echo "  Backend  PID $BACKEND_PID  → http://localhost:3000"
echo "  Frontend PID $FRONTEND_PID → http://localhost:5174"
echo ""
echo "Logs: server.log / server.err.log / web-dev.log"
echo "Stop: bash stop.sh"
