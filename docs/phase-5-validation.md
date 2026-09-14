# Phase 5 — network

Why: isolate wireless identity, link control, connectivity polling, and acquisition/cleanup while keeping existing configuration defaults and call sites. The module receives effective options and command/lifecycle adapters. No public API, JSON, command order, timeout, error, or cleanup migration.

Validation: full host/Linux race tests and vet; ARMv7 executable and all test packages compile. Tests cover ordered command fallbacks, first-success stopping, overrides, configured identity and sysfs candidate preference, HEAD status 200–499 acceptance, timeout and malformed URLs, and non-Linux behavior. The unchanged cycle lifecycle corpus passes through the new acquisition implementation. Self-review compared source operations and fallback ordering; no unresolved extraction findings.

The shared checkout's uncommitted SDIO implementation is excluded. The committed baseline has no SDIO bind-before-validation behavior to preserve; any necessary integration is a separate fix with device evidence. Repeated actual Wi-Fi down/up and automatic identity remain required at final device validation.

Rollback: revert this PR or restore the prior binary; no stored formats or installed artifacts change. Retain USB recovery access during subsequent device tests.
