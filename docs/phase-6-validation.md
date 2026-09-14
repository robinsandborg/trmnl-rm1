# Phase 6 — power

Why: isolate mode observations, battery sampling, wake/timer scheduling, and suspend behind effective options and the existing command runner. The facade maps battery/mode values and preserves configuration defaults; orchestration still decides when to call each operation. No public API, stored format, unit naming, or runtime policy migration.

Validation: full host/Linux race suites and vet, ARMv7 executable/all-package test compilation. Existing runtime precedence, USB operstate/carrier, A/B self-unit avoidance, RTC fallback/error identity, and full-cycle fixtures pass through the extracted code. New tests cover battery selection/absence/optional values, wakealarm content and command fallback, invalid intervals, and suspend override/fallback. All hardware effects in tests use temporary paths or recorders.

Self-review compared default mapping, Linux/non-Linux behavior, ignored cleanup failures, command order, and error wrapping with the pre-extraction source; no unresolved findings. Repeated physical wake/suspend and awake-fallback checks remain part of final RM1 validation.

Rollback: revert the PR or restore the prior binary with compatible files retained. Quiesce A/B and appliance units before a device swap; no installed artifacts change in this extraction.
