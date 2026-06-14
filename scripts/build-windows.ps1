param(
    [string]$Output = "dist\windows\pysentry.exe"
)

$ErrorActionPreference = "Stop"

$go = "${env:ProgramFiles}\Go\bin\go.exe"
if (-not (Test-Path $go)) {
    $go = "go"
}

$msys2Bin = "C:\msys64\ucrt64\bin"
if (Test-Path $msys2Bin) {
    $env:Path = "$msys2Bin;$env:Path"
}

$env:CGO_ENABLED = "1"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

New-Item -ItemType Directory -Force -Path (Split-Path $Output) | Out-Null

$windres = Get-Command windres.exe -ErrorAction SilentlyContinue
if ($windres) {
    & $windres.Source -O coff -o .\cmd\pysentry\rsrc_windows_amd64.syso .\packaging\windows\pysentry.rc
}

& $go build -trimpath -ldflags "-s -w" -o $Output .\cmd\pysentry

Write-Host "Built $Output"
