param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\SLAN",
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
  if (Test-IsAdministrator) {
    return
  }

  $argumentList = @(
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', ('"{0}"' -f $PSCommandPath),
    '-InstallDir', ('"{0}"' -f $InstallDir)
  )
  if ($NoLaunch) {
    $argumentList += '-NoLaunch'
  }
  if ($ShortcutDir -and $ShortcutDir.Trim()) {
    $argumentList += @('-ShortcutDir', ('"{0}"' -f $ShortcutDir.Trim()))
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

function Assert-InstallPayload {
  param([string]$PayloadDir)

  $required = @(
    'slan_app.exe',
    'app-core-helper.exe',
    'app-core-service.exe',
    'wintun.dll'
  )
  foreach ($name in $required) {
    $path = Join-Path $PayloadDir $name
    if (-not (Test-Path $path)) {
      throw "Missing required runtime component: $path"
    }
  }
}

function Resolve-ShortcutPath {
  param([string]$ShortcutDir)

  $resolvedShortcutDir =
    if ($ShortcutDir -and $ShortcutDir.Trim()) {
      $ShortcutDir.Trim()
    } elseif ($env:SLAN_SHORTCUT_DIR -and $env:SLAN_SHORTCUT_DIR.Trim()) {
      $env:SLAN_SHORTCUT_DIR.Trim()
    } else {
      [Environment]::GetFolderPath('Desktop')
    }
  return Join-Path $resolvedShortcutDir 'SLAN.lnk'
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
$payloadDir = Join-Path $scriptRoot 'payload'

Ensure-Administrator

if (-not (Test-Path $payloadDir)) {
  throw "Missing payload directory: $payloadDir"
}

Assert-InstallPayload -PayloadDir $payloadDir
Stop-SlanProcesses

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path (Join-Path $payloadDir '*') -Destination $InstallDir -Recurse -Force
Ensure-DedicatedAdapter -RuntimeDir $InstallDir
Register-AppCoreServiceTask -RuntimeDir $InstallDir

$shortcutPath = Resolve-ShortcutPath -ShortcutDir $ShortcutDir
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $shortcutPath) | Out-Null
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = Join-Path $InstallDir 'slan_app.exe'
$shortcut.WorkingDirectory = $InstallDir
$shortcut.IconLocation = Join-Path $InstallDir 'slan_app.exe'
$shortcut.Save()

if (-not $NoLaunch) {
  Start-InstalledApp -RuntimeDir $InstallDir
}

Write-Host "Installed SLAN to $InstallDir"
Write-Host "Installed runtime components: slan_app.exe, app-core-helper.exe, app-core-service.exe"
