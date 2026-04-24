param(
  [string]$RuntimeDir = 'D:\workspace\slan\slan\client\app\build\windows\x64\runner\Release',
  [string]$LogPath = 'C:\ProgramData\SLAN\app-core-service.log'
)

$ErrorActionPreference = 'Stop'

function Write-Section {
  param([string]$Title)
  Write-Host ""
  Write-Host "=== $Title ==="
}

function Show-RelevantAdapters {
  Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
    Where-Object {
      $_.Name -like 'SLAN*' -or
      $_.InterfaceDescription -like '*Wintun*' -or
      $_.InterfaceDescription -like '*WireGuard*' -or
      $_.InterfaceDescription -like '*KM-TEST*'
    } |
    Sort-Object -Property ifIndex |
    Format-Table Name, InterfaceDescription, Status, ifIndex -Auto
}

$resolvedRuntimeDir = [System.IO.Path]::GetFullPath($RuntimeDir)
$servicePath = Join-Path $resolvedRuntimeDir 'app-core-service.exe'
$wintunDllPath = Join-Path $resolvedRuntimeDir 'wintun.dll'

if (-not (Test-Path $servicePath)) {
  throw "Missing runtime service binary: $servicePath"
}
if (-not (Test-Path $wintunDllPath)) {
  throw "Missing wintun.dll beside runtime: $wintunDllPath"
}

Write-Section "Runtime"
Write-Host "RuntimeDir: $resolvedRuntimeDir"
Write-Host "ServicePath: $servicePath"
Write-Host "WintunDll: $wintunDllPath"

Write-Section "Adapters Before"
Show-RelevantAdapters

Write-Section "Prepare Adapter"
& $servicePath --driver wintun --prepare-adapter
$exitCode = $LASTEXITCODE
Write-Host "PrepareExitCode: $exitCode"

Write-Section "Adapters After"
Show-RelevantAdapters

Write-Section "Service Log Tail"
if (Test-Path $LogPath) {
  Get-Content $LogPath -Tail 60
} else {
  Write-Host "Service log not found: $LogPath"
}

exit $exitCode
