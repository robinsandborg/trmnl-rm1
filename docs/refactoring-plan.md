# Incremental refactoring plan

**Status: foundation phases and compatibility fixes merged in PR #13 (`eafb1d1`) and deployed on RM1. All remaining work authorized on 2026-09-14.** The user requested that each phase be implemented, checked, reviewed, and merged without further phase approval prompts, followed by comprehensive RM1 tests and fixes. After the foundation is verified and pushed, delegate each open repository issue into its own PR and review every PR. Optional Phase 8 fixes will be selected from concrete test/recovery findings, with migration and rollback notes, rather than changing unrelated behavior during extraction.

Phase 1 is isolated from the shared checkout's uncommitted Wi-Fi SDIO/stock-noise changes and starts at committed `2a86333`. Their integration must retain and extend the baseline (including interface enumeration before MAC fallback); those unrelated changes are not imported here.

## Approach and compatibility

Keep the current executable and `internal/trmnl` as the composition root and compatibility facade. Extract cohesive internal packages behind existing call sites, one package at a time. Legacy functions delegate to the extracted implementation until their callers and tests have migrated. Remove forwarding code only when unused and covered by the approved phase. Retain the single module and binary; no rewrite or new daemon is proposed.

New packages must not import `internal/trmnl`. Map current `Config`/`State` values to small module-specific inputs in the facade, avoiding a shared catch-all types package or circular imports. Introduce dependency interfaces only where production effects and deterministic test adapters differ. Prefer existing `http.Client`, runner, and runtime dependency seams over a general platform framework.

Preserve CLI commands/output/errors, exported Go surfaces, JSON names/defaults/omission rules, BYOS headers and URL resolution, raw-byte hashing, image output, XDG paths, file modes, systemd names, and ordering/error behavior. A package move should require no on-device data conversion. Keep legacy JSON ownership until a specific migration is approved.

Any public API, data-format, or intentional behavior change needs a migration note in the PR and relevant `docs/`: old/new behavior, affected consumers/files, rollout order, compatibility window, backfill if needed, and downgrade/rollback procedure. Merely moving code is not approval to fix the debt inventory's behavior risks.

## Phases

Phases run in order; optional Phase 8 may be skipped. The names below are proposed internal package destinations, not new public APIs. Refine the smallest useful interface within the approved scope.

| Phase | Authorized scope if approved | Exit evidence and rollback |
| --- | --- | --- |
| **1 — Baseline and cycle seam** | Add narrow injection points around `runOnce` effects while retaining production implementations. Characterize full-cycle outputs/order and failure paths. Add host/Linux test and ARM build CI. Correct TD-06's stale runbook guidance without changing runtime behavior. | Green host and Linux suites; deterministic success/unchanged/failure traces, persisted fixtures, and baseline contract matrix below. ARM build passes. Revert this phase's seam/tests/CI/docs; no data migration. |
| **2 — BYOS package** | Extract fetch and interval handling to `internal/byos`; pass resolved identity and existing HTTP client from the facade. Preserve the current call site through a wrapper. | Same requests, response/error semantics, resolved URLs, payload bytes, and intervals through old and new paths. Test malformed/missing JSON, status/download failures, relative/absolute URLs, and bounds. Roll back facade routing and revert extraction; files stay compatible. |
| **3 — Display package** | Extract `internal/display`: portable decode/crop/grayscale/rotation implementation plus Linux FBInk/custom-command adapters and non-Linux behavior. Keep refresh cadence in orchestration until Phase 9. | Pixel fixtures for portrait/landscape/crop/rotation and supported encodings; command traces for partial/full/custom rendering; same write-before-render failure behavior. Host/Linux tests and ARM build, plus RM1 orientation/full-refresh smoke check before device rollout. Roll back routing/binary; prepared-file format stays PNG. |
| **4 — Storage package** | Extract `internal/storage` for path resolution, config/state JSON I/O, cache writes, and cycle-log append. Retain legacy DTOs through explicit facade mappings; characterize round trips before moving ownership. | Golden config/state/JSONL fixtures, defaults/permissions/missing/malformed cases, and failure ordering match. No atomic-write, locking, schema split, or retention change in this phase. Roll back routing/binary with existing files readable. |
| **5 — Network package** | Extract `internal/network` for interface enumeration, identity lookup, association/checks, and cleanup, using production command/sysfs/time adapters and test substitutes. | Same bind-before-validation, command fallback, timeout, cleanup and error contracts, including overrides and best-effort failures. Linux tests/ARM build; repeated RM1 Wi-Fi down/up cycles including automatic identity. Roll back routing/binary; retain USB recovery access. |
| **6 — Power package** | Extract `internal/power` for runtime-mode observations/policy, battery, RTC/timer scheduling, and suspend. Reuse existing runner seams; keep orchestration decisions at current call sites. | Mode precedence, A/B self-unit avoidance, RTC fallback, command failure, battery absence, and suspend traces match. Linux tests/ARM build and repeated RM1 wake/awake-fallback checks before rollout. Roll back binary and quiesce newly scheduled units during deployment. |
| **7 — Appliance package** | Extract `internal/appliance` install/restore with the existing command/file adapters, service templates, and persisted restore snapshot mapped by the facade. | Install/restore sequence and partial-failure fixtures, missing units, repeated install behavior, resume `--no-block`, and save-before-first-start contracts match. Linux tests/ARM build; reversible RM1 install/restore trial with saved metadata. Revert routing/binary and restore backed-up unit/hook files if changed during rollout. |
| **8 — Approved recovery fixes, if selected** | Separate optional behavior work from extraction. Select a bounded subset of TD-03/04/06, such as pending-timer cleanup on restore or durable installation metadata. Approve an exact spec and migration note first. | Regression test for each selected issue, Linux checks, target recovery evidence, and explicit downgrade path. This phase may be skipped; it is not implicit authorization to fix all debt. |
| **9 — Cycle package and facade cleanup** | Extract `internal/cycle` using the established modules. Leave `internal/trmnl` responsible for CLI compatibility and composition. Remove only obsolete internal forwarding code; preserve exported surfaces unless separately migrated. | Phase 1 cycle corpus passes through the final composition; compare logs/state/images/effect traces against baseline, accounting only for approved migrations. Full host/Linux suites, ARM build, and representative RM1 cycles. Revert binary/package routing; preserve old-format data. |

## Phase 1 contract matrix

Tests should exercise the same interface as callers, with fixture HTTP responses, a fixed clock, temporary storage, recorded commands, and substituted device observations. Never execute real systemd, suspend, rfkill, or SDIO operations in unit tests.

- Successful changed and unchanged screens: raw hash, metadata/counter updates, partial/full cadence, cache writes, scheduling, log/state order, cleanup, and suspend decision.
- Maintenance sentinel, USB, boot grace, recovery threshold, appliance, and RTC fallback: preserve precedence, mode labels, timer choice, and effective-versus-persisted mode behavior.
- Network setup/timeout, display HTTP/JSON/image failures, cache/render/log/state/schedule/suspend failures: preserve returned errors, counters, logs, cleanup counts, retry attempts, and paths that currently bypass finalization.
- Existing file and CLI contracts: config defaults and nested precedence; representative state including restore metadata; cycle JSONL fields; usage/validation/device-ID behavior with XDG directories isolated.
- Existing Linux regressions: timer alternation, nonblocking resume hook, and persistence before the first install-triggered cycle. Where device evidence is unavailable, mark it pending rather than claiming equivalence.

## Per-PR completion and rollback

1. Recheck the working tree and approved scope. Isolate the PR from pre-existing Wi-Fi/install/skill work; Phase 0 does not own those changes. Use a `codex/` branch for implementation and a clean checkout/worktree if needed.
2. Implement the entire approved phase. Keep edits and tests focused on its paths; do not batch unrelated debt fixes.
3. Run green tests for all touched behavior and affected callers. Run host tests/vet, Linux tests/vet for Linux-dependent changes, and ARMv7 compilation. Use [the command map](architecture.md#build-and-test-commands). Per the user's 2026-09-14 instruction, device verification may wait until the refactor is complete. Keep automated checks green per phase, track device evidence as pending, and perform final RM1 verification before declaring the whole refactor verified.
4. Self-review the final diff for behavior, API/data compatibility, effect ordering, failure/recovery behavior, and phase scope. Fix findings and rerun affected checks. Update the map, debt statuses, and relevant runbook sections.
5. Open a PR with a short **why**, concrete resulting behavior, checks and results, migration note when applicable, and rollback notes. Do not stop at a draft implementation; opening the PR completes delivery of the approved phase. The user has authorized merging green, reviewed phase PRs and final device deployment.

For pure package extraction, rollback is reverting the phase PR or restoring the prior binary while retaining compatible state/config. Before a device rollout, retain the previous binary, configuration, state (including restore metadata), and any changed unit/hook files; quiesce appliance and both A/B timer/service paths while swapping versions. Never delete state as a generic rollback. Changes that alter persistent data or installed artifacts must supply a more specific tested reversal.

Done means tests pass, behavior is matched or explicitly migrated, documentation is current, self-review is resolved, and the approved phase's PR is open. Load only documentation relevant to the current work; `AGENTS.md` remains a thin table of contents.

## Remaining delivery

Phase 9 is merged. Phase 8 addresses the reproduced deployed SDIO/restore-metadata compatibility findings and boot mount race, with [migration and completed device evidence](device-validation.md). The verified foundation is merged and deployed. Issues #3 and #8 are delegated into separate implementation PRs for review. Issue #3 adds the explicit [recovery and battery migration](recovery-and-battery.md), with physical depletion/charger acceptance still pending. Automated checks remain required for every phase.
