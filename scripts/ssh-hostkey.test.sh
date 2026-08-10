#!/usr/bin/env bash
# Pure-bash regression tests for the shared SSH host-key policy (w1/m66 F7).
# No network, no ssh: the policy is a pure function of the environment, and the
# integration half asserts fetch-app-kubeconfig.sh actually passes the resulting
# options to ssh — with an `ssh` fake that records its argv and, in the pinned
# case, refuses the connection the way a real unknown-host-key failure would.
set -euo pipefail
cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fails=0
# Negated assertion helper: `assert "..." ! cmd` is not valid (bash resolves `!`
# as a command name once it is an argument), so express refusals explicitly.
refute() { ! "$@"; }

assert() {
  local description="$1"
  shift
  if "$@"; then
    echo "    ok: $description"
  else
    echo "FAIL: $description" >&2
    fails=$((fails + 1))
  fi
}

echo "lib/ssh-hostkey.sh"

# --- policy: unset => accept-new (byte-identical to pre-m66), and it says so ---
(
  unset BEX_SSH_KNOWN_HOSTS_FILE
  # shellcheck source=scripts/lib/ssh-hostkey.sh
  source scripts/lib/ssh-hostkey.sh
  bex_ssh_hostkey_args 2> "$tmp/unset.err"
  printf '%s\n' "${BEX_SSH_HOSTKEY_ARGS[@]}" > "$tmp/unset.args"
)
assert "unset pin keeps accept-new" grep -qx "StrictHostKeyChecking=accept-new" "$tmp/unset.args"
assert "unset pin is announced, never silent" grep -q "trusted on first use" "$tmp/unset.err"

# --- policy: set => fail-closed pinning ---
printf '1.2.3.4 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI0000000000000000000000000000000000000000000\n' > "$tmp/known_hosts"
(
  export BEX_SSH_KNOWN_HOSTS_FILE="$tmp/known_hosts"
  # shellcheck source=scripts/lib/ssh-hostkey.sh
  source scripts/lib/ssh-hostkey.sh
  bex_ssh_hostkey_args
  printf '%s\n' "${BEX_SSH_HOSTKEY_ARGS[@]}" > "$tmp/set.args"
)
assert "a pinned run fails closed on an unknown key" grep -qx "StrictHostKeyChecking=yes" "$tmp/set.args"
assert "a pinned run uses the supplied file" grep -qx "UserKnownHostsFile=$tmp/known_hosts" "$tmp/set.args"
assert "a pinned run ignores the global known-hosts file" grep -qx "GlobalKnownHostsFile=/dev/null" "$tmp/set.args"
assert "a pinned run never re-enables accept-new" refute grep -q "accept-new" "$tmp/set.args"

# --- policy: set but empty/missing => refuse, never silently degrade ---
rc=0
(
  export BEX_SSH_KNOWN_HOSTS_FILE="$tmp/does-not-exist"
  # shellcheck source=scripts/lib/ssh-hostkey.sh
  source scripts/lib/ssh-hostkey.sh
  bex_ssh_hostkey_args
) > /dev/null 2> "$tmp/empty.err" || rc=$?
assert "an empty/missing pin file is an error" test "$rc" -ne 0
assert "the empty-pin refusal explains itself" grep -q "refusing to fall back" "$tmp/empty.err"

echo "fetch-app-kubeconfig.sh"

# --- integration: the options actually reach ssh, and a host-key failure stops
#     the fetch BEFORE any kubeconfig is written ---
mkdir -p "$tmp/bin"
cat > "$tmp/bin/curl" <<'EOF'
#!/usr/bin/env bash
# The hcloud server list: one running control-plane candidate.
cat <<'JSON'
{"servers":[{"name":"bex-control-plane-abc","status":"running","created":"2026-01-01T00:00:00Z","public_net":{"ipv4":{"ip":"1.2.3.4"}}}]}
JSON
EOF
cat > "$tmp/bin/ssh" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$tmp/ssh.argv"
# Refuse exactly as ssh does when the host key is not in the pinned file.
echo "Host key verification failed." >&2
exit 255
EOF
cat > "$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "kubectl must not run after a host-key failure" >&2
exit 1
EOF
chmod +x "$tmp/bin"/*

out="$tmp/app.kubeconfig"
rc=0
PATH="$tmp/bin:$PATH" \
  HCLOUD_TOKEN=fake \
  BEX_SSH_KEY_PATH="$tmp/key" \
  BEX_SSH_KNOWN_HOSTS_FILE="$tmp/known_hosts" \
  bash scripts/fetch-app-kubeconfig.sh "$out" > "$tmp/fetch.out" 2>&1 || rc=$?

assert "a host-key failure fails the fetch" test "$rc" -ne 0
assert "no kubeconfig is written when the host key is not authenticated" test ! -e "$out"
assert "ssh was told to fail closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/ssh.argv"
assert "ssh was given the pinned known-hosts file" grep -qx "UserKnownHostsFile=$tmp/known_hosts" "$tmp/ssh.argv"
assert "the legacy accept-new flag is gone from the fetch path" refute grep -q "accept-new" "$tmp/ssh.argv"

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "all ssh host-key checks passed"
