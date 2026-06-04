param(
  [string]$ServerUrl = '',
  [string]$Email = '',
  [string]$Password = '',
  [string]$DeviceName = '',
  [switch]$EnableNetwork,
  [switch]$RestartService
)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'SlanWindowsInstall.psm1') -Force
$manifest = Get-SlanWindowsInstallManifest
if ([string]::IsNullOrWhiteSpace($ServerUrl)) {
  $ServerUrl = $manifest.DefaultControlBaseUrl
}

$configDir = $manifest.ProgramDataDir
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
  Restart-Service -Name $manifest.ServiceName -ErrorAction Stop
}

Write-Host "Wrote console bootstrap config: $envPath"
Write-Host "serverUrl=$ServerUrl"
