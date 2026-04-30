@echo off
setlocal enabledelayedexpansion

:: PathMan Build Script for Windows
:: ============================================================

set "APP_NAME=pathman"
set "BUILD_DIR=build"
set "GO=go"
set "GOFLAGS=-trimpath -ldflags="-s -w""
set "VERSION=1.0.0"

:: Colors for Windows Console (requires Windows 10+)
:: Using ANSI escape codes
for /F %%a in ('echo prompt $E ^| cmd') do set "ESC=%%a"
set "GREEN=%ESC%[32m"
set "YELLOW=%ESC%[33m"
set "RED=%ESC%[31m"
set "CYAN=%ESC%[36m"
set "MAGENTA=%ESC%[35m"
set "BLUE=%ESC%[34m"
set "WHITE=%ESC%[37m"
set "BOLD=%ESC%[1m"
set "RESET=%ESC%[0m"

:: Check if help requested
if "%1"=="help" goto :help
if "%1"=="-h" goto :help
if "%1"=="--help" goto :help

:: Default target
if "%1"=="" goto :build

:: Parse commands
if "%1"=="build" goto :build
if "%1"=="install" goto :install
if "%1"=="test" goto :test
if "%1"=="clean" goto :clean
if "%1"=="lint" goto :lint
if "%1"=="run" goto :run
if "%1"=="build-all" goto :build-all
if "%1"=="release" goto :release
if "%1"=="dev" goto :dev
if "%1"=="deps" goto :deps
if "%1"=="fmt" goto :fmt
if "%1"=="vet" goto :vet
if "%1"=="coverage" goto :coverage
if "%1"=="all" goto :all

:: If no match, show help
goto :help

:: ============================================================
:: BUILD
:: ============================================================
:build
echo.
echo %CYAN%%BOLD%🔨 Building %APP_NAME%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%.exe" .\cmd\%APP_NAME%
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Build failed!%RESET%
    exit /b 1
)

echo %GREEN%✅ Build complete:%RESET% %WHITE%%BUILD_DIR%\%APP_NAME%.exe%RESET%
echo.
echo %MAGENTA%📦 Binary size:%RESET%
dir "%BUILD_DIR%\%APP_NAME%.exe" | find "%APP_NAME%.exe"
echo.
goto :eof

:: ============================================================
:: INSTALL
:: ============================================================
:install
echo.
echo %CYAN%%BOLD%📦 Installing %APP_NAME%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

call :build
if %ERRORLEVEL% NEQ 0 exit /b 1

:: Check if GOPATH/bin exists
if not defined GOPATH (
    set "GOPATH=%USERPROFILE%\go"
)

set "INSTALL_DIR=%GOPATH%\bin"
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

copy /Y "%BUILD_DIR%\%APP_NAME%.exe" "%INSTALL_DIR%\%APP_NAME%.exe" >nul
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Installation failed!%RESET%
    exit /b 1
)

echo %GREEN%✅ Installed successfully to:%RESET% %WHITE%%INSTALL_DIR%\%APP_NAME%.exe%RESET%

:: Check if in PATH
echo %PATH% | find /i "%INSTALL_DIR%" >nul
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  Warning: %INSTALL_DIR% is not in your PATH%RESET%
    echo %WHITE%   Add it to use %APP_NAME% from anywhere%RESET%
)
echo.
goto :eof

:: ============================================================
:: TEST
:: ============================================================
:test
echo.
echo %CYAN%%BOLD%🧪 Running tests...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

%GO% test -v -race -coverprofile=coverage.out ./...
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Tests failed!%RESET%
    exit /b 1
)

:: Generate coverage report
%GO% tool cover -html=coverage.out -o coverage.html
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  Coverage report generation failed%RESET%
) else (
    echo %GREEN%✅ Coverage report generated: coverage.html%RESET%
)

echo %GREEN%✅ All tests passed!%RESET%
echo.
goto :eof

:: ============================================================
:: CLEAN
:: ============================================================
:clean
echo.
echo %CYAN%%BOLD%🧹 Cleaning build artifacts...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

if exist "%BUILD_DIR%" (
    rmdir /S /Q "%BUILD_DIR%"
    echo %GREEN%✅ Removed build directory%RESET%
)

if exist "coverage.out" (
    del /Q "coverage.out"
    echo %GREEN%✅ Removed coverage.out%RESET%
)

if exist "coverage.html" (
    del /Q "coverage.html"
    echo %GREEN%✅ Removed coverage.html%RESET%
)

if exist "%APP_NAME%.exe" (
    del /Q "%APP_NAME%.exe"
    echo %GREEN%✅ Removed local binary%RESET%
)

echo %GREEN%✅ Clean complete!%RESET%
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
    echo %YELLOW%⚠️  golangci-lint not found. Installing...%RESET%
    %GO% install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    if %ERRORLEVEL% NEQ 0 (
        echo %RED%❌ Failed to install golangci-lint%RESET%
        exit /b 1
    )
)

golangci-lint run ./...
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Lint issues found!%RESET%
    exit /b 1
)

echo %GREEN%✅ No lint issues found!%RESET%
echo.
goto :eof

:: ============================================================
:: RUN
:: ============================================================
:run
echo.
echo %CYAN%%BOLD%🚀 Running %APP_NAME%...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%
echo.

%GO% run .\cmd\%APP_NAME% %2 %3 %4 %5 %6 %7 %8 %9
echo.
goto :eof

:: ============================================================
:: BUILD ALL
:: ============================================================
:build-all
echo.
echo %CYAN%%BOLD%🚀 Building for all platforms...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"

echo %WHITE%Building for Windows amd64...%RESET%
set GOOS=windows
set GOARCH=amd64
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%-win64.exe" .\cmd\%APP_NAME%
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Windows amd64 build failed!%RESET%
    exit /b 1
)
echo %GREEN%  ✅ %APP_NAME%-win64.exe%RESET%

echo %WHITE%Building for Windows 386...%RESET%
set GOOS=windows
set GOARCH=386
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%-win32.exe" .\cmd\%APP_NAME%
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Windows 386 build failed!%RESET%
    exit /b 1
)
echo %GREEN%  ✅ %APP_NAME%-win32.exe%RESET%

echo %WHITE%Building for Windows arm64...%RESET%
set GOOS=windows
set GOARCH=arm64
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%-arm64.exe" .\cmd\%APP_NAME%
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%  ⚠️  Windows arm64 build failed (may not be supported)%RESET%
) else (
    echo %GREEN%  ✅ %APP_NAME%-arm64.exe%RESET%
)

echo.
echo %GREEN%✅ All builds complete!%RESET%
echo %WHITE%Output directory: %BUILD_DIR%\%RESET%
dir "%BUILD_DIR%" | find ".exe"
echo.
goto :eof

:: ============================================================
:: RELEASE
:: ============================================================
:release
echo.
echo %CYAN%%BOLD%📦 Creating release...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

:: Check for goreleaser
where goreleaser >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  goreleaser not found. Installing...%RESET%
    %GO% install github.com/goreleaser/goreleaser@latest
    if %ERRORLEVEL% NEQ 0 (
        echo %RED%❌ Failed to install goreleaser%RESET%
        exit /b 1
    )
)

:: Build release
goreleaser release --snapshot --clean
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Release build failed!%RESET%
    exit /b 1
)

echo %GREEN%✅ Release created in dist/ directory%RESET%
echo.
goto :eof

:: ============================================================
:: DEV (watch mode)
:: ============================================================
:dev
echo.
echo %CYAN%%BOLD%👨‍💻 Starting development mode...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

:: Check for air (live reload)
where air >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo %YELLOW%⚠️  air not found. Installing...%RESET%
    %GO% install github.com/cosmtrek/air@latest
    if %ERRORLEVEL% NEQ 0 (
        echo %RED%❌ Failed to install air%RESET%
        echo %YELLOW%Falling back to manual build...%RESET%
        goto :manual-watch
    )
)

echo %GREEN%✅ Watching for changes with air...%RESET%
air
goto :eof

:manual-watch
echo %YELLOW%⚠️  Manual watch mode (Ctrl+C to stop)%RESET%
echo.
:watch-loop
call :build
echo %WHITE%Waiting for changes... (press Ctrl+C to stop)%RESET%
timeout /t 5 /nobreak >nul
goto :watch-loop

:: ============================================================
:: DEPS
:: ============================================================
:deps
echo.
echo %CYAN%%BOLD%📥 Managing dependencies...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

echo %WHITE%Downloading dependencies...%RESET%
%GO% mod download
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Failed to download dependencies%RESET%
    exit /b 1
)

echo %WHITE%Tidying modules...%RESET%
%GO% mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Failed to tidy modules%RESET%
    exit /b 1
)

echo %WHITE%Verifying modules...%RESET%
%GO% mod verify
if %ERRORLEVEL% NEQ 0 (
    echo %RED%❌ Module verification failed%RESET%
    exit /b 1
)

echo %GREEN%✅ Dependencies updated!%RESET%
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
    echo %RED%❌ Formatting failed!%RESET%
    exit /b 1
)

echo %GREEN%✅ Code formatted!%RESET%
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
    echo %RED%❌ vet found issues!%RESET%
    exit /b 1
)

echo %GREEN%✅ No issues found!%RESET%
echo.
goto :eof

:: ============================================================
:: COVERAGE
:: ============================================================
:coverage
echo.
echo %CYAN%%BOLD%📊 Generating coverage report...%RESET%
echo %WHITE%------------------------------------------------------------%RESET%

%GO% test -coverprofile=coverage.out ./...
%GO% tool cover -html=coverage.out -o coverage.html

echo %GREEN%✅ Coverage report generated:%RESET%
echo %WHITE%   coverage.out  - Raw coverage data%RESET%
echo %WHITE%   coverage.html - HTML report%RESET%

:: Open in browser
start coverage.html
echo.
goto :eof

:: ============================================================
:: ALL
:: ============================================================
:all
echo.
echo %CYAN%%BOLD%🔄 Running full pipeline...%RESET%
echo %WHITE%============================================================%RESET%

call :clean
call :deps
call :fmt
call :vet
call :lint
call :test
call :build

echo.
echo %GREEN%%BOLD%✅ All tasks completed successfully!%RESET%
echo.
goto :eof

:: ============================================================
:: HELP
:: ============================================================
:help
echo.
echo %CYAN%%BOLD%🗺️  PathMan Build System%RESET%
echo %WHITE%============================================================%RESET%
echo.
echo %YELLOW%Usage:%RESET% makefile.bat [command]
echo.
echo %MAGENTA%%BOLD%Available Commands:%RESET%
echo.
echo   %GREEN%build%RESET%      🔨 Build the application
echo   %GREEN%install%RESET%    📦 Build and install to GOPATH/bin
echo   %GREEN%test%RESET%       🧪 Run tests with coverage
echo   %GREEN%clean%RESET%      🧹 Remove build artifacts
echo   %GREEN%lint%RESET%       🔍 Run code linter
echo   %GREEN%run%RESET%        🚀 Run the application
echo   %GREEN%build-all%RESET%  🚀 Build for all Windows architectures
echo   %GREEN%release%RESET%    📦 Create a release with goreleaser
echo   %GREEN%dev%RESET%        👨‍💻 Development mode with live reload
echo   %GREEN%deps%RESET%       📥 Update dependencies
echo   %GREEN%fmt%RESET%        🎨 Format code
echo   %GREEN%vet%RESET%        🔍 Run go vet
echo   %GREEN%coverage%RESET%   📊 Generate and view coverage report
echo   %GREEN%all%RESET%        🔄 Run full pipeline (clean,deps,fmt,vet,lint,test,build)
echo   %GREEN%help%RESET%       ❓ Show this help message
echo.
echo %YELLOW%Examples:%RESET%
echo   makefile.bat build
echo   makefile.bat install
echo   makefile.bat all
echo   makefile.bat run -- --help
echo.
echo %WHITE%Note: Use -- to pass flags to the application when using 'run'%RESET%
echo   Example: makefile.bat run -- --scope system path list
echo.
goto :eof

:: ============================================================
:: End of script
:: ============================================================