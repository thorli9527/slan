param(
  [string]$ReleaseDir = "$PSScriptRoot\..\client\app_flutter\build\windows\x64\runner\Release",
  [string]$OutputDir = "$PSScriptRoot\..\client\.tmp\installer\slan-client-v2-windows",
  [switch]$Build,
  [switch]$DebugBuild
)

$ErrorActionPreference = 'Stop'

if ($DebugBuild -and -not $MyInvocation.BoundParameters.ContainsKey('ReleaseDir')) {
  $ReleaseDir = "$PSScriptRoot\..\client\app_flutter\build\windows\x64\runner\Debug"
}

$RootDir = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$PackageScript = Join-Path $RootDir 'client\install\windows\package-installer.ps1'
$ReleasePath = [System.IO.Path]::GetFullPath($ReleaseDir)
$OutputPath = [System.IO.Path]::GetFullPath($OutputDir)
$ZipPath = "$OutputPath.zip"
$SetupExePath = Join-Path $OutputPath 'SLAN-Client-V2-Setup.exe'
$GeneratedIssPath = Join-Path $OutputPath 'SLAN-Client-V2.generated.iss'

function Invoke-LoggedCommand {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [Parameter(Mandatory = $true)][string[]]$Arguments,
    [string]$WorkingDirectory = $RootDir
  )

  Write-Host "==> $FilePath $($Arguments -join ' ')"
  $process = Start-Process `
    -FilePath $FilePath `
    -ArgumentList $Arguments `
    -WorkingDirectory $WorkingDirectory `
    -NoNewWindow `
    -Wait `
    -PassThru
  if ($process.ExitCode -ne 0) {
    throw "Command failed with exit code $($process.ExitCode): $FilePath"
  }
}

function Assert-ZipEntry {
  param(
    [Parameter(Mandatory = $true)]$Entries,
    [Parameter(Mandatory = $true)][string]$Name
  )

  $normalized = $Name.Replace('\', '/')
  if (-not ($Entries | Where-Object { $_.FullName.Replace('\', '/') -eq $normalized })) {
    throw "Missing Windows installer archive entry: $Name"
  }
}

if (-not (Test-Path -LiteralPath $PackageScript -PathType Leaf)) {
  throw "Missing Windows package implementation: $PackageScript"
}

if ($Build) {
  if ($DebugBuild) {
    Invoke-LoggedCommand -FilePath 'cargo' -Arguments @('build', '-p', 'client-core-service') -WorkingDirectory (Join-Path $RootDir 'client\rust')
    Invoke-LoggedCommand -FilePath 'flutter' -Arguments @('build', 'windows', '--debug') -WorkingDirectory (Join-Path $RootDir 'client\app_flutter')
    $serviceSource = Join-Path $RootDir 'client\rust\target\debug\client-core-service.exe'
  } else {
    Invoke-LoggedCommand -FilePath 'cargo' -Arguments @('build', '-p', 'client-core-service', '--release') -WorkingDirectory (Join-Path $RootDir 'client\rust')
    Invoke-LoggedCommand -FilePath 'flutter' -Arguments @('build', 'windows') -WorkingDirectory (Join-Path $RootDir 'client\app_flutter')
    $serviceSource = Join-Path $RootDir 'client\rust\target\release\client-core-service.exe'
  }

  $serviceDestination = Join-Path $ReleasePath 'client-core-service.exe'
  if (-not (Test-Path -LiteralPath $serviceSource -PathType Leaf)) {
    throw "Missing built Windows service binary: $serviceSource"
  }
  Copy-Item -LiteralPath $serviceSource -Destination $serviceDestination -Force
}

& $PackageScript -ReleaseDir $ReleasePath -OutputDir $OutputPath

if (-not (Test-Path -LiteralPath $ZipPath -PathType Leaf)) {
  throw "Missing Windows installer archive: $ZipPath"
}

Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
try {
  Assert-ZipEntry -Entries $zip.Entries -Name 'slan_client_v2.exe'
  Assert-ZipEntry -Entries $zip.Entries -Name 'client-core-service.exe'
  Assert-ZipEntry -Entries $zip.Entries -Name 'wintun.dll'
  Assert-ZipEntry -Entries $zip.Entries -Name 'flutter_windows.dll'
  Assert-ZipEntry -Entries $zip.Entries -Name 'client_core_plugin_plugin.dll'
  Assert-ZipEntry -Entries $zip.Entries -Name 'tools/SlanWindowsInstall.psm1'
  Assert-ZipEntry -Entries $zip.Entries -Name 'tools/slan-console.ps1'
  Assert-ZipEntry -Entries $zip.Entries -Name 'tools/upload-installer-log.ps1'
} finally {
  $zip.Dispose()
}

Write-Host "windowsPackageDir: $OutputPath"
Write-Host "windowsPackageZip: $ZipPath"
if (Test-Path -LiteralPath $SetupExePath -PathType Leaf) {
  Write-Host "windowsInstallerExe: $SetupExePath"
} else {
  Write-Host "windowsInstallerExe: skipped; install Inno Setup 6 to compile $GeneratedIssPath"
  Write-Host "windowsInstallerScript: $GeneratedIssPath"
}
