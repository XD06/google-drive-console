@echo off
REM Quick-start both backend and frontend for development.
REM Usage: start.bat
cd /d "%~dp0"

if exist .dev-pids (
    echo Already running. Run stop.bat first.
    pause
    exit /b 1
)

echo Starting backend (go run ./cmd/server) on :3000 ...
start /b "" cmd /c "go run ./cmd/server > server.log 2> server.err.log"

echo Starting frontend (npm run dev) on :5174 ...
start /b "" cmd /c "cd web && npm run dev > ..\web-dev.log 2>&1"

REM Save marker so stop.bat knows we're running
echo running > .dev-pids

echo.
echo   Backend  -^> http://localhost:3000
echo   Frontend -^> http://localhost:5174
echo.
echo Logs: server.log / server.err.log / web-dev.log
echo Stop: stop.bat
