param(
  [string]$ServiceHost = $(if ($env:SLAN_CLIENT_CORE_SERVICE_HOST) { $env:SLAN_CLIENT_CORE_SERVICE_HOST } else { "127.0.0.1:46392" }),
  [int]$MinRelaySessions = 1,
  [switch]$AllowMissingPeerSessions,
  [switch]$RequireDerpCandidate,
  [switch]$RequireDirectUdpActive,
  [switch]$RequireMtuOk,
  [switch]$RequireMssOk,
  [switch]$ExportOnFailure
)

$ErrorActionPreference = "Stop"

function Invoke-SlanService {
  param([string]$Method)

  $separator = $ServiceHost.LastIndexOf(':')
  if ($separator -le 0 -or $separator -eq ($ServiceHost.Length - 1)) {
    throw "Invalid service host: $ServiceHost"
  }
  $hostname = $ServiceHost.Substring(0, $separator)
  $port = [int]$ServiceHost.Substring($separator + 1)
  $client = [System.Net.Sockets.TcpClient]::new()
  $client.ReceiveTimeout = 90000
  $client.SendTimeout = 5000
  $client.Connect($hostname, $port)
  try {
    $stream = $client.GetStream()
    $writer = [System.IO.StreamWriter]::new($stream, [System.Text.Encoding]::UTF8)
    $writer.NewLine = "`n"
    $writer.AutoFlush = $true
    $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8)
    $body = @{
      method = $Method
      args = @{}
    } | ConvertTo-Json -Depth 8 -Compress

    $writer.WriteLine($body)
    $line = $reader.ReadLine()
    if (-not $line) {
      throw "Empty response from SLAN client service"
    }
    return ($line | ConvertFrom-Json)
  } finally {
    $client.Close()
  }
}

function Fail-SlanRelayCheck {
  param([string]$Message)

  if ($ExportOnFailure) {
    try {
      $export = Invoke-SlanService -Method "localDiagnosticsExport"
      if ($export.path) {
        Write-Host "Diagnostics exported: $($export.path)"
      }
    } catch {
      Write-Host "Diagnostics export failed: $($_.Exception.Message)"
    }
  }
  Write-Error $Message
}

$diagnose = Invoke-SlanService -Method "localPathDiagnose"
if ($diagnose.error) {
  Fail-SlanRelayCheck "localPathDiagnose failed: $($diagnose.error)"
}
$relay = $diagnose.relay

if (-not $relay) {
  Fail-SlanRelayCheck "No relay runtime stats found. Enable network first, then rerun this tool."
}

Write-Host "Network: $($diagnose.networkId)"
if ($diagnose.health) {
  Write-Host "Health: $($diagnose.health.status)"
  foreach ($reason in @($diagnose.health.reasons)) {
    Write-Host "  $($reason.severity): $($reason.code) - $($reason.message)"
  }
}
Write-Host "Active path: $($diagnose.activePathType)"
if ($diagnose.activePathCounts) {
  $pathCounts = @($diagnose.activePathCounts | ForEach-Object { "$($_.pathType)=$($_.count)" })
  Write-Host "Active path counts: $($pathCounts -join ', ')"
}
Write-Host "Relay: $($relay.address)"
if ($relay.activePath) {
  Write-Host "Relay active path: $($relay.activePath)"
}
$attachedPeerSessions = if ($null -ne $relay.attachedPeerSessionCount) { [int]$relay.attachedPeerSessionCount } else { [int]$relay.relaySessionCount }
$attachedTransports = if ($null -ne $relay.attachedTransportCount) { [int]$relay.attachedTransportCount } else { [int]$relay.relaySessionCount }
Write-Host "Relay peer sessions: $($relay.relaySessionCount)/$($relay.requestedRelaySessionCount)"
Write-Host "Attached peer sessions: $attachedPeerSessions"
Write-Host "Attached transports: $attachedTransports"
Write-Host "Ticket expires: $($relay.ticketExpiresAt)"
Write-Host "Failures: $($relay.failures)"
Write-Host "Config hash mismatches: $($relay.relayConfigHashMismatches)"
Write-Host "Unroutable packets: $($relay.unroutableTunPackets)"
Write-Host "Oversized packets: $($relay.oversizedTunPackets)"
if ($relay.maxFramePayload) {
  Write-Host "Max frame payload: $($relay.maxFramePayload)"
}
if ($relay.lastUnroutableDestination) {
  Write-Host "Last unroutable destination: $($relay.lastUnroutableDestination)"
}
if ($relay.lastOversizedTunPacketSize) {
  Write-Host "Last oversized packet size: $($relay.lastOversizedTunPacketSize)"
}
if ($relay.lastRelayError) {
  Write-Host "Last relay error: $($relay.lastRelayError)"
}
if ($relay.lastRelayAttachError) {
  Write-Host "Last attach error: $($relay.lastRelayAttachError)"
}
if ($diagnose.dns) {
  Write-Host "DNS expected: $(@($diagnose.dns.expectedServers) -join ',')"
  Write-Host "DNS actual: $(@($diagnose.dns.actualServers) -join ',')"
  if ($diagnose.dns.missingServers) {
    Write-Host "DNS missing: $(@($diagnose.dns.missingServers) -join ',')"
  }
}
if ($diagnose.mtu) {
  Write-Host "MTU policy: relayMtu=$($diagnose.mtu.relayMtu) payload=$($diagnose.mtu.maxFramePayload) scope=$($diagnose.mtu.policyScope) path=$($diagnose.mtu.policyPathType)"
  Write-Host "MTU check: checked=$($diagnose.mtu.actualMtuChecked) ok=$($diagnose.mtu.actualMtuOk)"
  Write-Host "MSS check: checked=$($diagnose.mtu.actualMssChecked) ok=$($diagnose.mtu.actualMssOk)"
}

if ($relay.peers) {
  Write-Host ""
  Write-Host "Peers:"
  foreach ($peer in $relay.peers) {
    $ips = if ($peer.peerVirtualIps) { $peer.peerVirtualIps -join "," } else { "-" }
    $lastPath = if ($peer.lastSendPath) { $peer.lastSendPath } else { "-" }
    Write-Host ("- {0} session={1} attached={2} path={3} ips={4} tx={5} rx={6} replay={7} sendFail={8} recvFail={9} writeFail={10}" -f `
      $peer.peerNodeId, `
      $peer.sessionId, `
      $peer.attached, `
      $lastPath, `
      $ips, `
      $peer.tunPacketsSent, `
      $peer.relayPacketsReceived, `
      $peer.replayedFrames, `
      $peer.sendFailures, `
      $peer.receiveFailures, `
      $peer.wintunWriteFailures)
    if ($peer.configHashMismatches -gt 0) {
      Write-Host "  configHashMismatches=$($peer.configHashMismatches)"
    }
    if ($peer.attachError) {
      Write-Host "  attachError=$($peer.attachError)"
    }
    if ($peer.lastRelayError) {
      Write-Host "  relayError=$($peer.lastRelayError)"
    }
    if ($peer.lastPathChange) {
      Write-Host "  pathChange=$($peer.lastPathChange) downgrades=$($peer.pathDowngrades) upgrades=$($peer.pathUpgrades)"
    }
  }
}

if ($diagnose.peerPaths) {
  Write-Host ""
  Write-Host "Peer paths:"
  foreach ($path in $diagnose.peerPaths) {
    $ips = if ($path.peerVirtualIps) { $path.peerVirtualIps -join "," } else { "-" }
    $active = if ($path.activePath) { $path.activePath } else { "-" }
    Write-Host ("- {0} active={1} ips={2}" -f $path.peerNodeId, $active, $ips)
    foreach ($candidate in @($path.candidates)) {
      $address = if ($candidate.address) { $candidate.address } else { "-" }
      $transport = if ($candidate.transport) { $candidate.transport } else { "-" }
      $score = if ($null -ne $candidate.pathScore) { $candidate.pathScore } else { "-" }
      $rtt = if ($null -ne $candidate.rttMs) { $candidate.rttMs } else { "-" }
      Write-Host ("  {0} state={1} transport={2} score={3} rtt={4} address={5}" -f `
        $candidate.kind, `
        $candidate.state, `
        $transport, `
        $score, `
        $rtt, `
        $address)
      if ($candidate.lastError) {
        Write-Host "    error=$($candidate.lastError)"
      }
    }
  }
}

if ($RequireDerpCandidate) {
  $derpCandidates = @()
  if ($diagnose.peerPaths) {
    $derpCandidates = @($diagnose.peerPaths | ForEach-Object { $_.candidates } | Where-Object { $_.kind -eq "derp_tcp_tls_443" })
  }
  if ($derpCandidates.Count -eq 0) {
    Fail-SlanRelayCheck "No derp_tcp_tls_443 candidate found in peer path diagnose output."
  }
}

if ($RequireDirectUdpActive) {
  $directActive = $false
  if ($diagnose.activePathCounts) {
    $directActive = @($diagnose.activePathCounts | Where-Object { $_.pathType -eq "direct_udp" -and $_.count -gt 0 }).Count -gt 0
  }
  if (-not $directActive -and $diagnose.peerPaths) {
    $directActive = @($diagnose.peerPaths | Where-Object { $_.activePath -eq "direct_udp" }).Count -gt 0
  }
  if (-not $directActive) {
    Fail-SlanRelayCheck "No active direct_udp path found."
  }
}

if ($RequireMtuOk -and $diagnose.mtu) {
  if ($diagnose.mtu.actualMtuChecked -ne $true -or $diagnose.mtu.actualMtuOk -ne $true) {
    Fail-SlanRelayCheck "MTU check failed or was not available."
  }
}

if ($RequireMssOk -and $diagnose.mtu) {
  if ($diagnose.mtu.actualMssChecked -ne $true -or $diagnose.mtu.actualMssOk -ne $true) {
    Fail-SlanRelayCheck "MSS check failed or was not available."
  }
}

if ($relay.relaySessionCount -lt $MinRelaySessions) {
  Fail-SlanRelayCheck "Relay peer session count $($relay.relaySessionCount) is below required minimum $MinRelaySessions."
}

if (-not $AllowMissingPeerSessions -and $relay.requestedRelaySessionCount -gt 0 -and $attachedPeerSessions -lt $relay.requestedRelaySessionCount) {
  Fail-SlanRelayCheck "Some relay peer sessions are not attached: $attachedPeerSessions/$($relay.requestedRelaySessionCount)."
}

if ($attachedTransports -lt $MinRelaySessions) {
  Fail-SlanRelayCheck "Attached transport count $attachedTransports is below required minimum $MinRelaySessions."
}

if ($relay.unroutableTunPackets -gt 0) {
  Fail-SlanRelayCheck "Unroutable TUN packets detected. Last destination: $($relay.lastUnroutableDestination)"
}

if ($relay.oversizedTunPackets -gt 0) {
  Fail-SlanRelayCheck "Oversized TUN packets detected. Last size: $($relay.lastOversizedTunPacketSize), max payload: $($relay.maxFramePayload)"
}

if ($relay.relayConfigHashMismatches -gt 0) {
  Fail-SlanRelayCheck "Relay config hash mismatches detected. Old or wrong data-plane frames reached this client."
}

if ($diagnose.dns -and $diagnose.dns.checked -and $diagnose.dns.ok -eq $false) {
  Fail-SlanRelayCheck "DNS configuration mismatch. Missing: $(@($diagnose.dns.missingServers) -join ',')"
}

$replayed = @($relay.peers | Where-Object { $_.replayedFrames -gt 0 })
if ($replayed.Count -gt 0) {
  Fail-SlanRelayCheck "Relay replayed/old frames detected for $($replayed.Count) peer(s)."
}

Write-Host ""
Write-Host "Windows relay multi-peer diagnose check passed."
