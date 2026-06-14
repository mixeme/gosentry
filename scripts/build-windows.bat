@echo off
setlocal enabledelayedexpansion

set "OUTPUT=%~1"
if "%OUTPUT%"=="" set "OUTPUT=dist\windows\pysentry.exe"

set "GOEXE=%ProgramFiles%\Go\bin\go.exe"
if not exist "%GOEXE%" set "GOEXE=go"

if exist "C:\msys64\ucrt64\bin" set "PATH=C:\msys64\ucrt64\bin;%PATH%"

set "CGO_ENABLED=1"
set "GOOS=windows"
set "GOARCH=amd64"

for %%I in ("%OUTPUT%") do set "OUTDIR=%%~dpI"
if not exist "%OUTDIR%" mkdir "%OUTDIR%"

where windres.exe >nul 2>nul
if %ERRORLEVEL%==0 (
    windres.exe -O coff -o cmd\pysentry\rsrc_windows_amd64.syso packaging\windows\pysentry.rc
)

"%GOEXE%" build -trimpath -ldflags "-s -w" -o "%OUTPUT%" .\cmd\pysentry
if errorlevel 1 exit /b 1

xcopy /E /I /Y assets "%OUTDIR%assets" >nul

echo Built %OUTPUT%
