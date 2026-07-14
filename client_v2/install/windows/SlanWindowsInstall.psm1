Set-StrictMode -Version Latest

function Get-SlanWindowsInstallManifest {
  $clientRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
  $programDataRoot = if ([string]::IsNullOrWhiteSpace($env:ProgramData)) {
    'C:\ProgramData'
  } else {
    $env:ProgramData
  }
  $localAppDataRoot = if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    Join-Path $HOME 'AppData\Local'
  } else {
    $env:LOCALAPPDATA
  }
  $roamingAppDataRoot = if ([string]::IsNullOrWhiteSpace($env:APPDATA)) {
    Join-Path $HOME 'AppData\Roaming'
  } else {
    $env:APPDATA
  }
  $programDataDir = Join-Path $programDataRoot 'SLAN'
  $relayDiagnoseTool = [pscustomobject]@{
    Source = 'app_flutter\tool\test-windows-multipeer-relay.ps1'
    Destination = 'tools\test-windows-multipeer-relay.ps1'
    Required = $true
  }
  $consoleTool = [pscustomobject]@{
    Source = 'install\windows\slan-console.ps1'
    Destination = 'tools\slan-console.ps1'
    Required = $true
  }
  $installModuleTool = [pscustomobject]@{
    Source = 'install\windows\SlanWindowsInstall.psm1'
    Destination = 'tools\SlanWindowsInstall.psm1'
    Required = $true
  }

  [pscustomobject]@{
    ClientRoot = $clientRoot
    AppName = 'SLAN Client V2'
    AppVersion = '0.1.0'
    AppExeName = 'slan_client_v2.exe'
    ServiceExeName = 'client-core-service.exe'
    ServiceName = 'SLANClientV2Service'
    ServiceDisplayName = 'SLAN Client V2 Service'
    AdapterName = 'SLAN LAN Adapter'
    InstallDir = Join-Path $localAppDataRoot 'Programs\SLAN Client V2'
    ProgramDataDir = $programDataDir
    DefaultControlBaseUrl = 'http://47.245.40.231:28080'
    RelayDiagnoseToolDestination = $relayDiagnoseTool.Destination
    RequiredReleaseFiles = @(
      'slan_client_v2.exe',
      'client-core-service.exe',
      'wintun.dll',
      'flutter_windows.dll',
      'client_core_plugin_plugin.dll'
    )
    RequiredReleaseDirectories = @(
      'data'
    )
    PackagedTools = @(
      $relayDiagnoseTool,
      $consoleTool,
      $installModuleTool
    )
    LegacyTaskNames = @(
      'SLAN Client V2 Helper',
      'SLAN Client V2 Service'
    )
    StateFiles = @(
      'config.json',
      'client-v2-session.json',
      'client-v2-control-tasks.xml',
      'client-v2-device-id.txt',
      'client-v2-network-state.json',
      'client-v2-assigned-ip.txt',
      'client-v2-relay-stats.json',
      'client-v2-relay-policy.json',
      'mqtt-inbox.xml'
    )
    AppDataDirectories = @(
      (Join-Path $roamingAppDataRoot 'slan_client_v2'),
      (Join-Path $localAppDataRoot 'slan_client_v2'),
      (Join-Path $roamingAppDataRoot 'SLAN Client V2'),
      (Join-Path $localAppDataRoot 'SLAN Client V2')
    )
  }
}

function Resolve-SlanWindowsPath {
  param(
    [Parameter(Mandatory = $true)][string]$BasePath,
    [Parameter(Mandatory = $true)][string]$RelativePath
  )

  [System.IO.Path]::GetFullPath((Join-Path $BasePath $RelativePath))
}

function Assert-SlanWindowsReleaseRuntime {
  param(
    [Parameter(Mandatory = $true)][string]$ReleasePath,
    [Parameter(Mandatory = $true)]$Manifest
  )

  foreach ($name in $Manifest.RequiredReleaseFiles) {
    $path = Join-Path $ReleasePath $name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "Missing required release runtime component: $path"
    }
  }

  foreach ($name in $Manifest.RequiredReleaseDirectories) {
    $path = Join-Path $ReleasePath $name
    if (-not (Test-Path -LiteralPath $path -PathType Container)) {
      throw "Missing required release runtime directory: $path"
    }
  }
}

function Resolve-SlanWindowsIsccPath {
  $candidates = @(
    (Get-Command ISCC.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -ErrorAction SilentlyContinue),
    'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
    'C:\Program Files\Inno Setup 6\ISCC.exe'
  ) | Where-Object { $_ }

  foreach ($candidate in $candidates) {
    if (Test-Path -LiteralPath $candidate -PathType Leaf) {
      return $candidate
    }
  }
  return $null
}

function Copy-SlanWindowsPackagedTools {
  param(
    [Parameter(Mandatory = $true)][string]$StageDir,
    [Parameter(Mandatory = $true)]$Manifest
  )

  foreach ($tool in $Manifest.PackagedTools) {
    $source = Resolve-SlanWindowsPath -BasePath $Manifest.ClientRoot -RelativePath $tool.Source
    $destination = Join-Path $StageDir $tool.Destination
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
      if ($tool.Required) {
        throw "Missing Windows packaged tool: $source"
      }
      continue
    }

    $destinationParent = Split-Path -Parent $destination
    New-Item -ItemType Directory -Force -Path $destinationParent | Out-Null
    Copy-Item -Path $source -Destination $destination -Force

    if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
      throw "Failed to stage Windows packaged tool: $destination"
    }
  }
}

function New-SlanWindowsInnoSetupScript {
  param(
    [Parameter(Mandatory = $true)][string]$TemplatePath,
    [Parameter(Mandatory = $true)][string]$StageDir,
    [Parameter(Mandatory = $true)][string]$GeneratedPath,
    [Parameter(Mandatory = $true)]$Manifest
  )

  $sourceDir = [System.IO.Path]::GetFullPath($StageDir).Replace('\', '\\')
  $outputDir = [System.IO.Path]::GetFullPath($StageDir).Replace('\', '\\')
  $content = Get-Content -LiteralPath $TemplatePath -Raw
  $defines = @(
    "#define SourceDir `"$sourceDir`"",
    "#define OutputDir `"$outputDir`"",
    "#define MyAppVersion `"$($Manifest.AppVersion)`""
  ) -join "`r`n"
  Set-Content -Path $GeneratedPath -Value ($defines + "`r`n" + $content) -Encoding UTF8
}

Export-ModuleMember -Function `
  Get-SlanWindowsInstallManifest, `
  Resolve-SlanWindowsPath, `
  Assert-SlanWindowsReleaseRuntime, `
  Resolve-SlanWindowsIsccPath, `
  Copy-SlanWindowsPackagedTools, `
  New-SlanWindowsInnoSetupScript
