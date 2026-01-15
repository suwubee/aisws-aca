@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "ROOT=%~dp0"
if exist "%ROOT%scripts\\start.bat" (
  call "%ROOT%scripts\\start.bat" %*
  exit /b %errorlevel%
)

echo [ERROR] Missing script: %ROOT%scripts\\start.bat
exit /b 1

