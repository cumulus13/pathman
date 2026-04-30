@echo off
setlocal enabledelayedexpansion

:: PathMan Build Script (Simple)
:: ============================================================

set "APP_NAME=pathman"
set "BUILD_DIR=build"
set "GO=go"
set "GOFLAGS=-trimpath -ldflags="-s -w""

if "%1"=="" goto :build
if "%1"=="help" goto :help
if "%1"=="build" goto :build
if "%1"=="install" goto :install
if "%1"=="test" goto :test
if "%1"=="clean" goto :clean
if "%1"=="lint" goto :lint
if "%1"=="run" goto :run
if "%1"=="build-all" goto :build-all
if "%1"=="all" goto :all
goto :help

:build
echo [BUILD] Building %APP_NAME%...
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%.exe" .\cmd\%APP_NAME%
if %ERRORLEVEL% NEQ 0 (echo [ERROR] Build failed! & exit /b 1)
echo [OK] Build complete: %BUILD_DIR%\%APP_NAME%.exe
goto :eof

:install
echo [INSTALL] Installing %APP_NAME%...
call :build
if not defined GOPATH set "GOPATH=%USERPROFILE%\go"
set "INSTALL_DIR=%GOPATH%\bin"
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /Y "%BUILD_DIR%\%APP_NAME%.exe" "%INSTALL_DIR%\%APP_NAME%.exe" >nul
echo [OK] Installed to: %INSTALL_DIR%\%APP_NAME%.exe
goto :eof

:test
echo [TEST] Running tests...
%GO% test -v -race -coverprofile=coverage.out ./...
%GO% tool cover -html=coverage.out -o coverage.html
echo [OK] Tests complete
goto :eof

:clean
echo [CLEAN] Cleaning...
if exist "%BUILD_DIR%" rmdir /S /Q "%BUILD_DIR%"
if exist "coverage.out" del /Q "coverage.out"
if exist "coverage.html" del /Q "coverage.html"
echo [OK] Clean complete
goto :eof

:lint
echo [LINT] Running linter...
where golangci-lint >nul 2>&1
if %ERRORLEVEL% NEQ 0 %GO% install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
echo [OK] Lint complete
goto :eof

:run
echo [RUN] Running %APP_NAME%...
%GO% run .\cmd\%APP_NAME% %2 %3 %4 %5 %6 %7 %8 %9
goto :eof

:build-all
echo [BUILD] Building for all platforms...
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
set GOOS=windows
set GOARCH=amd64
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%-win64.exe" .\cmd\%APP_NAME%
set GOARCH=386
%GO% build %GOFLAGS% -o "%BUILD_DIR%\%APP_NAME%-win32.exe" .\cmd\%APP_NAME%
echo [OK] All builds complete
goto :eof

:all
echo [ALL] Running full pipeline...
call :clean
call :test
call :build
echo [OK] All tasks complete
goto :eof

:help
echo.
echo PathMan Build System
echo ============================================================
echo.
echo Usage: makefile.bat [command]
echo.
echo Commands:
echo   build      - Build the application
echo   install    - Build and install to GOPATH/bin
echo   test       - Run tests with coverage
echo   clean      - Remove build artifacts
echo   lint       - Run code linter
echo   run        - Run the application
echo   build-all  - Build for all Windows architectures
echo   all        - Run full pipeline (clean, test, build)
echo   help       - Show this help
echo.
goto :eof