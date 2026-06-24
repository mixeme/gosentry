@echo off
setlocal enabledelayedexpansion

REM Double-clicking a .bat file can start it with an arbitrary working directory.
REM Move to the repository root before using relative paths.
cd /d "%~dp0\.."

for /f "tokens=4" %%V in ('findstr /C:"var Version" src\app\version.go') do set "VERSION=%%~V"
if "%VERSION%"=="" set "VERSION=0.0.0-dev"
set "VERSION=%VERSION:"=%"

set "ARCH=windows-amd64"
set "EXE_PATH=dist\windows\gosentry-%VERSION%-%ARCH%.exe"
set "ZIP_PATH=dist\windows\gosentry-%VERSION%-%ARCH%.zip"
set "STAGING=dist\windows\_staging-%VERSION%-%ARCH%"

REM Build the Windows executable via the shared build script.
call scripts\build-windows.bat "%EXE_PATH%"
if errorlevel 1 exit /b 1

REM Assemble portable bundle in a staging directory.
if exist "%STAGING%" rmdir /s /q "%STAGING%"
mkdir "%STAGING%"

copy "%EXE_PATH%" "%STAGING%\gosentry.exe" >nul
copy README.md "%STAGING%\README.md" >nul
copy docs\CHANGELOG.md "%STAGING%\CHANGELOG.md" >nul

REM Remove any previous zip so Compress-Archive does not append to it.
if exist "%ZIP_PATH%" del /f "%ZIP_PATH%"

REM Compress the staging contents. Each file lands at the root of the zip so
REM the user can extract directly into any folder and run gosentry.exe.
powershell -NoProfile -Command "Compress-Archive -Path '%STAGING%\*' -DestinationPath '%ZIP_PATH%'"
if errorlevel 1 (
    echo Compress-Archive failed. Ensure PowerShell 5+ is available.
    exit /b 1
)

rmdir /s /q "%STAGING%"
echo Packaged %ZIP_PATH%
