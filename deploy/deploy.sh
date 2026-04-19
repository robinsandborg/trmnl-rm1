#!/usr/bin/env bash
# One-shot deploy: copies the cross-compiled binary and starter config to the
# reMarkable over SSH, then runs validate / print-device-id / run-once so you
# can confirm a frame renders before flipping on appliance mode.
#
# Assumes:
#   - key-based SSH to the device is already working (see bootstrap-ssh-key.sh)
#   - the binary has been built: GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
#         go build -trimpath -ldflags="-s -w" -o build/trmnl-rm1 ./cmd/trmnl-rm1
#   - deploy/config.json exists locally with your real base_url filled in
#
# Usage:
#   DEVICE=10.11.99.1 ./deploy/deploy.sh             # copy + validate + run-once
#   DEVICE=10.11.99.1 ./deploy/deploy.sh appliance   # same, then install-appliance
set -euo pipefail

DEVICE="${DEVICE:-10.11.99.1}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_ROOT/build/trmnl-rm1"
CFG="$REPO_ROOT/deploy/config.json"

if [[ ! -x "$BIN" ]]; then
  echo "missing $BIN — run the build command from the header first" >&2
  exit 1
fi
if [[ ! -f "$CFG" ]]; then
  echo "missing $CFG — copy deploy/config.example.json to deploy/config.json and fill in base_url" >&2
  exit 1
fi

SSH="ssh -o StrictHostKeyChecking=accept-new root@${DEVICE}"
SCP="scp -o StrictHostKeyChecking=accept-new"

echo "==> probing device"
$SSH "uname -a; cat /etc/version 2>/dev/null || true"

echo "==> creating remote directories"
$SSH 'mkdir -p /home/root/.config/trmnl-rm1 /home/root/.local/state/trmnl-rm1 /home/root/bin'

echo "==> copying binary"
$SCP "$BIN" "root@${DEVICE}:/home/root/bin/trmnl-rm1"
$SSH 'chmod +x /home/root/bin/trmnl-rm1'

echo "==> copying config"
$SCP "$CFG" "root@${DEVICE}:/home/root/.config/trmnl-rm1/config.json"

echo "==> checking FBInk presence"
if ! $SSH 'test -x /home/root/bin/fbink && test -x /home/root/bin/fbdepth'; then
  cat <<'MSG' >&2
FBInk is not on the device. Install it before running run-once:
  1) Download the reMarkable build of FBInk from NiLuJe's releases:
       https://github.com/NiLuJe/FBInk/releases
     (pick the *_remarkable.tar.gz asset matching your device model)
  2) scp the tarball to the device and extract fbink + fbdepth into /usr/local/bin
  3) chmod +x /usr/local/bin/fbink /usr/local/bin/fbdepth
  4) re-run this script
MSG
  exit 2
fi

echo "==> forcing maintenance mode so the cycle can't suspend the device"
$SSH 'touch /home/root/.config/trmnl-rm1/maintenance'

echo "==> validate"
$SSH '/home/root/bin/trmnl-rm1 validate'

echo "==> print-device-id"
$SSH '/home/root/bin/trmnl-rm1 print-device-id'

echo "==> run-once (should draw a frame on the display)"
$SSH '/home/root/bin/trmnl-rm1 run-once'

if [[ "${1:-}" == "appliance" ]]; then
  echo "==> install-appliance"
  $SSH '/home/root/bin/trmnl-rm1 install-appliance'
  echo "==> clearing maintenance sentinel"
  $SSH 'rm -f /home/root/.config/trmnl-rm1/maintenance'
  echo "appliance mode installed. Watch /home/root/.local/state/trmnl-rm1/cycles.log"
else
  echo "skipped install-appliance. Re-run with 'appliance' arg when you're ready."
fi
