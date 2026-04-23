$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot
$EnvFile = Join-Path $RootDir '.env.local'
$ComposeFile = Join-Path $RootDir 'docker-compose.local.yml'

if (-not (Test-Path $EnvFile)) {
  throw ".env.local not found: $EnvFile"
}

function Get-EnvValue {
  param(
    [string]$Path,
    [string]$Key,
    [string]$DefaultValue
  )

  $match = Get-Content $Path |
    Where-Object { $_ -match "^\s*$Key=(.*)$" } |
    Select-Object -First 1
  if (-not $match) {
    return $DefaultValue
  }

  $value = ($match -replace "^\s*$Key=", '').Trim()
  if ($value.Length -eq 0) {
    return $DefaultValue
  }
  return $value
}

function Require-NonEmpty {
  param(
    [string]$Name,
    [AllowEmptyString()]
    [string]$Value
  )

  if ([string]::IsNullOrWhiteSpace($Value)) {
    throw "missing required field: $Name"
  }
}

function Invoke-SmokeRequest {
  param(
    [string]$Method,
    [string]$Uri,
    [hashtable]$Body,
    [string]$Token
  )

  $headers = @{}
  if (-not [string]::IsNullOrWhiteSpace($Token)) {
    $headers['Authorization'] = "Bearer $Token"
  }

  $params = @{
    Method      = $Method
    Uri         = $Uri
    Headers     = $headers
    ContentType = 'application/json'
  }

  if ($Body) {
    $params['Body'] = ($Body | ConvertTo-Json -Depth 10 -Compress)
  }

  return Invoke-RestMethod @params
}

$skipProtocolCheck = if ($null -eq $env:SKIP_PROTOCOL_CONTRACT_CHECK -or $env:SKIP_PROTOCOL_CONTRACT_CHECK -eq '') {
  '0'
} else {
  $env:SKIP_PROTOCOL_CONTRACT_CHECK
}

if ($skipProtocolCheck -ne '1') {
  & powershell -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'check_protocol_contracts.ps1')
}

$BizPort = Get-EnvValue -Path $EnvFile -Key 'SLAN_BIZ_PUBLIC_PORT' -DefaultValue '28080'
$BizBase = "http://127.0.0.1:$BizPort"
$Stamp = [guid]::NewGuid().ToString('N').Substring(0, 12)

$Email = if ($env:SMOKE_EMAIL) { $env:SMOKE_EMAIL } else { "e2e-user-$Stamp@local.slan" }
$Password = if ($env:SMOKE_PASSWORD) { $env:SMOKE_PASSWORD } else { 'e2e-user-password-2026' }
$MachineId = if ($env:SMOKE_MACHINE_ID) { $env:SMOKE_MACHINE_ID } else { "machine-e2e-$Stamp" }
$NodeId = if ($env:SMOKE_NODE_ID) { $env:SMOKE_NODE_ID } else { "node-e2e-$Stamp" }

$composeArgs = @('--env-file', $EnvFile, '-f', $ComposeFile, 'ps')
docker compose @composeArgs | Out-Null
if ($LASTEXITCODE -ne 0) {
  throw "docker compose ps failed; make sure Docker Desktop is running and the current shell can access the docker engine"
}

$register = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/auth/register" -Body @{
  email    = $Email
  password = $Password
} -Token ''
$token = [string]$register.accessToken
$userId = [string]$register.userId
Require-NonEmpty -Name 'accessToken' -Value $token
Require-NonEmpty -Name 'userId' -Value $userId

$device = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/devices/register" -Body @{
  name      = 'Windows E2E'
  platform  = 'windows'
  machineId = $MachineId
  publicKey = 'pub-device-e2e'
} -Token $token
$deviceId = [string]$device.deviceId
Require-NonEmpty -Name 'deviceId' -Value $deviceId

$homeBefore = Invoke-SmokeRequest -Method 'GET' -Uri "$BizBase/networks/home" -Body $null -Token $token
$network = if ($null -ne $homeBefore.ownedNetwork) { $homeBefore.ownedNetwork } else {
  Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/networks" -Body @{
    name        = 'Local E2E Net'
    description = 'small-scale rollout check'
    cidr        = '100.96.0.0/24'
  } -Token $token
}
$networkId = [string]$network.networkId
Require-NonEmpty -Name 'networkId' -Value $networkId

$node = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/nodes/register" -Body @{
  deviceId      = $deviceId
  nodeId        = $NodeId
  nodePublicKey = 'node-pub-e2e'
  capabilities  = @('desktop')
} -Token $token
$registeredNodeId = [string]$node.nodeId
Require-NonEmpty -Name 'nodeId' -Value $registeredNodeId

$join = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/networks/$networkId/join" -Body @{
  deviceId = $deviceId
} -Token $token
$joinMemberId = [string]$join.member.memberId
Require-NonEmpty -Name 'join.memberId' -Value $joinMemberId

$activate = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/networks/$networkId/activate" -Body @{
  deviceId = $deviceId
} -Token $token
$activateMemberId = [string]$activate.member.memberId
$activateAttachmentId = [string]$activate.attachment.attachmentId
Require-NonEmpty -Name 'activate.memberId' -Value $activateMemberId
Require-NonEmpty -Name 'activate.attachmentId' -Value $activateAttachmentId

$control = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/control/sessions" -Body @{
  nodeId    = $NodeId
  networkId = $networkId
} -Token $token
$controlSessionId = [string]$control.controlSessionId
Require-NonEmpty -Name 'controlSessionId' -Value $controlSessionId

$bootstrap = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/bootstrap" -Body @{
  nodeId    = $NodeId
  networkId = $networkId
} -Token $token
$bootstrapNetworkId = [string]$bootstrap.networkMap.networkId
$bootstrapWsUrl = [string]$bootstrap.controlPlane.wsUrl
Require-NonEmpty -Name 'bootstrap.networkMap.networkId' -Value $bootstrapNetworkId
Require-NonEmpty -Name 'bootstrap.controlPlane.wsUrl' -Value $bootstrapWsUrl

$ticket = Invoke-SmokeRequest -Method 'POST' -Uri "$BizBase/relay/tickets" -Body @{
  networkId = $networkId
  srcNodeId = $NodeId
  dstNodeId = $NodeId
  reason    = 'e2e-smoke'
} -Token $token
$ticketId = [string]$ticket.ticketId
$ticketRelayUrl = [string]$ticket.relayUrl
Require-NonEmpty -Name 'ticketId' -Value $ticketId
Require-NonEmpty -Name 'relayUrl' -Value $ticketRelayUrl

$networkHome = Invoke-SmokeRequest -Method 'GET' -Uri "$BizBase/networks/home" -Body $null -Token $token
if (-not $networkHome.hasNetwork) {
  throw "expected hasNetwork=true after join, got: $($networkHome | ConvertTo-Json -Depth 10 -Compress)"
}

Write-Output "REGISTER=$($register | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "USER_ID=$userId"
Write-Output "DEVICE=$($device | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "NETWORK=$($network | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "NODE=$($node | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "JOIN=$($join | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "ACTIVATE=$($activate | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "CONTROL=$($control | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "BOOTSTRAP=$($bootstrap | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "TICKET=$($ticket | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "HOME=$($networkHome | ConvertTo-Json -Depth 10 -Compress)"
Write-Output "SMOKE_OK network=$networkId device=$deviceId node=$registeredNodeId session=$controlSessionId ticket=$ticketId"
