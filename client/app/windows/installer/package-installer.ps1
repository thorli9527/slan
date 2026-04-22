param(
  [string]$ReleaseDir = "$PSScriptRoot\..\..\build\windows\x64\runner\Release",
  [string]$OutputDir = "$PSScriptRoot\..\..\..\.tmp\installer\slan-windows-installer"
)

$ErrorActionPreference = 'Stop'

$releasePath = (Resolve-Path $ReleaseDir).Path
$outputPath = [System.IO.Path]::GetFullPath($OutputDir)
$zipPath = "$outputPath.zip"

if (Test-Path $outputPath) {
  Remove-Item -Recurse -Force $outputPath
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $outputPath 'payload') | Out-Null

Copy-Item -Path (Join-Path $releasePath '*') -Destination (Join-Path $outputPath 'payload') -Recurse -Force
Copy-Item -Path (Join-Path $PSScriptRoot 'install-slan.ps1') -Destination $outputPath -Force
Copy-Item -Path (Join-Path $PSScriptRoot 'install-slan.cmd') -Destination $outputPath -Force

if (Test-Path $zipPath) {
  Remove-Item -Force $zipPath
}
Compress-Archive -Path (Join-Path $outputPath '*') -DestinationPath $zipPath -Force

Write-Host "Installer directory: $outputPath"
Write-Host "Installer archive: $zipPath"
