param(
  [Parameter(Mandatory = $true)][string]$ApiBaseUrl,
  [Parameter(Mandatory = $true)][string]$Stage,
  [Parameter(Mandatory = $true)][string]$ErrorPath,
  [string]$InstallerLogPath = '',
  [string]$Version = '0.1.0'
)

$ErrorActionPreference = 'Stop'
$diagnosticsDir = Join-Path $env:ProgramData 'SLAN\diagnostics'
New-Item -ItemType Directory -Force -Path $diagnosticsDir | Out-Null
$installationId = [guid]::NewGuid().ToString()
$resultPath = Join-Path $diagnosticsDir "installer-upload-$installationId.json"
$bundlePath = Join-Path $diagnosticsDir "installer-bundle-$installationId.json"
$endpoint = $ApiBaseUrl.TrimEnd('/') + '/api/app/diagnostics/installer'

function Read-RedactedTail {
  param([string]$Path, [int]$MaxCharacters = 24576)

  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    return ''
  }
  $content = Get-Content -LiteralPath $Path -Raw -ErrorAction Stop
  if ($content.Length -gt $MaxCharacters) {
    $content = $content.Substring($content.Length - $MaxCharacters)
  }
  if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
    $content = $content.Replace($env:USERPROFILE, '%USERPROFILE%')
  }
  $content = $content -replace '(?i)(authorization:\s*bearer\s+)[^\s"'']+', '$1[REDACTED]'
  $content = $content -replace '(?i)("(?:accessToken|refreshToken|deviceToken|password)"\s*:\s*")[^"]+', '$1[REDACTED]'
  return $content
}

try {
  $errorText = Read-RedactedTail -Path $ErrorPath -MaxCharacters 2048
  if ([string]::IsNullOrWhiteSpace($errorText)) {
    $errorText = 'Windows installer failed without an error message.'
  }
  $files = @{
    'installer-error.txt' = $errorText
  }
  $installerLog = Read-RedactedTail -Path $InstallerLogPath
  if (-not [string]::IsNullOrWhiteSpace($installerLog)) {
    $files['installer.log'] = $installerLog
  }
  $serviceLog = Read-RedactedTail -Path (Join-Path $env:ProgramData 'SLAN\client-core-service.log') -MaxCharacters 12288
  if (-not [string]::IsNullOrWhiteSpace($serviceLog)) {
    $files['client-core-service.log'] = $serviceLog
  }
  $runtimeDiagnostics = @(
    'service:'
    (Get-Service -Name 'SLANClientV2Service' -ErrorAction SilentlyContinue | Format-List Name, Status, StartType | Out-String)
    'network-adapters:'
    (Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
      Where-Object { $_.Name -eq 'SLAN LAN Adapter' -or $_.InterfaceDescription -like '*Wintun*' } |
      Format-List Name, InterfaceDescription, Status, AdminStatus, LinkSpeed, MacAddress | Out-String)
    'pnp-devices:'
    (Get-PnpDevice -Class Net -ErrorAction SilentlyContinue |
      Where-Object { $_.FriendlyName -eq 'SLAN LAN Adapter' -or $_.FriendlyName -like '*Wintun*' } |
      Format-List FriendlyName, Status, Problem, InstanceId | Out-String)
  ) -join "`r`n"
  if ($runtimeDiagnostics.Length -gt 8192) {
    $runtimeDiagnostics = $runtimeDiagnostics.Substring($runtimeDiagnostics.Length - 8192)
  }
  $files['windows-runtime.txt'] = $runtimeDiagnostics
  $payload = @{
    installationId = $installationId
    platform = 'windows'
    version = $Version
    capturedAt = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    stage = $Stage
    error = $errorText
    files = $files
  } | ConvertTo-Json -Depth 4
  $payload | Set-Content -LiteralPath $bundlePath -Encoding UTF8
  $response = Invoke-RestMethod -Method Post -Uri $endpoint -ContentType 'application/json' -Body $payload -TimeoutSec 10
  @{
    uploaded = $true
    installationId = $installationId
    uploadId = $response.uploadId
    bundlePath = $bundlePath
    endpoint = $endpoint
  } | ConvertTo-Json | Set-Content -LiteralPath $resultPath -Encoding UTF8
  exit 0
} catch {
  @{
    uploaded = $false
    installationId = $installationId
    bundlePath = $bundlePath
    endpoint = $endpoint
    error = $_.Exception.Message
  } | ConvertTo-Json | Set-Content -LiteralPath $resultPath -Encoding UTF8
  exit 1
}
