#!/usr/bin/env bash
# Self-test for scripts/edge-proxyprotocol-validate.sh (w1/m150 t006). The
# anti-tautology rule: a guard with no proven red case is decoration. Both
# failure modes it guards are silent until traffic hits them — an HTTP(S)
# listener left without PROXY protocol makes every inbound IP allow-list match
# the load balancer (10.10.0.7) instead of the client, and a widened Traefik
# trust set lets any peer in it assert a client address.
#
# Method: copy the real main.tf and prod Traefik values into a fixture, point
# the validator at them with TERRAFORM_MAIN / TRAEFIK_PROD_VALUES, mutate one
# fact per case, and assert the exit code and the message.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/.." && pwd)"
SCRIPT="$here/edge-proxyprotocol-validate.sh"

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fixture <name> — a fresh copy of both inputs; echoes the fixture dir.
fixture() {
  local dir="$tmp/$1"
  mkdir -p "$dir"
  cp "$root/infra/terraform/main.tf" "$dir/main.tf"
  cp "$root/deploy/gitops/overlays/prod/values/traefik.values.yaml" "$dir/traefik.values.yaml"
  echo "$dir"
}

# set_proxyprotocol <dir> <listener> <true|false> — rewrite one listener's value.
set_proxyprotocol() {
  awk -v name="$2" -v value="$3" '
    $0 == "resource \"hcloud_load_balancer_service\" \"" name "\" {" { found=1 }
    found && /^[[:space:]]*proxyprotocol[[:space:]]*=/ { sub(/=.*/, "= " value); found=0 }
    { print }
  ' "$1/main.tf" >"$1/main.tf.new"
  mv "$1/main.tf.new" "$1/main.tf"
}

# assert <label> <want-rc> <fixture-dir> [expected-stderr-substring]
assert() {
  local label="$1" want="$2" dir="$3" needle="${4:-}" err got
  set +e
  err="$(TERRAFORM_MAIN="$dir/main.tf" TRAEFIK_PROD_VALUES="$dir/traefik.values.yaml" "$SCRIPT" 2>&1 >/dev/null)"
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    printf '%s\n' "$err" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  if [ -n "$needle" ] && ! printf '%s' "$err" | grep -qF "$needle"; then
    echo "FAIL: $label — stderr did not name '$needle'" >&2
    printf '%s\n' "$err" | sed 's/^/    /' >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

# ── GREEN: the canonical tree ────────────────────────────────────────────────
assert "canonical tree passes" 0 "$(fixture green)"

# ── RED: listeners ───────────────────────────────────────────────────────────
# The pre-m150 production shape: Traefik sees every client as 10.10.0.7, and
# because Traefik trusts 10.10.0.7, a client-written PROXY line is believed.
for listener in http https; do
  d="$(fixture "$listener-plain")"
  set_proxyprotocol "$d" "$listener" false
  assert "$listener listener without PROXY protocol" 1 "$d" "edge listener $listener must set proxyprotocol=true"
done

# ssh has its own PROXY chain (Traefik → ssh-gateway); a header from the LB
# would land in the SSH version exchange.
d="$(fixture ssh-proxy)"
set_proxyprotocol "$d" ssh true
assert "ssh listener with PROXY protocol" 1 "$d" "edge listener ssh must set proxyprotocol=false"

d="$(fixture postgres-plain)"
set_proxyprotocol "$d" postgres false
assert "postgres listener without PROXY protocol" 1 "$d" "edge listener postgres must set proxyprotocol=true"

d="$(fixture http-port)"
sed -i.bak 's/destination_port = 31218/destination_port = 31219/' "$d/main.tf"
assert "http listener on the wrong NodePort" 1 "$d" "edge listener http must map :80 to NodePort 31218"

# ── RED: Traefik entrypoints ─────────────────────────────────────────────────
for ep in web websecure; do
  d="$(fixture "$ep-missing")"
  yq -i "del(.ports.$ep.proxyProtocol)" "$d/traefik.values.yaml"
  assert "$ep without proxyProtocol" 1 "$d" "Traefik entrypoint $ep proxyProtocol"

  d="$(fixture "$ep-wide")"
  yq -i ".ports.$ep.proxyProtocol.trustedIPs = [\"10.10.0.0/16\"]" "$d/traefik.values.yaml"
  assert "$ep trusting the whole private network" 1 "$d" "Traefik entrypoint $ep proxyProtocol"

  d="$(fixture "$ep-extra")"
  yq -i ".ports.$ep.proxyProtocol.trustedIPs += [\"0.0.0.0/0\"]" "$d/traefik.values.yaml"
  assert "$ep trusting an extra range" 1 "$d" "Traefik entrypoint $ep proxyProtocol"

  d="$(fixture "$ep-insecure")"
  yq -i ".ports.$ep.proxyProtocol.insecure = true" "$d/traefik.values.yaml"
  assert "$ep with proxyProtocol.insecure" 1 "$d" "Traefik entrypoint $ep proxyProtocol"
done

if [ "$fails" -ne 0 ]; then
  echo "$fails edge PROXY-protocol guard case(s) failed" >&2
  exit 1
fi
echo "edge PROXY-protocol guard: all cases pass"
