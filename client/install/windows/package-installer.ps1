param(
  [string]$ReleaseDir = "$PSScriptRoot\..\..\app_flutter\build\windows\x64\runner\Release",
  [string]$OutputDir = "$PSScriptRoot\..\..\.tmp\installer\slan-client-v2-windows"
)

$ErrorActionPreference = 'Stop'

Import-Module (Join-Path $PSScriptRoot 'SlanWindowsInstall.psm1') -Force
$manifest = Get-SlanWindowsInstallManifest

$releasePath = (Resolve-Path $ReleaseDir).Path
Assert-SlanWindowsReleaseRuntime -ReleasePath $releasePath -Manifest $manifest
$outputPath = [System.IO.Path]::GetFullPath($OutputDir)
$zipPath = "$outputPath.zip"
$setupExePath = Join-Path $outputPath 'SLAN-Client-V2-Setup.exe'
$issTemplatePath = Join-Path $PSScriptRoot 'SLAN-Client-V2.iss'
$generatedIssPath = Join-Path $outputPath 'SLAN-Client-V2.generated.iss'
$isccPath = Resolve-SlanWindowsIsccPath

if (Test-Path $outputPath) {
  Remove-Item -Recurse -Force $outputPath
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
Copy-Item -Path (Join-Path $releasePath '*') -Destination $outputPath -Recurse -Force
Remove-Item -Path (Join-Path $outputPath 'client-core-helper.exe') -Force -ErrorAction SilentlyContinue
Copy-SlanWindowsPackagedTools -StageDir $outputPath -Manifest $manifest
New-SlanWindowsInnoSetupScript `
  -TemplatePath $issTemplatePath `
  -StageDir $outputPath `
  -GeneratedPath $generatedIssPath `
  -Manifest $manifest

if (Test-Path $zipPath) {
  Remove-Item -Force $zipPath
}
Compress-Archive -Path (Join-Path $outputPath '*') -DestinationPath $zipPath -Force

if ($isccPath) {
  & $isccPath $generatedIssPath | Out-Null
  if (-not (Test-Path $setupExePath)) {
    throw "Failed to create native installer executable: $setupExePath"
  }
} else {
  Write-Warning 'Inno Setup (ISCC.exe) was not found. Generated a native installer stage, but did not compile SLAN-Client-V2-Setup.exe.'
}

Write-Host "Installer directory: $outputPath"
Write-Host "Installer archive: $zipPath"
if (Test-Path $setupExePath) {
  Write-Host "Installer executable: $setupExePath"
} else {
  Write-Host "Installer script: $generatedIssPath"
}
