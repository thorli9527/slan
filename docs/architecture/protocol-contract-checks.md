# Protocol Contract Checks

This note records the local entrypoints for the protocol drift checks added
during the M3 contract-unification pass.

## Entrypoints

Unix-like shells:

```bash
./scripts/check_protocol_contracts.sh
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\check_protocol_contracts.ps1
```

Make target when `make` is available:

```bash
make protocol-contract-check
```

Checker unit tests:

```bash
go test scripts/check_protocol_contracts.go scripts/check_protocol_contracts_test.go
```

## Coverage

Current checks cover:

- `protocol/contracts/*.yaml` -> web `server/server-ui/web/src/ui/api-contracts.ts`
- `protocol/contracts/*.yaml` -> Flutter
  `client/app/lib/infra/api_contracts/request_models.dart`
- `protocol/contracts/*.yaml` -> Flutter transport DTO subset in
  `client/app/lib/infra/control_api_responses/response_dtos.dart`
- `protocol/contracts/*.yaml` -> Rust controller client DTOs in
  `client/app_core/crates/controller-client/src/dto.rs`
- `protocol/contracts/*.yaml` -> Go server DTOs in
  `server/server-biz/api/dto`
- `protocol/contracts/*.yaml` -> OpenAPI schemas in
  `protocol/openapi/phase1.yaml`
- `protocol/contracts/*.yaml` -> protobuf control-channel messages in
  `protocol/protobuf/control.proto`
- Public Go HTTP routes in `server/server-biz/api/http` -> OpenAPI
  paths/methods in `protocol/openapi/phase1.yaml`

If local PowerShell policy blocks `.ps1` execution, run the underlying
checks directly:

```powershell
go run scripts/check_protocol_contracts.go --root . --target web --target flutter --target rust-controller --target go-server --target openapi --target protobuf --target http-routes
```

## Expected Result

Successful runs print:

```text
[protocol-contracts] checking web transport contracts
protocol contract check passed for target=web
[protocol-contracts] checking flutter transport contracts
protocol contract check passed for target=flutter
[protocol-contracts] checking rust controller transport contracts
protocol contract check passed for target=rust-controller
[protocol-contracts] checking go server transport contracts
protocol contract check passed for target=go-server
[protocol-contracts] checking OpenAPI transport contracts
protocol contract check passed for target=openapi
[protocol-contracts] checking protobuf control contracts
protocol contract check passed for target=protobuf
[protocol-contracts] checking public HTTP routes
protocol contract check passed for target=http-routes
[protocol-contracts] all checks passed
```
