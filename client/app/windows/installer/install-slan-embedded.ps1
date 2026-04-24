param(
  [string]$InstallDir = '',
  [switch]$NoLaunch,
  [string]$ShortcutDir = ''
)

$ErrorActionPreference = 'Stop'
$taskName = 'SLAN AppCore Service'
$serviceHost = '127.0.0.1:46391'

function Test-IsAdministrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($identity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Ensure-Administrator {
  param(
    [string]$ResolvedInstallDir,
    [bool]$ResolvedNoLaunch,
    [string]$ResolvedShortcutDir
  )

  if (Test-IsAdministrator) {
    return
  }

  $argumentList = @(
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', ('"{0}"' -f $PSCommandPath),
    '-InstallDir', ('"{0}"' -f $ResolvedInstallDir)
  )
  if ($ResolvedNoLaunch) {
    $argumentList += '-NoLaunch'
  }
  if ($ResolvedShortcutDir -and $ResolvedShortcutDir.Trim()) {
    $argumentList += @('-ShortcutDir', ('"{0}"' -f $ResolvedShortcutDir.Trim()))
  }

  $process = Start-Process -FilePath 'powershell.exe' -ArgumentList $argumentList -Verb RunAs -Wait -PassThru
  exit $process.ExitCode
}

function Stop-SlanProcesses {
  foreach ($name in @('slan_app', 'app-core-service', 'app-core-helper')) {
    $running = Get-Process $name -ErrorAction SilentlyContinue
    if ($running) {
      foreach ($process in $running) {
        try {
          $process | Stop-Process -Force -ErrorAction Stop
        } catch {
          Write-Warning "Unable to stop process $($process.ProcessName) ($($process.Id)); continuing install."
        }
      }
    }
  }
  Start-Sleep -Seconds 1
}

function Ensure-DedicatedAdapter {
  param([string]$RuntimeDir)

  $servicePath = Join-Path $RuntimeDir 'app-core-service.exe'
  if (-not (Test-Path $servicePath)) {
    throw "Missing app-core-service runtime: $servicePath"
  }

  & $servicePath --driver wintun --prepare-adapter
  if ($LASTEXITCODE -ne 0) {
    throw "app-core-service failed to prepare the Wintun / WireGuardNT adapter (exit code $LASTEXITCODE)."
  }

  $legacy = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
    Where-Object {
      $_.InterfaceDescription -like '*KM-TEST*Loopback Adapter*'
    }
  if ($legacy) {
    throw 'Legacy KM-TEST loopback adapter is still present after Wintun preparation.'
  }
}

function Assert-InstalledRuntime {
  param([string]$InstallDir)

  $required = @(
    'slan_app.exe',
    'app-core-helper.exe',
    'app-core-service.exe',
    'wintun.dll'
  )
  foreach ($name in $required) {
    $path = Join-Path $InstallDir $name
    if (-not (Test-Path $path)) {
      throw "Missing required installed runtime component: $path"
    }
  }
}

function Resolve-ShortcutPath {
  param([string]$ResolvedShortcutDir)

  $shortcutDir =
    if ($ResolvedShortcutDir -and $ResolvedShortcutDir.Trim()) {
      $ResolvedShortcutDir.Trim()
    } else {
      [Environment]::GetFolderPath('Desktop')
    }
  return Join-Path $shortcutDir 'SLAN.lnk'
}

function Register-AppCoreServiceTask {
  param([string]$RuntimeDir)

  $servicePath = Join-Path $RuntimeDir 'app-core-service.exe'
  if (-not (Test-Path $servicePath)) {
    throw "Missing app-core-service runtime: $servicePath"
  }

  schtasks.exe /End /TN $taskName | Out-Null
  $taskCommand = ('"{0}" --tcp-host {1}' -f $servicePath, $serviceHost)
  schtasks.exe /Create /TN $taskName /TR $taskCommand /SC ONSTART /RL HIGHEST /RU SYSTEM /F | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to register scheduled task $taskName (exit code $LASTEXITCODE)."
  }

  schtasks.exe /Run /TN $taskName | Out-Null
}

function Start-InstalledApp {
  param([string]$RuntimeDir)

  $appPath = Join-Path $RuntimeDir 'slan_app.exe'
  if (-not (Test-Path $appPath)) {
    throw "Missing installed app executable: $appPath"
  }

  $quotedAppPath = '"' + $appPath + '"'
  $quotedRuntimeDir = '"' + $RuntimeDir + '"'
  $launchCommand = "start `"`" /D $quotedRuntimeDir $quotedAppPath"
  Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', $launchCommand -WindowStyle Hidden | Out-Null
}

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$defaultInstallDir = Join-Path $env:LOCALAPPDATA 'Programs\SLAN'
$resolvedInstallDir = if ($InstallDir -and $InstallDir.Trim()) {
  $InstallDir.Trim()
} elseif ($env:SLAN_INSTALL_DIR -and $env:SLAN_INSTALL_DIR.Trim()) {
  $env:SLAN_INSTALL_DIR.Trim()
} else {
  $defaultInstallDir
}
$installDir = [System.IO.Path]::GetFullPath($resolvedInstallDir)
$resolvedNoLaunch = $NoLaunch.IsPresent
if (-not $resolvedNoLaunch -and $env:SLAN_INSTALL_NO_LAUNCH) {
  $normalized = $env:SLAN_INSTALL_NO_LAUNCH.Trim().ToLowerInvariant()
  $resolvedNoLaunch = $normalized -eq '1' -or $normalized -eq 'true' -or $normalized -eq 'yes'
}
$resolvedShortcutDir = if ($ShortcutDir -and $ShortcutDir.Trim()) { $ShortcutDir.Trim() } else { $env:SLAN_SHORTCUT_DIR }

$payloadArchive = Join-Path $scriptRoot 'slan-payload.zip'

Ensure-Administrator -ResolvedInstallDir $installDir -ResolvedNoLaunch $resolvedNoLaunch -ResolvedShortcutDir $resolvedShortcutDir

Stop-SlanProcesses

if (-not (Test-Path $payloadArchive)) {
  throw "Missing embedded payload archive: $payloadArchive"
}

if (Test-Path $installDir) {
  Remove-Item -Recurse -Force $installDir
}
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Expand-Archive -Path $payloadArchive -DestinationPath $installDir -Force
Assert-InstalledRuntime -InstallDir $installDir
Ensure-DedicatedAdapter -RuntimeDir $installDir
Register-AppCoreServiceTask -RuntimeDir $installDir

$shortcutPath = Resolve-ShortcutPath -ResolvedShortcutDir $resolvedShortcutDir
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $shortcutPath) | Out-Null
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = Join-Path $installDir 'slan_app.exe'
$shortcut.WorkingDirectory = $installDir
$shortcut.IconLocation = Join-Path $installDir 'slan_app.exe'
$shortcut.Save()

if (-not $resolvedNoLaunch) {
  Start-InstalledApp -RuntimeDir $installDir
}

Write-Host "Installed SLAN to $installDir"
Write-Host "Installed runtime components: slan_app.exe, app-core-helper.exe, app-core-service.exe"
