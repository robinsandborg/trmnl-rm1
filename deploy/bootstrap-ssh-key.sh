#!/usr/bin/env bash
# Push your local SSH public key to the reMarkable so every subsequent
# command can use key-based auth. Run once; it uses `expect` to feed the
# factory root password to ssh-copy-id.
#
# Usage:
#   DEVICE=10.11.99.1 RM_PASSWORD='nKEZnjOHGs' ./deploy/bootstrap-ssh-key.sh
set -euo pipefail

DEVICE="${DEVICE:-10.11.99.1}"
PUBKEY="${PUBKEY:-$HOME/.ssh/id_ed25519.pub}"

if [[ -z "${RM_PASSWORD:-}" ]]; then
  echo "set RM_PASSWORD to the device's root password (from Settings > Help > Copyrights and licenses)" >&2
  exit 1
fi
if [[ ! -f "$PUBKEY" ]]; then
  echo "public key $PUBKEY does not exist. Generate one with: ssh-keygen -t ed25519" >&2
  exit 1
fi

ssh-keygen -R "$DEVICE" >/dev/null 2>&1 || true

expect <<EXPECT_EOF
set timeout 20
spawn ssh-copy-id -o StrictHostKeyChecking=accept-new -i "$PUBKEY" root@$DEVICE
expect {
  "password:" { send -- "$RM_PASSWORD\r" }
  timeout { puts "timeout waiting for password prompt"; exit 2 }
  eof { puts "ssh-copy-id exited early"; exit 3 }
}
expect eof
EXPECT_EOF

echo "verifying key-based auth..."
ssh -o BatchMode=yes root@"$DEVICE" 'echo ok'
