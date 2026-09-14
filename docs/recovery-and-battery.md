# Startup recovery and battery operation

Issue #3 extends the verified refactoring foundation (PR #13, `eafb1d1`). The original device investigation confirmed a boot mount race and an unchanged-content redraw gap; it did not establish that a corrupt state file caused the owner's depletion incident. [Device evidence](device-validation.md) records the original observations.

## Scheduling and ownership

Installation now writes `trmnl-rm1-recovery.timer`, a five-minute `OnUnitInactiveSec` safety timer targeting the same appliance service used by boot and the resume hook. Its monotonic clock pauses during suspend; it has no `WakeSystem` setting. It therefore adds no periodic hardware wake to a successful RTC sleep. The service has a five-minute execution timeout. This behavior uses [systemd 255 timer semantics](https://raw.githubusercontent.com/systemd/systemd/v255/man/systemd.timer.xml), matching the tested RM1 firmware's systemd 255.21.

The installed service and awake A/B timers use the new `run-scheduled` command. A process lock covers both scheduled and explicit `run-once` entrypoints. The OS releases it when a process exits or is killed. A durable `next_attempt_at` deadline and independent `schedule_boot_id` suppress duplicate scheduled fetches even when every boot display attempt fails; a 30-second tolerance accommodates RTC rounding. A boot ID change bypasses an old deadline. Explicit `run-once` remains an immediate manual refresh. An overlapping invocation returns successfully without device effects; the independent safety timer retains recovery ownership if a resume request arrived while the previous process was still exiting.

Normal cycles retain RTC wake or the existing A/B awake timer. Cycle failures record a retry deadline at 5, 10, 20, 40, then at most 60 minutes. The pre-cycle failure count survives errors in late planning, logging and persistence; partial success cannot reset that backoff. Recovery also attempts RTC suspend when the battery is known to be discharging, preventing the old repeated-failure awake mode from exhausting the device. If planning or persistence fails, the safety timer remains independent of both. Invalid configuration remains unchanged and is reported in the journal; it is retried at the conservative five-minute safety interval. These guarantees assume installation succeeded, systemd remains running, and the filesystem can execute the binary. They cannot recover a kernel hang or power on a shut-down tablet.

`restore-stock` first sets a `restore-in-progress` marker that excludes both manual and scheduled cycles, removes the resume hook and safety timer, and stops/removes the main service. It then stops A/B producers, acquires the cycle lock, and finally clears both A/B timer/service pairs before restoring the stock UI and radio. A still-running manual cycle makes restore fail with a retry instruction; the marker remains until a successful retry or explicit reinstallation, preventing a partial restore from silently starting new cycles. The lock is acquired after services stop, so it cannot deadlock their shutdown. Reinstallation captures all original service choices durably before the first masking operation. The installation marker also handles an original snapshot in which every enablement flag was false.

## Durable files and migration

`state.json` is replaced using a private same-directory temporary file, file fsync, rename, and directory fsync. `state.json.bak` retains a good predecessor. Missing, empty, truncated, invalid, or `null` runtime data recovers from a valid backup or safe empty runtime state. Damaged bytes remain in `.corrupt-<timestamp>` files for diagnosis. Read/permission errors are surfaced rather than disguised as corruption.

A versioned `install-state.json` and its independently durable backup own the stock UI/sync/noise service restoration choices. Existing legacy fields migrate on the first successful load/save, remain in `state.json` for downgrade compatibility, and are reapplied from the installation snapshot after runtime recovery. Ordinary runtime writes cannot change these choices. Explicit installation saves may add newly discovered service names while preserving existing flags. Corrupt installation metadata without a usable backup fails closed. If every copy and the original legacy state are lost, the original choices cannot be reconstructed; retain the deployment backup.

New optional runtime fields are `boot_id`, `schedule_boot_id`, `local_screen`, `battery_low`, and `next_attempt_at`. The BYOS API and downloaded/prepared image formats are unchanged. A successful render in a new boot forces a full refresh even when the raw server hash matches. A local warning/offline image similarly forces a full normal redraw on recovery. Ordinary suspend/resume retains the same boot ID and unchanged-image optimization.

When network acquisition or fetching fails before boot restoration, the client renders a local offline banner over a valid downloaded image, or a plain local status image when no valid cache exists. Successful normal rendering validates a changed payload before it replaces the raw cache, retaining the last usable download if decoding/rendering fails. Local screens do not replace the raw download cache or masquerade as a successful server update. A transient outage after a successful normal render in the same boot keeps that content visible.

## Battery policy and limits

The client validates capacity as 0–100 and recognizes kernel `Charging`, `Full`, and `Discharging` statuses. It does not infer charge from an active USB network interface. Missing/invalid capacity or an unknown status reduces activity and shows a battery-unavailable status; it never triggers an inferred shutdown. See the [kernel power-supply ABI](https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-class-power).

| Configuration | Provisional default | Effect |
| --- | --- | --- |
| `battery_low_percent` | 20 | Enter protection at or below this value. |
| `battery_recovery_percent` | 30 | Leave protection at or above this value, with hysteresis. |
| `battery_critical_percent` | 5 | Optional orderly shutdown threshold while discharging. |
| `battery_check_seconds` | 1800 | RTC interval between protected checks; allowed range 300–86400. |
| `critical_battery_shutdown` | false | Explicitly enable `systemctl poweroff` at critical charge. |

These defaults are deliberately **provisional, not calibrated measurements**. The connected RM1 reported capacity 100% while Charging, but its charge-now/full values did not support deriving a trustworthy discharge curve. The physical depletion/charging acceptance tests remain open before choosing measured production thresholds or enabling automatic shutdown. With explicit critical shutdown enabled, the poweroff request runs independently of rendering and diagnostics: draw/save/log failures are reported but cannot prevent it. Critical shutdown does not depend on an RTC/timer plan and never falls back to ordinary retry sleep. A failed poweroff command is returned to the journal for the independent safety timer to retry. Existing kernel/firmware battery protection remains the default final safeguard. Sleep reduces activity but cannot prevent eventual depletion.

Protection runs before interface enumeration or network work. It renders “Please charge the device” once per boot or transition and suspends between checks. Adequate external power follows normal maintenance precedence; charging below the recovery threshold remains paused with a charging status. Thus battery protection takes precedence over boot grace, USB/sentinel maintenance and repeated-failure recovery when charge is low. At adequate capacity, existing sentinel → USB network → boot grace → failure recovery → appliance precedence remains usable. Wall charging does not require a computer network link; the client observes it at the next bounded battery check. Charger insertion wake and power-off auto-boot are hardware behavior, still unverified. A power-button press may be required after charging.

## RM1 framebuffer configuration

The tested RM1 resets to 16-bpp, 1872×1404, rotation 0 on boot. Its installed FBDepth rejects canonical `-R` but supports raw `-r`. Set the new explicit `fbink_raw_rotation: true` together with the existing `fbink_rotation: 3`, `fbink_skip_rotation: true`, and `fbink_bit_depth: 8` for that measured device setup. This requests raw framebuffer rotation while retaining the existing software image rotation. Every render reapplies depth and raw geometry; a cold-boot redraw repairs firmware's reset geometry automatically. Other devices retain existing canonical/skip behavior unless this new flag is set. Do not infer that raw rotation numbers are portable across models.

## Rollout, rollback and verification

Back up the binary, configuration, state/cache, service and hook; stop the main cycle, safety timer and both A/B paths before swapping versions. Add the explicit raw rotation configuration only on the verified RM1. Run `install-appliance` to refresh unit/timer/hook wiring; a binary-only copy cannot install the independent safety timer. Keep both installation snapshot files with any retained device backups.

To downgrade, first disable/remove `trmnl-rm1-recovery.timer` and stop all cycle paths, restore the prior binary/configuration, then run the old install command to restore its service/hook. Keep `state.json` and both installation snapshots. Old binaries ignore the new fields but can still read legacy restore metadata. Do not remove state as a refresh workaround. If a prior binary rewrites runtime state during rollback, the independent snapshot will restore its authority when upgrading again.

Automated tests exercise boot/local redraw, ordinary unchanged skipping, lock/deadline ownership, early Wi-Fi/runtime/cache/log/state failures, corrupt runtime recovery, installation metadata and hysteresis through the real `App.Run` composition with injected hardware effects. Critical shutdown is injected and never executes on the test host. Host and Linux race tests/vet and ARMv7 compilation are required for this PR.

Physical acceptance still requires normal boot without computer USB, charger insertion and sufficient-charge recovery, controlled outages and low-battery behavior on the real device. Record the deployed commit, battery readings, wake frequency, framebuffer geometry and observed screen. A successful fake-sensor test does not establish physical depletion, charger wake or endurance.

### Controlled RM1 checks (2026-09-14 UTC)

With isolated XDG directories, a fake power-supply directory, a regular-file RTC alarm and `/bin/true` suspend override, the connected RM1 passed these checks:

- Starting at firmware-like 16-bpp/raw rotation 0, the 19% Discharging warning restored 1404×1872, 8-bpp/raw rotation 3. The actual radio remained unbound and no Wi-Fi hook ran.
- 25% Discharging retained the warning without redraw/network work (about 0.04 seconds); 29% Charging showed charging status and stayed paused (about 5.7 seconds with panel refresh).
- 30% Charging fetched and fully restored normal content (about 5.5 seconds); 31% Charging skipped unchanged content (about 0.34 seconds). Exactly two up/down hook pairs occurred for those two normal cycles.
- Invalid capacity/status produced a local battery-unavailable screen without Wi-Fi effects. `run-scheduled` before its deadline produced no additional cycle log or Wi-Fi effects.

These are real ARM/FBInk executions with simulated sensor observations. They establish reduced work and geometry recovery, not real depletion, wake-from-charger, current consumption or endurance. The final PR also corrects local-screen log refresh flags discovered during these checks; automated regressions cover actual-draw versus retained-warning reporting.

Further isolated RM1 checks stopped the local test server and simulated a new boot. The device restored cached content with an offline banner, recorded bounded retries of 5 then 10 minutes, and retained the unchanged raw cache. Restoring the endpoint triggered a full normal redraw and reset the failure count. A separate test seeded legacy state from the backed-up production snapshot, migrated all nine stock-noise choices, then truncated runtime state to zero bytes. The next cycle recovered automatically, retained `.corrupt` evidence, and preserved the identical installation snapshot hash and all nine choices. These tests changed scratch files only; production credentials and state were not replaced.


A further controlled systemd trial used isolated service/timer names and the same `OnUnitInactiveSec` ownership accelerated to 15 seconds, with `WakeSystem=no`. Malformed scratch configuration produced exit-1 failures at 22:10:50 and 22:11:05 UTC; replacing only that configuration (without manually starting the service) recovered automatically at 22:11:21–26. The following timer activation at 22:11:42 honored the deadline and produced no second cycle log. Renderer/suspend were no-ops and RTC used a regular file: this confirms live systemd retry/deadline ownership, not physical wake behavior. The prior implementation's complete ARM suite also passed 90 top-level tests across eight binaries before these review fixes; updated tests are rerun separately.
