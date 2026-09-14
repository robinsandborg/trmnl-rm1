# RM1 validation and deployed compatibility

Validation began on 2026-09-14 against extraction merge `f6bad14`. The connected RM1 runs ARMv7, kernel `5.4.70-v1.6.3-rm10x`, systemd 255.21, and FBInk 1.25.0-git. USB SSH is `root@10.11.99.1`. Battery was 100%, charging.

## Recovery artifacts

Before changes, all appliance/A/B cycle entrypoints were stopped and checked inactive. The original binary, config, full state/cache, service, and resume hook are retained on the device in `/home/root/trmnl-refactor-backup.dwAgx8` (mode 0700). Original binary SHA-256: `b5ebf747c7e082200131dba9a45d94dd6692206c3c3329794668d0e3a00fe803`. The maintenance sentinel was originally absent. Never copy credential-bearing backup configuration into this repository.

## Findings and compatibility changes

The installed binary included the previously uncommitted SDIO and stock-service work, while the committed extraction baseline did not. This was verified from device behavior and persisted `masked_noise`, not assumed from source.

- With brcmfmac unbound, both old and extracted `validate` failed automatic identity. Manually binding the SDIO devices made the unchanged extracted binary validate successfully. The corrected client enumerates the interface before cycle/install validation, and before CLI identity validation when an explicit ID is absent. On RM1, link-up unblocks the radio and starts the supplicant; cleanup stops it, blocks the radio and unbinds SDIO. Platforms without the driver retain ordinary command fallbacks. Custom link commands retain control of their radio behavior. Enumeration remains bounded/best effort; normal validation/connectivity errors report failure.
- A cold-boot trial failed with systemd `203/EXEC`: the appliance started at 6.981 seconds, before the `nofail` `/home` mount completed at 7.245 seconds. The executable was available and validated after mounting. The installed unit now requires the `/home/root` mount hierarchy; a regression first failed against the old template.
- A stock restore trial recovered the UI/services but left `wlan0` absent. Restore now attempts radio recovery before starting the stock UI and aggregates a radio failure without preventing UI restoration.
- The deployed state contained nine stock-service restore entries in `masked_noise`. A failing round-trip regression confirmed that the committed baseline would drop them. The facade and cycle state now retain this optional map; restore acts only on known stock units. Install retains existing original enablement flags and recorded noise entries rather than replacing them with the currently masked state, and masks the same stock units as the deployed implementation.

Migration: `masked_noise` is additive to the committed `State` Go type and already exists on this device's JSON format. Existing files without it remain readable and serialize as before. Adding a map makes `State` non-comparable in Go; callers needing whole-value comparisons should use `reflect.DeepEqual` or compare relevant fields. No configuration or state-file conversion is required. Run `install-appliance` after upgrading so the unit receives `RequiresMountsFor=/home/root`; swapping only the executable does not update installed wiring. The directive is supported by the tested systemd 255.21. The private snapshot mapping retains all cycle fields. Existing all-false/no-sync snapshots without recorded noise still lack an explicit installation marker; comprehensive durable metadata recovery remains part of issue #3.

Rollback: quiesce all appliance/A/B entrypoints, restore the backed-up deployed binary and any changed service/hook artifacts, and retain the current compatible state including `masked_noise`. Do not downgrade to an intermediate extraction binary that drops this deployed field. Preserve the original maintenance choice. The backup also contains original state for diagnosis; avoid replacing newer state unless recovery requires it.

## Evidence collected

- The complete compiled ARM test suite passed on the actual RM1 before and after compatibility changes. The final run passed 69 top-level tests across seven test binaries, including the new radio and metadata regressions.
- Live manual and service-triggered cycles recovered from an unbound radio, fetched the actual BYOS payload, skipped unchanged content, scheduled the next awake cycle, and unbound the radio afterward. All nine restore entries survived.
- Isolated local HTTP/image fixtures exercised partial refresh, unchanged skip, and full refresh through real FBInk. Prepared output was 1404×1872 after rotation. Every framebuffer pixel matched the prepared PNG (zero mismatches over the image, 1408-byte framebuffer stride).
- An injected display HTTP 404 produced the expected failure log/counter and retry timer. Restoring the endpoint cleared the counter on the next successful unchanged cycle.
- An injected RTC failure produced `awake-fallback`/`rtc-fallback` and an awake timer.
- Two real RTC suspend/resume cycles lasted 20 seconds each, confirmed in the systemd journal (21:28:41–21:29:01 and 21:29:01–21:29:21 UTC). The temporary suspend wrapper checked that an RTC alarm was armed before allowing sleep. A further 20-second cycle with the normal resume hook enabled started the appliance automatically at 21:30:35 and completed successfully at 21:30:57.

The fixture HTTP server and isolated config/state live under `/home/root/trmnl-smoke`; test configuration never replaces production credentials. The original service hook was temporarily non-executable for isolated RTC testing and restored to mode 0755 afterward.

## Remaining validation

The actual A-to-B handoff completed with service exit 0 and the B timer armed. Corrected restore exited successfully with xochitl and supplicant active, DHCP on wlan0, the saved stock services enabled, and appliance artifacts removed. Finish reinstall and boot/startup checks. Restore the live display, remove test services, preserve all rollback artifacts, return the maintenance sentinel to its original state, then record the final deployed revision and results here. Panel orientation/refresh appearance was also requested from the user; framebuffer equivalence does not measure physical ghosting or battery endurance. Backlog battery-depletion/long-duration battery tests belong to issues #3 and #8.
