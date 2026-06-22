@echo off
setlocal enabledelayedexpansion
REM GoSentry test runner
REM Runs go vet and go test with race detection

REM Move to repository root
cd /d "%~dp0\.."

REM Fyne uses native libraries through CGO. MSYS2 UCRT64 provides the GCC toolchain
REM expected by the Windows build; prepending it keeps the script self-contained
REM without permanently changing the user's system PATH.
if exist "C:\msys64\ucrt64\bin" set "PATH=C:\msys64\ucrt64\bin;%PATH%"

REM Race detector requires CGO
set "CGO_ENABLED=1"

echo Running go vet...
go vet ./...
if errorlevel 1 (
    echo.
    echo ✗ go vet failed
    exit /b 1
)

echo.
echo Running go test with race detection...
go test -race ./...
if errorlevel 1 (
    echo.
    echo ✗ go test failed
    exit /b 1
)

echo.
echo ✓ All tests passed
