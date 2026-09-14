# Phase 1 validation

Approved scope: baseline characterization, private cycle seams, macOS/Linux CI, ARMv7 builds, and runbook corrections. Implementation starts at `2a86333` on `codex/phase-1-cycle-baseline`. The shared checkout's Wi-Fi SDIO, stock-noise masking, and skill changes are excluded. The Phase 0 documents are included and updated to map this branch.

## Why and compatibility

The cycle previously called device effects directly, preventing deterministic whole-cycle checks on the development host. `App` now owns private function dependencies for those effects and writes. Production defaults still call the same implementations. Network acquisition, installation, and scheduling expose small private dependency helpers for their ordering contracts.

No exported function signatures, CLI behavior, JSON fields/defaults, file formats, device commands, or retry policies change. No migration or data conversion is required. Existing failure quirks are captured rather than corrected; later intentional changes require a migration note.

## Evidence

Local checks on 2026-09-14:

| Check | Result |
| --- | --- |
| Go 1.26.2, darwin/arm64: `go test -count=1 -race -cover ./...` | Pass; `internal/trmnl` statement coverage 73.0%. |
| Host `go vet ./...` | Pass. |
| Go 1.26.2, Linux/arm64 Docker: same test command | Pass; `internal/trmnl` statement coverage 55.3%, including Linux mode, scheduling, install, and whole-cycle tests. |
| Linux `go vet ./...` | Pass. |
| `GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0` executable build and test compilation | Pass. |
| Documentation links, code fences, diff whitespace | Checked before PR delivery. |

Linux checks ran in `golang:1.26.2`, mounting this checkout read-only and using separate module/build cache volumes. The [CI workflow](../.github/workflows/checks.yml) repeats tests/race checks/vet on macOS and Linux and builds the Linux ARMv7 executable and test binary. Its action revisions are pinned to the official checkout v7.0.1 and setup-go v7.0.0 releases. GitHub reports the check results for each PR revision.

## Behavioral baseline

- [`cycle_test.go`](../internal/trmnl/cycle_test.go) drives `App.Run` with real config and file persistence, a fixed clock, fixture HTTP transport, and device-effect recorders. It verifies cache/render payloads, display headers, success/unchanged/full-refresh behavior, mode outcomes, ordering, cleanup counts, counters, timestamps, restore metadata, and failure finalization. Wi-Fi setup/timeout, HTTP/JSON/image reads, cache/render/log/state/scheduling/suspend errors, and best-effort failures are covered.
- [`testdata/cycle`](../internal/trmnl/testdata/cycle) fixes a small PNG, previous state including restore metadata, successful state JSON, and cycle JSONL. Tests compare successful output bytes exactly and characterize failure state/log fields explicitly. These are cycle fixtures, not hardware-rendering golden images.
- [`contracts_test.go`](../internal/trmnl/contracts_test.go) preserves config defaults, nonpositive fallback, nested precedence, validation bounds, CLI output/argument behavior, XDG paths, configured device-ID platform differences, state round trips, JSONL append, and 0600 state/log permissions.
- [`cycle_linux_test.go`](../internal/trmnl/cycle_linux_test.go) composes the real Linux runtime-mode and scheduler policy into the cycle. It verifies sentinel/USB/boot-grace/recovery precedence, RTC versus awake timers, fallback, and persistence/network cleanup before suspend.
- [`install_linux_test.go`](../internal/trmnl/install_linux_test.go) records installed artifacts and command order. The simulated first cycle reads the persisted restore metadata and writes its own state; the test proves installation leaves that state intact. A failed state save prevents start. The resume hook remains `post`-only and nonblocking.
- Existing runtime-mode, restore, and alternating-timer tests remain in place. New scheduler cases cover RTC failure, timer failure, and the existing distinction between wrapped RTC errors and formatted timer errors.

## Self-review

The final production diff was checked against the baseline. After normalizing only the introduced dependency names, the bodies of `runOnce`, `finishCycle`, network acquisition, installation, and next-cycle planning match their original control flow exactly. Default adapters map to the original functions; there is no package extraction or global mutable override.

Three temporary Go overlays deliberately introduced regressions: forcing a full first render, omitting cleanup before suspend, and saving state before the log. The changed-cycle golden test failed for all three. The overlays did not modify repository files. This confirms the trace/fixture checks detect those ordering and cadence regressions.

Review also checked that fixtures use temporary XDG paths and explicit device IDs, and that Linux tests substitute all mutating device effects. No unit test invokes real systemd, suspend, rfkill, or SDIO writes.

## Limits and rollback

Coverage percentages describe different platform builds and are not acceptance thresholds. CLI command dispatch is tested through `App.Run`; the small executable wrapper has no direct test. No physical RM1, FBInk output, battery measurements, actual Wi-Fi association, or suspend/resume lifecycle was exercised. The new code keeps those implementations intact; device-specific evidence is required before a later hardware rollout.

Revert the Phase 1 PR or restore the previous binary to roll back. Keep current compatible configuration and state, especially stock-service restore metadata. There are no installed-artifact or schema changes in this phase. The [operations rollback procedure](operations.md#roll-back-a-binary-update) describes quiescing cycles and retaining backups; its device execution remains unverified.

Phase 2 and deployment require separate approval.
