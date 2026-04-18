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
| Next-cycle transient | `trmnl-rm1-next.timer` / `.service` |
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
ssh root@10.11.99.1 'systemctl status trmnl-rm1-next.timer --no-pager'
```

The `Trigger:` line shows when the next `run-once` fires.

## Force a refresh now (skip the timer)

```bash
ssh root@10.11.99.1 'systemctl start trmnl-rm1-appliance.service'
```

Or, to force a **full flash refresh** even if the image hash hasn't changed, wipe state and re-run:

```bash
ssh root@10.11.99.1 'rm -f /home/root/.local/state/trmnl-rm1/state.json && /home/root/bin/trmnl-rm1 run-once'
```

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
ssh root@10.11.99.1 'journalctl -u trmnl-rm1-next.service -n 50 --no-pager'
```

## Restore to stock tablet mode

Reverses `install-appliance` — unmasks `xochitl` and `rm-sync`, removes the appliance unit and resume hook, starts `xochitl` again.

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

To disable the appliance service before it suspends the device again:

```bash
ssh root@10.11.99.1 'systemctl stop trmnl-rm1-appliance.service trmnl-rm1-next.timer; systemctl disable trmnl-rm1-appliance.service'
```

Or run a single clean restore:

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
  trmnl-rm1-next.timer           # transient, between cycles
  trmnl-rm1-next.service
```
