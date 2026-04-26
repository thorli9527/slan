param(
    [switch]$ExpectMqttCredential,
    [switch]$VerifyMqttBroker
)

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

function ConvertTo-MqttRemainingLength {
    param([int]$Length)

    $bytes = [System.Collections.Generic.List[byte]]::new()
    do {
        $encoded = $Length % 128
        $Length = [math]::Floor($Length / 128)
        if ($Length -gt 0) {
            $encoded = $encoded -bor 128
        }
        $bytes.Add([byte]$encoded) | Out-Null
    } while ($Length -gt 0)
    return $bytes.ToArray()
}

function Write-MqttString {
    param(
        [System.IO.MemoryStream]$Stream,
        [string]$Value
    )

    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Value)
    $Stream.WriteByte([byte](($bytes.Length -shr 8) -band 0xff))
    $Stream.WriteByte([byte]($bytes.Length -band 0xff))
    $Stream.Write($bytes, 0, $bytes.Length)
}

function New-MqttConnectPacket {
    param(
        [string]$ClientId,
        [string]$Username,
        [string]$Password
    )

    $variable = [System.IO.MemoryStream]::new()
    Write-MqttString $variable 'MQTT'
    $variable.WriteByte(4)
    $variable.WriteByte(0xc2)
    $variable.WriteByte(0)
    $variable.WriteByte(30)

    $payload = [System.IO.MemoryStream]::new()
    Write-MqttString $payload $ClientId
    Write-MqttString $payload $Username
    Write-MqttString $payload $Password

    $body = [byte[]]($variable.ToArray() + $payload.ToArray())
    return [byte[]](@(0x10) + (ConvertTo-MqttRemainingLength $body.Length) + $body)
}

function New-MqttPublishPacket {
    param(
        [string]$Topic,
        [string]$Payload
    )

    $bodyStream = [System.IO.MemoryStream]::new()
    Write-MqttString $bodyStream $Topic
    $payloadBytes = [System.Text.Encoding]::UTF8.GetBytes($Payload)
    $bodyStream.Write($payloadBytes, 0, $payloadBytes.Length)
    $body = $bodyStream.ToArray()
    return [byte[]](@(0x30) + (ConvertTo-MqttRemainingLength $body.Length) + $body)
}

function Send-MqttPublish {
    param(
        [string]$BrokerUrl,
        [string]$ClientId,
        [string]$Username,
        [string]$Password,
        [string]$Topic,
        [string]$Payload
    )

    $uri = [Uri]$BrokerUrl
    $hostName = $uri.Host
    $port = if ($uri.Port -gt 0) { $uri.Port } else { 1883 }
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $connectTask = $client.ConnectAsync($hostName, $port)
        if (-not $connectTask.Wait([TimeSpan]::FromSeconds(10))) {
            throw "timed out connecting to MQTT broker $hostName`:$port"
        }
        $stream = $client.GetStream()
        $stream.WriteTimeout = 10000
        $stream.ReadTimeout = 10000

        $connect = New-MqttConnectPacket $ClientId $Username $Password
        $stream.Write($connect, 0, $connect.Length)
        $connAck = New-Object byte[] 4
        $read = $stream.Read($connAck, 0, $connAck.Length)
        if ($read -ne 4 -or $connAck[0] -ne 0x20 -or $connAck[1] -ne 0x02 -or $connAck[3] -ne 0x00) {
            throw "MQTT CONNACK rejected or malformed: $([BitConverter]::ToString($connAck))"
        }

        $publish = New-MqttPublishPacket $Topic $Payload
        $stream.Write($publish, 0, $publish.Length)
    } finally {
        $client.Close()
    }
}

function Find-Device {
    param(
        [object]$DevicesResponse,
        [string]$DeviceId
    )

    if ($null -eq $DevicesResponse -or $null -eq $DevicesResponse.items) {
        return $null
    }
    foreach ($item in $DevicesResponse.items) {
        if ($item.deviceId -eq $DeviceId) {
            return $item
        }
    }
    return $null
}

function Wait-MqttNetworkState {
    param(
        [string]$DeviceId,
        [string]$NetworkId,
        [string]$Bearer
    )

    for ($attempt = 0; $attempt -lt 10; $attempt++) {
        Start-Sleep -Milliseconds 300
        $devices = Invoke-Api 'devices list after mqtt publish' GET "$BaseUrl/devices" $null $Bearer
        $publishedDevice = Find-Device $devices $DeviceId
        if ($publishedDevice -and
            $publishedDevice.networkState -and
            $publishedDevice.networkState.networkId -eq $NetworkId -and
            $publishedDevice.networkState.controlReachable -eq $true -and
            $publishedDevice.networkState.networkOnline -eq $true -and
            $publishedDevice.networkState.tunnelUp -eq $true) {
            return $true
        }
    }
    return $false
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

    if ($VerifyMqttBroker) {
        $mqttPayload = @{
            controlReachable = $true
            networkOnline    = $true
            tunnelUp         = $true
            lastProbeOk      = $true
            virtualIp        = '10.0.0.10'
            reportedAt       = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
        } | ConvertTo-Json -Depth 12 -Compress
        $mqttTopic = "$($device.mqtt.topicPrefix)/networks/$networkId/state"
        try {
            Send-MqttPublish $device.mqtt.brokerUrl $device.mqtt.clientId $device.mqtt.username $device.mqtt.password $mqttTopic $mqttPayload
            Add-Result 'mqtt broker publish' $true "topic=$mqttTopic"
            $mqttPersisted = Wait-MqttNetworkState $deviceId $networkId $token
            Add-Result 'mqtt subscriber persisted state' $mqttPersisted "deviceId=$deviceId networkId=$networkId"
        } catch {
            Add-Result 'mqtt broker publish' $false $_.Exception.Message
        }
    }
} else {
    Add-Result 'mqtt credential from register' (-not $ExpectMqttCredential) 'mqtt disabled in local config, credential omitted as expected'
}

Invoke-Api 'device network state offline' PUT "$BaseUrl/devices/$deviceId/networks/$networkId/state" @{
    controlReachable = $true
    networkOnline    = $false
    tunnelUp         = $false
    lastProbeOk      = $false
    reportedAt       = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
} $token | Out-Null
Invoke-Api 'network deactivate' POST "$BaseUrl/networks/$networkId/deactivate" @{ deviceId = $deviceId } $token | Out-Null

$failures = @($Results | Where-Object { -not $_.OK })
Write-Host ''
Write-Host 'SUMMARY'
$Results | Format-Table -AutoSize | Out-String | Write-Host
if ($failures.Count -gt 0) {
    exit 2
}
