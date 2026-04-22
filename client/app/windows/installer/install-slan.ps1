param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\SLAN",
  [switch]$NoLaunch
)

$ErrorActionPreference = 'Stop'

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$payloadDir = Join-Path $scriptRoot 'payload'

if (-not (Test-Path $payloadDir)) {
  throw "Missing payload directory: $payloadDir"
}

$running = Get-Process slan_app -ErrorAction SilentlyContinue
if ($running) {
  $running | Stop-Process -Force
  Start-Sleep -Seconds 1
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path (Join-Path $payloadDir '*') -Destination $InstallDir -Recurse -Force

$shortcutPath = Join-Path ([Environment]::GetFolderPath('Desktop')) 'SLAN.lnk'
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = Join-Path $InstallDir 'slan_app.exe'
$shortcut.WorkingDirectory = $InstallDir
$shortcut.IconLocation = Join-Path $InstallDir 'slan_app.exe'
$shortcut.Save()

if (-not $NoLaunch) {
  Start-Process -FilePath (Join-Path $InstallDir 'slan_app.exe') -WorkingDirectory $InstallDir
}

Write-Host "Installed SLAN to $InstallDir"
