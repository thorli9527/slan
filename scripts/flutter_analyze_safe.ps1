param(
  [string]$ProjectDir = '',
  [int]$StaleLanguageServerMinutes = 0,
  [switch]$KillAllLanguageServers,
  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$AnalyzeArgs
)

$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ProjectDir)) {
  $ProjectDir = Join-Path $RootDir 'client\app'
}
$ProjectDir = [System.IO.Path]::GetFullPath($ProjectDir)
$LockDir = Join-Path $RootDir '.tmp'
$LockPath = Join-Path $LockDir 'flutter-analyze.lock'

function Stop-DartLanguageServers {
  $now = Get-Date
  $processes = @()
  try {
    $processes = Get-CimInstance Win32_Process |
      Where-Object {
        $_.CommandLine -and
        ($_.CommandLine -like '*dart.exe language-server*' -or
          $_.CommandLine -like '*dart language-server*')
      } |
      ForEach-Object { Get-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
  } catch {
    Write-Host '[flutter-analyze] cannot read Dart command lines; falling back to dart.exe cleanup'
    $processes = Get-Process dart -ErrorAction SilentlyContinue
  }

  foreach ($process in $processes) {
    $age = $now - $process.StartTime
    $isStale = $KillAllLanguageServers -or
      $age.TotalMinutes -ge $StaleLanguageServerMinutes
    if (-not $isStale) {
      continue
    }
    Write-Host "[flutter-analyze] stopping Dart language-server pid=$($process.Id) age=$([int]$age.TotalMinutes)m"
    Stop-Process -Id $process.Id -Force
  }
}

function Enter-FlutterToolLock {
  New-Item -ItemType Directory -Force -Path $LockDir | Out-Null
  $deadline = (Get-Date).AddSeconds(120)
  while ((Get-Date) -lt $deadline) {
    try {
      return [System.IO.File]::Open(
        $LockPath,
        [System.IO.FileMode]::OpenOrCreate,
        [System.IO.FileAccess]::ReadWrite,
        [System.IO.FileShare]::None
      )
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }
  throw "Timed out waiting for Flutter tool lock: $LockPath"
}

$lock = $null
try {
  Stop-DartLanguageServers
  $lock = Enter-FlutterToolLock
  Push-Location $ProjectDir
  try {
    Write-Host "[flutter-analyze] project=$ProjectDir"
    if ($AnalyzeArgs -and $AnalyzeArgs.Length -gt 0) {
      flutter analyze @AnalyzeArgs
    } else {
      flutter analyze
    }
    if ($LASTEXITCODE -ne 0) {
      throw "flutter analyze failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
} finally {
  if ($lock) {
    $lock.Dispose()
  }
}
