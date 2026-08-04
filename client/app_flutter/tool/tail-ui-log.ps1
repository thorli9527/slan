param(
  [int]$Tail = 120
)

$ErrorActionPreference = "Stop"
$logPath = Join-Path $env:ProgramData "SLAN\client-v2-ui.log"
if (-not (Test-Path $logPath)) {
  Write-Host "UI diagnostics log not found: $logPath"
  exit 0
}

Get-Content $logPath -Tail $Tail
