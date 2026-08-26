#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verify="$repo_root/scripts/ssh-verify.sh"
openssh_shim="$repo_root/scripts/ssh-verify-openssh.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# The missing-Render-CLI preflight must not depend on the host carrying the Go
# toolchain used by the later live PTY probe. Provide only command discovery;
# this path must fail before the verifier could execute it.
mkdir -p "$tmp/bin"
printf '#!/usr/bin/env bash\nexit 99\n' > "$tmp/bin/go"
chmod +x "$tmp/bin/go"

set +e
output="$(
  PATH="$tmp/bin:$PATH" \
    BEX_API_URL=https://api.example.test \
    BEX_API_TOKEN=test-token \
    BEX_SSH_VERIFY_PRIVATE_KEY_FILE="$tmp/not-read" \
    BEX_SSH_EXPECTED_HOST_FINGERPRINT=SHA256:test \
    BEX_RENDER_CLI_VERIFY=1 \
    BEX_RENDER_CLI_BIN="$tmp/missing-render" \
    bash "$verify" 2>&1
)"
status=$?
set -e
[[ "$status" != 0 && "$output" == "FAIL Render CLI is not executable: $tmp/missing-render" ]] || {
  echo "exact Render CLI validation did not fail before fixture setup: $output" >&2
  exit 1
}

mock_ssh="$tmp/ssh"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'printf "%s\n" "$@" >"$TEST_SSH_ARGS"' >"$mock_ssh"
chmod +x "$mock_ssh"

export TEST_SSH_ARGS="$tmp/ssh.args"
export BEX_SSH_VERIFY_REAL_SSH="$mock_ssh"
export BEX_SSH_VERIFY_CLI_KNOWN_HOSTS="$tmp/known_hosts"
export BEX_SSH_VERIFY_PRIVATE_KEY_FILE="$tmp/client-key"
export BEX_SSH_VERIFY_CLI_TARGET_LOG="$tmp/target"

touch "$BEX_SSH_VERIFY_CLI_KNOWN_HOSTS"
bash "$openssh_shim" 'srv-test-instance@ssh.example.test' 'exit 37'

[[ "$(<"$BEX_SSH_VERIFY_CLI_TARGET_LOG")" == 'srv-test-instance@ssh.example.test' ]]
for expected in \
  '-tt' \
  'BatchMode=yes' \
  'ConnectTimeout=10' \
  'StrictHostKeyChecking=yes' \
  "UserKnownHostsFile=$BEX_SSH_VERIFY_CLI_KNOWN_HOSTS" \
  'IdentitiesOnly=yes' \
  '-i' \
  "$BEX_SSH_VERIFY_PRIVATE_KEY_FILE" \
  'srv-test-instance@ssh.example.test' \
  'exit 37'; do
  grep -Fxq -- "$expected" "$TEST_SSH_ARGS" || {
    echo "OpenSSH shim omitted argument: $expected" >&2
    exit 1
  }
done

echo 'PASS SSH verifier CLI pinning and OpenSSH shim'
