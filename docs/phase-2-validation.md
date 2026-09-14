# Phase 2 validation

Phase 1 [PR #4](https://github.com/robinsandborg/trmnl-rm1/pull/4) was merged with the user's approval as `dba4fb7`. Phase 2 starts there on `codex/phase-2-byos` and covers only the BYOS package extraction. The shared checkout's Wi-Fi SDIO/stock-noise changes remain excluded. Device verification is deferred until the completed refactor, as authorized on 2026-09-14.

## Why and resulting interface

[`internal/byos`](../internal/byos/fetch.go) now owns display fetching, image URL resolution/download, and refresh clamping. Its `Fetch` interface accepts the existing HTTP client plus BYOS request values and returns original response metadata, raw image bytes, the resolved image URL, and the effective interval. `RefreshPolicy` receives already-effective fallback/min/max durations; it does not acquire platform dependencies or define application defaults.

[`internal/trmnl/byos.go`](../internal/trmnl/byos.go) retains device-ID resolution and configuration defaults and maps the result into the existing `fetchCyclePayload` return shape. The cycle call site, all Phase 1 fixtures, persisted types, and CLI remain unchanged. BYOS imports only the Go standard library; it does not import `trmnl`.

The matching `TerminalResponse` types deliberately retain the original JSON field names and type name. Go's JSON decoder includes that name in type-error messages. Self-review caught an initial rename that would have changed logged error text; an exact error regression test now covers it.

## Checks

Local results on 2026-09-14, Go 1.26.2:

| Check | Result |
| --- | --- |
| macOS/arm64 `go test -count=1 -race -cover ./...` and `go vet ./...` | Pass; BYOS coverage 97.9%, trmnl 70.9%. |
| Linux/arm64 in `golang:1.26.2`: same tests and vet | Pass; BYOS coverage 97.9%, trmnl 53.0%. |
| Linux ARMv7 executable build | Pass. |
| Linux ARMv7 test compilation for **all packages** | Pass; CI now compiles each package's tests, including BYOS. |
| Pre-extraction comparison on macOS and Linux | All trmnl tests, including new facade tests and the unchanged Phase 1 cycle corpus, pass with the old fetch/clamp implementation. |

The comparison used temporary Go source overlays: restore `app.go` from `dba4fb7` and substitute an empty package file for the new facade. This exercises the same facade assertions, persisted fixtures, and cycle outcomes against both implementations without retaining duplicate legacy code in the repository. The new black-box BYOS suite additionally covers relative/absolute/scheme-relative URLs, headers, unmodified image bytes, optional/malformed/missing JSON, status/transport/read errors, zero results on failure, body closure, client timeout/redirect policy, and refresh bounds.

The production fetch and clamp bodies also match the old control flow after normalizing their input and output mappings. Self-review checked that the facade preserves original versus resolved image URLs, effective configuration defaults, identity resolution, and error behavior, and that no package dependency points back into `trmnl`. Documentation links and diff whitespace were checked before PR delivery. GitHub reports macOS, Linux, and ARMv7 checks for each PR revision.

## Compatibility and rollback

Existing exported Go surfaces, CLI commands, BYOS requests, JSON/defaults, response/error semantics, and data files are preserved. No migration, backfill, compatibility window, or device configuration change is required. The new symbols are inside an internal package. HTTP/image resource limits and other known debt are unchanged.

Rollback is reverting the Phase 2 PR or restoring the preceding binary. Configuration/state remain compatible and must be retained, including stock-service restore metadata. No unit/hook changes or on-device installation occurred.

Device rendering, actual Wi-Fi association, power behavior, and suspend/restore remain unverified until the final RM1 verification. Phase 3 retains its separate approval gate.
