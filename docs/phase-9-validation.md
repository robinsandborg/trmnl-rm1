# Phase 9 — cycle composition

Why: move the remaining state machine, refresh cadence, and failure finalization into `internal/cycle`, leaving the legacy package responsible for CLI compatibility, loading/validating serialized DTOs, and composing the extracted modules. No extracted module imports `trmnl`. Existing exported types remain owned by the facade; explicit state/log/battery/mode mappings preserve their output contracts.

Validation: full host/Linux race suites and vet, ARMv7 executable/all-package test compilation. The original cycle corpus runs through `App.Run` and the new cycle implementation: changed/unchanged/full refresh, raw hash/cache, exact JSONL/state fixtures, mode precedence, post-success failures, log/state/scheduling order, cleanup count, fallback mode differences, and suspend failures. Earlier display pixel/command fixtures and package checks also pass. Tests use temporary files and fake hardware effects.

Self-review compared the moved state transitions and clock/effect call ordering line by line. Compile-time state conversions and explicit log mappings retain all fields, including stock restore metadata and optional battery. Unused legacy command-fallback code and filename constants were removed. No unresolved extraction findings and no API/data/behavior migration.

Rollback: revert this PR or restore the previous binary, retaining compatible state/config/cache/PNG. No generated service or hook changes. The complete refactor still requires comprehensive RM1 testing; automated equivalence is not physical device verification. Phase 8 recovery changes, if needed, will be separate fixes with migration and rollback notes.
