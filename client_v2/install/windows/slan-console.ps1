param(
  [Parameter(Mandatory=$true)][string]$ServerUrl,
  [string]$Email = '',
  [string]$Password = '',
  [string]$JoinKey = '',
  [string]$Invite = '',
  [string]$DeviceName = '',
  [switch]$EnableNetwork,
  [switch]$RestartService
)

$ErrorActionPreference = 'Stop'

function Get-InviteKey {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return '' }
  if ($Value -match '(?:joinKey|code|key)=([^&#]+)') {
    return [System.Uri]::UnescapeDataString($Matches[1])
  }
  return $Value.Trim()
}

$resolvedJoinKey = if ($JoinKey) { $JoinKey.Trim() } else { Get-InviteKey $Invite }
$configDir = Join-Path $env:ProgramData 'SLAN'
$envPath = Join-Path $configDir 'client-v2-console.env'
New-Item -ItemType Directory -Force -Path $configDir | Out-Null

$lines = @(
  "SLAN_CONTROL_BASE_URL=$ServerUrl",
  "SLAN_PENDING_EMAIL=$Email",
  "SLAN_PENDING_PASSWORD=$Password",
  "SLAN_PENDING_JOIN_KEY=$resolvedJoinKey",
  "SLAN_PENDING_DEVICE_NAME=$DeviceName",
  "SLAN_PENDING_ENABLE_NETWORK=$($EnableNetwork.IsPresent.ToString().ToLowerInvariant())"
)
Set-Content -Path $envPath -Value $lines -Encoding UTF8

if ($RestartService) {
  Restart-Service -Name 'SLANClientV2Service' -ErrorAction Stop
}

Write-Host "Wrote console bootstrap config: $envPath"
Write-Host "serverUrl=$ServerUrl"
if ($resolvedJoinKey) {
  Write-Host 'joinKey=provided'
}
