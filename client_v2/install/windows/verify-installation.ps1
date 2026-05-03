param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\SLAN Client V2",
  [string]$ServiceName = "SLANClientV2Service",
  [string]$AdapterName = "SLAN LAN Adapter",
  [switch]$RunRelayDiagnose,
  [switch]$AllowMissingPeerSessions,
  [switch]$SkipRelayDiagnoseToolCheck,
  [switch]$ExpectUninstalled,
  [switch]$Json
)

$ErrorActionPreference = 'Stop'

function New-Check {
  param(
    [string]$Name,
    [bool]$Ok,
    [string]$Message = ''
  )
  [pscustomobject]@{
    name = $Name
    ok = $Ok
    message = $Message
  }
}

function Test-FileExists {
  param([string]$Name, [string]$Path)
  New-Check $Name (Test-Path -LiteralPath $Path -PathType Leaf) $Path
}

function Get-ServiceCheck {
  param([string]$Name)
  $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
  if (-not $service) {
    return New-Check 'service' $false "service $Name missing"
  }
  New-Check 'service' ($service.Status -eq 'Running') "status=$($service.Status)"
}

function Get-AdapterCheck {
  param([string]$Name)
  $adapter = Get-NetAdapter -IncludeHidden -Name $Name -ErrorAction SilentlyContinue
  if (-not $adapter) {
    return New-Check 'adapter' $false "adapter $Name missing"
  }
  New-Check 'adapter' ($adapter.AdminStatus -eq 'Up') "status=$($adapter.AdminStatus); ifIndex=$($adapter.ifIndex)"
}

function Get-UninstallEntryCheck {
  $roots = @(
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
  )
  $entry = $roots |
    ForEach-Object { Get-ItemProperty $_ -ErrorAction SilentlyContinue } |
    Where-Object { $_.DisplayName -eq 'SLAN Client V2' } |
    Select-Object -First 1
  New-Check 'uninstallEntry' ($null -ne $entry) ($(if ($entry) { $entry.DisplayVersion } else { 'missing' }))
}

function Get-RelayDiagnoseCheck {
  $toolPath = Join-Path $InstallDir 'tools\test-windows-multipeer-relay.ps1'
  if (-not (Test-Path -LiteralPath $toolPath -PathType Leaf)) {
    return New-Check 'relayDiagnose' $false "missing $toolPath"
  }
  $args = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $toolPath, '-ExportOnFailure')
  if ($AllowMissingPeerSessions) {
    $args += '-AllowMissingPeerSessions'
  }
  $output = & powershell.exe @args 2>&1
  New-Check 'relayDiagnose' ($LASTEXITCODE -eq 0) (($output | Out-String).Trim())
}

function Get-InstalledChecks {
  $checks = @(
    (Test-FileExists 'appExe' (Join-Path $InstallDir 'slan_client_v2.exe')),
    (Test-FileExists 'serviceExe' (Join-Path $InstallDir 'client-core-service.exe')),
    (Test-FileExists 'wintunDll' (Join-Path $InstallDir 'wintun.dll')),
    (Test-FileExists 'flutterDll' (Join-Path $InstallDir 'flutter_windows.dll')),
    (Get-ServiceCheck $ServiceName),
    (Get-AdapterCheck $AdapterName),
    (Get-UninstallEntryCheck)
  )
  if (-not $SkipRelayDiagnoseToolCheck) {
    $checks += Test-FileExists 'relayDiagnoseTool' (Join-Path $InstallDir 'tools\test-windows-multipeer-relay.ps1')
  }
  if ($RunRelayDiagnose) {
    $checks += Get-RelayDiagnoseCheck
  }
  $checks
}

function Get-UninstalledChecks {
  $installExists = Test-Path -LiteralPath $InstallDir
  $stateDir = Join-Path $env:ProgramData 'SLAN'
  $desktopShortcuts = @(
    (Join-Path ([Environment]::GetFolderPath('Desktop')) 'SLAN Client V2.lnk'),
    (Join-Path ([Environment]::GetFolderPath('CommonDesktopDirectory')) 'SLAN Client V2.lnk')
  )
  $remainingDesktopShortcuts = @($desktopShortcuts | Where-Object { Test-Path -LiteralPath $_ })
  $appDataDirs = @(
    (Join-Path $env:APPDATA 'slan_client_v2'),
    (Join-Path $env:LOCALAPPDATA 'slan_client_v2'),
    (Join-Path $env:APPDATA 'SLAN Client V2'),
    (Join-Path $env:LOCALAPPDATA 'SLAN Client V2')
  )
  $remainingAppDataDirs = @($appDataDirs | Where-Object { Test-Path -LiteralPath $_ })
  $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  schtasks.exe /Query /TN 'SLAN Client V2 Helper' 2>$null 1>$null
  $helperTaskMissing = $LASTEXITCODE -ne 0
  schtasks.exe /Query /TN 'SLAN Client V2 Service' 2>$null 1>$null
  $serviceTaskMissing = $LASTEXITCODE -ne 0
  $stateFiles = @(
    'client-v2-session.json',
    'client-v2-control-tasks.xml',
    'client-v2-device-id.txt',
    'client-v2-network-state.json',
    'client-v2-assigned-ip.txt',
    'client-v2-relay-stats.json',
    'client-v2-relay-policy.json',
    'mqtt-inbox.xml'
  )
  $remainingStateFiles = @(
    $stateFiles |
      ForEach-Object { Join-Path $stateDir $_ } |
      Where-Object { Test-Path -LiteralPath $_ }
  )
  $diagnosticsDir = Join-Path $stateDir 'diagnostics'
  $adapter = Get-NetAdapter -IncludeHidden -Name $AdapterName -ErrorAction SilentlyContinue
  @(
    (New-Check 'installDirRemoved' (-not $installExists) $InstallDir),
    (New-Check 'serviceRemoved' ($null -eq $service) $ServiceName),
    (New-Check 'adapterRemoved' ($null -eq $adapter) $AdapterName),
    (New-Check 'desktopShortcutRemoved' ($remainingDesktopShortcuts.Count -eq 0) ($remainingDesktopShortcuts -join ';')),
    (New-Check 'appDataDirsRemoved' ($remainingAppDataDirs.Count -eq 0) ($remainingAppDataDirs -join ';')),
    (New-Check 'helperTaskRemoved' $helperTaskMissing 'SLAN Client V2 Helper'),
    (New-Check 'serviceTaskRemoved' $serviceTaskMissing 'SLAN Client V2 Service'),
    (New-Check 'stateFilesRemoved' ($remainingStateFiles.Count -eq 0) ($remainingStateFiles -join ';')),
    (New-Check 'diagnosticsRemoved' (-not (Test-Path -LiteralPath $diagnosticsDir)) $diagnosticsDir)
  )
}

$checks = if ($ExpectUninstalled) { Get-UninstalledChecks } else { Get-InstalledChecks }
$ok = -not ($checks | Where-Object { -not $_.ok } | Select-Object -First 1)
$result = [pscustomobject]@{
  ok = $ok
  mode = $(if ($ExpectUninstalled) { 'uninstalled' } else { 'installed' })
  checkedAt = (Get-Date).ToUniversalTime().ToString('o')
  checks = $checks
}

if ($Json) {
  $result | ConvertTo-Json -Depth 5
  if (-not $ok) {
    exit 1
  }
  exit 0
} else {
  $checks | Format-Table -AutoSize
  if (-not $ok) {
    throw 'SLAN Client V2 installation verification failed.'
  }
}
