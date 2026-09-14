# Battery life: measure the awake gaps first

[Issue #8](https://github.com/robinsandborg/trmnl-rm1/issues/8) reports 5–7 days of runtime. We have not yet measured an improvement. This change adds a read-only reporting tool and a repeatable comparison; it changes no refresh, rendering, battery threshold, service, or suspend behavior. Recovery policy work belongs to [issue #3](https://github.com/robinsandborg/trmnl-rm1/issues/3).

## Evidence and limits

The 2026-09-14 device trial verified SDIO radio teardown, stock-service masking, and three 20-second suspend/resume cycles. The final live cycle took about 22 seconds with an approximately eight-hour requested interval. See [device validation](device-validation.md). These are functional checks while charging, not an endurance baseline.

A read-only report of the production log, ending at 2026-09-14 21:52:40 UTC, found 242 records since April 18: 173 appliance and 69 maintenance cycles, including 56 USB-network and 11 sentinel-file reasons. Recorded cycle duration was 21.92 seconds median and 44.10 seconds p95. Requested intervals ranged from 900 to 28,797 seconds. The only usable adjacent discharging sample window was April 19 18:50–19:36: four observations falling from 98% to 92%. This 46-minute window and mixed test history do not establish normal battery life. Records contain intent to suspend, not proof that the device actually slept or remained asleep.

The connected battery gauge reported capacity 100%, `charge_now=1289000`, and `charge_full=1946000` while charging. Their ratio is about 66%, so substituting charge ratios for capacity or projecting runtime from a single reading is unjustified. The report deliberately does neither. Current measured during a live SSH session also includes the observation's awake workload.

## Collect a report without adding wakeups

Build the standalone tool; it does not modify the installed appliance:

```sh
go build -o build/trmnl-power-report-host ./cmd/trmnl-power-report
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/trmnl-power-report ./cmd/trmnl-power-report
scp build/trmnl-power-report root@10.11.99.1:/home/root/trmnl-power-report
```

At the end of an unplugged trial, reconnect USB and summarize the trial's timestamps, excluding the maintenance session. Use actual UTC start/end times; `-since` includes the start, `-until` excludes it:

```sh
ssh root@10.11.99.1 '/home/root/trmnl-power-report \
  -since 2026-09-15T08:00:00Z -until 2026-09-17T08:00:00Z \
  < /home/root/.local/state/trmnl-rm1/cycles.log' > baseline-report.json
```

For an existing local log, run `build/trmnl-power-report-host < cycles.log`. Reports omit image URLs, device identity, error messages, and unrecognized free-text labels. Raw logs can contain private server URLs; keep them private. The report does not read configuration, start services, access the radio, poll the battery, or arm any timer. Run it on demand, not periodically during the trial.

`selected_records` counts valid input records within the time filter. `unique_cycles` retains the last record for each identical start timestamp: a failed suspend may append another outcome for the same cycle. Mode, failure, and rendering counts use these final outcomes. The input must be one device's unmodified log; separately concatenated logs with coincident timestamps cannot be distinguished. Unknown additive JSON fields are ignored. Invalid records are counted across the entire input and break discharge windows; a record larger than 1 MiB or a read failure exits nonzero without emitting a partial report.

Cycle durations use logged wall-clock start/end times and nearest-rank p95; the median averages the middle pair. They omit startup, network cleanup, and time awake between cycles. Clock adjustments can distort duration. Requested intervals omit nonpositive/missing entries. **Neither the interval nor an `appliance` mode record proves suspend residency.** No duty-cycle or battery-life projection is produced.

Discharge windows link adjacent valid `Discharging` samples with 0–100 integer capacities. Charging, missing/invalid samples, backward timestamps, and capacity increases break windows. Flat percentages are retained to show gauge resolution. Unobserved charging, a reboot or missing cycles between samples can still invalidate a window: compare it with the trial notes and journal. Do not add windows across different trials or interpret percentage points as measured energy.

## Baseline and comparison

1. Record the firmware, installed Git revision, refresh interval, full-refresh cadence, server/content schedule, battery's approximate starting range, and ambient conditions. Keep these fixed between runs. Record UTC boundaries and any outages or manual interaction. Keep configuration and credentials private. Avoid using the first charging plateau at 100% as the only baseline.
2. In USB maintenance, verify a successful live cycle and its next schedule. Check whether a maintenance sentinel was intentionally left behind. Before an unattended test, clear only a sentinel created for testing, unplug USB, and let boot grace elapse. A wall charger does not provide an unplugged battery baseline. Follow [operations](operations.md) for safe maintenance and rollback.
3. Run at least 24–48 hours and long enough to observe several percentage-point changes at normal refresh cadence, without SSH polling or extra diagnostic timers. Stop before risking depletion; critical-battery recovery is tracked separately. A short run can locate obvious awake faults but cannot prove a weeks-long target.
4. Reconnect and collect the filtered report. Inspect `journalctl -b -u systemd-suspend.service --no-pager` for paired sleep/resume events and their duration; use the relevant earlier boot if the trial crossed a reboot. Check `systemctl list-timers --all --no-pager` for unintended repeated wake activity. Journals may not survive reboot, so absence of older entries is missing evidence, not proof of no suspend. Record any absent logs. Do not publish unreviewed raw journals.
5. Change one variable from the table below, repeat the same duration/range/content trial, and compare sampled capacity decline plus observed suspend residency, cycle counts, errors, and freshness. Repeat an apparent win before choosing it. If measurements disagree or the gauge jumps, retain both results and validate the gauge with an independent power measurement before claiming a gain.

## Order the work by what the evidence shows

| Observation | Next change to evaluate | Acceptance evidence |
| --- | --- | --- |
| Sentinel maintenance outside planned servicing | Remove the unintended sentinel after the test; retain deliberate maintenance access. | Unplugged cycles leave maintenance and journal confirms long sleep periods. |
| RTC fallback, repeated failures, or no matching suspend events | Fix the scheduler/network fault; evaluate #3's bounded recovery so outages do not leave an unattended device awake indefinitely. | Normal and injected-outage trials resume automatically, have bounded retries, and spend the intended gaps suspended. |
| Radio or stock services remain active during gaps | Verify the already implemented SDIO teardown and masks survived the actual installed configuration/custom command overrides. | Device inspection at a controlled cycle boundary and suspend evidence; no blanket masking of additional services. |
| Healthy suspend but too many useful/unchanged fetches | Increase the **BYOS server's** refresh interval within the accepted content staleness budget. Unchanged images already skip panel rendering but still require network/download work. | Fewer wake/fetch cycles with agreed freshness and a repeated decline comparison. |
| Network acquisition dominates recorded duration | Compare signal/server reachability and retry behavior; isolate acquisition timing before changing it. | Reduced measured wake cost without more failed updates. Do not merely shorten the timeout below real association time. |
| Healthy long sleep, few cycles, persistent drain | Measure whole-device suspended power and investigate wake sources/battery health. | Independent current/energy readings and repeated same-workload endurance data. |

At a 22-second recorded cycle every eight hours, only about 66 seconds of cycle work occur daily. Halving that work saves about 33 recorded seconds per day, before accounting for omitted startup/cleanup. This arithmetic is not an energy estimate: awake and suspended power are unknown. It does show why optimizing image code alone should not precede checking hours-long awake gaps and suspended draw.

`refresh_fallback_seconds` is used only when the server does not provide a positive interval. Increasing it alone does not slow a valid BYOS schedule. `refresh_min_seconds`/`refresh_max_seconds` clamp the effective interval and changing them changes freshness; document the chosen old/new values and reverse them for rollback. Do not trade away full refresh quality, Wi-Fi recovery, or USB maintenance merely to obtain a lower cycle count. Runtime defaults remain unchanged in this PR.

## Validation, compatibility, and rollback

The report has a separate versioned JSON output (`schema_version: 1`), with no migration of existing configuration/state/logs and no changes to the existing CLI. Tests cover privacy, charging exclusion, uncertain/corrupt samples, clock reversal, percentage increases/plateaus, duplicate suspend outcomes, timestamp filtering, malformed/oversized input, recovery labels, and CLI errors. Host/Linux race tests and vet plus ARMv7 builds cover the tool and existing appliance.

On 2026-09-14 the standalone ARM report ran successfully on the connected RM1, producing the same summary as the host report. Its initial seven top-level diagnostic/CLI tests passed on the actual device. Battery observations were 194 charging, 34 full, and 14 discharging; only the four-sample window above met the report's adjacency checks. The subsequent recovery-label compatibility test is also covered by host/Linux checks and ARM compilation. This verification did not change production configuration/state or perform an unplugged endurance run.

Rollback is deletion of the standalone diagnostic executable; it has no service, persistent data, or runtime settings to unwind. Preserve reports for comparison. Issue #8 remains open until an unplugged comparison supports an improvement; this PR delivers the measurement and optimization strategy, not a claimed endurance fix.
