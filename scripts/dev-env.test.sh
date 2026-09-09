#!/usr/bin/env bash
# Unit tests for scripts/dev-env.sh — the single dev-N harness w1/m72 folded ten
# copies into. Pure bash: no cluster, no kubectl, no network.
#
# What is worth testing here is exactly what makes ONE shared harness riskier
# than ten copies: a derivation collision or a `down` that reaches past its own N
# would now be everyone's bug at once. Two dev-N stacks sharing a control-plane
# identity have already deleted each other's tenant namespaces once (.pm/w3/017.md).
#
#   bash scripts/dev-env.test.sh   # exits 0 on pass
set -euo pipefail
cd "$(dirname "$0")/.."

fails=0
assert() { # desc  condition
  if eval "$2"; then echo "    ok: $1"; else
    echo "FAIL: $1" >&2
    fails=$((fails + 1))
  fi
}

devenv() { bash scripts/dev-env.sh "$@"; }
value_of() { # N KEY
  devenv "$1" env | sed -n "s/^$2=//p"
}

echo "==> per-N derivation is distinct for N=1..10"
for key in DEV_NS DEV_AUTH_NS BEX_CP_IDENTITY KUBECONFIG_FILE \
  DASHBOARD_PORT KRATOS_PUBLIC_PORT KRATOS_ADMIN_PORT HYDRA_ADMIN_PORT \
  HYDRA_PUBLIC_PORT MAILPIT_HTTP_PORT MAILPIT_SMTP_PORT BEX_API_PORT \
  BEX_DB_PORT BEX_CP_PORT LOKI_PORT \
  OPENSANDBOX_PORT AGENT_ATTACH_PORT OPENFGA_PORT OPENBAO_PORT SANDBOX_EXEC_PORT; do
  values=""
  for n in 1 2 3 4 5 6 7 8 9 10; do values+="$(value_of "$n" "$key")"$'\n'; done
  distinct="$(printf '%s' "$values" | sort -u | wc -l | tr -d ' ')"
  assert "$key is distinct across all ten workstreams" "[ '$distinct' -eq 10 ]"
done

echo "==> no two settings collide on one port for a given N"
# The retired copies collided here twice: w5's LOKI_PORT and w3/w9's
# HYDRA_PUBLIC_PORT both claimed 58000+N*10, and w1's Mailpit SMTP forward
# claimed the same slot again.
for n in 1 5 10; do
  ports="$(devenv "$n" env | sed -n 's/^[A-Z_]*PORT=//p')"
  total="$(printf '%s\n' "$ports" | wc -l | tr -d ' ')"
  uniq_total="$(printf '%s\n' "$ports" | sort -u | wc -l | tr -d ' ')"
  assert "dev-$n's ports are mutually distinct ($total settings)" "[ '$total' -eq '$uniq_total' ]"
done

echo "==> the agent overlay wires derived ports and removes insecure authz"
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
agent_env_block="$(sed -n '/if agent_enabled; then/,/  else/p' scripts/dev-env.sh)"
assert "OpenFGA uses the per-N forward" \
  "printf '%s' \"\$agent_env_block\" | grep -q 'BEX_OPENFGA_URL=\"http://localhost:\$OPENFGA_PORT\"'"
assert "OpenBao uses the per-N forward" \
  "printf '%s' \"\$agent_env_block\" | grep -q 'BEX_OPENBAO_URL=\"http://localhost:\$OPENBAO_PORT\"'"
assert "agent attach uses the per-N gateway forward" \
  "printf '%s' \"\$agent_env_block\" | grep -q 'BEX_AGENT_SESSION_GATEWAY_URL=\"http://localhost:\$AGENT_ATTACH_PORT\"'"
assert "sandbox exec uses the per-N gateway forward" \
  "printf '%s' \"\$agent_env_block\" | grep -q 'BEX_SANDBOX_EXEC_URL=\"http://localhost:\$SANDBOX_EXEC_PORT/sandbox-exec\"'"
assert "agent mode does not set BEX_ALLOW_INSECURE_AUTHZ" \
  "! printf '%s' \"\$agent_env_block\" | grep -q 'BEX_ALLOW_INSECURE_AUTHZ'"
assert "real GitHub App OAuth credentials are forwarded to bex-api" \
  "grep -q 'BEX_GITHUB_APP_CLIENT_ID=\"\$BEX_GITHUB_APP_CLIENT_ID\"' scripts/dev-env.sh && grep -q 'BEX_GITHUB_APP_CLIENT_SECRET=\"\$BEX_GITHUB_APP_CLIENT_SECRET\"' scripts/dev-env.sh"

echo "==> agent-enabled status covers every live gate"
for needle in "OpenFGA reachable" "OpenBao reachable" "OpenSandbox lifecycle server healthy" \
  "OpenSandbox reverse hop reaches host bex-api" "ssh-gateway ready" "capabilities.enabled=true"; do
  assert "status checks $needle" "grep -q '$needle' scripts/dev-env.sh"
done
assert "reverse-hop bridge uses the derived host IPv4" \
  "grep -q 'upstream = (\"__HOST_DOCKER_IPV4__\", __BEX_CP_PORT__)' scripts/dev-env/agent/host-api.yaml"
assert "reverse-hop bridge is control-plane host-networked" \
  "grep -q 'hostNetwork: true' scripts/dev-env/agent/host-api.yaml"
assert "OpenSandbox uses the in-cluster reverse-hop Service" \
  "grep -q '__AGENT_HOST_API_HOST__:__AGENT_HOST_API_PORT__' scripts/dev-env/agent/sandbox-local.toml"
assert "ssh-gateway uses the in-cluster reverse-hop Service" \
  "grep -q '__AGENT_HOST_API_HOST__:__AGENT_HOST_API_PORT__' scripts/dev-env/agent/ssh-gateway.yaml"
assert "local sandboxes co-locate with the gateway on the control-plane node" \
  "grep -q 'node-role.kubernetes.io/control-plane' scripts/dev-env/agent/batchsandbox-template.local.yaml"
assert "agent-stub refreshes both gateway forwards after its rollout" \
  "sed -n '/cmd_agent_stub()/,/cmd_agent_stub_off()/p' scripts/dev-env.sh | grep -q 'refresh_agent_gateway_forwards'"
assert "agent-stub-off recreates the gateway without strategic-patch residue" \
  "sed -n '/cmd_agent_stub_off()/,/cmd_agent_down()/p' scripts/dev-env.sh | grep -q 'delete deploy bex-ssh-gateway'"
assert "agent-down removes model-stub resources" \
  "sed -n '/cmd_agent_down()/,/entrypoint/p' scripts/dev-env.sh | grep -q 'deploy/model-stub'"
assert "the live verifier supports the local gateway stream origin" \
  "grep -q 'BEX_VERIFY_STREAM_URL' scripts/agent-session-verify.sh"

echo "==> the identity every prune is scoped by is per-N"
for n in 1 2 3 4 5 6 7 8 9 10; do
  assert "dev-$n's BEX_CP_IDENTITY is dev-$n" "[ \"$(value_of "$n" BEX_CP_IDENTITY)\" = 'dev-$n' ]"
done

echo "==> the checked-in ports.env records agree with the derivation"
# ports.env is generated from the same derivation; a drift between the two is
# how the values files ended up documenting ports nothing listened on.
for n in 1 2 3 4 5 6 7 8 9 10; do
  f=".pm/w$n/dev-$n/ports.env"
  [ -f "$f" ] || continue
  mismatch=0
  while IFS='=' read -r key want; do
    case "$key" in
    '' | \#*) continue ;;
    esac
    got="$(value_of "$n" "$key")"
    [ -z "$got" ] && continue # a key the derivation does not own
    [ "$got" = "$want" ] || {
      echo "      $f: $key=$want but dev-env.sh derives $got" >&2
      mismatch=1
    }
  done <"$f"
  assert "$f matches the derivation" "[ '$mismatch' -eq 0 ]"
done

echo "==> Kratos can return native OAuth login flows to dev-N Hydra"
assert "Kratos allowlists the rendered Hydra public origin" \
  "grep -q 'http://localhost:__HYDRA_PUBLIC_PORT__' scripts/dev-env/values/kratos.values.yaml"

echo "==> invalid N fails loudly with no side effect"
for bad in "" 0 11 -1 abc 1.5 "2; rm -rf /"; do
  set +e
  out="$(devenv "$bad" env 2>&1)"
  rc=$?
  set -e
  label="${bad:-<empty>}"
  assert "N='$label' is rejected" "[ $rc -ne 0 ]"
  assert "N='$label' prints an actionable error" "printf '%s' \"\$out\" | grep -q 'error: N must be'"
done

echo "==> down targets only its own N"
for n in 2 7; do
  # shellcheck disable=SC2034 # read inside the eval'd assert conditions below
  plan="$(devenv "$n" down --dry-run)"
  assert "dev-$n down names its own namespaces" "printf '%s' \"\$plan\" | grep -q 'would delete namespaces: dev-$n-auth dev-$n$'"
  # Nothing shared, and no other workstream's namespace, may appear anywhere in
  # the plan — this is the cross-N leakage guard.
  for other in 1 3 5 9 10; do
    [ "$other" = "$n" ] && continue
    assert "dev-$n down does not mention dev-$other" "! printf '%s' \"\$plan\" | grep -qE '(^| )dev-$other( |-auth|$)'"
  done
  # The shared cluster's own `auth` namespace, as a bare word — dev-N-auth must
  # not trip this, which is why the boundary is explicit.
  assert "dev-$n down does not mention the shared auth namespace" "! printf '%s' \"\$plan\" | grep -qE '(^| )auth( |$)'"
  assert "dev-$n down does not mention bex-system" "! printf '%s' \"\$plan\" | grep -q 'bex-system'"
done

echo "==> clean refuses while the environment is up"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
# Stand in for a live environment: a pid file pointing at a real running process
# (this shell), in a throwaway dev-N tree.
fake_env=".pm/w10/dev-10"
mkdir -p "$fake_env/.pids"
had_pid=0
[ -f "$fake_env/.pids/test-fixture.pid" ] && had_pid=1
echo $$ >"$fake_env/.pids/test-fixture.pid"
set +e
out="$(devenv 10 clean 2>&1)"
rc=$?
set -e
assert "clean refuses while a pid is alive" "[ $rc -ne 0 ]"
assert "clean explains how to proceed" "printf '%s' \"\$out\" | grep -q 'down'"
[ "$had_pid" -eq 1 ] || rm -f "$fake_env/.pids/test-fixture.pid"

echo "==> clean runs when the environment is down"
mkdir -p "$fake_env/logs" "$fake_env/bin"
: >"$fake_env/logs/probe.log"
devenv 10 clean >/dev/null
assert "clean removed logs/" "[ ! -d '$fake_env/logs' ]"
assert "clean removed bin/" "[ ! -d '$fake_env/bin' ]"

echo "==> up truncates rather than appends its logs"
# The 3.9 GB that motivated t004 was accumulated port-forward logs; the fix is a
# truncate on up, so assert the script actually contains it rather than trusting
# a comment.
assert "up truncates bex-api.log" "grep -q ': >\"\$ENVDIR/logs/bex-api.log\"' scripts/dev-env.sh"
assert "up clears stale port-forward logs" "grep -q 'rm -f \"\$ENVDIR\"/logs/pf-\*.log' scripts/dev-env.sh"

echo "==> the override hook cannot break cross-N isolation"
override="$fake_env/override.env"
had_override=0
[ -f "$override" ] && had_override=1 && cp "$override" "$tmp/override.bak"
printf 'DEV_NS=dev-1\n' >"$override"
set +e
# shellcheck disable=SC2034 # read inside the eval'd assert conditions below
out="$(devenv 10 env 2>&1)"
rc=$?
set -e
assert "an override that renames the namespace is refused" "[ $rc -ne 0 ]"
assert "the refusal names the reason" "printf '%s' \"\$out\" | grep -q 'not overridable'"
printf 'DEV_ENV_OBSERVABILITY=0\n' >"$override"
assert "a benign override is accepted" "devenv 10 env >/dev/null"
if [ "$had_override" -eq 1 ]; then cp "$tmp/override.bak" "$override"; else rm -f "$override"; fi

echo "==> agent image identity helpers (w1/m137)"
# Source the helpers without running the entrypoint: everything from the
# AGENT_NODE_IMAGES inventory through agent_reconcile_node_images, stopping
# before image_stale / agent_build_images.
eval "$(awk '
  /^AGENT_NODE_IMAGES=\(/ {keep=1}
  keep {print}
  /^agent_reconcile_node_images\(\)/ {in_fn=1}
  in_fn && /^}$/ {exit}
' scripts/dev-env.sh)"

assert "all four agent images are inventoried" \
  "[ \${#AGENT_NODE_IMAGES[@]} -eq 4 ]"
assert "inventory includes bex-lego:dev" \
  "printf '%s\n' \"\${AGENT_NODE_IMAGES[@]}\" | grep -qx 'bex-lego:dev'"
assert "inventory includes bex-agent-sandbox:dev" \
  "printf '%s\n' \"\${AGENT_NODE_IMAGES[@]}\" | grep -qx 'bex-agent-sandbox:dev'"
assert "inventory includes opensandbox-server:0.2.2-local" \
  "printf '%s\n' \"\${AGENT_NODE_IMAGES[@]}\" | grep -qx 'opensandbox-server:0.2.2-local'"
assert "inventory includes opensandbox-controller:v0.2.0-bex" \
  "printf '%s\n' \"\${AGENT_NODE_IMAGES[@]}\" | grep -qx 'opensandbox-controller:v0.2.0-bex'"
assert "agent-up calls identity reconciliation" \
  "grep -q 'agent_reconcile_node_images' scripts/dev-env.sh"
assert "agent-up no longer keys reloads on AGENT_RELOAD_IMAGES" \
  "! grep -q 'AGENT_RELOAD_IMAGES' scripts/dev-env.sh"
assert "node identity uses crictl status.id, not ctr manifest digest" \
  "grep -q 'crictl inspecti' scripts/dev-env.sh && grep -q 'status.id' scripts/dev-env.sh"

# Pure decision coverage — the interrupted-import bug is "tag present ⇒ skip".
assert "missing observed identity needs import" \
  "agent_image_import_needed 'sha256:aaa' ''"
assert "mismatched observed identity needs import" \
  "agent_image_import_needed 'sha256:aaa' 'sha256:bbb'"
assert "matching observed identity skips import" \
  "! agent_image_import_needed 'sha256:aaa' 'sha256:aaa'"

set +e
agent_local_image_identity "bex-m137-definitely-absent:test" >/dev/null 2>&1
missing_rc=$?
set -e
assert "local inspect distinguishes missing (exit 2)" "[ '$missing_rc' -eq 2 ]"

echo "==> interrupted import converges on CAPD nodes (w1/m137)"
# Stateful Docker/containerd fixture against the live mock nodes. Skips cleanly
# when the cluster is down so CI without CAPD still runs the decision tests.
PROBE_IMG="bex-m137-probe:dev"
PROBE_REF="docker.io/library/$PROBE_IMG"
drill_nodes=()
while IFS= read -r n; do
  [ -n "$n" ] && drill_nodes+=("$n")
done < <(docker ps --filter label=io.x-k8s.kind.cluster=bex \
  --filter label=io.x-k8s.kind.role=worker --format '{{.Names}}' 2>/dev/null)
# Include the control-plane — agent-up loads onto every kubectl node.
cp_node="$(docker ps --filter label=io.x-k8s.kind.cluster=bex \
  --filter label=io.x-k8s.kind.role=control-plane --format '{{.Names}}' 2>/dev/null | head -1 || true)"
[ -n "$cp_node" ] && drill_nodes+=("$cp_node")

if [ "${#drill_nodes[@]}" -lt 2 ]; then
  echo "    skip: need ≥2 bex CAPD nodes for the interrupted-import fixture"
else
  # Two disposable nodes are enough to prove partial-import recovery; keep the
  # fixture bounded — each ctr import is multi-second on CAPD.
  drill_nodes=("${drill_nodes[0]}" "${drill_nodes[1]}")
  cleanup_probe() {
    local n
    for n in "${drill_nodes[@]}"; do
      docker exec "$n" ctr -n k8s.io images rm "$PROBE_REF" >/dev/null 2>&1 || true
    done
    docker rmi "$PROBE_IMG" >/dev/null 2>&1 || true
  }
  cleanup_probe

  docker build -q -t "$PROBE_IMG" - <<'EOF' >/dev/null
FROM alpine:3.20
RUN echo m137-v1 > /m137-probe.txt
EOF
  old_id="$(agent_local_image_identity "$PROBE_IMG")"
  # Seed both nodes with v1 (the "previous successful agent-up").
  for n in "${drill_nodes[@]}"; do
    agent_import_image_to_node "$PROBE_IMG" "$n" "$old_id" >/dev/null
  done

  docker build -q -t "$PROBE_IMG" - <<'EOF' >/dev/null
FROM alpine:3.20
RUN echo m137-v2 > /m137-probe.txt
EOF
  new_id="$(agent_local_image_identity "$PROBE_IMG")"
  assert "rebuild under the same tag minted a new identity" "[ '$old_id' != '$new_id' ]"

  # Partial import: only the first node receives v2 (interrupted agent-up).
  first="${drill_nodes[0]}"
  agent_import_image_to_node "$PROBE_IMG" "$first" "$new_id" >/dev/null
  stale="${drill_nodes[1]}"
  stale_id="$(agent_node_image_identity "$stale" "$PROBE_IMG")"
  assert "interrupted update left a node on the old identity" "[ '$stale_id' = '$old_id' ]"

  # Pre-fix regression: tag-presence skip would treat the stale node as done.
  set +e
  docker exec "$stale" ctr -n k8s.io images ls -q 2>/dev/null | grep -qx "$PROBE_REF"
  old_skip_rc=$?
  set -e
  assert "pre-fix tag-presence check would skip the stale node" "[ '$old_skip_rc' -eq 0 ]"
  assert "identity reconcile still requires import on the stale node" \
    "agent_image_import_needed '$new_id' '$stale_id'"

  # Retry without AGENT_REBUILD / source edits: reconcile every node to new_id.
  for n in "${drill_nodes[@]}"; do
    observed="$(agent_node_image_identity "$n" "$PROBE_IMG" 2>/dev/null || true)"
    if agent_image_import_needed "$new_id" "$observed"; then
      agent_import_image_to_node "$PROBE_IMG" "$n" "$new_id" >/dev/null
    fi
  done
  for n in "${drill_nodes[@]}"; do
    got="$(agent_node_image_identity "$n" "$PROBE_IMG")"
    assert "$n converged to the intended identity" "[ '$got' = '$new_id' ]"
  done

  # Matching identity skips reimport (no churn).
  before_skip="$(agent_node_image_identity "$first" "$PROBE_IMG")"
  assert "matching identity skips reimport" \
    "! agent_image_import_needed '$new_id' '$before_skip'"

  # Post-import mismatch must fail closed (false-success guard).
  set +e
  agent_import_image_to_node "$PROBE_IMG" "$first" "sha256:deadbeef" >/dev/null 2>"$tmp/mismatch.err"
  mismatch_rc=$?
  set -e
  assert "post-import identity mismatch fails" "[ '$mismatch_rc' -ne 0 ]"
  assert "mismatch diagnostic names the image" "grep -q '$PROBE_IMG' '$tmp/mismatch.err'"
  assert "mismatch diagnostic names the node" "grep -q '$first' '$tmp/mismatch.err'"

  cleanup_probe
fi

echo
if [ "$fails" -eq 0 ]; then
  echo "dev-env: all checks passed"
else
  echo "dev-env: $fails check(s) FAILED" >&2
  exit 1
fi
