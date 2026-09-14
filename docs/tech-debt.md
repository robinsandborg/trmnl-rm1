# Technical debt

Inventory updated after extraction, 2026-09-14. The [architecture map](architecture.md) describes the extracted modules and legacy compatibility facade. Phase 0 observations about uncommitted Wi-Fi SDIO and stock-noise work are called out below; they are excluded from this PR. Priorities below are local sequencing judgments, not GitHub triage labels. Risks inferred from code are not claimed as reproduced device failures. Phase 1 adds characterization and documentation corrections; Phases 2 and 3 extract BYOS and display while preserving the runtime risks below.

## TD-01 — Cycle behavior lacks characterization

**Phase 1 baseline established; extend per extraction.** [`cycle/cycle.go`](../internal/cycle/cycle.go) now owns state transitions, render policy, effect ordering, and failure finalization; [`trmnl/app.go`](../internal/trmnl/app.go) retains CLI dispatch and initial DTO loading/validation. Phase 2 moves both HTTP requests, URL resolution, and interval clamping to [`internal/byos`](../internal/byos/fetch.go) behind the existing facade. `App` now owns private effect dependencies. Cycle tests exercise `App.Run` with real configuration, state/cache/log I/O and BYOS decoding, checking ordered effects and failure outcomes; [fixtures and coverage](phase-1-validation.md) define the extraction baseline.

The difficult contract is effect ordering: state/log writes before suspend, cleanup on failures, counters after a successful render followed by scheduling failure, and fallback errors. The uncommitted SDIO work observed in Phase 0 additionally needs enumeration-before-validation characterization when it is integrated. A refactor could silently change recoverability. Use the captured outputs and effect traces when migrating callers one seam at a time. Preserve current quirks unless separately approved with a migration note.

## TD-02 — Host checks omit device-critical paths

**Automated platform gap addressed in Phase 1; hardware gap remains.** Build tags exclude Linux implementations on macOS. [CI](../.github/workflows/checks.yml) now runs both platforms with race detection/vet plus ARMv7 builds. The local Linux suite executes in Docker. Cycle/persistence/network-lifecycle/install-order contracts are covered. Phase 3 adds portable pixel fixtures and Linux renderer command/failure traces. Actual association, sysfs power effects, full install rollback, and device timing still need device evidence.

Run host and Linux tests plus ARMv7 builds. Add meaningful contract fixtures and failure cases per seam, rather than relying on a coverage percentage. Hardware smoke checks remain necessary for FBInk orientation, repeated suspend/resume, SDIO rebinding, and service restoration; cross-compilation cannot establish these properties.

## TD-03 — Restore metadata and cycle state share fragile persistence

**High; persistence contract captured, durability/lifecycle risks remain.** [`saveState`](../internal/trmnl/config.go) truncates and rewrites one JSON file. No atomic replacement or cycle lock exists. `State` combines replaceable display history with stock-service restoration metadata. A crash during a write or overlapping manual/service cycles could lose metadata or overwrite newer state.

[`runInstall`](../internal/trmnl/install_linux.go) changes service state before saving its snapshot; a failure in between can leave side effects without durable recovery metadata. Reinstall reads current enabled states again and can overwrite the original snapshot. Restore always enables xochitl and records only enabled status for other units, not a complete original active/masked state. These are concrete implementation limits, not proof of a device incident.

First preserve the existing file contract behind a storage seam. Atomic writes, locking, durable installation progress, or a split/versioned state format need separately approved changes, failure tests, and downgrade instructions; do not hide them in a file move.

## TD-04 — Recovery scheduling and diagnostics are inconsistent

**High; characterized in Phase 1, behavior unchanged.** Initial loading in [`trmnl/app.go`](../internal/trmnl/app.go) and some file/mode failures in [`cycle/cycle.go`](../internal/cycle/cycle.go) bypass failure finalization. `finishCycle` ignores mode/scheduling errors after persistence. An RTC alarm can be armed during failure finalization even though that path never suspends. Failure recovery therefore does not uniformly guarantee an awake retry. Network cleanup errors and battery read errors are also discarded.

Success records `LastMode` before RTC fallback changes the effective mode. A suspend failure can append a second log after a success log/state write. Scheduling failure occurs after success counters have been reset. The cycle tests now preserve these observable results; changing error categories, counter semantics, retry behavior, or log shape requires a focused behavior change and migration note.

## TD-05 — Hardware policy and low-level effects are intertwined

**High for extraction risk.** [`network_linux.go`](../internal/network/network_linux.go) combines link control and real-time connectivity polling. The uncommitted implementation observed in Phase 0 additionally mixes systemd, rfkill, SDIO discovery, and fixed sysfs paths; that work is outside this PR. Phase 3 separates portable crop/grayscale/rotation into [`display/prepare.go`](../internal/display/prepare.go) and Linux FBInk effects into [`display/render_linux.go`](../internal/display/render_linux.go), with a command runner supplied by the facade. Pixel and command contracts are captured; physical rendering remains unverified. [`power_linux.go`](../internal/power/power_linux.go) combines battery sampling, runtime observations, RTC writes, timers, and suspend.

Existing runner/dependency seams in power, runtime mode, and restore provide a starting point. Keep their device-specific ordering: A/B timers must not stop their own service; the resume hook must remain nonblocking; the pending SDIO changes require Wi-Fi re-enumeration before MAC fallback. Historical fixes `379df59`, `0ccf51d`, and `2d6db09` show these orderings have already mattered. Move one cohesive module at a time and verify command traces plus actual device behavior where applicable.

## TD-06 — Operations and deployment drift

**Documentation corrected in Phase 1; runtime risks remain.** [`operations.md`](operations.md) now names both `next-a` and `next-b`, explains how to quiesce cycles, preserves state during refresh/rollback, and documents deployment-helper limitations. [`runRestoreWithOps`](../internal/trmnl/install_common.go) disables the appliance service but does not stop either transient timer/service; a pending timer could restart cycling after restore. Timer cleanup is a behavior fix requiring its own approval, beyond correcting documentation.

The old runbook suggested deleting `state.json` to force a full refresh, losing install metadata without forcing a full first render. That instruction has been replaced with a direct FBInk redraw of the prepared file. [`deploy.sh`](../deploy/deploy.sh) checks FBInk under `/home/root/bin` but its missing-tool message says `/usr/local/bin`. It overwrites binary/config without retaining a previous version and clears the maintenance sentinel after installation even if it predated deployment.

The rollback procedure retains the previous binary and compatible configuration/state; physical-device verification remains pending before rollout. Separately approve changes to restore/deploy behavior and guard them with lifecycle tests. Keep destructive device experiments outside automated unit tests.

## TD-07 — Configuration contracts are implicit

**Medium; BYOS mapping characterized in Phase 2 and display mapping in Phase 3.** [`types.go`](../internal/trmnl/types.go) mixes wire structures, installation state, defaults, and policy helpers. Effective accessors replace nonpositive values with defaults; several validation checks therefore cannot reject raw nonpositive input. Rotation zero also selects the default. Nested and top-level full-refresh fields coexist; `framebuffer_device` is reserved but unused. Linux and non-Linux configured device-ID trimming differ.

Characterize omitted/zero/negative fields, nested precedence, unknown JSON fields, and platform differences. Preserve public Go surfaces and serialized forms during package moves using legacy wrappers and explicit narrow mappings. Changing accepted inputs or precedence is a compatibility change, not incidental cleanup.

## TD-08 — Resource limits and command cancellation are absent

**Medium; evaluate separately from structural work.** [`byos.Fetch`](../internal/byos/fetch.go) retains unbounded `io.ReadAll` for images; decoding/scaling can allocate according to input dimensions. HTTP requests have timeouts, but [`system.go`](../internal/trmnl/system.go) uses `exec.Command` without a deadline. Cycle logs append indefinitely. On a constrained appliance, large inputs, a stuck command, or growing logs could exhaust resources or prevent a cycle from finishing.

Body/pixel limits, command cancellation, and log retention would change accepted input or runtime behavior. Define limits and failure semantics in a separately approved change, with representative payloads and rollback notes.

## Proposed order and verification limits

Follow [the strangler plan](refactoring-plan.md): establish behavioral evidence, extract leaf modules, then move orchestration and retire forwarding code. This inventory is not an instruction to fix every item in one PR. Phase 3 verifies host and Linux suites/vet, compiles the executable and all package tests for ARMv7, and compares display pixels and facade/cycle tests against pre-extraction code. Physical-device behavior remains unverified and is deferred until the completed refactor as authorized; see [validation evidence](phase-3-validation.md).

## Phase 4 status

Storage file layout and I/O are extracted behind the existing DTOs and defaults. State/log golden fixtures, missing/malformed JSON, cache bytes, modes, and encode/open failure ordering are covered. TD-03 durability, locking, and metadata lifecycle behavior remain unchanged. See [Phase 4 validation](phase-4-validation.md).

## Phase 5 status

Network acquisition, identity, and link/connectivity operations are extracted. Command/identity/connectivity tests complement the existing cycle cleanup corpus. Pending shared-checkout SDIO changes remain outside extraction; enumeration-before-validation and repeated physical Wi-Fi cycles must be resolved in final device verification. See [validation](phase-5-validation.md).

## Phase 6 status

Power and mode policy now live in `internal/power`; existing precedence and scheduling traces still apply. New tests exercise battery absence/optional fields, RTC file writes/fallback, and suspend command fallback without device effects. Physical wake and recovery evidence remains pending. See [validation](phase-6-validation.md).

## Phase 7 status

Appliance install/restore is isolated behind command/file operations and a three-field service snapshot. Legacy full-state persistence remains in the facade. Save-before-start, nonblocking resume, aggregate restore errors, and missing-artifact behavior remain covered. Reinstallation metadata and pending-timer cleanup risks are unchanged; final recovery fixes must have separate notes. See [validation](phase-7-validation.md).

## Phase 9 status

Cycle orchestration is extracted and composed through the original compatibility facade. Existing exact state/log/image fixtures and event traces pass through the final composition. Obsolete command-fallback code and filename constants were removed. Optional Phase 8 behavior work is reserved for concrete recovery/device findings; the durability and early-retry debt above is not silently changed by extraction. See [validation](phase-9-validation.md).

## Device validation findings

The deployed appliance had SDIO and `masked_noise` behavior absent from the committed baseline. Device validation reproduced identity failure with an unbound radio and metadata loss in a state round trip. The compatibility fix integrates bounded enumeration, the deployed radio lifecycle, metadata retention/restoration, and network recovery when returning to stock. Existing recorded enablement flags now survive reinstall. General durable metadata recovery (including unmarked all-false snapshots), atomic writes, retry ownership, and timer cleanup remain backlog work. See [device evidence and rollback](device-validation.md).
