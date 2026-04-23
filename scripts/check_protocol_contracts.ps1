$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot

Write-Host "[protocol-contracts] checking web transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target web

Write-Host "[protocol-contracts] checking flutter transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target flutter

Write-Host "[protocol-contracts] all checks passed"
