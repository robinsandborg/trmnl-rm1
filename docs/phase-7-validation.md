# Phase 7 — appliance

Why: isolate appliance lifecycle and generated artifacts while the facade retains config validation, platform rejection, and legacy state ownership. The extracted snapshot contains only stock sync identity and original enablement flags. Saving maps those values back into the full existing state, preserving unrelated cycle history.

Validation: host/Linux race suites and vet; ARMv7 executable and all-package test compilation. Existing install/restore tests retain artifact contents/modes, nonblocking resume, command order, save-before-start, no state overwrite after start, save failures preventing start, aggregate failures, and missing-unit behavior. New module tests cover exact removal error text, stock-unit fallback detection, and an initial write failure preventing commands.

Self-review compared lifecycle order, templates, error text and snapshot mapping. A capitalization drift in removal errors was corrected and regression-tested before delivery. No unresolved extraction findings. No public API/data format migration or changed installed artifact contents.

Rollback: revert this PR or restore the prior binary, retaining existing configuration/state. For the later install/restore device trial, back up unit/hook files and restore metadata and quiesce timers as documented in operations. Physical lifecycle verification remains pending until the full refactor is complete.
