@echo off
rem Double-click entry point: scan the local session logs, serve the dashboard,
rem and open it in a browser. Runs the agentmeter.exe sitting beside this file.
setlocal
"%~dp0agentmeter.exe" up %*
if errorlevel 1 (
    echo.
    echo agentmeter exited with an error. The window stays open so you can read it.
    pause
)
endlocal
