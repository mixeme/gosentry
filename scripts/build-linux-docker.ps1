param(
    [string]$Output = "dist\linux\pysentry"
)

$ErrorActionPreference = "Stop"

docker build -f Dockerfile.linux -t pysentry-linux-builder .
$containerId = docker create pysentry-linux-builder
New-Item -ItemType Directory -Force -Path (Split-Path $Output) | Out-Null
docker cp "${containerId}:/out/pysentry" $Output
docker rm $containerId | Out-Null

Write-Host "Built $Output"
