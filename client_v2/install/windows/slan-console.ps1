param(
  [string]$ServerUrl = 'http://47.245.40.231:28080',
  [string]$Email = '',
  [string]$Password = '',
  [string]$DeviceName = '',
  [switch]$EnableNetwork,
  [switch]$RestartService
)

$ErrorActionPreference = 'Stop'

$configDir = Join-Path $env:ProgramData 'SLAN'
$envPath = Join-Path $configDir 'client-v2-console.env'
New-Item -ItemType Directory -Force -Path $configDir | Out-Null

$lines = @(
  "SLAN_CONTROL_BASE_URL=$ServerUrl",
  "SLAN_PENDING_EMAIL=$Email",
  "SLAN_PENDING_PASSWORD=$Password",
  "SLAN_PENDING_DEVICE_NAME=$DeviceName",
  "SLAN_PENDING_ENABLE_NETWORK=$($EnableNetwork.IsPresent.ToString().ToLowerInvariant())"
)
Set-Content -Path $envPath -Value $lines -Encoding UTF8

if ($RestartService) {
  Restart-Service -Name 'SLANClientV2Service' -ErrorAction Stop
}

Write-Host "Wrote console bootstrap config: $envPath"
Write-Host "serverUrl=$ServerUrl"
