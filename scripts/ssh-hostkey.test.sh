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
# The stable control-plane key has a default path under $HOME: a maintainer's
# real ~/.ssh/bex_cp_host_ed25519 must never decide a case here.
export HOME="$tmp/home"
mkdir -p "$HOME/.ssh"
unset BEX_SSH_KNOWN_HOSTS_FILE BEX_SSH_KNOWN_HOSTS BEX_CP_SSH_HOST_KEY_FILE BEX_SSH_REQUIRE_KNOWN_HOSTS
# Temp pins land here, so a test can prove none survives.
export TMPDIR="$tmp/tmpdir"
mkdir -p "$TMPDIR"

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

# policy_args <name>: run the policy in a subshell with the caller's
# environment; the options land in $tmp/<name>.args, stderr in <name>.err, the
# tier in <name>.tier and the temp pin's content in <name>.pin (removed after,
# as the callers' traps do). Returns the policy's status.
policy_args() {
  local name="$1"
  (
    # shellcheck source=scripts/lib/ssh-hostkey.sh
    source scripts/lib/ssh-hostkey.sh
    rc=0
    bex_ssh_hostkey_args 2> "$tmp/$name.err" || rc=$?
    printf '%s\n' "${BEX_SSH_HOSTKEY_ARGS[@]+"${BEX_SSH_HOSTKEY_ARGS[@]}"}" > "$tmp/$name.args"
    printf '%s\n' "$BEX_SSH_HOSTKEY_TIER" > "$tmp/$name.tier"
    : > "$tmp/$name.pin"
    if [ -n "$BEX_SSH_HOSTKEY_TMP" ]; then
      cat "$BEX_SSH_HOSTKEY_TMP" > "$tmp/$name.pin"
      ls -l "$BEX_SSH_HOSTKEY_TMP" | cut -c1-10 > "$tmp/$name.mode"
    fi
    bex_ssh_hostkey_cleanup
    exit "$rc"
  )
}
no_temp_pins() { [ -z "$(ls -A "$TMPDIR")" ]; }

# --- policy: nothing set => accept-new (byte-identical to pre-m66), and it says so ---
policy_args unset
assert "unset pin keeps accept-new" grep -qx "StrictHostKeyChecking=accept-new" "$tmp/unset.args"
assert "unset pin is announced, never silent" grep -q "trusted on first use" "$tmp/unset.err"
assert "unset pin reports the tofu tier" grep -qx tofu "$tmp/unset.tier"

# --- policy: set => fail-closed pinning ---
printf '1.2.3.4 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI0000000000000000000000000000000000000000000\n' > "$tmp/known_hosts"
BEX_SSH_KNOWN_HOSTS_FILE="$tmp/known_hosts" policy_args set
assert "a pinned run fails closed on an unknown key" grep -qx "StrictHostKeyChecking=yes" "$tmp/set.args"
assert "a pinned run uses the supplied file" grep -qx "UserKnownHostsFile=$tmp/known_hosts" "$tmp/set.args"
assert "a pinned run ignores the global known-hosts file" grep -qx "GlobalKnownHostsFile=/dev/null" "$tmp/set.args"
assert "a pinned run never re-enables accept-new" refute grep -q "accept-new" "$tmp/set.args"
assert "a pinned run reports the pinned tier" grep -qx pinned "$tmp/set.tier"

# --- policy: set but empty/missing => refuse, never silently degrade ---
rc=0
BEX_SSH_KNOWN_HOSTS_FILE="$tmp/does-not-exist" policy_args empty || rc=$?
assert "an empty/missing pin file is an error" test "$rc" -ne 0
assert "the empty-pin refusal explains itself" grep -q "refusing to fall back" "$tmp/empty.err"

# --- policy: the pin's content (BEX_SSH_KNOWN_HOSTS) => a 0600 temp pin ---
BEX_SSH_KNOWN_HOSTS="$(cat "$tmp/known_hosts")" policy_args content
assert "a pin given as content is written to a temp file" grep -qx "UserKnownHostsFile=$TMPDIR/bex-known-hosts.*" "$tmp/content.args"
assert "the content pin holds exactly the given entries" cmp -s "$tmp/known_hosts" "$tmp/content.pin"
assert "the content pin fails closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/content.args"
assert "the content pin is private (0600)" grep -qx -- "-rw-------" "$tmp/content.mode"
assert "the content pin is pinned, not derived" grep -qx pinned "$tmp/content.tier"

# --- policy: the stable control-plane key => one derived wildcard line ---
ssh-keygen -q -t ed25519 -N '' -C bex-control-plane -f "$tmp/cp_host" > /dev/null
ssh-keygen -q -t ed25519 -N '' -C other -f "$tmp/other_host" > /dev/null
read -r cp_type cp_blob _ < "$tmp/cp_host.pub"
want_pin="* $cp_type $cp_blob"
BEX_CP_SSH_HOST_KEY_FILE="$tmp/cp_host" policy_args derived
assert "the stable key pins UserKnownHostsFile to a temp file" grep -qx "UserKnownHostsFile=$TMPDIR/bex-known-hosts.*" "$tmp/derived.args"
assert "the stable key ignores the global known-hosts file" grep -qx "GlobalKnownHostsFile=/dev/null" "$tmp/derived.args"
assert "the stable key fails closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/derived.args"
assert "the derived pin is exactly one '* ssh-ed25519 <key>' line" test "$(cat "$tmp/derived.pin")" = "$want_pin"
assert "the derived pin is private (0600)" grep -qx -- "-rw-------" "$tmp/derived.mode"
assert "the derived tier is reported" grep -qx derived "$tmp/derived.tier"
assert "the derived tier is announced by name" grep -q "derived from BEX_CP_SSH_HOST_KEY_FILE" "$tmp/derived.err"
assert "no key material reaches stderr" refute grep -qF "$cp_blob" "$tmp/derived.err"

# Only the private half: derived with ssh-keygen -y. Only the public half
# (the variable pointing at the .pub): read as is.
mkdir -p "$tmp/private-only" "$tmp/public-only"
cp "$tmp/cp_host" "$tmp/private-only/cp_host"
cp "$tmp/cp_host.pub" "$tmp/public-only/cp_host.pub"
BEX_CP_SSH_HOST_KEY_FILE="$tmp/private-only/cp_host" policy_args private-only
assert "a private key without .pub derives the same pin" test "$(cat "$tmp/private-only.pin")" = "$want_pin"
BEX_CP_SSH_HOST_KEY_FILE="$tmp/public-only/cp_host.pub" policy_args public-only
assert "the .pub half alone gives the same pin" test "$(cat "$tmp/public-only.pin")" = "$want_pin"

# The documented default path, when the variable is unset.
cp "$tmp/cp_host.pub" "$HOME/.ssh/bex_cp_host_ed25519.pub"
policy_args default
assert "the stable key defaults to ~/.ssh/bex_cp_host_ed25519" test "$(cat "$tmp/default.pin")" = "$want_pin"
assert "the default stable key fails closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/default.args"
assert "the default stable key never trusts on first use" refute grep -q "accept-new" "$tmp/default.args"

# An explicit pin file outranks the stable key, and so does the pin's content.
BEX_SSH_KNOWN_HOSTS_FILE="$tmp/known_hosts" BEX_CP_SSH_HOST_KEY_FILE="$tmp/cp_host" policy_args file-wins
assert "an explicit BEX_SSH_KNOWN_HOSTS_FILE wins over the stable key" grep -qx "UserKnownHostsFile=$tmp/known_hosts" "$tmp/file-wins.args"
assert "the stable key is ignored under an explicit pin" test ! -s "$tmp/file-wins.pin"
BEX_SSH_KNOWN_HOSTS="$(cat "$tmp/known_hosts")" BEX_CP_SSH_HOST_KEY_FILE="$tmp/cp_host" policy_args content-wins
assert "BEX_SSH_KNOWN_HOSTS wins over the stable key" cmp -s "$tmp/known_hosts" "$tmp/content-wins.pin"
rm -f "$HOME/.ssh/bex_cp_host_ed25519.pub"

# A configured stable key that is missing or not a key is an error, never a
# silent fall back to trust on first use.
rc=0
BEX_CP_SSH_HOST_KEY_FILE="$tmp/no-such-key" policy_args missing-key || rc=$?
assert "a missing stable key is an error" test "$rc" -ne 0
assert "the missing-key refusal names the variable" grep -q "BEX_CP_SSH_HOST_KEY_FILE=$tmp/no-such-key" "$tmp/missing-key.err"
assert "a missing stable key never degrades to accept-new" refute grep -q "accept-new" "$tmp/missing-key.args"
printf 'not a key\n' > "$tmp/garbage.pub"
rc=0
BEX_CP_SSH_HOST_KEY_FILE="$tmp/garbage.pub" policy_args garbage-key || rc=$?
assert "a malformed stable key is an error" test "$rc" -ne 0
assert "a malformed stable key never degrades to accept-new" refute grep -q "accept-new" "$tmp/garbage-key.args"

# --- policy: the pin requirement (CI) is met by any authenticated tier ---
rc=0
BEX_SSH_REQUIRE_KNOWN_HOSTS=1 policy_args require-none || rc=$?
assert "the requirement refuses trust on first use" test "$rc" -ne 0
assert "the requirement's refusal names every pin input" grep -q "BEX_CP_SSH_HOST_KEY_FILE" "$tmp/require-none.err"
BEX_SSH_REQUIRE_KNOWN_HOSTS=1 BEX_CP_SSH_HOST_KEY_FILE="$tmp/cp_host" policy_args require-derived
assert "a derived pin satisfies the requirement" grep -qx derived "$tmp/require-derived.tier"
assert "no temp pin survives the policy runs" no_temp_pins

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
  BEX_KUBECONFIG_FETCH_RETRY_SECONDS=0 \
  bash scripts/fetch-app-kubeconfig.sh "$out" > "$tmp/fetch.out" 2>&1 || rc=$?

assert "a host-key failure fails the fetch" test "$rc" -ne 0
assert "no kubeconfig is written when the host key is not authenticated" test ! -e "$out"
assert "ssh was told to fail closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/ssh.argv"
assert "ssh was given the pinned known-hosts file" grep -qx "UserKnownHostsFile=$tmp/known_hosts" "$tmp/ssh.argv"
assert "the legacy accept-new flag is gone from the fetch path" refute grep -q "accept-new" "$tmp/ssh.argv"

# --- integration: the derived pin against a server that verifies like ssh ---
# This ssh fake checks host keys the way OpenSSH does under the options it is
# given: StrictHostKeyChecking=yes, and the served key ($SERVED_KEY) must be in
# UserKnownHostsFile as a `*` (or exact-IP) entry. It copies the pin it saw,
# since the script removes the temp file on exit.
cat > "$tmp/bin/ssh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$SSH_ARGV"
known="" strict="" host=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      case "$2" in
        UserKnownHostsFile=*) known="${2#UserKnownHostsFile=}" ;;
        StrictHostKeyChecking=*) strict="${2#StrictHostKeyChecking=}" ;;
      esac
      shift 2
      ;;
    -i) shift 2 ;;
    root@*) host="${1#root@}"; shift ;;
    *) shift ;;
  esac
done
[ -z "$known" ] || cp "$known" "$SSH_PIN"
read -r type blob _ < "$SERVED_KEY"
if [ "$strict" = yes ] && [ -n "$known" ] \
  && awk -v h="$host" -v t="$type" -v b="$blob" '($1 == "*" || $1 == h) && $2 == t && $3 == b { found = 1 } END { exit !found }' "$known"; then
  echo "apiVersion: v1"
  exit 0
fi
echo "Host key verification failed." >&2
exit 255
EOF
printf '#!/usr/bin/env bash\nexit 0\n' > "$tmp/bin/kubectl"
chmod +x "$tmp/bin"/*
# A stale ~/.ssh/known_hosts entry for the control-plane IP, as on the
# maintainer machines that hit this (w3/m472).
printf '1.2.3.4 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIstale0000000000000000000000000000000000000\n' > "$HOME/.ssh/known_hosts"
cp "$HOME/.ssh/known_hosts" "$tmp/known_hosts.before"

fetch_derived() {
  rm -f "$out"
  env PATH="$tmp/bin:$PATH" HCLOUD_TOKEN=fake BEX_SSH_KEY_PATH="$tmp/key" \
    BEX_CP_SSH_HOST_KEY_FILE="$tmp/cp_host" SSH_ARGV="$tmp/ssh.argv" SSH_PIN="$tmp/ssh.pin" \
    BEX_KUBECONFIG_FETCH_RETRY_SECONDS=0 BEX_KUBECONFIG_FETCH_ATTEMPTS=2 "$@" \
    bash scripts/fetch-app-kubeconfig.sh "$out" > "$tmp/fetch.out" 2>&1
}

rc=0
fetch_derived SERVED_KEY="$tmp/cp_host.pub" || rc=$?
assert "the derived pin authenticates the stable key despite a stale ~/.ssh/known_hosts" test "$rc" -eq 0
assert "the derived run writes the kubeconfig" test -s "$out"
assert "the derived run fails closed" grep -qx "StrictHostKeyChecking=yes" "$tmp/ssh.argv"
assert "the derived run ignores the global known-hosts file" grep -qx "GlobalKnownHostsFile=/dev/null" "$tmp/ssh.argv"
assert "the derived run never points ssh at ~/.ssh/known_hosts" refute grep -q "UserKnownHostsFile=$HOME/.ssh/known_hosts" "$tmp/ssh.argv"
assert "ssh saw exactly the derived wildcard pin" test "$(cat "$tmp/ssh.pin")" = "$want_pin"
assert "the stale ~/.ssh/known_hosts is left as it was" cmp -s "$tmp/known_hosts.before" "$HOME/.ssh/known_hosts"
assert "no temp pin survives a successful fetch" no_temp_pins

rc=0
fetch_derived SERVED_KEY="$tmp/other_host.pub" || rc=$?
assert "a server whose key does not match the stable key fails closed" test "$rc" -ne 0
assert "no kubeconfig is written for a mismatched server key" test ! -e "$out"
assert "a derived-pin mismatch keeps the bounded mid-roll retry" grep -q "after 2 attempt" "$tmp/fetch.out"
assert "no temp pin survives a failed fetch" no_temp_pins

if [ "$fails" -ne 0 ]; then
  echo "$fails check(s) failed" >&2
  exit 1
fi
echo "all ssh host-key checks passed"
