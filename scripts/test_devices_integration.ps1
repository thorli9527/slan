param(
  [string[]]$TestFiles = @()
)

$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot
$AppDir = Join-Path $RootDir 'client/app'
$CleanupScript = Join-Path $PSScriptRoot 'cleanup_devices_integration.ps1'
$DefaultLogDir = Join-Path $RootDir 'artifacts/devices-integration'
$LogDir = if ([string]::IsNullOrWhiteSpace($env:DEVICES_INTEGRATION_LOG_DIR)) {
  $DefaultLogDir
} else {
  $env:DEVICES_INTEGRATION_LOG_DIR
}

if ($TestFiles.Count -eq 0) {
  $TestFiles = Get-ChildItem -Path (Join-Path $AppDir 'integration_test') -Filter 'devices_*_flow_test.dart' |
    Sort-Object Name |
    ForEach-Object { "integration_test/$($_.Name)" }
}

New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

Push-Location $AppDir
try {
  foreach ($testFile in $TestFiles) {
    & $CleanupScript

    Write-Host "==> flutter test $testFile"
    $logFile = Join-Path $LogDir ("{0}.log" -f [System.IO.Path]::GetFileNameWithoutExtension($testFile))
    & flutter test $testFile 2>&1 | Tee-Object -FilePath $logFile
    if ($LASTEXITCODE -ne 0) {
      throw "flutter test failed for $testFile"
    }

    & $CleanupScript
  }
}
finally {
  Pop-Location
}
