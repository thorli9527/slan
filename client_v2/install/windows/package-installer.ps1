param(
  [string]$ReleaseDir = "$PSScriptRoot\..\..\app_flutter\build\windows\x64\runner\Release",
  [string]$OutputDir = "$PSScriptRoot\..\..\.tmp\installer\slan-client-v2-windows"
)

$ErrorActionPreference = 'Stop'

function Assert-ReleaseRuntime {
  param([string]$ReleasePath)

  $required = @(
    'slan_client_v2.exe',
    'client-core-helper.exe',
    'client-core-service.exe',
    'flutter_windows.dll',
    'client_core_plugin_plugin.dll'
  )
  foreach ($name in $required) {
    $path = Join-Path $ReleasePath $name
    if (-not (Test-Path $path)) {
      throw "Missing required release runtime component: $path"
    }
  }

  $dataDir = Join-Path $ReleasePath 'data'
  if (-not (Test-Path $dataDir)) {
    throw "Missing required release runtime directory: $dataDir"
  }
}

function Resolve-IsccPath {
  $candidates = @(
    (Get-Command ISCC.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source -ErrorAction SilentlyContinue),
    'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
    'C:\Program Files\Inno Setup 6\ISCC.exe'
  ) | Where-Object { $_ }

  foreach ($candidate in $candidates) {
    if (Test-Path $candidate) {
      return $candidate
    }
  }
  return $null
}

function New-InnoSetupScript {
  param(
    [string]$TemplatePath,
    [string]$StageDir,
    [string]$GeneratedPath
  )

  $sourceDir = [System.IO.Path]::GetFullPath($StageDir).Replace('\', '\\')
  $outputDir = [System.IO.Path]::GetFullPath($StageDir).Replace('\', '\\')
  $content = Get-Content $TemplatePath -Raw
  $content = "#define SourceDir `"$sourceDir`"`r`n#define OutputDir `"$outputDir`"`r`n" + $content
  Set-Content -Path $GeneratedPath -Value $content -Encoding UTF8
}

$releasePath = (Resolve-Path $ReleaseDir).Path
Assert-ReleaseRuntime -ReleasePath $releasePath
$outputPath = [System.IO.Path]::GetFullPath($OutputDir)
$zipPath = "$outputPath.zip"
$setupExePath = Join-Path $outputPath 'SLAN-Client-V2-Setup.exe'
$issTemplatePath = Join-Path $PSScriptRoot 'SLAN-Client-V2.iss'
$generatedIssPath = Join-Path $outputPath 'SLAN-Client-V2.generated.iss'
$isccPath = Resolve-IsccPath

if (Test-Path $outputPath) {
  Remove-Item -Recurse -Force $outputPath
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null
Copy-Item -Path (Join-Path $releasePath '*') -Destination $outputPath -Recurse -Force
New-InnoSetupScript -TemplatePath $issTemplatePath -StageDir $outputPath -GeneratedPath $generatedIssPath

if (Test-Path $zipPath) {
  Remove-Item -Force $zipPath
}
Compress-Archive -Path (Join-Path $outputPath '*') -DestinationPath $zipPath -Force

if ($isccPath) {
  & $isccPath $generatedIssPath | Out-Null
  if (-not (Test-Path $setupExePath)) {
    throw "Failed to create native installer executable: $setupExePath"
  }
} else {
  Write-Warning 'Inno Setup (ISCC.exe) was not found. Generated a native installer stage, but did not compile SLAN-Client-V2-Setup.exe.'
}

Write-Host "Installer directory: $outputPath"
Write-Host "Installer archive: $zipPath"
if (Test-Path $setupExePath) {
  Write-Host "Installer executable: $setupExePath"
} else {
  Write-Host "Installer script: $generatedIssPath"
}
