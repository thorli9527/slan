$ErrorActionPreference = 'Stop'

$routeText = route print | Out-String
$targetLine = $routeText -split "`r?`n" | Where-Object { $_ -match 'Microsoft KM-TEST' } | Select-Object -First 1
if (-not $targetLine) {
  Write-Host 'KM-TEST adapter not found.'
  exit 0
}

$parts = ($targetLine.Trim() -split '\s+')
$ifIndex = [int]$parts[0]

Get-NetIPAddress -InterfaceIndex $ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue |
  Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue

Disable-NetAdapter -InterfaceDescription 'Microsoft KM-TEST*' -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
netsh interface set interface name="$ifIndex" admin=disabled | Out-Null
Write-Host "Disabled KM-TEST adapter ifIndex=$ifIndex"
