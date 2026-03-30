@echo off
setlocal enabledelayedexpansion

set BUILD_DIR=bin

:: Get version info
for /f "tokens=*" %%i in ('git describe --tags --always --dirty 2^>nul') do set VERSION=%%i
if not defined VERSION set VERSION=dev

for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=unknown

for /f "tokens=*" %%i in ('powershell -command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ' -AsUTC"') do set BUILD_DATE=%%i
if not defined BUILD_DATE set BUILD_DATE=unknown

set LDFLAGS=-s -w -X 'main.version=%VERSION%' -X 'main.commit=%COMMIT%' -X 'main.buildDate=%BUILD_DATE%'

if "%~1"=="" goto help
goto %~1

:build
    if not exist %BUILD_DIR% mkdir %BUILD_DIR%
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell.exe .
    goto end

:build-linux
    if not exist %BUILD_DIR% mkdir %BUILD_DIR%
    set CGO_ENABLED=0
    set GOOS=linux
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell_linux_amd64 .
    set GOARCH=arm64
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell_linux_arm64 .
    goto end

:build-windows
    if not exist %BUILD_DIR% mkdir %BUILD_DIR%
    set CGO_ENABLED=0
    set GOOS=windows
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell_windows_amd64.exe .
    goto end

:build-darwin
    if not exist %BUILD_DIR% mkdir %BUILD_DIR%
    set CGO_ENABLED=0
    set GOOS=darwin
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell_darwin_amd64 .
    set GOARCH=arm64
    go build -ldflags "%LDFLAGS%" -o %BUILD_DIR%\inkwell_darwin_arm64 .
    goto end

:build-all
    call :build-linux
    call :build-windows
    call :build-darwin
    goto end

:clean
    if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
    go clean
    goto end

:run
    call :build
    %BUILD_DIR%\inkwell.exe
    goto end

:fmt
    go fmt ./...
    goto end

:vet
    go vet ./...
    goto end

:lint
    call :fmt
    call :vet
    goto end

:help
    echo Available commands:
    echo   build          - Build binary (current OS/arch)
    echo   build-linux    - Cross-compile for Linux (amd64 + arm64)
    echo   build-windows  - Cross-compile for Windows (amd64)
    echo   build-darwin   - Cross-compile for macOS (amd64 + arm64)
    echo   build-all      - Cross-compile for all platforms
    echo   clean          - Remove build artifacts
    echo   run            - Build and run
    echo   fmt            - Format code
    echo   vet            - Run go vet
    echo   lint           - Run fmt and vet
    goto end

:end
    endlocal
