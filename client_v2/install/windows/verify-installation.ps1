param(
  [string]$InstallDir = '',
  [string]$ServiceName = '',
  [string]$AdapterName = '',
  [switch]$RunRelayDiagnose,
  [switch]$AllowMissingPeerSessions,
  [switch]$SkipRelayDiagnoseToolCheck,
  [switch]$ExpectUninstalled,
  [switch]$Json
)

$ErrorActionPreference = 'Stop'
Import-Module (Join-Path $PSScriptRoot 'SlanWindowsInstall.psm1') -Force
$manifest = Get-SlanWindowsInstallManifest

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
  $InstallDir = $manifest.InstallDir
}
if ([string]::IsNullOrWhiteSpace($ServiceName)) {
  $ServiceName = $manifest.ServiceName
}
if ([string]::IsNullOrWhiteSpace($AdapterName)) {
  $AdapterName = $manifest.AdapterName
}

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
  # A signed-out or disabled SLAN network intentionally leaves Wintun disabled
  # or disconnected. Installation only requires the adapter to exist; network
  # activation is responsible for enabling it and assigning the device IP.
  New-Check 'adapter' $true "status=$($adapter.Status); adminStatus=$($adapter.AdminStatus); ifIndex=$($adapter.ifIndex)"
}

function Get-UninstallEntryCheck {
  $roots = @(
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
  )
  $entry = $roots |
    ForEach-Object { Get-ItemProperty $_ -ErrorAction SilentlyContinue } |
    Where-Object { $_.DisplayName -eq $manifest.AppName } |
    Select-Object -First 1
  New-Check 'uninstallEntry' ($null -ne $entry) ($(if ($entry) { $entry.DisplayVersion } else { 'missing' }))
}

function Get-RelayDiagnoseCheck {
  $toolPath = Join-Path $InstallDir $manifest.RelayDiagnoseToolDestination
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
    (Test-FileExists 'appExe' (Join-Path $InstallDir $manifest.AppExeName)),
    (Test-FileExists 'serviceExe' (Join-Path $InstallDir $manifest.ServiceExeName)),
    (Test-FileExists 'wintunDll' (Join-Path $InstallDir 'wintun.dll')),
    (Test-FileExists 'flutterDll' (Join-Path $InstallDir 'flutter_windows.dll')),
    (Get-ServiceCheck $ServiceName),
    (Get-AdapterCheck $AdapterName),
    (Get-UninstallEntryCheck)
  )
  foreach ($tool in $manifest.PackagedTools) {
    if ($SkipRelayDiagnoseToolCheck -and $tool.Destination -eq $manifest.RelayDiagnoseToolDestination) {
      continue
    }
    $checks += Test-FileExists "tool:$($tool.Destination)" (Join-Path $InstallDir $tool.Destination)
  }
  if ($RunRelayDiagnose) {
    $checks += Get-RelayDiagnoseCheck
  }
  $checks
}

function Get-UninstalledChecks {
  $installExists = Test-Path -LiteralPath $InstallDir
  $stateDir = $manifest.ProgramDataDir
  $desktopShortcuts = @(
    (Join-Path ([Environment]::GetFolderPath('Desktop')) "$($manifest.AppName).lnk"),
    (Join-Path ([Environment]::GetFolderPath('CommonDesktopDirectory')) "$($manifest.AppName).lnk")
  )
  $remainingDesktopShortcuts = @($desktopShortcuts | Where-Object { Test-Path -LiteralPath $_ })
  $remainingAppDataDirs = @($manifest.AppDataDirectories | Where-Object { Test-Path -LiteralPath $_ })
  $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
  $taskChecks = @{}
  foreach ($taskName in $manifest.LegacyTaskNames) {
    schtasks.exe /Query /TN $taskName 2>$null 1>$null
    $taskChecks[$taskName] = ($LASTEXITCODE -ne 0)
  }
  $helperTaskName = $manifest.LegacyTaskNames[0]
  $serviceTaskName = $manifest.LegacyTaskNames[1]
  $remainingStateFiles = @(
    $manifest.StateFiles |
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
    (New-Check 'helperTaskRemoved' $taskChecks[$helperTaskName] $helperTaskName),
    (New-Check 'serviceTaskRemoved' $taskChecks[$serviceTaskName] $serviceTaskName),
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
