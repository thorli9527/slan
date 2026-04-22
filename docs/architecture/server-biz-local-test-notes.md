# Server Biz Local Test Notes

This note records the local test setup changes made for `server/server-biz`
during the M2 rollout work.

## Why This Changed

The original sqlite-backed Go tests depended on `mattn/go-sqlite3`, which
requires CGO and a local C toolchain.

On the Windows validation machine used for M2:

- `CGO_ENABLED=0`
- `gcc`, `clang`, and `zig` were not installed

That meant the sqlite-backed Go tests could not run even when the application
code itself was otherwise fine.

## What Changed

### 1. Pure Go sqlite for tests

The sqlite-backed tests now use `modernc.org/sqlite` through GORM's configurable
sqlite driver entrypoint.

Files updated:

- `server/server-biz/configs/runtime_test.go`
- `server/server-biz/internal/service/impl/network_test.go`
- `server/server-biz/go.mod`
- `server/server-biz/go.sum`

### 2. In-memory token store for service tests

`dbState.tokens` was widened from a concrete Redis type to a small interface,
and a test-only in-memory token store was added.

That keeps service tests self-contained and avoids a hidden dependency on a live
Redis instance when a test only needs token semantics.

Files updated:

- `server/server-biz/internal/service/impl/state.go`
- `server/server-biz/internal/service/impl/token_store_test.go`

## Validation

The following commands passed on Windows after the change:

```bash
go test ./configs/...
go test ./internal/service/impl/...
go test ./...
```

## Note About `go.mod`

Adding the pure Go sqlite driver caused the local Go toolchain to keep
`server/server-biz/go.mod` at `go 1.25.0`.

Attempting to force the file back to the older `go 1.23` plus
`toolchain go1.24.4` combination immediately led `go test` to request another
module-file update in this environment.

For now, the checked-in state should favor "module file consistent with passing
tests" over preserving the previous top-level `go` declaration.
