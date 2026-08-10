# shellcheck shell=bash
# Shared SSH host-key policy for the scripts that reach a control-plane node
# (w1/m66 F7, from the 2026-08-10 codex-security scan).
#
# THE PROBLEM: fetch-app-kubeconfig.sh reads /etc/kubernetes/admin.conf — full
# cluster-admin — over SSH, and both it and verify-substrate.sh hardcoded
# StrictHostKeyChecking=accept-new. The server's identity therefore rested on
# trust-on-first-use: an attacker able to intercept the first connection to the
# hcloud-reported control-plane IP could serve its own host key, hand back an
# attacker-controlled admin.conf, and every later step (kubectl apply, Secret
# writes, deploys) would target the attacker's API server. The follow-on
# `kubectl cluster-info` check does not help: it authenticates against the CA
# embedded in that same untrusted kubeconfig.
#
# THE POLICY, in one place so the two callers cannot drift:
#   * BEX_SSH_KNOWN_HOSTS_FILE set  => pin to it and FAIL CLOSED
#     (StrictHostKeyChecking=yes, global known-hosts ignored). An unknown or
#     changed host key aborts before admin.conf is ever read.
#   * unset => accept-new, exactly as before (local/dev, and CI until the
#     operator provisions the key), but say so on stderr — the weaker mode must
#     never be silent.
#
# Provisioning the pinned file is an out-of-band operator step, same custody
# model as the SSH private key itself (docs/ADR019-infra-credentials.md).

# bex_ssh_hostkey_args populates the BEX_SSH_HOSTKEY_ARGS array with the ssh -o
# options implementing the policy above. Always non-empty, so callers can expand
# it under `set -u` on any bash.
bex_ssh_hostkey_args() {
  BEX_SSH_HOSTKEY_ARGS=()
  local known="${BEX_SSH_KNOWN_HOSTS_FILE:-}"
  if [ -z "$known" ]; then
    BEX_SSH_HOSTKEY_ARGS=(-o StrictHostKeyChecking=accept-new)
    echo "note: BEX_SSH_KNOWN_HOSTS_FILE is unset — the control-plane host key is trusted on first use (unauthenticated server identity)" >&2
    return 0
  fi
  # A pin that points at nothing is worse than no pin: it looks like the control
  # is on while ssh would silently have nothing to compare against.
  if [ ! -s "$known" ]; then
    echo "error: BEX_SSH_KNOWN_HOSTS_FILE=$known is missing or empty — refusing to fall back to trust-on-first-use" >&2
    return 1
  fi
  BEX_SSH_HOSTKEY_ARGS=(
    -o "UserKnownHostsFile=$known"
    -o GlobalKnownHostsFile=/dev/null
    -o StrictHostKeyChecking=yes
  )
}
