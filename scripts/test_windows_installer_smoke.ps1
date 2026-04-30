param(
  [string]$WorkspaceRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path,
  [string]$ReleaseDir = '',
  [string]$InstallerOutputDir = '',
  [switch]$SkipBuild,
  [switch]$ExecutableOnly,
  [switch]$KeepArtifacts
)

$ErrorActionPreference = 'Stop'

function Invoke-Step {
  param(
    [string]$Message,
    [scriptblock]$Action
  )

  Write-Host "==> $Message"
  & $Action
}

function Wait-Until {
  param(
    [string]$Label,
    [scriptblock]$Condition,
    [int]$TimeoutSeconds = 20,
    [int]$PollMilliseconds = 500
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    if (& $Condition) {
      return
    }
    Start-Sleep -Milliseconds $PollMilliseconds
  }
  throw "Timed out waiting for $Label"
}

function Get-SmokePaths {
  param(
    [string]$Root,
    [string]$ReleaseOverride,
    [string]$OutputOverride
  )

  $clientAppRoot = Join-Path $Root 'client\app'
  $resolvedReleaseDir =
    if ($ReleaseOverride.Trim()) {
      [System.IO.Path]::GetFullPath($ReleaseOverride)
    } else {
      [System.IO.Path]::GetFullPath((Join-Path $clientAppRoot 'build\windows\x64\runner\Release'))
    }

  $resolvedOutputDir =
    if ($OutputOverride.Trim()) {
      [System.IO.Path]::GetFullPath($OutputOverride)
    } else {
      [System.IO.Path]::GetFullPath((Join-Path $Root '.tmp\installer\smoke\bundle'))
    }

  return @{
    ClientAppRoot = $clientAppRoot
    InstallerRoot = Join-Path $clientAppRoot 'windows\installer'
    ReleaseDir = $resolvedReleaseDir
    OutputDir = $resolvedOutputDir
    OutputZip = "$resolvedOutputDir.zip"
    SetupExe = Join-Path $resolvedOutputDir 'SLAN-Setup.exe'
    StartupLog = Join-Path $env:TEMP 'slan_app_startup.log'
    SmokeShortcutDir = [System.IO.Path]::GetFullPath((Join-Path $Root '.tmp\installer\smoke\shortcuts'))
    SmokeShortcut = [System.IO.Path]::GetFullPath((Join-Path $Root '.tmp\installer\smoke\shortcuts\SLAN.lnk'))
    ScriptInstallDir = [System.IO.Path]::GetFullPath((Join-Path $Root '.tmp\installer\smoke\script-installed\SLAN'))
    ExeInstallDir = [System.IO.Path]::GetFullPath((Join-Path $Root '.tmp\installer\smoke\exe-installed\SLAN'))
  }
}

function Use-NativeInstallerLayout {
  param([hashtable]$Paths)

  return (Test-Path (Join-Path $Paths.OutputDir 'SLAN.generated.iss'))
}

function Stop-SlanAppProcess {
  $running = Get-Process slan_app -ErrorAction SilentlyContinue
  if ($running) {
    $running | Stop-Process -Force
    Start-Sleep -Seconds 1
  }
}

function Remove-PathWithRetry {
  param(
    [string]$Path,
    [int]$Attempts = 30,
    [int]$DelayMilliseconds = 1000
  )

  for ($i = 1; $i -le $Attempts; $i++) {
    try {
      Remove-Item -Recurse -Force $Path
      return
    } catch {
      if ($i -eq $Attempts) {
        throw
      }
      Start-Sleep -Milliseconds $DelayMilliseconds
    }
  }
}

function Prepare-InstallSandbox {
  param(
    [string]$InstallDir,
    [hashtable]$Paths
  )

  if (Test-Path $InstallDir) {
    Remove-Item -Recurse -Force $InstallDir
  }
  $installParent = Split-Path -Parent $InstallDir
  if ($installParent -and -not (Test-Path $installParent)) {
    New-Item -ItemType Directory -Force -Path $installParent | Out-Null
  }
  if (Test-Path $Paths.SmokeShortcutDir) {
    Remove-Item -Recurse -Force $Paths.SmokeShortcutDir
  }
  New-Item -ItemType Directory -Force -Path $Paths.SmokeShortcutDir | Out-Null
  if (Test-Path $Paths.StartupLog) {
    Remove-Item -Force $Paths.StartupLog
  }
}

function Wait-ForInstalledApp {
  param(
    [string]$ExpectedExe,
    [hashtable]$Paths,
    [switch]$StartIfMissing
  )

  $started = $false
  try {
    Wait-Until -Label 'installed SLAN process' -TimeoutSeconds 10 -Condition {
      $processes = Get-Process slan_app -ErrorAction SilentlyContinue
      if (-not $processes) {
        return $false
      }
      foreach ($process in $processes) {
        if ($process.Path -eq $ExpectedExe) {
          return $true
        }
      }
      return $false
    }
  } catch {
    if (-not $StartIfMissing) {
      throw
    }
    if (-not (Test-Path $ExpectedExe)) {
      throw "Installed executable not found before fallback launch: $ExpectedExe"
    }
    Start-Process -FilePath $ExpectedExe -WorkingDirectory (Split-Path -Parent $ExpectedExe)
    $started = $true
  }

  Wait-Until -Label 'installed SLAN process' -TimeoutSeconds 25 -Condition {
    $processes = Get-Process slan_app -ErrorAction SilentlyContinue
    if (-not $processes) {
      return $false
    }
    foreach ($process in $processes) {
      if ($process.Path -eq $ExpectedExe) {
        return $true
      }
      }
      return $false
  }

  Wait-Until -Label 'startup log creation' -TimeoutSeconds 25 -Condition {
    if (-not (Test-Path $Paths.StartupLog)) {
      return $false
    }
    return (Get-Item $Paths.StartupLog).Length -gt 0
  }
}

function Assert-InstalledOutputs {
  param(
    [string]$InstallDir,
    [hashtable]$Paths
  )

  $required = @(
    (Join-Path $InstallDir 'slan_app.exe'),
    (Join-Path $InstallDir 'app-core-helper.exe'),
    (Join-Path $InstallDir 'app-core-service.exe')
  )
  foreach ($path in $required) {
    if (-not (Test-Path $path)) {
      throw "Installed runtime component not found: $path"
    }
  }
  if (-not (Use-NativeInstallerLayout -Paths $Paths) -and -not (Test-Path $Paths.SmokeShortcut)) {
    throw "Smoke shortcut not found: $($Paths.SmokeShortcut)"
  }
  $logTail = Get-Content $Paths.StartupLog -Tail 20
  if (-not $logTail) {
    throw "Startup log is empty: $($Paths.StartupLog)"
  }
}

function Invoke-ScriptInstallerSmoke {
  param([hashtable]$Paths)

  if (Use-NativeInstallerLayout -Paths $Paths) {
    Write-Host '==> Skipping legacy script-installer smoke for native installer layout'
    return
  }

  Invoke-Step 'Preparing script-installer sandbox' {
    Prepare-InstallSandbox -InstallDir $Paths.ScriptInstallDir -Paths $Paths
  }

  Invoke-Step 'Running script installer and launching installed app' {
    & (Join-Path $Paths.OutputDir 'install-slan.ps1') `
      -InstallDir $Paths.ScriptInstallDir `
      -ShortcutDir $Paths.SmokeShortcutDir
  }

  Invoke-Step 'Waiting for script-installed app' {
    Wait-ForInstalledApp -ExpectedExe (Join-Path $Paths.ScriptInstallDir 'slan_app.exe') -Paths $Paths
  }

  Invoke-Step 'Validating script-installed outputs' {
    Assert-InstalledOutputs -InstallDir $Paths.ScriptInstallDir -Paths $Paths
  }

  Stop-SlanAppProcess
}

function Invoke-ExecutableInstallerSmoke {
  param([hashtable]$Paths)

  $installerProcess = $null
  Invoke-Step 'Preparing setup.exe installer sandbox' {
    Prepare-InstallSandbox -InstallDir $Paths.ExeInstallDir -Paths $Paths
  }

  Invoke-Step 'Running setup.exe and launching installed app' {
    if (Use-NativeInstallerLayout -Paths $Paths) {
      $arguments = @(
        '/VERYSILENT',
        '/SUPPRESSMSGBOXES',
        '/NORESTART',
        ('/DIR="{0}"' -f $Paths.ExeInstallDir)
      )
      $installerProcess = Start-Process -FilePath $Paths.SetupExe -ArgumentList $arguments -PassThru -Wait
    } else {
      $env:SLAN_INSTALL_DIR = $Paths.ExeInstallDir
      $env:SLAN_INSTALL_NO_LAUNCH = 'false'
      $env:SLAN_SHORTCUT_DIR = $Paths.SmokeShortcutDir
      try {
        $installerProcess = Start-Process -FilePath $Paths.SetupExe -PassThru
      } finally {
        Remove-Item Env:SLAN_INSTALL_DIR -ErrorAction SilentlyContinue
        Remove-Item Env:SLAN_INSTALL_NO_LAUNCH -ErrorAction SilentlyContinue
        Remove-Item Env:SLAN_SHORTCUT_DIR -ErrorAction SilentlyContinue
      }
    }
  }

  Invoke-Step 'Waiting for setup-installed app' {
    Wait-ForInstalledApp -ExpectedExe (Join-Path $Paths.ExeInstallDir 'slan_app.exe') -Paths $Paths -StartIfMissing
  }

  Invoke-Step 'Validating setup-installed outputs' {
    Assert-InstalledOutputs -InstallDir $Paths.ExeInstallDir -Paths $Paths
  }

  if ($installerProcess -and -not $installerProcess.HasExited) {
    $installerProcess | Stop-Process -Force
  }

  Stop-SlanAppProcess
}

$paths = Get-SmokePaths `
  -Root $WorkspaceRoot `
  -ReleaseOverride $ReleaseDir `
  -OutputOverride $InstallerOutputDir

try {
  Invoke-Step 'Stopping running SLAN desktop app' {
    Stop-SlanAppProcess
  }

  if (-not $SkipBuild) {
    Invoke-Step 'Building Windows desktop release' {
      Push-Location $paths.ClientAppRoot
      try {
        flutter build windows
      } finally {
        Pop-Location
      }
    }
  }

  Invoke-Step 'Packaging installer payload and setup.exe' {
    & (Join-Path $paths.InstallerRoot 'package-installer.ps1') `
      -ReleaseDir $paths.ReleaseDir `
      -OutputDir $paths.OutputDir
  }

  Invoke-Step 'Validating packaged installer contents' {
    if (Use-NativeInstallerLayout -Paths $paths) {
      $required = @(
        (Join-Path $paths.OutputDir 'SLAN.generated.iss'),
        (Join-Path $paths.OutputDir 'slan_app.exe'),
        (Join-Path $paths.OutputDir 'app-core-helper.exe'),
        (Join-Path $paths.OutputDir 'app-core-service.exe'),
        (Join-Path $paths.OutputDir 'data'),
        $paths.OutputZip,
        $paths.SetupExe
      )
    } else {
      $required = @(
        (Join-Path $paths.OutputDir 'install-slan.ps1'),
        (Join-Path $paths.OutputDir 'install-slan.cmd'),
        (Join-Path $paths.OutputDir 'payload\slan_app.exe'),
        (Join-Path $paths.OutputDir 'payload\app-core-helper.exe'),
        (Join-Path $paths.OutputDir 'payload\app-core-service.exe'),
        $paths.OutputZip,
        $paths.SetupExe
      )
    }
    foreach ($path in $required) {
      if (-not (Test-Path $path)) {
        throw "Missing installer artifact: $path"
      }
    }
  }

  if (-not $ExecutableOnly) {
    Invoke-ScriptInstallerSmoke -Paths $paths
  }

  Invoke-ExecutableInstallerSmoke -Paths $paths

  Write-Host ''
  Write-Host 'WINDOWS_INSTALLER_SMOKE_OK'
  Write-Host "InstallerDir: $($paths.OutputDir)"
  Write-Host "ScriptInstallDir: $($paths.ScriptInstallDir)"
  Write-Host "ExeInstallDir: $($paths.ExeInstallDir)"
  Write-Host "StartupLog: $($paths.StartupLog)"
} finally {
  Invoke-Step 'Stopping installed SLAN app' {
    Stop-SlanAppProcess
  }

  $exeUninstaller = Join-Path $paths.ExeInstallDir 'unins000.exe'
  if (Test-Path $exeUninstaller) {
    Invoke-Step 'Uninstalling setup.exe smoke install' {
      & $exeUninstaller /VERYSILENT /SUPPRESSMSGBOXES /NORESTART
    }
  }

  if (Test-Path $paths.SmokeShortcut) {
    Remove-Item -Force $paths.SmokeShortcut
  }

  if (-not $KeepArtifacts) {
    foreach ($path in @($paths.ScriptInstallDir, $paths.ExeInstallDir, $paths.OutputDir, $paths.SmokeShortcutDir)) {
      if ($path -and (Test-Path $path)) {
        Remove-PathWithRetry -Path $path
      }
    }
    if (Test-Path $paths.OutputZip) {
      Remove-Item -Force $paths.OutputZip
    }
  }

  if (-not $KeepArtifacts) {
    Write-Host 'Cleaned temporary desktop shortcut.'
  }
}
