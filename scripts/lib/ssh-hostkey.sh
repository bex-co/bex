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
# THE POLICY, in one place so the callers cannot drift. The first source that
# is set wins:
#   1. BEX_SSH_KNOWN_HOSTS_FILE => pin to it (tier "pinned").
#   2. BEX_SSH_KNOWN_HOSTS (the pin's content, as the workflows hold it) =>
#      written to a 0600 temp file and pinned (tier "pinned").
#   3. The stable control-plane host key (w6/m153 t008; w3/m472): every CP node
#      serves one ed25519 key, so its public half pins every node with one
#      wildcard line, `* ssh-ed25519 <key>`, written to a 0600 temp file (tier
#      "derived"). The key is BEX_CP_SSH_HOST_KEY_FILE, or the documented
#      ~/.ssh/bex_cp_host_ed25519 when that is unset and exists. The public
#      half is read from the `.pub` sibling, or derived with `ssh-keygen -y`.
#   Each of these FAILS CLOSED: StrictHostKeyChecking=yes, the global
#   known-hosts ignored, so an unknown or changed key aborts before admin.conf
#   is read, and ~/.ssh/known_hosts is neither read nor written.
#   4. None of them, BEX_SSH_REQUIRE_KNOWN_HOSTS truthy => FAIL CLOSED (codex
#      F10). CI and production bootstrap set this so a missing pin aborts the
#      run instead of silently trusting the first-seen key.
#   5. None of them, requirement not asserted => accept-new against
#      ~/.ssh/known_hosts (tier "tofu", local/dev only), with a notice on
#      stderr: the weaker mode is never silent.
#
# Provisioning the pin is an out-of-band operator step, same custody model as
# the SSH private key itself (docs/ADR019-infra-credentials.md).

# bex_ssh_hostkey_args populates the BEX_SSH_HOSTKEY_ARGS array with the ssh -o
# options implementing the policy above (always non-empty, so callers can
# expand it under `set -u` on any bash), sets BEX_SSH_HOSTKEY_TIER to
# pinned|derived|tofu, and BEX_SSH_HOSTKEY_TMP to the temp pin file it wrote
# (empty when none). Callers remove that file on exit: bex_ssh_hostkey_cleanup.
# It reports the tier on stderr, never key material.
# shellcheck disable=SC2034 # the BEX_SSH_HOSTKEY_* globals are the callers' to read
bex_ssh_hostkey_args() {
  BEX_SSH_HOSTKEY_ARGS=()
  BEX_SSH_HOSTKEY_TIER=""
  BEX_SSH_HOSTKEY_TMP=""
  local known="${BEX_SSH_KNOWN_HOSTS_FILE:-}" cp_key="${BEX_CP_SSH_HOST_KEY_FILE:-}" line origin=""
  if [ -n "$known" ]; then
    # A pin that points at nothing is worse than no pin: it looks like the
    # control is on while ssh would silently have nothing to compare against.
    if [ ! -s "$known" ]; then
      echo "error: BEX_SSH_KNOWN_HOSTS_FILE=$known is missing or empty — refusing to fall back to trust-on-first-use" >&2
      return 1
    fi
    BEX_SSH_HOSTKEY_TIER=pinned
    origin="$known"
  elif [ -n "${BEX_SSH_KNOWN_HOSTS:-}" ]; then
    bex_ssh_hostkey_write_pin "$BEX_SSH_KNOWN_HOSTS" || return 1
    known="$BEX_SSH_HOSTKEY_TMP"
    BEX_SSH_HOSTKEY_TIER=pinned
    origin="BEX_SSH_KNOWN_HOSTS"
  else
    if [ -z "$cp_key" ] && { [ -e "$HOME/.ssh/bex_cp_host_ed25519" ] || [ -e "$HOME/.ssh/bex_cp_host_ed25519.pub" ]; }; then
      cp_key="$HOME/.ssh/bex_cp_host_ed25519"
    fi
    if [ -n "$cp_key" ]; then
      line="$(bex_cp_host_public_key "$cp_key")" || return 1
      bex_ssh_hostkey_write_pin "* $line" || return 1
      known="$BEX_SSH_HOSTKEY_TMP"
      BEX_SSH_HOSTKEY_TIER=derived
      origin="derived from BEX_CP_SSH_HOST_KEY_FILE ($cp_key)"
    fi
  fi

  if [ -z "$known" ]; then
    case "${BEX_SSH_REQUIRE_KNOWN_HOSTS:-}" in
      1 | true | TRUE | yes | YES)
        echo "error: BEX_SSH_REQUIRE_KNOWN_HOSTS is set but no host-key pin is (BEX_SSH_KNOWN_HOSTS_FILE, BEX_SSH_KNOWN_HOSTS or BEX_CP_SSH_HOST_KEY_FILE; codex F10) — refusing to fetch cluster-admin over an unauthenticated (trust-on-first-use) SSH host key. Provision the BEX_SSH_KNOWN_HOSTS secret / pinned known_hosts file out of band (docs/ADR019-infra-credentials.md)." >&2
        return 1
        ;;
    esac
    BEX_SSH_HOSTKEY_TIER=tofu
    BEX_SSH_HOSTKEY_ARGS=(-o StrictHostKeyChecking=accept-new)
    echo "note: no host-key pin (BEX_SSH_KNOWN_HOSTS_FILE, BEX_CP_SSH_HOST_KEY_FILE) — the control-plane host key is trusted on first use against ~/.ssh/known_hosts (unauthenticated server identity)" >&2
    return 0
  fi
  BEX_SSH_HOSTKEY_ARGS=(
    -o "UserKnownHostsFile=$known"
    -o GlobalKnownHostsFile=/dev/null
    -o StrictHostKeyChecking=yes
  )
  echo "note: control-plane host key pinned, fail closed: $origin" >&2
}

# bex_ssh_hostkey_write_pin <content>: write the pin to a fresh temp file
# (mktemp creates it 0600) and record it in BEX_SSH_HOSTKEY_TMP.
bex_ssh_hostkey_write_pin() {
  BEX_SSH_HOSTKEY_TMP="$(mktemp "${TMPDIR:-/tmp}/bex-known-hosts.XXXXXX")" || {
    echo "error: could not create a temp known-hosts file" >&2
    return 1
  }
  printf '%s\n' "$1" > "$BEX_SSH_HOSTKEY_TMP"
}

# bex_cp_host_public_key <key file>: print `<type> <base64>` for the stable
# control-plane host key, from its `.pub` sibling (or the file itself when it
# is the `.pub`), else derived from the private key. An unreadable or malformed
# key is an error: a configured pin source never degrades to trust on first use.
bex_cp_host_public_key() {
  local key="$1" pub type="" blob=""
  case "$key" in
    *.pub) pub="$key" ;;
    *) pub="$key.pub" ;;
  esac
  if [ -r "$pub" ]; then
    read -r type blob _ < "$pub" || true
  elif [ -r "$key" ]; then
    read -r type blob _ < <(ssh-keygen -y -P '' -f "$key" 2> /dev/null) || true
  else
    echo "error: BEX_CP_SSH_HOST_KEY_FILE=$key: neither it nor $pub is readable — refusing to fall back to trust-on-first-use" >&2
    return 1
  fi
  if [[ ! "$type" =~ ^(ssh-ed25519|ecdsa-sha2-nistp(256|384|521)|ssh-rsa)$ ]] || [[ ! "$blob" =~ ^[A-Za-z0-9+/]+=*$ ]]; then
    echo "error: BEX_CP_SSH_HOST_KEY_FILE=$key does not yield an SSH public key — refusing to fall back to trust-on-first-use" >&2
    return 1
  fi
  printf '%s %s\n' "$type" "$blob"
}

# bex_ssh_hostkey_cleanup: remove the temp pin bex_ssh_hostkey_args wrote, if
# any. Safe to call more than once and under `set -u`.
bex_ssh_hostkey_cleanup() {
  [ -z "${BEX_SSH_HOSTKEY_TMP:-}" ] || rm -f -- "$BEX_SSH_HOSTKEY_TMP"
}
