# Phase 4 — storage

Why: centralize file layout and persistence while retaining the existing serialized types and configuration policy. `internal/storage` accepts existing DTO pointers/values; facade mappings retain `Paths` and existing return types. No public API, schema, defaults, paths, or data migration changes.

Validation: full host/Linux race suites and vet; ARMv7 executable and all-package test compilation. Existing exact state/JSONL/cycle fixtures and config/XDG/default/permission checks pass. New tests cover missing files, wrapped JSON type errors, save-before-open versus append-before-encode error ordering, raw cache bytes, and existing permissions. Self-review compared each file operation and its order with the original implementation; no unresolved findings.

Rollback: revert this PR or restore the previous binary. Keep compatible state/config/cache; no device artifacts change. Device verification follows completion of all extraction phases.
