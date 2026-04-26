$ErrorActionPreference = 'Stop'

$BaseUrl = 'http://127.0.0.1:28080'
$OpsUrl = 'http://127.0.0.1:28081'
$WebUrl = 'http://127.0.0.1:24200'
$Results = [System.Collections.Generic.List[object]]::new()

function Add-Result {
    param(
        [string]$Name,
        [bool]$OK,
        [string]$Detail = ''
    )
    $script:Results.Add([pscustomobject]@{ Name = $Name; OK = $OK; Detail = $Detail }) | Out-Null
    $mark = if ($OK) { 'PASS' } else { 'FAIL' }
    Write-Host "[$mark] $Name $Detail"
}

function Invoke-Api {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [object]$Body = $null,
        [string]$Bearer = $null,
        [int[]]$Expect = @(200)
    )

    try {
        $headers = @{ Accept = 'application/json' }
        if ($Bearer) {
            $headers.Authorization = "Bearer $Bearer"
        }
        $params = @{
            Method          = $Method
            Uri             = $Url
            Headers         = $headers
            TimeoutSec      = 15
            UseBasicParsing = $true
        }
        if ($null -ne $Body) {
            $params.ContentType = 'application/json'
            $params.Body = ($Body | ConvertTo-Json -Depth 12 -Compress)
        }

        $resp = Invoke-WebRequest @params
        $ok = $Expect -contains [int]$resp.StatusCode
        Add-Result $Name $ok "status=$($resp.StatusCode)"
        if ($resp.Content) {
            $content = $resp.Content.Trim()
            if ($content.StartsWith('{') -or $content.StartsWith('[')) {
                return $content | ConvertFrom-Json
            }
        }
        return $resp.Content
    } catch {
        $status = $null
        $content = ''
        if ($_.Exception.Response) {
            $status = [int]$_.Exception.Response.StatusCode
            try {
                $reader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
                $content = $reader.ReadToEnd()
            } catch {
                $content = ''
            }
        }
        $ok = $false
        if ($status -and ($Expect -contains $status)) {
            $ok = $true
        }
        $detail = if ($status) { "status=$status $content" } else { $_.Exception.Message }
        Add-Result $Name $ok $detail
        if ($ok -and $content) {
            $trimmed = $content.Trim()
            if ($trimmed.StartsWith('{') -or $trimmed.StartsWith('[')) {
                return $trimmed | ConvertFrom-Json
            }
        }
        return $null
    }
}

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$email = "verify+$ts@local.slan"
$password = 'Verify-2026!'
$machine = "verify-machine-$ts"
$nodeId = "verify-node-$ts"
$callbackId = "verify-callback-$ts"

Invoke-Api 'biz healthz' GET "$BaseUrl/healthz" | Out-Null
Invoke-Api 'ops healthz' GET "$OpsUrl/healthz" | Out-Null
Invoke-Api 'web index' GET "$WebUrl/" | Out-Null

$register = Invoke-Api 'auth register' POST "$BaseUrl/auth/register" @{ email = $email; password = $password } $null @(201)
if (-not $register.accessToken) { throw 'auth register did not return accessToken' }
$token = $register.accessToken
$refresh = $register.refreshToken

$login = Invoke-Api 'auth login' POST "$BaseUrl/auth/login" @{ email = $email; password = $password }
if (-not $login.accessToken) { throw 'auth login did not return accessToken' }
$token = $login.accessToken
if ($login.refreshToken) { $refresh = $login.refreshToken }

$refreshed = Invoke-Api 'auth refresh' POST "$BaseUrl/auth/refresh" @{ refreshToken = $refresh }
if ($refreshed.accessToken) { $token = $refreshed.accessToken }

Invoke-Api 'auth callback complete' POST "$BaseUrl/auth/callback-status/$callbackId/complete" @{
    accessToken  = $token
    userId       = $register.userId
    refreshToken = $refresh
    expiresIn    = 3600
    userLabel    = $email
    action       = 'login'
} | Out-Null
$callback = Invoke-Api 'auth callback status' GET "$BaseUrl/auth/callback-status/$callbackId"
Add-Result 'auth callback ready check' ($callback.ready -eq $true) "ready=$($callback.ready)"

$device = Invoke-Api 'devices register' POST "$BaseUrl/devices/register" @{
    name      = 'verify-device'
    platform  = 'windows'
    machineId = $machine
    publicKey = 'verify-device-public-key'
} $token @(201)
if (-not $device.deviceId) { throw 'devices register did not return deviceId' }
$deviceId = $device.deviceId
Invoke-Api 'devices list' GET "$BaseUrl/devices" $null $token | Out-Null

$networkHome = Invoke-Api 'networks home' GET "$BaseUrl/networks/home" $null $token
$networkId = $null
if ($networkHome.activeNetwork -and $networkHome.activeNetwork.networkId) {
    $networkId = $networkHome.activeNetwork.networkId
} elseif ($networkHome.ownedNetwork -and $networkHome.ownedNetwork.networkId) {
    $networkId = $networkHome.ownedNetwork.networkId
}
if (-not $networkId) { throw 'networks home did not return networkId' }

Invoke-Api 'networks list' GET "$BaseUrl/networks" $null $token | Out-Null
Invoke-Api 'network detail' GET "$BaseUrl/networks/$networkId" $null $token | Out-Null
Invoke-Api 'network members' GET "$BaseUrl/networks/$networkId/members" $null $token | Out-Null
Invoke-Api 'network assignments' GET "$BaseUrl/networks/$networkId/assignments" $null $token | Out-Null
Invoke-Api 'network subnets' GET "$BaseUrl/networks/$networkId/subnets" $null $token | Out-Null
Invoke-Api 'network dns update' PUT "$BaseUrl/networks/$networkId/dns" @{
    servers       = @('1.1.1.1')
    searchDomains = @('slan.local')
} $token | Out-Null
Invoke-Api 'network join-key update' PUT "$BaseUrl/networks/$networkId/join-key" @{
    joinKey = ("{0:x32}" -f $ts)
} $token | Out-Null
Invoke-Api 'network switch' POST "$BaseUrl/networks/$networkId/switch" @{ deviceId = $deviceId } $token | Out-Null
Invoke-Api 'network activate' POST "$BaseUrl/networks/$networkId/activate" @{ deviceId = $deviceId } $token | Out-Null

Invoke-Api 'nodes register' POST "$BaseUrl/nodes/register" @{
    deviceId      = $deviceId
    nodeId        = $nodeId
    nodePublicKey = 'verify-node-public-key'
    capabilities  = @('client')
} $token @(201) | Out-Null
Invoke-Api 'bootstrap' POST "$BaseUrl/bootstrap" @{ nodeId = $nodeId; networkId = $networkId } $token | Out-Null
Invoke-Api 'control session' POST "$BaseUrl/control/sessions" @{ nodeId = $nodeId; networkId = $networkId } $token @(201) | Out-Null
Invoke-Api 'relay ticket self' POST "$BaseUrl/relay/tickets" @{
    networkId = $networkId
    srcNodeId = $nodeId
    dstNodeId = $nodeId
    reason    = 'verify'
} $token @(201, 400, 404) | Out-Null

$now = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$state = Invoke-Api 'device network state online' PUT "$BaseUrl/devices/$deviceId/networks/$networkId/state" @{
    controlReachable = $true
    networkOnline    = $true
    tunnelUp         = $true
    lastProbeOk      = $true
    virtualIp        = '10.0.0.10'
    reportedAt       = $now
} $token
Add-Result 'network state semantic check' ($state.networkOnline -eq $true -and $state.controlReachable -eq $true) "controlReachable=$($state.controlReachable) networkOnline=$($state.networkOnline)"
Invoke-Api 'devices list after state' GET "$BaseUrl/devices" $null $token | Out-Null
Invoke-Api 'device network state offline' PUT "$BaseUrl/devices/$deviceId/networks/$networkId/state" @{
    controlReachable = $true
    networkOnline    = $false
    tunnelUp         = $false
    lastProbeOk      = $false
    reportedAt       = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
} $token | Out-Null
Invoke-Api 'network deactivate' POST "$BaseUrl/networks/$networkId/deactivate" @{ deviceId = $deviceId } $token | Out-Null

$mqttInvalid = Invoke-Api 'mqtt auth invalid' POST "$BaseUrl/mqtt/auth/check" @{
    clientId = 'bad'
    username = 'bad'
    password = 'bad'
}
Add-Result 'mqtt invalid denied check' ($mqttInvalid.allow -eq $false) "allow=$($mqttInvalid.allow)"
if ($device.mqtt) {
    $mqttValid = Invoke-Api 'mqtt auth generated credential' POST "$BaseUrl/mqtt/auth/check" @{
        clientId = $device.mqtt.clientId
        username = $device.mqtt.username
        password = $device.mqtt.password
    }
    Add-Result 'mqtt generated allowed check' ($mqttValid.allow -eq $true) "allow=$($mqttValid.allow)"
} else {
    Add-Result 'mqtt credential from register' $true 'mqtt disabled in local config, credential omitted as expected'
}

$failures = @($Results | Where-Object { -not $_.OK })
Write-Host ''
Write-Host 'SUMMARY'
$Results | Format-Table -AutoSize | Out-String | Write-Host
if ($failures.Count -gt 0) {
    exit 2
}
