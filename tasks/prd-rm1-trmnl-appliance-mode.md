# PRD: RM1 TRMNL Appliance Mode

## Introduction / Overview

Build a battery-first TRMNL client for the reMarkable 1 that behaves like a dedicated appliance instead of a continuously running Linux app.

The product will use the existing Larapaper BYOS server as its content source, render full-screen landscape images on the RM1, and minimize power draw by waking only when needed, briefly enabling Wi-Fi, fetching the next screen, rendering it, disabling Wi-Fi, scheduling the next wake, and suspending again.

This feature exists to make the RM1 practical as an unplugged wall display. The main problem to solve is not basic rendering, but reliable low-power operation with a strong recovery path back to stock behavior. The design must explicitly prevent operator lockout when the device is mostly asleep.

## Goals

- Achieve appliance-like duty cycling instead of continuous polling.
- Use the RM1 as a dedicated full-screen landscape display.
- Reuse saved system Wi-Fi settings instead of managing Wi-Fi credentials in-app.
- Support Larapaper BYOS with minimal configuration.
- Provide a supported restore path that returns the device to normal stock behavior over SSH.
- Provide a maintenance mode that keeps SSH reachable over USB when the device is plugged into a computer.
- Reduce battery drain materially versus an always-awake, always-connected poll loop.

## User Stories

### US-001: Configure the device with a local config file
**Description:** As the device owner, I want to configure the client with a file so that setup is simple and repeatable over SSH.

**Acceptance Criteria:**
- [ ] The app reads configuration from `~/.config/trmnl-rm1/config.json`.
- [ ] The config supports `base_url`, `device_id`, optional `access_token`, refresh fallback values, Wi-Fi timeout, and display power settings.
- [ ] `trmnl-rm1 validate` exits with a clear non-zero error when required fields are missing or malformed.
- [ ] A valid config produces a zero exit code from `validate`.

### US-002: Auto-detect or override the device identity
**Description:** As the device owner, I want the app to determine the correct device ID so that I do not have to manually enter the MAC address unless I want to.

**Acceptance Criteria:**
- [ ] If `device_id` is present in config, that value is used as-is.
- [ ] If `device_id` is blank, the app attempts to derive it from the wireless MAC address.
- [ ] `trmnl-rm1 print-device-id` prints the value the app will use.
- [ ] Validation fails with a clear message if no usable device ID can be determined.

### US-003: Fetch one screen from BYOS in a one-shot run
**Description:** As the device owner, I want a single-run command so that the production power model does not rely on an infinite polling loop.

**Acceptance Criteria:**
- [ ] `trmnl-rm1 run-once` performs exactly one fetch-render-power cycle.
- [ ] The BYOS request sends the `ID` header on every call.
- [ ] The request sends the `access-token` header only when configured.
- [ ] Non-200 responses and network errors are logged and return a non-zero exit code.

### US-004: Render a full-screen landscape image on RM1
**Description:** As the device owner, I want TRMNL screens rendered for the RM1's actual display so that the result looks intentional and readable.

**Acceptance Criteria:**
- [ ] The client renders the fetched image in fixed landscape orientation.
- [ ] The render path supports the v1 happy path of 1-bit PNG images.
- [ ] The rendered image fills the RM1 screen without relying on a stock `800x480` layout.
- [ ] The displayed image remains visible after the worker process exits.

### US-005: Skip unnecessary redraws
**Description:** As the device owner, I want unchanged screens to be skipped so that the device avoids needless refreshes and battery use.

**Acceptance Criteria:**
- [ ] The client stores the last displayed image identifier or hash in a state file.
- [ ] If the next fetch resolves to the same image, the client does not redraw the screen.
- [ ] Skipped redraws still count as a successful cycle and still schedule the next wake.
- [ ] State is stored separately from the user-edited config.

### US-006: Manage ghosting with controlled full refreshes
**Description:** As the device owner, I want the client to balance ghosting and power usage so that the display stays clean without excessive flashing.

**Acceptance Criteria:**
- [ ] The client supports partial refresh behavior for routine updates.
- [ ] The client forces a full refresh every configurable N rendered updates.
- [ ] The full-refresh counter survives process restarts through persisted state.
- [ ] The refresh strategy is configurable without code changes.

### US-007: Bring Wi-Fi up only when needed
**Description:** As the device owner, I want Wi-Fi enabled only for the network window so that idle power draw stays low.

**Acceptance Criteria:**
- [ ] The app uses the device's existing saved Wi-Fi configuration and does not manage SSIDs or passwords.
- [ ] The app enables Wi-Fi at the start of a cycle and waits only up to a configured timeout for connectivity.
- [ ] The app disables Wi-Fi after the fetch/log phase on both success and failure paths.
- [ ] A failed Wi-Fi association does not leave the device awake indefinitely.

### US-008: Schedule the next wake and suspend the device
**Description:** As the device owner, I want the RM1 to sleep between updates so that it behaves like a low-power display appliance.

**Acceptance Criteria:**
- [ ] The next wake interval is derived from server `refresh_rate` when present.
- [ ] Local min, max, and fallback intervals are enforced when the server value is missing or unreasonable.
- [ ] The app schedules the next wake before suspending.
- [ ] The app suspends the device at the end of a completed cycle.

### US-009: Install appliance mode
**Description:** As the device owner, I want a dedicated install mode so that the device boots into display behavior consistently.

**Acceptance Criteria:**
- [ ] `trmnl-rm1 install-appliance` sets up the commands and units required for unattended operation.
- [ ] Appliance mode disables `xochitl` while active.
- [ ] Appliance mode disables the stock sync unit while active, accounting for either `sync.service` or `rm-sync.service`.
- [ ] Rebooting the device preserves appliance behavior.

### US-010: Keep the device recoverable over USB
**Description:** As the device owner, I want a maintenance mode that activates when the device is connected to a computer by USB so that I am not locked out by the sleep schedule.

**Acceptance Criteria:**
- [ ] When a USB computer connection is detected, the device enters maintenance mode before the next suspend.
- [ ] In maintenance mode, the system does not suspend automatically.
- [ ] In maintenance mode, SSH remains reachable over the standard USB network path.
- [ ] Maintenance mode does not disable `dropbear` or the USB network gadget.
- [ ] Maintenance mode can also be forced by a sentinel file at `~/.config/trmnl-rm1/maintenance`.

### US-011: Delay appliance sleep after boot
**Description:** As the device owner, I want a short grace period after boot so that I have time to connect and inspect the system before it starts its low-power loop.

**Acceptance Criteria:**
- [ ] Appliance mode does not attempt suspend for the first 10 minutes after boot.
- [ ] During the boot grace window, SSH remains available over USB networking.
- [ ] The grace timer is skipped only when the operator explicitly chooses a faster startup setting in config.
- [ ] The grace window behavior is documented in the install instructions.

### US-012: Fall back to recovery behavior after repeated failures
**Description:** As the device owner, I want the system to stop trying to sleep into a broken state so that repeated failures do not make recovery harder.

**Acceptance Criteria:**
- [ ] The app tracks consecutive failed cycles in persistent state.
- [ ] After a configurable threshold of failures, the device stops entering normal appliance suspend behavior.
- [ ] Failure fallback keeps the device awake and reachable for recovery.
- [ ] The fallback state is clearly logged with the reason it was entered.

### US-013: Restore stock behavior safely
**Description:** As the device owner, I want a supported restore path so that experimentation with the display mode is reversible.

**Acceptance Criteria:**
- [ ] `trmnl-rm1 restore-stock` re-enables `xochitl`.
- [ ] `restore-stock` re-enables the previously disabled stock sync unit.
- [ ] `restore-stock` removes or disables the appliance-specific startup behavior.
- [ ] The restore path is executable entirely over SSH.

### US-014: Capture battery and cycle diagnostics
**Description:** As the device owner, I want basic battery and cycle logs so that I can evaluate whether the power-saving design is actually working.

**Acceptance Criteria:**
- [ ] Each cycle records start time, end time, chosen refresh interval, and whether the screen changed.
- [ ] Each cycle records battery data from `/sys/class/power_supply` when available.
- [ ] Failures are logged with a distinct reason category such as Wi-Fi, HTTP, render, or suspend scheduling.
- [ ] Logs are stored under the user-writable home directory.

## Functional Requirements

- FR-1: The system must read its user configuration from `~/.config/trmnl-rm1/config.json`.
- FR-2: The system must keep mutable runtime state in a separate state file from the main config.
- FR-3: The system must support `base_url` for Larapaper BYOS.
- FR-4: The system must use `device_id` from config when provided.
- FR-5: The system must auto-detect a device ID from the RM1 network hardware when config does not provide one.
- FR-6: The system must support an optional `access_token` but must not require one for all deployments.
- FR-7: The system must expose `validate`, `print-device-id`, `run-once`, `install-appliance`, and `restore-stock` commands.
- FR-8: The system must fetch display content from the BYOS display endpoint using the device identity headers required by the server.
- FR-9: The system must render full-screen landscape images sized for the RM1 instead of assuming the stock TRMNL display dimensions.
- FR-10: The system must persist enough render state to skip unchanged images and track forced full-refresh cadence.
- FR-11: The system must reuse the device's saved Wi-Fi settings and must not introduce a custom Wi-Fi setup flow.
- FR-12: The system must keep Wi-Fi disabled between update cycles when `disable_wifi_between_updates` is enabled.
- FR-13: The system must bound the maximum network wait time per cycle to prevent runaway battery drain.
- FR-14: The system must derive the next wake interval from server `refresh_rate`, subject to local fallback and clamping rules.
- FR-15: The system must schedule the next wake before suspending the device.
- FR-16: The system must support a dedicated appliance mode that disables the stock UI and stock sync behavior while active.
- FR-17: The system must provide a supported restore path that returns the device to stock behavior without reflashing the OS.
- FR-18: The system must write logs and operational data only to locations under `/home/root` so changes survive normal runtime and avoid system partition risk.
- FR-19: The system must fail safely on network, server, or render errors and still attempt to return the device to a low-power or recoverable state.
- FR-20: The system must default to a 30-minute refresh cadence when the server does not provide a usable interval.
- FR-21: The system must enter maintenance mode when a USB computer connection is detected.
- FR-22: Maintenance mode must disable automatic suspend until the USB computer connection is removed or the operator exits maintenance mode.
- FR-23: The system must preserve SSH over USB networking in both maintenance mode and boot grace mode.
- FR-24: The system must include a 10-minute boot grace period before first entering unattended suspend behavior.
- FR-25: The system must enter a recovery-safe awake state after a configurable number of consecutive failed cycles.

## Non-Goals (Out of Scope)

- Managing Wi-Fi SSIDs or passwords inside this application.
- Building an on-device setup UI for v1.
- Publishing Vellum or Toltec packages in v1.
- Supporting both "dedicated display" and "normal note-taking tablet" workflows simultaneously.
- Matching TRMNL OG battery life exactly as a contractual requirement.
- Supporting multiple orientations in v1.
- Supporting color or 2-bit rendering in v1.
- Building a generalized open-source distribution flow for all reMarkable users in v1.

## Design Considerations

- The product is intentionally headless after setup.
- The device should be treated as a dedicated landscape wall display.
- The screen should continue to show the last rendered image while the tablet is suspended.
- Setup and recovery should be SSH-first and documented as terminal commands.
- User-facing behavior should feel appliance-like, not app-like.
- Recovery and maintainability take precedence over absolute battery savings when those goals conflict.
- Plugging the device into a computer by USB is an acceptable way to regain continuous access for maintenance.

## Technical Considerations

- The existing local `trmnl-display` project is useful for API/config structure, but its Raspberry Pi `show_img` render dependency should not be reused on RM1.
- The render backend should target RM1-native framebuffer tooling suitable for e-ink rendering.
- RTC wake should be the preferred wake mechanism, with a fallback strategy defined if the exact RM1 OS build does not support reliable timed wake.
- The names of stock services can vary by OS generation, especially around sync service naming.
- Battery claims must be based on measured on-device comparisons against a baseline always-awake loop.
- The implementation should minimize writes and background activity between update cycles.
- USB maintenance detection should rely on the presence of active USB networking or an equivalent reliable OS signal, not on charger presence alone.
- The implementation must not disable `dropbear`, USB networking, or any other service required for SSH recovery.

## Success Metrics

- The production runtime uses one-shot wake cycles and does not rely on a resident infinite polling loop.
- Wi-Fi is disabled between update cycles in the default appliance configuration.
- The device successfully wakes, fetches, renders, and suspends again on a 30-minute cadence without manual intervention.
- Unchanged screens do not trigger redraws.
- Plugging the RM1 into a computer by USB causes the device to remain awake and reachable over SSH for maintenance.
- `restore-stock` returns the tablet to normal stock UI behavior over SSH.
- A 24-hour battery test at a 30-minute cadence shows materially lower drain than a baseline always-awake, Wi-Fi-on polling prototype.
- Overnight unattended operation completes without the device getting stuck awake after network or server failures.

## Open Questions

- Which exact suspend mechanism is most reliable on the target RM1 OS build: RTC wakealarm plus suspend, or a wake-capable system timer fallback?
- Which specific command path is most reliable for bringing Wi-Fi up and down on the target device software version?
- What battery-drain threshold should be considered "close enough" to TRMNL OG for this project to be considered successful?
- Should the v1 renderer support only pre-sized full-screen images from Larapaper, or should it also include local fit or crop behavior as a safety net?
