#!/usr/bin/env bash
# Self-test for scripts/lib/verify-bootstrap-secret.sh. A verify script that
# seeds bex-bootstrap on the current kubeconfig cluster uses the secret the
# caller exported or .env holds, makes up a 256-bit random one only when the
# apiserver is on loopback, and otherwise refuses before touching the cluster.
# A static pass pins every script that seeds the cluster's own Hydra to that
# helper. Stub kubectl, no cluster.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/.." && pwd)"
helper="$repo/scripts/lib/verify-bootstrap-secret.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fails=0
ok() { echo "ok: $1"; }
bad() { echo "FAIL: $1" >&2; fails=$((fails + 1)); }

# kubectl stub: answers only the apiserver lookup, with $FAKE_SERVER.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
[ "$*" = "config view --minify -o jsonpath={.clusters[0].cluster.server}" ] \
  || { echo "kubectl stub: unexpected call: $*" >&2; exit 2; }
printf '%s' "${FAKE_SERVER:-}"
EOF
chmod +x "$tmp/bin/kubectl"

# workdir <name> [.env body] [mode] — a repo-root stand-in, with .env when given.
workdir() {
  local d="$tmp/$1"
  mkdir -p "$d/scripts/lib"
  ln -sf "$repo/scripts/lib/env-file.sh" "$d/scripts/lib/env-file.sh"
  if [ "$#" -ge 2 ]; then printf '%s\n' "$2" >"$d/.env"; chmod "${3:-600}" "$d/.env"; fi
  printf '%s' "$d"
}

# resolve <dir> <server> [VAR=value…] — runs the helper from <dir> with nothing
# but the given variables; stdout "<secret>|<DOTENV_OTHER>", rc and stderr kept.
resolve() {
  local dir="$1" server="$2"; shift 2
  (cd "$dir" && env -i HOME="$tmp" PATH="$tmp/bin:$PATH" FAKE_SERVER="$server" "$@" \
    bash -c '. "$1" && bex_verify_bootstrap_secret && printf "%s|%s" "$BEX_BOOTSTRAP_CLIENT_SECRET" "${DOTENV_OTHER:-}"' _ "$helper")
}

REMOTE=https://203.0.113.10:6443
FROM_DOTENV=value-read-from-dotenv

# 1. An exported secret wins over .env, whatever the cluster.
d="$(workdir exported "BEX_BOOTSTRAP_CLIENT_SECRET=$FROM_DOTENV")"
got="$(resolve "$d" "$REMOTE" BEX_BOOTSTRAP_CLIENT_SECRET=value-the-caller-exported)" || got="rc=$?"
[ "$got" = "value-the-caller-exported|" ] && ok "exported secret wins over .env" || bad "exported secret: got '$got'"

# 2. .env's secret is reused against a remote cluster, and nothing else from
#    .env reaches the caller.
d="$(workdir dotenv "BEX_BOOTSTRAP_CLIENT_SECRET=$FROM_DOTENV
DOTENV_OTHER=leaked")"
got="$(resolve "$d" "$REMOTE")" || got="rc=$?"
[ "$got" = "$FROM_DOTENV|" ] && ok ".env secret reused; other .env keys stay out of the caller" || bad ".env secret: got '$got'"

# 3. A .env other users can read is refused, not read.
d="$(workdir dotenv-open "BEX_BOOTSTRAP_CLIENT_SECRET=$FROM_DOTENV" 644)"
if err="$(resolve "$d" "$REMOTE" 2>&1)"; then bad "0644 .env was read: '$err'"
elif printf '%s' "$err" | grep -qF "chmod 600"; then ok "0644 .env refused with the fix named"
else bad "0644 .env refusal did not name the fix: '$err'"; fi

# 4. No known secret on a loopback cluster: a fresh 256-bit random value.
d="$(workdir none)"
for server in https://127.0.0.1:6443 https://localhost:6443 'https://[::1]:6443' https://0.0.0.0:41234 https://127.0.0.1:6443/; do
  a="$(resolve "$d" "$server")" || a="rc=$?"
  b="$(resolve "$d" "$server")" || b="rc=$?"
  a="${a%|}"; b="${b%|}"
  if [[ "$a" =~ ^[0-9a-f]{64}$ ]] && [[ "$b" =~ ^[0-9a-f]{64}$ ]] && [ "$a" != "$b" ]; then
    ok "loopback $server: fresh 256-bit random secret per run"
  else bad "loopback $server: got '$a' and '$b'"; fi
done

# 5. No known secret on any other cluster: refused, nothing exported. Includes
#    a .env carrying the key empty, as .env.example does.
d_empty="$(workdir empty-key "BEX_BOOTSTRAP_CLIENT_SECRET=")"
for server in "$REMOTE" https://bex-cp.example.com:6443 https://127.0.0.1.example.com:6443 https://10.0.0.5:6443 ""; do
  for dir in "$d" "$d_empty"; do
    if out="$(resolve "$dir" "$server" 2>"$tmp/err")"; then bad "server '$server' ($(basename "$dir")): accepted, got '$out'"
    elif [ -n "$out" ]; then bad "server '$server' ($(basename "$dir")): refused but printed '$out'"
    elif grep -qF "not local" "$tmp/err"; then ok "server '$server' ($(basename "$dir")): refused without a known secret"
    else bad "server '$server' ($(basename "$dir")): refusal did not say why: $(cat "$tmp/err")"; fi
  done
done

# 6. Static, over every script that reaches the cluster's own Hydra (a
#    port-forwarded hydra-admin; throwaway docker Hydras are out of scope):
#    one that seeds bex-bootstrap resolves its secret through the helper and
#    sets it nowhere else, and none derives a client secret from the clock.
cd "$repo"
seeders=0
for f in scripts/*.sh; do
  grep -qE 'port-forward "?(service|svc)/hydra-admin' "$f" || continue
  if grep -nE 'CLIENT_SECRET[A-Za-z_]*=.*(date \+%s|\$STAMP|\$\{STAMP)' "$f" | grep -q .; then
    bad "$f derives a client secret from the clock"
  fi
  grep -qE 'bash scripts/auth-bootstrap-client\.sh' "$f" || continue
  seeders=$((seeders + 1))
  if ! grep -qE '^\. scripts/lib/verify-bootstrap-secret\.sh && bex_verify_bootstrap_secret \|\| exit 1$' "$f"; then
    bad "$f seeds the cluster's bex-bootstrap without bex_verify_bootstrap_secret"
  elif grep -qE 'BEX_BOOTSTRAP_CLIENT_SECRET(=|:-)' "$f"; then
    bad "$f sets BEX_BOOTSTRAP_CLIENT_SECRET outside the helper"
  else ok "$f resolves the bex-bootstrap secret through the helper"; fi
done
[ "$seeders" -ge 6 ] || bad "found only $seeders cluster-seeding verify scripts; the static scan no longer matches them"

[ "$fails" -eq 0 ] && echo "PASS: verify-bootstrap-secret.sh" || { echo "FAIL: $fails case(s)" >&2; exit 1; }
