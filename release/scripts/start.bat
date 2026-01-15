@echo off
setlocal EnableExtensions EnableDelayedExpansion

rem ACA Release - Windows starter (no Node required; embedded/disk static)

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..") do set "ROOT=%%~fI"

set "ENV_FILE=%ROOT%\.env"
set "BACKEND_EXE=%ROOT%\ai-coding-assistant.exe"

set "RUNTIME_DIR=%ROOT%\.aca"
set "LOG_DIR=%RUNTIME_DIR%\logs"
set "PID_DIR=%RUNTIME_DIR%\pids"
set "BACKEND_LOG=%LOG_DIR%\backend.log"
set "BACKEND_PID_FILE=%PID_DIR%\backend.pid"

call :load_env

if not defined SERVER_HOST set "SERVER_HOST=0.0.0.0"
if not defined SERVER_PORT set "SERVER_PORT=34007"

if not exist "%LOG_DIR%" mkdir "%LOG_DIR%" >nul 2>&1
if not exist "%PID_DIR%" mkdir "%PID_DIR%" >nul 2>&1

if "%~1"=="" (
  call :guide
  exit /b %errorlevel%
)

if /i "%~1"=="guide" (
  call :guide
  exit /b %errorlevel%
)
if /i "%~1"=="setup" (
  call :run_setup
  exit /b %errorlevel%
)
if /i "%~1"=="start" (
  call :start_backend
  exit /b %errorlevel%
)
if /i "%~1"=="stop" (
  call :stop_backend
  exit /b %errorlevel%
)
if /i "%~1"=="restart" (
  call :stop_backend
  call :start_backend
  exit /b %errorlevel%
)
if /i "%~1"=="status" (
  call :status
  exit /b %errorlevel%
)
if /i "%~1"=="logs" (
  call :logs
  exit /b %errorlevel%
)

echo [ERROR] Unknown command: %~1
call :usage
exit /b 1

:usage
echo Usage: start.bat [command]
echo.
echo Commands:
echo   guide    Step-by-step interactive guide ^(default when no args^)
echo   setup    Run web setup wizard ^(first run^)
echo   start    Start backend ^(serves UI + API^)
echo   stop     Stop backend
echo   restart  Restart backend
echo   status   Show status and URLs
echo   logs     Tail backend logs
exit /b 0

:load_env
if not exist "%ENV_FILE%" exit /b 0
for /f "usebackq delims=" %%L in ("%ENV_FILE%") do (
  set "line=%%L"
  if not "!line!"=="" if not "!line:~0,1!"=="#" (
    if /i "!line:~0,7!"=="export " set "line=!line:~7!"
    for /f "tokens=1* delims==" %%A in ("!line!") do (
      if not "%%A"=="" (
        set "%%A=%%B"
      )
    )
  )
)
exit /b 0

:print_urls
set "label=%~1"
set "host=%~2"
set "port=%~3"
if "%port%"=="" exit /b 0

echo %label% URLs:
if "%host%"=="0.0.0.0" goto :print_urls_all
if "%host%"=="::" goto :print_urls_all
echo   - http://%host%:%port%
exit /b 0

:print_urls_all
rem List all IPv4 addresses (LAN/public) via PowerShell. Always include localhost.
for /f "usebackq delims=" %%I in (`powershell -NoProfile -Command ^
  "$ips = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue ^
    | Where-Object { $_.IPAddress -and $_.IPAddress -notlike '127.*' } ^
    | Select-Object -ExpandProperty IPAddress ^
    | Sort-Object -Unique; ^
  foreach ($ip in $ips) { ^
    $scope = 'public'; ^
    if ($ip -like '10.*' -or $ip -like '192.168.*' -or ($ip -match '^172\.(\d+)\.' -and [int]$matches[1] -ge 16 -and [int]$matches[1] -le 31)) { $scope = 'lan' } ^
    elseif ($ip -like '169.254.*') { $scope = 'link' }; ^
    Write-Output ('  - (' + $scope + ') http://' + $ip + ':%port%'); ^
  }"`) do (
  echo %%I
)
echo   - (local) http://localhost:%port%
echo   - (local) http://127.0.0.1:%port%
exit /b 0

:status
echo === AI Coding Assistant (Release / Windows) ===
echo Root: %ROOT%
echo Backend bind: %SERVER_HOST%:%SERVER_PORT%
call :print_urls App %SERVER_HOST% %SERVER_PORT%
echo Log: %BACKEND_LOG%

if exist "%BACKEND_PID_FILE%" (
  set /p PID=<"%BACKEND_PID_FILE%"
  if not "!PID!"=="" (
    tasklist /FI "PID eq !PID!" | findstr /R /C:" !PID! " >nul 2>&1
    if !errorlevel! equ 0 (
      echo Backend: Running (PID !PID!)
      exit /b 0
    )
  )
)
echo Backend: Stopped
exit /b 0

:takeover_port
set "port=%~1"
if "%port%"=="" exit /b 0

for /f "usebackq delims=" %%P in (`powershell -NoProfile -Command ^
  "$pids = Get-NetTCPConnection -LocalPort %port% -State Listen -ErrorAction SilentlyContinue ^
    | Select-Object -ExpandProperty OwningProcess ^
    | Sort-Object -Unique; ^
  foreach ($pid in $pids) { Write-Output $pid }"`) do (
  echo [WARN] Port %port% in use, stopping PID %%P
  taskkill /PID %%P /F >nul 2>&1
)
exit /b 0

:start_backend
if not exist "%BACKEND_EXE%" (
  echo [ERROR] Backend binary not found: %BACKEND_EXE%
  echo         This release package should contain ai-coding-assistant.exe
  exit /b 1
)

call :status >nul 2>&1

rem If port is in use, ask for takeover.
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort %SERVER_PORT% -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }" >nul 2>&1
if %errorlevel% equ 0 (
  set "ans="
  set /p ans=Port %SERVER_PORT% is in use. Take over (kill existing process)? [y/N]:
  if /i "!ans!"=="y" (
    call :takeover_port %SERVER_PORT%
  ) else if /i "!ans!"=="yes" (
    call :takeover_port %SERVER_PORT%
  ) else (
    echo [ERROR] Port %SERVER_PORT% is already in use. Change SERVER_PORT or take over.
    exit /b 1
  )
)

echo [INFO] Starting backend...
echo [INFO] Bind: %SERVER_HOST%:%SERVER_PORT%
call :print_urls App %SERVER_HOST% %SERVER_PORT%

powershell -NoProfile -Command ^
  "$p = Start-Process -FilePath '%BACKEND_EXE%' -WorkingDirectory '%ROOT%' -PassThru -RedirectStandardOutput '%BACKEND_LOG%' -RedirectStandardError '%BACKEND_LOG%'; ^
   $p.Id | Out-File -Encoding ascii '%BACKEND_PID_FILE%'; ^
   Write-Output ('[INFO] Backend PID: ' + $p.Id)"

exit /b 0

:stop_backend
if exist "%BACKEND_PID_FILE%" (
  set /p PID=<"%BACKEND_PID_FILE%"
  if not "!PID!"=="" (
    echo [INFO] Stopping backend PID !PID!...
    taskkill /PID !PID! /F >nul 2>&1
  )
  del "%BACKEND_PID_FILE%" >nul 2>&1
  exit /b 0
)

echo [WARN] No PID file found, trying to free port %SERVER_PORT%...
call :takeover_port %SERVER_PORT%
exit /b 0

:logs
if not exist "%BACKEND_LOG%" (
  echo [WARN] No log file: %BACKEND_LOG%
  exit /b 0
)
echo === Backend Logs (tail 120) ===
powershell -NoProfile -Command "Get-Content -Path '%BACKEND_LOG%' -Tail 120"
exit /b 0

:run_setup
if not exist "%BACKEND_EXE%" (
  echo [ERROR] Backend binary not found: %BACKEND_EXE%
  exit /b 1
)
echo [INFO] Starting setup wizard...
echo [INFO] It will print a setup URL in the terminal.
cd /d "%ROOT%"
"%BACKEND_EXE%" setup
exit /b %errorlevel%

:guide
echo === AI Coding Assistant (Release / Windows) ===
call :status
echo.
echo Choose:
echo   1) Setup wizard (first run)
echo   2) Start backend
echo   3) Stop backend
echo   4) Restart backend
echo   5) Status
echo   6) View logs
echo   7) Exit
echo.
set "choice="
set /p choice=Select [2]:
if "%choice%"=="" set "choice=2"

if "%choice%"=="1" (
  call :run_setup
  exit /b %errorlevel%
)
if "%choice%"=="2" (
  call :start_backend
  exit /b %errorlevel%
)
if "%choice%"=="3" (
  call :stop_backend
  exit /b %errorlevel%
)
if "%choice%"=="4" (
  call :stop_backend
  call :start_backend
  exit /b %errorlevel%
)
if "%choice%"=="5" (
  call :status
  exit /b %errorlevel%
)
if "%choice%"=="6" (
  call :logs
  exit /b %errorlevel%
)
if "%choice%"=="7" (
  exit /b 0
)

echo [ERROR] Invalid selection
exit /b 1

