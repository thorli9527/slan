$ErrorActionPreference = 'Stop'

$processes = Get-Process slan_app -ErrorAction SilentlyContinue
if ($null -ne $processes) {
  $processes | Stop-Process -Force
}

Start-Sleep -Milliseconds 500
