@echo off
setlocal enabledelayedexpansion

REM Double-clicking a .bat file can start it with an arbitrary working
REM directory. Move to the repository root (the parent of scripts\) before using
REM relative paths such as .\cmd\gosentry and packaging\windows\gosentry.rc.
cd /d "%~dp0\.."

for /f "tokens=4" %%V in ('findstr /C:"var Version" src\app\version.go') do set "VERSION=%%~V"
if "%VERSION%"=="" set "VERSION=0.0.0-dev"
set "VERSION=%VERSION:"=%"

REM Optional first argument allows CI or a developer to choose another output
REM path. The default keeps all generated binaries under dist\ so the source tree
REM stays clean and the old bin\ folder is no longer needed.
set "OUTPUT=%~1"
if "%OUTPUT%"=="" set "OUTPUT=dist\windows\gosentry-%VERSION%-windows-amd64.exe"

REM Prefer the standard Go installer path on Windows, but fall back to PATH for
REM machines where Go was installed by another package manager.
set "GOEXE=%ProgramFiles%\Go\bin\go.exe"
if not exist "%GOEXE%" set "GOEXE=go"

REM Fyne uses native libraries through CGO. MSYS2 UCRT64 provides the GCC toolchain
REM expected by the Windows build; prepending it keeps the script self-contained
REM without permanently changing the user's system PATH.
if exist "C:\msys64\ucrt64\bin" set "PATH=C:\msys64\ucrt64\bin;%PATH%"

REM Build a 64-bit Windows binary. CGO must stay enabled for Fyne; disabling it
REM would make the native GUI backend fail to compile.
set "CGO_ENABLED=1"
set "GOOS=windows"
set "GOARCH=amd64"

REM Create the target directory before invoking Go so custom output paths work.
for %%I in ("%OUTPUT%") do set "OUTDIR=%%~dpI"
if not exist "%OUTDIR%" mkdir "%OUTDIR%"

REM windres embeds the .ico file into the PE executable so Windows Explorer,
REM shortcuts, and the taskbar can show the GoSentry icon. The Go embed package
REM handles Fyne's runtime icon, but Explorer reads this Windows resource instead.
where windres.exe >nul 2>nul
if %ERRORLEVEL%==0 (
    windres.exe -O coff -o cmd\gosentry\rsrc_windows_amd64.syso packaging\windows\gosentry.rc
)

REM -trimpath removes local machine paths from the binary, -s -w reduce binary
REM size, and -H=windowsgui prevents a separate console window from opening when
REM the GUI app starts from Explorer or a shortcut.
"%GOEXE%" build -trimpath -ldflags "-s -w -H=windowsgui -X gitea.mixdep.ru/mix/gosentry/src/app.Version=%VERSION%" -o "%OUTPUT%" .\cmd\gosentry
if errorlevel 1 exit /b 1

REM Icons are embedded into the executable, so no assets directory is copied next
REM to the binary. Runtime YAML and log files are created by the app itself.
echo Built %OUTPUT%
