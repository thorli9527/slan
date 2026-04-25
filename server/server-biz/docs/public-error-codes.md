# Public API Error Codes

Public HTTP APIs return errors as:

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "human readable detail"
}
```

Stable codes currently used by client-facing APIs:

- `INVALID_ARGUMENT`: malformed JSON, missing fields, invalid request state, or unsupported method.
- `UNAUTHORIZED`: missing or invalid bearer token.
- `FORBIDDEN`: authenticated caller cannot access the target resource.
- `NOT_FOUND`: route or resource does not exist, or is not visible in the current context.
- `CONFLICT`: request conflicts with existing state.
- `REQUEST_TOO_LARGE`: JSON request body exceeds the 1 MiB limit.
- `RATE_LIMITED`: request was throttled. Check `Retry-After` when present.
- `NOT_IMPLEMENTED`: optional public capability is disabled on this deployment.
- `INTERNAL`: unexpected server error.

Ops-only APIs may later define additional operator-facing codes, but should not
change the public API codes above.
