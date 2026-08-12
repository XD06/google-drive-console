@echo off
REM Stop backend and frontend dev servers.
REM Usage: stop.bat
cd /d "%~dp0"

echo Stopping dev servers...

REM Kill Go backend (go run spawns a child with the actual binary)
for /f "tokens=2" %%a in ('tasklist /fi "imagename eq main.exe" /fo list 2^>nul ^| find "PID:"') do (
    taskkill /f /pid %%a >nul 2>&1
)
taskkill /f /im "main.exe" >nul 2>&1

REM Kill Vite dev server (node processes on port 5174)
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":5174 " ^| findstr "LISTENING"') do (
    taskkill /f /pid %%a >nul 2>&1
)

REM Kill any go run parent
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":3000 " ^| findstr "LISTENING"') do (
    taskkill /f /pid %%a >nul 2>&1
)

del /f .dev-pids >nul 2>&1
echo All stopped.
