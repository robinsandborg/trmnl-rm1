# Operations Guide — RM1 TRMNL

Practical runbook for working with the reMarkable 1 once it's running the `trmnl-rm1` client. Assumes key-based SSH is already bootstrapped (see [Bootstrap SSH](#bootstrap-ssh) if not).

## Device conventions

| What | Where |
|---|---|
| USB ethernet host | `10.11.99.1` |
| SSH user | `root` |
| Binary | `/home/root/bin/trmnl-rm1` |
| FBInk + fbdepth | `/home/root/bin/fbink`, `/home/root/bin/fbdepth` |
| Config | `/home/root/.config/trmnl-rm1/config.json` |
| Maintenance sentinel | `/home/root/.config/trmnl-rm1/maintenance` |
| State | `/home/root/.local/state/trmnl-rm1/state.json` |
| Cycle log (JSONL) | `/home/root/.local/state/trmnl-rm1/cycles.log` |
| Last rendered PNG | `/home/root/.local/state/trmnl-rm1/current.png` |
| Appliance unit | `trmnl-rm1-appliance.service` |
| Next-cycle transients | `trmnl-rm1-next-a.timer` / `.service` and `trmnl-rm1-next-b.timer` / `.service` |
| Stock services | `xochitl.service`, `rm-sync.service` |

All `ssh`/`scp` commands below target `root@10.11.99.1`. If the device is off USB, use the WLAN IP shown in Settings → General → Help → Copyrights and licenses. On firmware 3.x the USB network requires the device to be **awake** (screen on) — if `ssh` says "No route to host", tap the screen first.

## Bootstrap SSH

First-time setup only. Pushes your Mac's SSH key so later commands don't need a password.

```bash
# From the repo root:
RM_PASSWORD='<password-from-device-Settings>' DEVICE=10.11.99.1 \
  ./deploy/bootstrap-ssh-key.sh
```

The password is on the device: Settings → General → Help → Copyrights and licenses. It is per-device.

## Push a new build

Before a device rollout, run the host/Linux checks and ARM build in [the architecture map](architecture.md#build-and-test-commands). Keep a copy of the previous binary, configuration, state, and installed unit/hook files. `state.json` includes stock-service restore metadata as well as display history; preserve it.

Use a USB maintenance connection for the swap. Stop the appliance and both next-cycle units, then verify they are inactive (see [Quiesce appliance cycles](#quiesce-appliance-cycles)). Copy the new binary into place only after those checks. The copy commands below do not take backups for you.

After editing code on the Mac:

```bash
# From the repo root:
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o build/trmnl-rm1 ./cmd/trmnl-rm1

scp build/trmnl-rm1 root@10.11.99.1:/tmp/trmnl-rm1
ssh root@10.11.99.1 'mv /tmp/trmnl-rm1 /home/root/bin/trmnl-rm1 && chmod +x /home/root/bin/trmnl-rm1'
```

If appliance mode is running, the new binary takes effect on the next cycle automatically (the systemd unit points at `/home/root/bin/trmnl-rm1`).

To force it immediately:

```bash
ssh root@10.11.99.1 'systemctl start trmnl-rm1-appliance.service'
```

## Push a new config

```bash
scp deploy/config.json root@10.11.99.1:/home/root/.config/trmnl-rm1/config.json
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 validate'
```

## Manual cycle (one-shot fetch + render)

```bash
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 run-once'
```

Exit 0 = success. Check the last cycle:

```bash
ssh root@10.11.99.1 'tail -n 1 /home/root/.local/state/trmnl-rm1/cycles.log'
```

Pull the last rendered image to the Mac for inspection:

```bash
scp root@10.11.99.1:/home/root/.local/state/trmnl-rm1/current.png /tmp/trmnl-current.png
open /tmp/trmnl-current.png
```

## Print device ID

`device_id` in config is the wireless MAC by default. To read what the client will send to Larapaper:

```bash
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 print-device-id'
```

## Maintenance mode (keeps the device awake, no suspend)

Turn on:

```bash
ssh root@10.11.99.1 'touch /home/root/.config/trmnl-rm1/maintenance'
```

Turn off:

```bash
ssh root@10.11.99.1 'rm -f /home/root/.config/trmnl-rm1/maintenance'
```

Maintenance is also triggered automatically whenever `usb0` is up (plugging the device into a computer) — use that as a no-touch recovery path.

## Check scheduled next cycle

```bash
ssh root@10.11.99.1 'systemctl status trmnl-rm1-next-a.timer trmnl-rm1-next-b.timer --no-pager'
```

The active timer's `Trigger:` line shows when the next `run-once` fires. Awake cycles alternate A/B names to avoid stopping their own service. One timer may be inactive or absent; appliance suspend mode uses an RTC wake alarm instead of an awake timer.

## Force a refresh now (skip the timer)

```bash
ssh root@10.11.99.1 'systemctl start trmnl-rm1-appliance.service'
```

For a **full flash refresh** of the prepared PNG even when its hash is unchanged, use FBInk directly while cycles are quiesced:

```bash
ssh root@10.11.99.1 'FBINK_NO_SW_ROTA=1 /home/root/bin/fbink -g file=/home/root/.local/state/trmnl-rm1/current.png --waveform GC16 --noviewport --flash'
```

After a cold boot, first restore the configured framebuffer depth (for this RM1, `/home/root/bin/fbdepth -d 8`); firmware starts in 16-bit landscape mode and FBInk can reject a refresh there. Apply configured hardware rotation too if enabled. The client normally performs this preparation when it renders.

This redraws the prepared file; it does not fetch a new image or advance the client's refresh counter. Check the cycle log first: the prepared file is written before rendering and may belong to a failed render. Preserve `state.json`; deleting it discards restore metadata and the first default client render is partial, not full.

## Test FBInk directly

```bash
# Draw the last rendered PNG with a full flash:
ssh root@10.11.99.1 '/home/root/bin/fbink -g file=/home/root/.local/state/trmnl-rm1/current.png --waveform GC16 --dither ORDERED --flash'

# Clear the screen:
ssh root@10.11.99.1 '/home/root/bin/fbink -c --flash'

# Panel info:
ssh root@10.11.99.1 '/home/root/bin/fbink -e'
```

## Logs

```bash
# Last 20 cycles as JSON lines:
ssh root@10.11.99.1 'tail -n 20 /home/root/.local/state/trmnl-rm1/cycles.log'

# Appliance service journal:
ssh root@10.11.99.1 'journalctl -u trmnl-rm1-appliance.service -n 50 --no-pager'

# Next-cycle transient service journal:
ssh root@10.11.99.1 'journalctl -u trmnl-rm1-next-a.service -u trmnl-rm1-next-b.service -n 50 --no-pager'
```

## Restore to stock tablet mode

Unmasks and starts xochitl, restores the recorded sync unit according to its saved enabled status, and removes the appliance unit and resume hook. Quiesce the appliance first: current `restore-stock` does not cancel the A/B transient timers. A pending timer can start another cycle after restore.

Use [Quiesce appliance cycles](#quiesce-appliance-cycles), retain state, then run:

```bash
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 restore-stock'
```

After this the device is a regular tablet again. The `trmnl-rm1` binary and config stay on disk; delete them if you want a clean slate:

```bash
ssh root@10.11.99.1 'rm -rf /home/root/bin/trmnl-rm1 /home/root/.config/trmnl-rm1 /home/root/.local/state/trmnl-rm1'
```

## Re-enter appliance mode

```bash
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 install-appliance'
```

Disables `xochitl` and `rm-sync`, enables + starts `trmnl-rm1-appliance.service`, installs a systemd-sleep resume hook.

## Recovery — device is stuck

The client has a repeated-failure fallback: after `failure_threshold` (default 3) consecutive bad cycles, it stops suspending and schedules future runs while awake. You shouldn't normally end up locked out.

If SSH stops responding:

1. **Plug into USB.** USB ethernet forces maintenance mode; the device stays awake.
2. **Wake the screen.** Firmware 3.x parks the USB interface when suspended.
3. If still unreachable, hold the power button ~10s to force a reboot. After boot, the boot-grace window (default 10 min) prevents auto-suspend — that's the window to SSH in and fix things.
4. Worst case: reinstall stock firmware via the reMarkable recovery process (reMarkable's own docs).

To keep the appliance stopped, follow [Quiesce appliance cycles](#quiesce-appliance-cycles) and disable startup:

```bash
ssh root@10.11.99.1 'systemctl disable trmnl-rm1-appliance.service'
```

After quiescing cycles, restore stock mode:

```bash
ssh root@10.11.99.1 '/home/root/bin/trmnl-rm1 restore-stock'
```

## Rotating the SSH password

The factory password is printed on the device; if you want to change it:

```bash
ssh root@10.11.99.1 'passwd'
```

SSH key auth keeps working regardless.

## Enable SSH over Wi-Fi (optional)

Off by default. While connected via USB:

```bash
ssh root@10.11.99.1 'rm-ssh-over-wlan on'
```

After this, the WLAN IP in Settings also accepts SSH.

## Paths quick reference

```text
/home/root/bin/
  trmnl-rm1                      # client binary
  fbink, fbdepth                 # display tools

/home/root/.config/trmnl-rm1/
  config.json                    # base_url, access_token, rendering knobs
  maintenance                    # presence = no suspend

/home/root/.local/state/trmnl-rm1/
  state.json                     # last hash, failure counters
  cycles.log                     # JSONL history
  current.png                    # last rendered frame

/etc/systemd/system/
  trmnl-rm1-appliance.service    # written by install-appliance

/run/systemd/transient/
  trmnl-rm1-next-a.timer         # awake cycles alternate A/B
  trmnl-rm1-next-a.service
  trmnl-rm1-next-b.timer
  trmnl-rm1-next-b.service
```

## Quiesce appliance cycles

Connect over USB and keep the device awake. Record whether the maintenance sentinel already exists so you can preserve the operator's choice. Creating it prevents suspend but does not stop scheduling; stop all cycle entrypoints as well:

```bash
ssh root@10.11.99.1 'touch /home/root/.config/trmnl-rm1/maintenance'
ssh root@10.11.99.1 'systemctl stop trmnl-rm1-appliance.service trmnl-rm1-next-a.timer trmnl-rm1-next-b.timer trmnl-rm1-next-a.service trmnl-rm1-next-b.service'
ssh root@10.11.99.1 'systemctl is-active trmnl-rm1-appliance.service trmnl-rm1-next-a.timer trmnl-rm1-next-b.timer trmnl-rm1-next-a.service trmnl-rm1-next-b.service'
```

An absent transient unit can make the stop command return nonzero; inspect the result rather than assuming all stops failed or succeeded. All five units should report inactive/failed/unknown, never active or activating. A cycle finishing during shutdown can recreate a timer; repeat the stop and verify if necessary. Keep them stopped while backing up or swapping files.

## Roll back a binary update

This is the rollback procedure for a change that preserves the configuration and state formats. A change with a migration must provide its own downgrade instructions. The procedure is documented from the existing service wiring; it still requires verification on the target RM1 before rollout.

1. Quiesce cycles over USB as above. Retain both the current and pre-update copies of the binary, config, state, appliance service, and resume hook. Never delete state to roll back.
2. Restore the previous binary to `/home/root/bin/trmnl-rm1` using a temporary upload and rename as in the build procedure. For a pure code refactor, leave the current compatible state/config in place so recent cycles and install metadata survive.
3. If the rollout intentionally changed configuration or installed unit/hook files, restore the corresponding saved versions and run `systemctl daemon-reload`. Restore old state only when an approved migration requires it; a stale state snapshot can discard newer restore metadata.
4. Run `validate`, then start `trmnl-rm1-appliance.service` in maintenance mode. Inspect the log and display, and verify the next awake timer. Preserve a pre-existing maintenance sentinel; remove one created only for rollout after checks pass.

## Deployment helper limitations

`deploy/deploy.sh` checks FBInk and fbdepth under `/home/root/bin`; its missing-tool message names `/usr/local/bin`, which does not satisfy that check. Use `/home/root/bin` and configure the binary paths explicitly when necessary. The helper overwrites binary/config without retaining a backup and clears the maintenance sentinel after `appliance` installation. Use the manual rollout above when preserving an existing maintenance session or rollback artifacts. These helper behaviors are unchanged by Phase 1.

## SDIO recovery and deployed restore metadata

After upgrading the boot wiring fix, run `install-appliance` once: the unit now requires `/home/root` to be mounted before executing the client. This RM1 mounts `/home` with `nofail`; ordering only after `network.target` allowed a reproducible `203/EXEC` at boot. Reinstallation preserves the recorded stock-service metadata.

The RM1 radio is unbound between appliance cycles to preserve the deployed power behavior. Cycle/install entrypoints enumerate it before validation; `validate` and `print-device-id` also enumerate when they need automatic identity. This can take up to five seconds. Custom Wi-Fi commands retain control of their own radio lifecycle.

Keep `masked_noise` in state: it records original enablement of stock services touched by the deployed appliance. Install preserves recorded values on reinstall; restore unmasks the known entries and re-enables those originally enabled, and recovers Wi-Fi before starting the stock UI. Continue to quiesce A/B timers explicitly before restore. See [device validation](device-validation.md) for tested behavior, compatibility notes, and the retained on-device rollback backup.
