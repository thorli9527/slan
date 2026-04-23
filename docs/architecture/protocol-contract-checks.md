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

## Coverage

Current checks cover:

- `protocol/contracts/*.yaml` -> web `server/server-ui/web/src/ui/api-contracts.ts`
- `protocol/contracts/*.yaml` -> Flutter
  `client/app/lib/infra/api_contracts/request_models.dart`
- `protocol/contracts/*.yaml` -> Flutter transport DTO subset in
  `client/app/lib/infra/control_api_responses/response_dtos.dart`

## Expected Result

Successful runs print:

```text
[protocol-contracts] checking web transport contracts
protocol contract check passed for target=web
[protocol-contracts] checking flutter transport contracts
protocol contract check passed for target=flutter
[protocol-contracts] all checks passed
```
