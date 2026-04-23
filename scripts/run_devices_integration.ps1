param(
  [string[]]$TestFiles = @()
)

$ErrorActionPreference = 'Stop'

$CleanupScript = Join-Path $PSScriptRoot 'cleanup_devices_integration.ps1'
$TestScript = Join-Path $PSScriptRoot 'test_devices_integration.ps1'

& $CleanupScript
try {
  & $TestScript @TestFiles
}
finally {
  & $CleanupScript
}
