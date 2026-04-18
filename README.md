# RM1 TRMNL

First implementation of a battery-first TRMNL appliance-mode client for the reMarkable 1.

This client is built around a one-shot `wake -> fetch -> render -> schedule -> sleep` cycle for Larapaper BYOS. It is designed for a dedicated landscape display workflow, not a resident polling app and not a mixed note-taking/tablet mode.

## Status

- One-shot cycle via `trmnl-rm1 run-once`
- Config validation and device ID detection
- BYOS fetch with `ID` and optional `access-token` headers
- Persistent state for unchanged-screen skipping and full-refresh cadence
- Boot grace, USB/sentinel maintenance mode, and repeated-failure awake fallback
- Appliance install/restore with systemd service wiring and stock service masking
- FBInk-based grayscale rendering tuned for RM1 landscape appliance mode

## Build

```bash
go build -o trmnl-rm1 ./cmd/trmnl-rm1
```

## Configuration

The client reads config from `~/.config/trmnl-rm1/config.json`.

Example:

```json
{
  "base_url": "http://larapaper.local",
  "device_id": "",
  "access_token": "",
  "refresh_fallback_seconds": 1800,
  "refresh_min_seconds": 300,
  "refresh_max_seconds": 86400,
  "wifi_timeout_seconds": 45,
  "disable_wifi_between_updates": true,
  "full_refresh_every": 6,
  "boot_grace_seconds": 600,
  "failure_threshold": 3,
  "wifi_interface": "wlan0",
  "maintenance_interface": "usb0",
  "rtc_wakealarm_path": "/sys/class/rtc/rtc0/wakealarm",
  "power_supply_path": "/sys/class/power_supply",
  "fbink_binary": "fbink",
  "fbdepth_binary": "fbdepth",
  "fbink_bit_depth": 8,
  "fbink_rotation": 3,
  "fbink_waveform_partial": "GL16",
  "fbink_waveform_full": "GC16",
  "fbink_no_viewport": true
}
```

Optional advanced fields:

- `renderer_command`: override the default FBInk renderer. Use `{image}` and `{mode}` placeholders.
- `wifi_up_command`, `wifi_down_command`, `suspend_command`: override platform commands if your RM1 build differs.
- `connectivity_check_url`: override the URL used while waiting for Wi-Fi to associate.
- `fbink_dither_mode`: optional FBInk hardware dithering mode if you want to trade tone smoothness for a different update look.
- `framebuffer_device`: reserved for future low-level renderer wiring. The current default FBInk path ignores it.

## Commands

```bash
trmnl-rm1 validate
trmnl-rm1 print-device-id
trmnl-rm1 run-once
trmnl-rm1 install-appliance
trmnl-rm1 restore-stock
```

## Appliance Mode

`install-appliance` writes a systemd unit plus a resume hook, disables `xochitl`, and disables whichever stock sync unit exists on the device (`sync.service` or `rm-sync.service`).

`restore-stock` removes the appliance wiring, unmasks the stock services again, and starts `xochitl` over SSH.

## Recovery Model

- The first 10 minutes after boot default to a boot grace window with no automatic suspend.
- Plugging the RM1 into a computer over USB keeps the device awake when the USB network interface is active.
- Creating `~/.config/trmnl-rm1/maintenance` also forces maintenance mode.
- After repeated failed cycles, the client stops suspending and schedules future runs while awake so recovery remains possible.

## Operations

Day-to-day commands for SSH'ing in, pushing new builds, forcing a refresh, restoring the tablet, and recovering from a stuck state live in [docs/operations.md](docs/operations.md).

## Logs And State

- Config: `~/.config/trmnl-rm1/config.json`
- Maintenance sentinel: `~/.config/trmnl-rm1/maintenance`
- State: `~/.local/state/trmnl-rm1/state.json`
- Cycle log: `~/.local/state/trmnl-rm1/cycles.log`

## Renderer Note

This implementation now defaults to preparing a landscape 8-bit grayscale PNG, switching the framebuffer to 8bpp landscape mode via `fbdepth`, and drawing it with `fbink`.

Routine updates default to `GL16`, while forced full refreshes default to `GC16` plus flash. That preserves grayscale rendering instead of collapsing the image to 1-bit black and white.

`renderer_command` is still available as an escape hatch if your RM1 build needs a different final render/update command, but the intended v1 path is FBInk.
