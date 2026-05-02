param(
  [string]$FlutterArgs = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$argsList = @("test", "test/home_page_ui_test.dart")
if ($FlutterArgs.Trim().Length -gt 0) {
  $argsList += $FlutterArgs.Split(" ", [System.StringSplitOptions]::RemoveEmptyEntries)
}

flutter @argsList
