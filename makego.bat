@echo off
setlocal enabledelayedexpansion

:: ============================================================
:: MakeGo Build Script for Windows (Global Version)
:: ============================================================
:: Can be placed in C:\Windows\System32 or any PATH directory
:: Run from any Go project directory
:: Supports ntfy.sh and Growl notifications
:: ============================================================

:: Get the directory where makego.bat is located
set "MAKEFILE_DIR=%~dp0"
set "MAKEFILE_DIR=%MAKEFILE_DIR:~0,-1%"

:: Get current project directory (where command is executed)
set "PROJECT_DIR=%CD%"

:: Script name
set "SCRIPT_NAME=%~n0"

:: Default values
if not defined APP_NAME (
    for %%I in ("%PROJECT_DIR%") do set "APP_NAME=%%~nxI"
)
if not defined BUILD_DIR set "BUILD_DIR=%PROJECT_DIR%\build"
if not defined GO set "GO=go"
if not defined GOFLAGS set "GOFLAGS=-trimpath"
if not defined LDFLAGS set "LDFLAGS=-s -w"
if not defined VERSION set "VERSION=1.0.0"
if not defined TAGS set "TAGS="

:: Ensure BUILD_DIR is absolute path
set "BUILD_DIR_FIRST=%BUILD_DIR:~0,1%"
set "BUILD_DIR_SECOND=%BUILD_DIR:~1,1%"
if not "%BUILD_DIR_SECOND%"==":" (
    if "%BUILD_DIR_FIRST%"=="\" (
        set "BUILD_DIR=%PROJECT_DIR%%BUILD_DIR%"
    ) else (
        if not "%BUILD_DIR_FIRST%"=="\" (
            set "BUILD_DIR=%PROJECT_DIR%\%BUILD_DIR%"
        )
    )
)

:: Notification settings
if not defined NTFY_ENABLED set "NTFY_ENABLED=0"
if not defined NTFY_PATH set "NTFY_PATH=ntfy"
if not defined NTFY_URL set "NTFY_URL=https://ntfy.sh/mytopic"
if not defined NTFY_ICON set "NTFY_ICON=🔨"
if not defined NTFY_PRIORITY set "NTFY_PRIORITY=default"

if not defined GROWL_ENABLED set "GROWL_ENABLED=0"
if not defined GROWL_PRIORITY set "GROWL_PRIORITY=0"
if not defined GROWL_ICON set "GROWL_ICON=%MAKEFILE_DIR%\go-icon.png"

:: Colors for Windows Console (Windows 10+)
for /F %%a in ('echo prompt $E ^| cmd') do set "ESC=%%a"
set "GREEN=%ESC%[32m"
set "YELLOW=%ESC%[33m"
set "RED=%ESC%[31m"
set "CYAN=%ESC%[36m"
set "MAGENTA=%ESC%[35m"
set "BLUE=%ESC%[34m"
set "WHITE=%ESC%[37m"
set "BOLD=%ESC%[1m"
set "DIM=%ESC%[2m"
set "RESET=%ESC%[0m"

:: Parse arguments
set "COMMAND="
set "ARGS="

:parse_args
if "%~1"=="" goto :execute_command
if /i "%~1"=="help" set "COMMAND=help" & goto :execute_command
if /i "%~1"=="-h" set "COMMAND=help" & goto :execute_command
if /i "%~1"=="--help" set "COMMAND=help" & goto :execute_command

:: Notification flags
if /i "%~1"=="-ntfy" (
    set "NTFY_ENABLED=1"
    shift
    goto :parse_args
)
if /i "%~1"=="-growl" (
    set "GROWL_ENABLED=1"
    shift
    goto :parse_args
)
if /i "%~1"=="-notify" (
    set "NTFY_ENABLED=1"
    set "GROWL_ENABLED=1"
    shift
    goto :parse_args
)

:: Check for variable assignments (--VAR=VALUE)
echo %~1 | findstr /R "^--.*=.*" >nul
if !ERRORLEVEL! EQU 0 (
    set "ARG=%~1"
    set "ARG=!ARG:--=!"
    for /F "tokens=1,* delims==" %%a in ("!ARG!") do (
        set "%%a=%%b"
        echo %DIM%[CONFIG] %%a = %%b%RESET%
    )
    shift
    goto :parse_args
)

:: Check for variable assignments (-VAR VALUE)
if /i "%~1"=="-app-name" (
    set "APP_NAME=%~2"
    echo %DIM%[CONFIG] APP_NAME = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-build-dir" (
    set "TEMP_DIR=%~2"
    set "TEMP_FIRST=!TEMP_DIR:~0,1!"
    set "TEMP_SECOND=!TEMP_DIR:~1,1!"
    if "!TEMP_SECOND!"==":" (
        set "BUILD_DIR=!TEMP_DIR!"
    ) else (
        set "BUILD_DIR=%PROJECT_DIR%\!TEMP_DIR!"
    )
    echo %DIM%[CONFIG] BUILD_DIR = !BUILD_DIR!%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-go" (
    set "GO=%~2"
    echo %DIM%[CONFIG] GO = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-goflags" (
    set "GOFLAGS=%~2"
    echo %DIM%[CONFIG] GOFLAGS = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-ldflags" (
    set "LDFLAGS=%~2"
    echo %DIM%[CONFIG] LDFLAGS = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-tags" (
    set "TAGS=%~2"
    echo %DIM%[CONFIG] TAGS = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-version" (
    set "VERSION=%~2"
    echo %DIM%[CONFIG] VERSION = %~2%RESET%
    shift & shift
    goto :parse_args
)

:: Notification settings
if /i "%~1"=="-ntfy-url" (
    set "NTFY_URL=%~2"
    echo %DIM%[CONFIG] NTFY_URL = %~2%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-ntfy-topic" (
    set "NTFY_URL=https://ntfy.sh/%~2"
    echo %DIM%[CONFIG] NTFY_URL = !NTFY_URL!%RESET%
    shift & shift
    goto :parse_args
)
if /i "%~1"=="-ntfy-path" (
    set "NTFY_PATH=%~2"
    echo %DIM%[CONFIG] NTFY_PATH = %~2%RESET%
    shift & shift
    goto :parse_args
)

:: If it's the first non-option argument, it's the command
if not defined COMMAND (
    set "COMMAND=%~1"
    shift
    goto :parse_args
)

:: Remaining args are passed to the command
set "ARGS=!ARGS! %~1"
shift
goto :parse_args

:execute_command
:: Show project context
if not "%COMMAND%"=="help" (
    echo.
    echo %CYAN%%BOLD%🔨 MakeGo Build System%RESET% %DIM%v%VERSION%%RESET%
    echo %WHITE%============================================================%RESET%
    echo %DIM%Project:%RESET%    %GREEN%%PROJECT_DIR%%RESET%
    echo %DIM%Script:%RESET%     %DIM%%MAKEFILE_DIR%\%SCRIPT_NAME%.bat%RESET%
    echo %DIM%Configuration:%RESET%
    echo   %WHITE%APP_NAME%RESET%  = %GREEN%%APP_NAME%%RESET%
    echo   %WHITE%VERSION%RESET%   = %GREEN%%VERSION%%RESET%
    echo   %WHITE%BUILD_DIR%RESET% = %GREEN%%BUILD_DIR%%RESET%
    echo   %WHITE%GO%RESET%        = %GREEN%%GO%%RESET%
    echo   %WHITE%GOFLAGS%RESET%   = %GREEN%%GOFLAGS%%RESET%
    echo   %WHITE%LDFLAGS%RESET%   = %GREEN%%LDFLAGS%%RESET%
    if defined TAGS echo   %WHITE%TAGS%RESET%      = %GREEN%%TAGS%%RESET%
    if "%NTFY_ENABLED%"=="1" echo   %WHITE%NTFY%RESET%      = %GREEN%enabled%RESET% ^(%DIM%!NTFY_URL!%RESET%^)
    if "%GROWL_ENABLED%"=="1" echo   %WHITE%GROWL%RESET%     = %GREEN%enabled%RESET%
    echo %WHITE%------------------------------------------------------------%RESET%
)

:: Default command
if not defined COMMAND set "COMMAND=build"

:: Execute command
if /i "%COMMAND%"=="build" goto :build
if /i "%COMMAND%"=="install" goto :install
if /i "%COMMAND%"=="test" goto :test
if /i "%COMMAND%"=="clean" goto :clean
if /i "%COMMAND%"=="lint" goto :lint
if /i "%COMMAND%"=="run" goto :run
if /i "%COMMAND%"=="build-all" goto :build_all
if /i "%COMMAND%"=="release" goto :release
if /i "%COMMAND%"=="dev" goto :dev
if /i "%COMMAND%"=="deps" goto :deps
if /i "%COMMAND%"=="fmt" goto :fmt
if /i "%COMMAND%"=="vet" goto :vet
if /i "%COMMAND%"=="coverage" goto :coverage
if /i "%COMMAND%"=="all" goto :all
if /i "%COMMAND%"=="help" goto :help

echo %RED%%BOLD%❌ Unknown command:%RESET% %WHITE%%COMMAND%%RESET%
echo.
goto :help

:: ============================================================
:: BUILD HELPER - Construct build command
:: ============================================================
:build_command
set "BUILD_CMD=%GO% build"

:: Add goflags
if defined GOFLAGS (
    set "BUILD_CMD=!BUILD_CMD! %GOFLAGS%"
)

:: Add ldflags
if defined LDFLAGS (
    set "BUILD_CMD=!BUILD_CMD! -ldflags="%LDFLAGS% -X main.version=%VERSION%""
) else (
    set "BUILD_CMD=!BUILD_CMD! -ldflags="-X main.version=%VERSION%""
)

:: Add tags
if defined TAGS (
    set "BUILD_CMD=!BUILD_CMD! -tags=%TAGS%"
)

:: Add output
set "BUILD_CMD=!BUILD_CMD! -o "%BUILD_DIR%\%APP_NAME%.exe""

:: Add target
set "BUILD_CMD=!BUILD_CMD! !MAIN_PATH!"

goto :eof

:: ============================================================
:: NOTIFICATION FUNCTIONS
:: ============================================================
:send_notification
set "notif_type=%1"
set "notif_title=%2"
set "notif_message=%3"
set "notif_priority=%~4"

:: Get current date and time
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set datetime=%%I
set "current_date=%datetime:~0,4%-%datetime:~4,2%-%datetime:~6,2%"
set "current_time=%datetime:~8,2%:%datetime:~10,2%:%datetime:~12,2%"

:: ntfy.sh notification
if "%NTFY_ENABLED%"=="1" (
    if exist "%NTFY_PATH%" (
        echo %DIM%Sending ntfy notification...%RESET%
        "%NTFY_PATH%" pub -t "%notif_title% %current_date% %current_time%" -m "[%current_date% %current_time%] %notif_message%" -p "%notif_priority%" %NTFY_URL% >nul 2>&1
        if !ERRORLEVEL! EQU 0 (
            echo %GREEN%  ✅ ntfy notification sent%RESET%
        ) else (
            echo %YELLOW%  ⚠️  ntfy notification failed%RESET%
        )
    ) else (
        echo %YELLOW%  ⚠️  ntfy executable not found: %NTFY_PATH%%RESET%
    )
)

:: Growl notification
if "%GROWL_ENABLED%"=="1" (
    where sendgrowl >nul 2>&1
    if !ERRORLEVEL! EQU 0 (
        echo %DIM%Sending Growl notification...%RESET%
        
        set "growl_icon=%GROWL_ICON%"
        if "%notif_type%"=="success" set "growl_icon=%MAKEFILE_DIR%\success.png"
        if "%notif_type%"=="error" set "growl_icon=%MAKEFILE_DIR%\error.png"
        if "%notif_type%"=="warning" set "growl_icon=%MAKEFILE_DIR%\warning.png"
        
        sendgrowl GoBuild "%notif_type%" "%notif_title%" "%notif_message%" -p %notif_priority% -i "!growl_icon!" >nul 2>&1
        if !ERRORLEVEL! EQU 0 (
            echo %GREEN%  ✅ Growl notification sent%RESET%
        ) else (
            echo %YELLOW%  ⚠️  Growl notification failed%RESET%
        )
    ) else (
        echo %YELLOW%  ⚠️  sendgrowl not found in PATH%RESET%
    )
)
goto :eof

:build_success_notification
call :send_notification "success" "MakeGo Build: Success !current_date! !current_time!" "[!current_date! !current_time!] !APP_NAME! v!VERSION! built successfully" "low"
goto :eof

:build_error_notification
call :send_notification "error" "MakeGo Build: Failed !current_date! !current_time!" "[!current_date! !current_time!] !APP_NAME! v!VERSION! build failed" "high"
goto :eof

:test_success_notification
call :send_notification "success" "MakeGo Test: Passed !current_date! !current_time!" "[!current_date! !current_time!] All tests passed for !APP_NAME!" "low"
goto :eof

:test_error_notification
call :send_notification "error" "MakeGo Test: Failed !current_date! !current_time!" "[!current_date! !current_time!] Tests failed for !APP_NAME!" "high"
goto :eof

:: ============================================================
:: BUILD
:: ============================================================
:build
echo.
echo %CYAN%%BOLD%🔨 Building %APP_NAME% v%VERSION%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

:: Validate project directory
if not exist "%PROJECT_DIR%\go.mod" (
    echo %YELLOW%%BOLD%⚠️  Warning:%RESET% %WHITE%No go.mod found in %PROJECT_DIR%%RESET%
    echo %DIM%   This might not be a Go project directory%RESET%
    echo.
)

:: Auto-create build directory
if not exist "%BUILD_DIR%" (
    echo %DIM%Creating build directory: %BUILD_DIR%%RESET%
    mkdir "%BUILD_DIR%" 2>nul
    if !ERRORLEVEL! NEQ 0 (
        echo %RED%%BOLD%❌ Failed to create build directory%RESET%
        call :build_error_notification
        exit /b 1
    )
)

:: Check if main.go exists
set "MAIN_PATH=.\cmd\%APP_NAME%"
if not exist "%PROJECT_DIR%\cmd\%APP_NAME%\main.go" (
    if exist "%PROJECT_DIR%\main.go" (
        set "MAIN_PATH=."
    ) else (
        echo %RED%%BOLD%❌ No main.go found in project%RESET%
        echo %DIM%   Expected: %PROJECT_DIR%\cmd\%APP_NAME%\main.go%RESET%
        echo %DIM%   Or:       %PROJECT_DIR%\main.go%RESET%
        call :build_error_notification
        exit /b 1
    )
)

echo %DIM%Target:%RESET% %WHITE%!MAIN_PATH!%RESET%
echo %DIM%Output:%RESET% %WHITE%%BUILD_DIR%\%APP_NAME%.exe%RESET%

:: Build the command
call :build_command

echo %DIM%Command:%RESET% %WHITE%!BUILD_CMD!%RESET%
echo.

:: Execute build
%BUILD_CMD%
if !ERRORLEVEL! NEQ 0 (
    echo.
    echo %RED%%BOLD%❌ Build failed!%RESET%
    call :build_error_notification
    exit /b 1
)

:: Build success
echo.
echo %GREEN%%BOLD%✅ Build complete!%RESET%

:: Show binary info
if exist "%BUILD_DIR%\%APP_NAME%.exe" (
    for %%F in ("%BUILD_DIR%\%APP_NAME%.exe") do (
        set "size=%%~zF"
        set /a "size_kb=!size! / 1024"
        set "build_time=%%~tF"
        echo %WHITE%   Binary:%RESET%  %GREEN%%BUILD_DIR%\%APP_NAME%.exe%RESET%
        echo %WHITE%   Size:%RESET%    %GREEN%!size_kb! KB%RESET%
        echo %WHITE%   Built:%RESET%   %GREEN%!build_time!%RESET%
    )
)

call :build_success_notification
echo.
goto :eof

:: ============================================================
:: INSTALL
:: ============================================================
:install
echo.
echo %CYAN%%BOLD%📦 Installing %APP_NAME% v%VERSION%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

:: Run build first
call :build
if %ERRORLEVEL% NEQ 0 exit /b 1

:: Determine install directory
if not defined GOPATH (
    for /f "tokens=*" %%i in ('%GO% env GOPATH 2^>nul') do set "GOPATH=%%i"
    if not defined GOPATH set "GOPATH=%USERPROFILE%\go"
)
set "INSTALL_DIR=%GOPATH%\bin"

:: Auto-create install directory
if not exist "%INSTALL_DIR%" (
    echo %DIM%Creating install directory: %INSTALL_DIR%%RESET%
    mkdir "%INSTALL_DIR%"
)

echo %WHITE%Installing to:%RESET% %GREEN%%INSTALL_DIR%%RESET%

copy /Y "%BUILD_DIR%\%APP_NAME%.exe" "%INSTALL_DIR%\%APP_NAME%.exe" >nul
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Installation failed!%RESET%
    call :send_notification "error" "MakeGo Install: Failed" "Failed to install %APP_NAME%" "high"
    exit /b 1
)

echo %GREEN%%BOLD%✅ Installed successfully!%RESET%

:: Check if in PATH
echo %PATH% | find /i "%INSTALL_DIR%" >nul
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %YELLOW%%BOLD%⚠️  PATH Warning%RESET%
    echo %YELLOW%   %INSTALL_DIR% is not in your PATH%RESET%
    echo %WHITE%   Add it to use %APP_NAME% from anywhere:%RESET%
    echo %WHITE%   setx PATH "%%PATH%%;%INSTALL_DIR%"%RESET%
) else (
    echo %GREEN%   Ready to use: %APP_NAME% --help%RESET%
)

call :send_notification "success" "MakeGo Install: Success" "%APP_NAME% installed to %INSTALL_DIR%" "low"
echo.
goto :eof

:: ============================================================
:: TEST
:: ============================================================
:test
echo.
echo %CYAN%%BOLD%🧪 Running tests...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

:: Check for test files
dir /s /b "%PROJECT_DIR%\*_test.go" >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  No test files found%RESET%
    goto :eof
)

%GO% test -v -race -coverprofile="%BUILD_DIR%\coverage.out" ./...
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %RED%%BOLD%❌ Tests failed!%RESET%
    call :test_error_notification
    exit /b 1
)

:: Generate coverage report
if exist "%BUILD_DIR%\coverage.out" (
    %GO% tool cover -html="%BUILD_DIR%\coverage.out" -o "%BUILD_DIR%\coverage.html"
    if %ERRORLEVEL% EQU 0 (
        echo %GREEN%   Coverage report: %BUILD_DIR%\coverage.html%RESET%
    )
)

echo.
echo %GREEN%%BOLD%✅ All tests passed!%RESET%
call :test_success_notification
echo.
goto :eof

:: ============================================================
:: CLEAN
:: ============================================================
:clean
echo.
echo %CYAN%%BOLD%🧹 Cleaning build artifacts...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

set "DELETED=0"

if exist "%BUILD_DIR%" (
    echo %DIM%Removing %BUILD_DIR% directory...%RESET%
    rmdir /S /Q "%BUILD_DIR%" 2>nul
    if !ERRORLEVEL! EQU 0 (
        echo %GREEN%  ✅ Removed build directory%RESET%
        set /a DELETED=1
    )
)

if exist "%PROJECT_DIR%\coverage.out" (
    del /Q "%PROJECT_DIR%\coverage.out" 2>nul
    echo %GREEN%  ✅ Removed coverage.out%RESET%
    set /a DELETED=1
)

if exist "%PROJECT_DIR%\coverage.html" (
    del /Q "%PROJECT_DIR%\coverage.html" 2>nul
    echo %GREEN%  ✅ Removed coverage.html%RESET%
    set /a DELETED=1
)

if exist "%PROJECT_DIR%\%APP_NAME%.exe" (
    del /Q "%PROJECT_DIR%\%APP_NAME%.exe" 2>nul
    echo %GREEN%  ✅ Removed local binary%RESET%
    set /a DELETED=1
)

if %DELETED% EQU 0 (
    echo %DIM%   Nothing to clean%RESET%
)

echo.
echo %GREEN%%BOLD%✅ Clean complete!%RESET%
call :send_notification "success" "MakeGo Clean: Complete" "Build artifacts cleaned for %APP_NAME%" "low"
echo.
goto :eof

:: ============================================================
:: LINT
:: ============================================================
:lint
echo.
echo %CYAN%%BOLD%🔍 Running linter...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

where golangci-lint >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  golangci-lint not found%RESET%
    echo %WHITE%   Installing golangci-lint...%RESET%
    %GO% install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    if !ERRORLEVEL! NEQ 0 (
        echo %RED%%BOLD%❌ Failed to install golangci-lint%RESET%
        call :send_notification "error" "MakeGo Lint: Failed" "golangci-lint installation failed" "high"
        exit /b 1
    )
    echo %GREEN%   ✅ Installed%RESET%
)

echo %WHITE%Analyzing code...%RESET%
golangci-lint run ./...
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %RED%%BOLD%❌ Lint issues found!%RESET%
    call :send_notification "warning" "MakeGo Lint: Issues Found" "Lint issues detected in %APP_NAME%" "normal"
    exit /b 1
)

echo %GREEN%%BOLD%✅ No lint issues found!%RESET%
call :send_notification "success" "MakeGo Lint: Passed" "No lint issues in %APP_NAME%" "low"
echo.
goto :eof

:: ============================================================
:: RUN
:: ============================================================
:run
echo.
echo %CYAN%%BOLD%🚀 Running %APP_NAME%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

echo %DIM%Command:%RESET% %GO% run .\cmd\%APP_NAME% %ARGS%
echo.

%GO% run .\cmd\%APP_NAME% %ARGS%
set "EXIT_CODE=%ERRORLEVEL%"

if %EXIT_CODE% NEQ 0 (
    echo.
    echo %YELLOW%⚠️  Process exited with code: %EXIT_CODE%%RESET%
)
echo.
goto :eof

:: ============================================================
:: BUILD ALL
:: ============================================================
:build_all
echo.
echo %CYAN%%BOLD%🚀 Building for all platforms...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

set "SUCCESS=0"
set "FAILED=0"

:: Build helper for cross-compilation
set "CROSS_MAIN_PATH=!MAIN_PATH!"
if not defined CROSS_MAIN_PATH (
    set "CROSS_MAIN_PATH=.\cmd\%APP_NAME%"
    if not exist "%PROJECT_DIR%\cmd\%APP_NAME%\main.go" (
        if exist "%PROJECT_DIR%\main.go" set "CROSS_MAIN_PATH=."
    )
)

:: Windows amd64
echo.
echo %WHITE%%BOLD%Building for windows/amd64...%RESET%
set "GOOS=windows"
set "GOARCH=amd64"
set "BUILD_CMD=%GO% build %GOFLAGS% -ldflags="-s -w -X main.version=%VERSION%" -o "%BUILD_DIR%\%APP_NAME%-windows-amd64.exe" !CROSS_MAIN_PATH!"
echo %DIM%!BUILD_CMD!%RESET%
!BUILD_CMD!
if !ERRORLEVEL! EQU 0 (
    echo %GREEN%  ✅ %APP_NAME%-windows-amd64.exe%RESET%
    set /a SUCCESS+=1
) else (
    echo %RED%  ❌ Failed to build for windows/amd64%RESET%
    set /a FAILED+=1
)

:: Windows 386
echo.
echo %WHITE%%BOLD%Building for windows/386...%RESET%
set "GOOS=windows"
set "GOARCH=386"
set "BUILD_CMD=%GO% build %GOFLAGS% -ldflags="-s -w -X main.version=%VERSION%" -o "%BUILD_DIR%\%APP_NAME%-windows-386.exe" !CROSS_MAIN_PATH!"
echo %DIM%!BUILD_CMD!%RESET%
!BUILD_CMD!
if !ERRORLEVEL! EQU 0 (
    echo %GREEN%  ✅ %APP_NAME%-windows-386.exe%RESET%
    set /a SUCCESS+=1
) else (
    echo %RED%  ❌ Failed to build for windows/386%RESET%
    set /a FAILED+=1
)

:: Windows arm64
echo.
echo %WHITE%%BOLD%Building for windows/arm64...%RESET%
set "GOOS=windows"
set "GOARCH=arm64"
set "BUILD_CMD=%GO% build %GOFLAGS% -ldflags="-s -w -X main.version=%VERSION%" -o "%BUILD_DIR%\%APP_NAME%-windows-arm64.exe" !CROSS_MAIN_PATH!"
echo %DIM%!BUILD_CMD!%RESET%
!BUILD_CMD!
if !ERRORLEVEL! EQU 0 (
    echo %GREEN%  ✅ %APP_NAME%-windows-arm64.exe%RESET%
    set /a SUCCESS+=1
) else (
    echo %YELLOW%  ⚠️  Failed to build for windows/arm64 (may not be supported)%RESET%
    set /a FAILED+=1
)

echo.
echo %WHITE%============================================================%RESET%
echo %GREEN%Successful builds: %SUCCESS%%RESET%
if %FAILED% GTR 0 (
    echo %RED%Failed builds: %FAILED%%RESET%
    call :send_notification "error" "MakeGo Build-All: Failed" "%APP_NAME% cross-compilation had %FAILED% failures" "high"
) else (
    call :send_notification "success" "MakeGo Build-All: Success" "%APP_NAME% built for all platforms" "low"
)
echo %WHITE%Output directory: %BUILD_DIR%\%RESET%
if exist "%BUILD_DIR%" dir "%BUILD_DIR%" 2>nul | find ".exe"
echo.
goto :eof

:: ============================================================
:: RELEASE
:: ============================================================
:release
echo.
echo %CYAN%%BOLD%📦 Creating release v%VERSION%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

where goreleaser >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  goreleaser not found%RESET%
    echo %WHITE%   Installing goreleaser...%RESET%
    %GO% install github.com/goreleaser/goreleaser@latest
    if !ERRORLEVEL! NEQ 0 (
        echo %RED%%BOLD%❌ Failed to install goreleaser%RESET%
        call :send_notification "error" "MakeGo Release: Failed" "goreleaser installation failed" "high"
        exit /b 1
    )
)

goreleaser release --snapshot --clean
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %RED%%BOLD%❌ Release build failed!%RESET%
    call :send_notification "error" "MakeGo Release: Failed" "%APP_NAME% v%VERSION% release failed" "high"
    exit /b 1
)

echo.
echo %GREEN%%BOLD%✅ Release v%VERSION% created in dist/ directory%RESET%
call :send_notification "success" "MakeGo Release: Success" "%APP_NAME% v%VERSION% released" "low"
echo.
goto :eof

:: ============================================================
:: DEV
:: ============================================================
:dev
echo.
echo %CYAN%%BOLD%👨‍💻 Starting development mode...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

where air >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo %GREEN%✅ Watching for changes with air...%RESET%
    echo.
    air
    goto :eof
)

echo %YELLOW%⚠️  air not found for live reload%RESET%
echo %DIM%   Install with: go install github.com/cosmtrek/air@latest%RESET%
echo %YELLOW%   Using manual watch mode (Ctrl+C to stop)%RESET%
echo.

:watch_loop
cls
call :build
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %RED%Build failed. Fix errors and waiting...%RESET%
)
echo.
echo %DIM%[%DATE% %TIME%] Waiting for changes... (Ctrl+C to stop)%RESET%
timeout /t 3 /nobreak >nul
goto :watch_loop

:: ============================================================
:: DEPS
:: ============================================================
:deps
echo.
echo %CYAN%%BOLD%📥 Managing dependencies...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

cd /d "%PROJECT_DIR%"

echo %WHITE%Downloading dependencies...%RESET%
%GO% mod download
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Failed to download dependencies%RESET%
    call :send_notification "error" "MakeGo Deps: Failed" "Failed to download dependencies" "high"
    exit /b 1
)

echo %WHITE%Tidying modules...%RESET%
%GO% mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Failed to tidy modules%RESET%
    exit /b 1
)

echo %WHITE%Verifying modules...%RESET%
%GO% mod verify
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Module verification failed%RESET%
    exit /b 1
)

echo.
echo %GREEN%%BOLD%✅ Dependencies updated!%RESET%
call :send_notification "success" "MakeGo Deps: Updated" "Dependencies updated for %APP_NAME%" "low"
echo.
goto :eof

:: ============================================================
:: FMT
:: ============================================================
:fmt
echo.
echo %CYAN%%BOLD%🎨 Formatting code...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

%GO% fmt ./...
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Formatting failed!%RESET%
    exit /b 1
)

echo %GREEN%%BOLD%✅ Code formatted!%RESET%
echo.
goto :eof

:: ============================================================
:: VET
:: ============================================================
:vet
echo.
echo %CYAN%%BOLD%🔍 Running go vet...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

%GO% vet ./...
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo %RED%%BOLD%❌ vet found issues!%RESET%
    exit /b 1
)

echo %GREEN%%BOLD%✅ No issues found!%RESET%
echo.
goto :eof

:: ============================================================
:: COVERAGE
:: ============================================================
:coverage
echo.
echo %CYAN%%BOLD%📊 Generating coverage report...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

%GO% test -coverprofile="%BUILD_DIR%\coverage.out" ./...
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Failed to generate coverage data%RESET%
    exit /b 1
)

%GO% tool cover -html="%BUILD_DIR%\coverage.out" -o "%BUILD_DIR%\coverage.html"
if %ERRORLEVEL% NEQ 0 (
    echo %RED%%BOLD%❌ Failed to generate HTML report%RESET%
    exit /b 1
)

echo %GREEN%%BOLD%✅ Coverage report generated:%RESET%
echo %WHITE%   %BUILD_DIR%\coverage.out  - Raw coverage data%RESET%
echo %WHITE%   %BUILD_DIR%\coverage.html - HTML report%RESET%

start "" "%BUILD_DIR%\coverage.html"
echo %DIM%   Opening coverage.html in browser...%RESET%
echo.
goto :eof

:: ============================================================
:: ALL
:: ============================================================
:all
echo.
echo %CYAN%%BOLD%🔄 Running full CI/CD pipeline...%RESET%
echo %WHITE%============================================================%RESET%

set "PIPELINE_START=%time%"
set "PIPELINE_ERROR=0"

echo %DIM%[1/6] Clean...%RESET%
call :clean
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Clean failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Clean passed%RESET%

echo.
echo %DIM%[2/6] Dependencies...%RESET%
call :deps
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Dependencies failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Dependencies passed%RESET%

echo.
echo %DIM%[3/6] Format...%RESET%
call :fmt
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Format failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Format passed%RESET%

echo.
echo %DIM%[4/6] Vet...%RESET%
call :vet
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Vet failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Vet passed%RESET%

echo.
echo %DIM%[5/6] Tests...%RESET%
call :test
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Tests failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Tests passed%RESET%

echo.
echo %DIM%[6/6] Build...%RESET%
call :build
if %ERRORLEVEL% NEQ 0 (
    set "PIPELINE_ERROR=1"
    echo %RED%✖ Build failed%RESET%
    goto :pipeline_end
)
echo %GREEN%✔ Build passed%RESET%

:pipeline_end
set "PIPELINE_END=%time%"
echo.
echo %WHITE%============================================================%RESET%
echo %DIM%Pipeline started: %PIPELINE_START%%RESET%
echo %DIM%Pipeline ended:   %PIPELINE_END%%RESET%

if %PIPELINE_ERROR% EQU 0 (
    echo.
    echo %GREEN%%BOLD%✅ Full pipeline completed successfully!%RESET%
    call :send_notification "success" "MakeGo CI: Success" "%APP_NAME% v%VERSION% pipeline passed all stages" "low"
) else (
    echo.
    echo %RED%%BOLD%❌ Pipeline failed!%RESET%
    call :send_notification "error" "MakeGo CI: Failed" "%APP_NAME% v%VERSION% pipeline failed" "high"
)
echo.
exit /b %PIPELINE_ERROR%
goto :eof

:: ============================================================
:: HELP
:: ============================================================
:help
echo.
echo %CYAN%%BOLD%🔨 MakeGo Build System v%VERSION% (Global Edition)%RESET%
echo %WHITE%============================================================%RESET%
echo.
echo %YELLOW%%BOLD%Usage:%RESET% %SCRIPT_NAME%.bat [options] [command] [args...]
echo.
echo %DIM%Can be run from any Go project directory!%RESET%
echo.
echo %MAGENTA%%BOLD%Configuration Options:%RESET%
echo.
echo   %GREEN%--APP_NAME=VALUE%RESET%     Set application name (default: current dir name)
echo   %GREEN%--VERSION=VALUE%RESET%      Set version number (default: 1.0.0)
echo   %GREEN%--BUILD_DIR=VALUE%RESET%    Set build output directory (default: .\build)
echo   %GREEN%--GO=VALUE%RESET%           Set Go executable path (default: go)
echo   %GREEN%--GOFLAGS=VALUE%RESET%      Set Go build flags
echo   %GREEN%--LDFLAGS=VALUE%RESET%      Set linker flags (default: -s -w)
echo   %GREEN%--TAGS=VALUE%RESET%         Set build tags
echo.
echo %MAGENTA%%BOLD%Short Options:%RESET%
echo.
echo   %GREEN%-app-name VALUE%RESET%
echo   %GREEN%-version VALUE%RESET%
echo   %GREEN%-build-dir VALUE%RESET%
echo   %GREEN%-go VALUE%RESET%
echo   %GREEN%-goflags VALUE%RESET%
echo   %GREEN%-ldflags VALUE%RESET%
echo   %GREEN%-tags VALUE%RESET%
echo.
echo %MAGENTA%%BOLD%Notification Options:%RESET%
echo.
echo   %GREEN%-ntfy%RESET%                Enable ntfy.sh notifications
echo   %GREEN%-growl%RESET%               Enable Growl notifications
echo   %GREEN%-notify%RESET%              Enable both notification types
echo   %GREEN%--NTFY_URL=VALUE%RESET%     Set ntfy.sh URL
echo   %GREEN%-ntfy-topic VALUE%RESET%    Set ntfy.sh topic name
echo   %GREEN%--NTFY_PATH=VALUE%RESET%    Set ntfy executable path
echo.
echo %MAGENTA%%BOLD%Available Commands:%RESET%
echo.
echo   %GREEN%build%RESET%      🔨 Build the application
echo   %GREEN%install%RESET%    📦 Build and install to GOPATH/bin
echo   %GREEN%test%RESET%       🧪 Run tests with coverage
echo   %GREEN%clean%RESET%      🧹 Remove build artifacts
echo   %GREEN%lint%RESET%       🔍 Run code linter
echo   %GREEN%run%RESET%        🚀 Run the application
echo   %GREEN%build-all%RESET%  🚀 Cross-compile for all platforms
echo   %GREEN%release%RESET%    📦 Create release with goreleaser
echo   %GREEN%dev%RESET%        👨‍💻 Development mode with watch
echo   %GREEN%deps%RESET%       📥 Update dependencies
echo   %GREEN%fmt%RESET%        🎨 Format code
echo   %GREEN%vet%RESET%        🔍 Run go vet
echo   %GREEN%coverage%RESET%   📊 Generate coverage report
echo   %GREEN%all%RESET%        🔄 Run full CI pipeline
echo   %GREEN%help%RESET%       ❓ Show this help
echo.
echo %MAGENTA%%BOLD%Examples:%RESET%
echo.
echo   %WHITE%# Basic build%RESET%
echo   %SCRIPT_NAME%.bat build
echo.
echo   %WHITE%# Custom version with specific ldflags%RESET%
echo   %SCRIPT_NAME%.bat --VERSION=2.0.0 --LDFLAGS="-s -w -X main.build=production" build
echo.
echo   %WHITE%# Build with race detector%RESET%
echo   %SCRIPT_NAME%.bat --GOFLAGS=-race build
echo.
echo   %WHITE%# Cross-compile with notifications%RESET%
echo   %SCRIPT_NAME%.bat -ntfy -ntfy-topic my-builds build-all
echo.
echo   %WHITE%# Pass args to app%RESET%
echo   %SCRIPT_NAME%.bat run -- --help
echo.
echo %YELLOW%%BOLD%Current Context:%RESET%
echo   %WHITE%Project:%RESET%  %GREEN%%PROJECT_DIR%%RESET%
echo   %WHITE%Script:%RESET%   %DIM%%MAKEFILE_DIR%\%SCRIPT_NAME%.bat%RESET%
echo   %WHITE%App:%RESET%      %GREEN%%APP_NAME%%RESET% %DIM%v%VERSION%%RESET%
echo   %WHITE%Build:%RESET%    %GREEN%%BUILD_DIR%%RESET%
echo.
echo %DIM%Works from any directory! Just run: %SCRIPT_NAME%.bat [command]%RESET%
echo.
goto :eof