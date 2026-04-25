$ErrorActionPreference = 'Stop'

$RootDir = Split-Path -Parent $PSScriptRoot

Write-Host "[protocol-contracts] checking web transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target web

Write-Host "[protocol-contracts] checking flutter transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target flutter

Write-Host "[protocol-contracts] checking rust controller transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target rust-controller

Write-Host "[protocol-contracts] checking go server transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target go-server

Write-Host "[protocol-contracts] checking OpenAPI transport contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target openapi

Write-Host "[protocol-contracts] checking protobuf control contracts"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target protobuf

Write-Host "[protocol-contracts] checking public HTTP routes"
go run "$RootDir/scripts/check_protocol_contracts.go" --root "$RootDir" --target http-routes

Write-Host "[protocol-contracts] all checks passed"
