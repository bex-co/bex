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
  # Conditions pipe kilobytes into `grep -q`, which exits at its first match;
  # the writer then fails on the closed pipe, and pipefail would turn a true
  # condition into a FAIL. The condition's status is its last command's.
  local rc=0
  set +o pipefail
  eval "$2" || rc=$?
  set -o pipefail
  if [ "$rc" -eq 0 ]; then echo "    ok: $1"; else
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
  BEX_DB_PORT BEX_CP_PORT LOKI_PORT PROM_PORT \
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

echo "==> Prometheus feeds the disk-usage sweeper (w2/m255 t009)"
prom_tmpl="scripts/dev-env/prometheus/prometheus.yaml"
assert "every port stays a valid TCP port (dev-10's highest)" \
  "[ \"\$(devenv 10 env | sed -n 's/^[A-Z_]*PORT=//p' | sort -n | tail -1)\" -le 65535 ]"
assert "up installs Prometheus after Loki" \
  "sed -n '/^cmd_up()/,/^}/p' scripts/dev-env.sh | grep -A1 '^  ensure_observability$' | grep -q '^  ensure_prometheus$'"
assert "up forwards the shared prometheus-server to the per-N port" \
  "grep -q 'forward prometheus prometheus-server \"\$PROM_PORT:80\" monitoring' scripts/dev-env.sh"
assert "bex-api gets BEX_PROM_URL on the per-N forward" \
  "sed -n '/^start_api()/,/^}/p' scripts/dev-env.sh | grep -q 'BEX_PROM_URL=\"http://localhost:\$PROM_PORT\"'"
assert "BEX_PROM_URL follows the observability switch" \
  "sed -n '/^start_api()/,/^}/p' scripts/dev-env.sh | grep -B4 'BEX_PROM_URL=' | grep -q 'DEV_ENV_OBSERVABILITY'"
for needle in "Prometheus ready" "kubelet_volume_stats_used_bytes reports"; do
  assert "status checks $needle" "sed -n '/^cmd_status()/,/^}/p' scripts/dev-env.sh | grep -q '$needle'"
done
assert "the kubelet job is production's apiserver-proxy scrape" \
  "grep -q 'replacement: /api/v1/nodes/\${1}/proxy/metrics' $prom_tmpl && grep -q 'replacement: /api/v1/nodes/\${1}/proxy/metrics' deploy/gitops/base/prometheus.yaml"
prom_tag="$(sed -n 's|^ *image: quay.io/prometheus/prometheus:||p' "$prom_tmpl")"
assert "Prometheus runs production's reviewed server image" \
  "[ -n '$prom_tag' ] && grep -qF 'tag: $prom_tag' deploy/gitops/base/prometheus.yaml"
assert "the shared Prometheus template carries no per-N value" \
  "! grep -Eq '__(N|DEV_NS|DEV_AUTH_NS|[A-Z_]*_PORT)__' $prom_tmpl"
assert "the Service keeps production's prometheus-server name" \
  "grep -q 'name: prometheus-server # production' $prom_tmpl"
# The local-path stand-in for the kubelet series: the exact series and labels
# NewPrometheusDiskUsageSource queries, one per claim directory, in bytes.
vu_root="$(mktemp -d)"
mkdir -p "$vu_root/pvc-1111_dev-2_srv-abc-disk" "$vu_root/pvc-2222_dev-2-auth_bex-db-1" "$vu_root/not-a-claim"
dd if=/dev/zero of="$vu_root/pvc-1111_dev-2_srv-abc-disk/fill" bs=1024 count=2048 2>/dev/null
vu_out="$(sh scripts/dev-env/prometheus/volume-usage.sh once "$vu_root")"
vu_used="$(printf '%s\n' "$vu_out" | sed -n 's/^kubelet_volume_stats_used_bytes{namespace="dev-2",persistentvolumeclaim="srv-abc-disk"} //p')"
assert "volume usage names the namespace and claim from the directory, in bytes (2 MiB written)" \
  "[ '${vu_used:-0}' -ge 2097152 ]"
assert "a hyphenated namespace splits cleanly" \
  "printf '%s' '$vu_out' | grep -q '{namespace=\"dev-2-auth\",persistentvolumeclaim=\"bex-db-1\"}'"
assert "a non-claim directory is ignored" "! printf '%s' '$vu_out' | grep -q 'not-a-claim'"
assert "an empty root still yields a valid exposition" \
  "sh scripts/dev-env/prometheus/volume-usage.sh once '$vu_root/absent' | grep -q '^# TYPE kubelet_volume_stats_used_bytes gauge$'"
rm -rf "$vu_root"

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
bash scripts/dev-env-images.test.sh
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
  # grep reads the whole listing (no -q): an early exit would fail ctr on the
  # closed pipe, and pipefail would report a present tag as absent.
  docker exec "$stale" ctr -n k8s.io images ls -q 2>/dev/null | grep -x "$PROBE_REF" >/dev/null
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

echo "==> image staleness is keyed on this checkout's source, not build time (w3/m79)"
eval "$(awk '/^source_fingerprint\(\)/,/^}$/' scripts/dev-env.sh)"
eval "$(awk '/^image_stale\(\)/,/^}$/' scripts/dev-env.sh)"
fp_repo="$tmp/fp-repo"
mkdir -p "$fp_repo/src" && (
  cd "$fp_repo" && git init -q && printf 'a\n' >src/a.txt && git add . &&
    git -c user.name=t -c user.email=t@t commit -qm init
)
fp0=$(cd "$fp_repo" && source_fingerprint src)
printf 'b\n' >"$fp_repo/src/a.txt"
fp1=$(cd "$fp_repo" && source_fingerprint src)
printf 'new\n' >"$fp_repo/src/new.txt"
fp2=$(cd "$fp_repo" && source_fingerprint src)
assert "an uncommitted edit changes the fingerprint" "[ '$fp0' != '$fp1' ]"
assert "an untracked file changes the fingerprint" "[ '$fp1' != '$fp2' ]"
# A fake docker answers the image's label; a newer build of OLDER source must
# be stale even though its timestamp is newer than every source file.
docker() { [ -n "${FAKE_LABEL+x}" ] || return 1; printf '%s\n' "$FAKE_LABEL"; }
assert "a missing image is stale" "(cd '$fp_repo' && unset FAKE_LABEL; image_stale img src)"
assert "an image built from other source is stale" "(cd '$fp_repo' && FAKE_LABEL='$fp0' image_stale img src)"
assert "an image built from this source is fresh" "! (cd '$fp_repo' && FAKE_LABEL='$fp2' image_stale img src)"
assert "AGENT_REBUILD=1 forces a rebuild" "(cd '$fp_repo' && FAKE_LABEL='$fp2' AGENT_REBUILD=1 image_stale img src)"
unset -f docker

echo "==> a fresh cluster's missing edge/operator fail up early (w2/m289 t001/t002)"
eval "$(awk '/^require_shared_platform\(\)/,/^}$/' scripts/dev-env.sh)"
# The fake kubectl answers from SHARED_HAVE: which of traefik/crd/operator exist.
kubectl() {
  case "$*" in
  "get crd middlewares.traefik.io ingressroutes.traefik.io") [[ $SHARED_HAVE == *traefik* ]] ;;
  "get crd apps.app.bex.co") [[ $SHARED_HAVE == *crd* ]] ;;
  "-n bex-system get deploy bex-controller-manager") [[ $SHARED_HAVE == *operator* ]] ;;
  *) return 1 ;;
  esac
}
platform_out() { (SHARED_HAVE="$1" require_shared_platform) 2>&1 && echo "rc=0" || echo "rc=$?"; }
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
no_traefik="$(platform_out "crd operator")"
assert "no Traefik CRDs stops up" "printf '%s' \"\$no_traefik\" | grep -q 'rc=1'"
assert "the Traefik refusal names mock-cluster.sh" \
  "printf '%s' \"\$no_traefik\" | grep -q \"Run 'bash scripts/mock-cluster.sh'\""
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
no_operator="$(platform_out "traefik crd")"
assert "no operator Deployment stops up" "printf '%s' \"\$no_operator\" | grep -q 'rc=1'"
assert "the operator refusal names mock-cluster.sh operator" \
  "printf '%s' \"\$no_operator\" | grep -q \"Run 'bash scripts/mock-cluster.sh operator'\""
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
no_crd="$(platform_out "traefik operator")"
assert "no App CRD stops up" "printf '%s' \"\$no_crd\" | grep -q 'rc=1'"
assert "a complete shared platform passes" "[ \"\$(platform_out 'traefik crd operator')\" = 'rc=0' ]"
unset -f kubectl platform_out
assert "cmd_up checks the shared platform right after refreshing its kubeconfig" \
  "awk '/^cmd_up\\(\\)/,/^}\$/' scripts/dev-env.sh | grep -A1 '^  refresh_kubeconfig\$' | grep -q '^  require_shared_platform\$'"

assert "up waits for every CNPG Cluster to be Ready before installing Kratos/Hydra" \
  "awk '/^cmd_up\\(\\)/,/^}\$/' scripts/dev-env.sh | awk '/cluster.postgresql.cnpg.io\\/\\\$cluster\" --for=condition=Ready/{w=NR} /helm upgrade --install kratos/{h=NR} END{exit !(w && h && w<h)}'"

echo "==> dev-N never depends on the global kube context (w2/m289 t003)"
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
dev_code="$(sed -e 's/^[[:space:]]*#.*//' scripts/dev-env.sh)"
assert "dev-env never switches or edits a kube context" \
  "! printf '%s\n' \"\$dev_code\" | grep -qE 'kubectl[^|;]*config (use-context|set-context|unset)'"
assert "every kubectl in up/down/status uses the private per-N kubeconfig" \
  "[ \"\$(grep -c 'export KUBECONFIG=\"\$PWD/\$KUBECONFIG_FILE\"' scripts/dev-env.sh)\" -ge 4 ]"

echo "==> helm runs with isolated registry credentials (w2/m289 t004)"
assert "dev-env sources the helm isolation" "grep -q '^\. scripts/lib/helm-isolated.sh\$' scripts/dev-env.sh"
helm_tmp="$(mktemp -d)"
cat >"$helm_tmp/helm" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n%s\n' "${HELM_REGISTRY_CONFIG:-}" "${DOCKER_CONFIG:-}"
FAKE
chmod +x "$helm_tmp/helm"
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
helm_seen="$(PATH="$helm_tmp:$PATH" BEX_HELM_ISOLATED_DIR="$helm_tmp/reg" bash -c '. scripts/lib/helm-isolated.sh; helm repo list')"
assert "helm sees the private registry config" \
  "[ \"\$(sed -n 1p <<<\"\$helm_seen\")\" = '$helm_tmp/reg/config.json' ]"
assert "helm's Docker-config fallback is the same private dir" \
  "[ \"\$(sed -n 2p <<<\"\$helm_seen\")\" = '$helm_tmp/reg' ]"
assert "the private config names no credential store" \
  "! grep -qE 'credsStore|credHelpers' '$helm_tmp/reg/config.json'"
assert "the private config holds a placeholder auths entry (no native-store detection)" \
  "grep -q '\"auths\":{\"bex-anonymous.invalid\":{}}' '$helm_tmp/reg/config.json'"
rm -rf "$helm_tmp"

echo "==> CNPG Clusters apply waits out a not-yet-serving webhook (w2/m289)"
eval "$(awk '/^apply_retrying_webhook\(\)/,/^}$/' scripts/dev-env.sh)"
wh_tmp=$(mktemp -d)
kubectl() { echo x >>"$wh_tmp/calls"; [ "$(wc -l <"$wh_tmp/calls")" -ge 3 ] && return 0; echo 'Internal error occurred: failed calling webhook "mcluster.cnpg.io"'; return 1; }
sleep() { :; }
rc=0; echo 'kind: Cluster' | apply_retrying_webhook >/dev/null || rc=$?
assert "a not-yet-serving webhook is retried until the apply lands" "[ '$rc:$(wc -l <"$wh_tmp/calls" | tr -d ' ')' = '0:3' ]"
: >"$wh_tmp/calls"
kubectl() { echo x >>"$wh_tmp/calls"; echo 'error: the server could not find the requested resource'; return 1; }
rc=0; echo 'kind: Cluster' | apply_retrying_webhook >/dev/null 2>&1 || rc=$?
assert "any other apply error fails at once" "[ '$rc:$(wc -l <"$wh_tmp/calls" | tr -d ' ')' = '1:1' ]"
unset -f kubectl sleep
rm -rf "$wh_tmp"

echo "==> CNPG fallback installs production's lock-pinned chart (w2/m295 t003)"
assert "dev-env sources the shared chart pins" "grep -q '^\. scripts/lib/helm-pinned.sh\$' scripts/dev-env.sh"
assert "no version-label CNPG install remains" "! grep -qE 'cnpg/cloudnative-pg|--version 0\.26' scripts/dev-env.sh"
assert "ensure_cnpg installs through install_pinned_cnpg" \
  "awk '/^ensure_cnpg\\(\\)/,/^}\$/' scripts/dev-env.sh | grep -q '^    install_pinned_cnpg\$'"
cn_tmp=$(mktemp -d)
# shellcheck source=lib/helm-pinned.sh
. scripts/lib/helm-pinned.sh
eval "$(awk '/^install_pinned_cnpg\(\)/,/^}$/' scripts/dev-env.sh)"
helm() {
  echo "helm $*" >>"$cn_tmp/calls"
  if [ "$1" = pull ]; then
    local dest="" ver=""
    while [ $# -gt 0 ]; do
      case $1 in --destination) dest=$2; shift ;; --version) ver=$2; shift ;; esac
      shift
    done
    printf 'fake-cnpg-bytes' >"$dest/cloudnative-pg-$ver.tgz"
  fi
}
# shellcheck disable=SC2034 # referenced by the assertion commands evaluated below.
lock_line="$(awk -F'|' '$1 == "cloudnative-pg" { print $2, $3 }' deploy/helm-artifacts.lock)"
rc=0; (install_pinned_cnpg) >/dev/null 2>&1 || rc=$?
assert "the chart is pulled at the lock's version from the lock's repository" \
  "grep -q \"^helm pull cloudnative-pg --repo \$(cut -d' ' -f2 <<<\"\$lock_line\") --version \$(cut -d' ' -f1 <<<\"\$lock_line\") \" '$cn_tmp/calls'"
assert "the lock pins production's 0.29.1" "[ \"\$(cut -d' ' -f1 <<<\"\$lock_line\")\" = 0.29.1 ]"
assert "an archive that misses the lock's sha256 stops up" "[ '$rc' = 1 ]"
assert "a mismatched archive is never installed" "! grep -q '^helm upgrade' '$cn_tmp/calls'"
: >"$cn_tmp/calls"
printf 'fake-cnpg-bytes' >"$cn_tmp/fake.tgz"
helm_pin() { echo "0.29.1 https://cloudnative-pg.github.io/charts $(sha256_of "$cn_tmp/fake.tgz")"; }
rc=0; (install_pinned_cnpg) >/dev/null 2>&1 || rc=$?
assert "a verified archive installs" "[ '$rc' = 0 ]"
assert "it installs as release cnpg in cnpg-system, from the verified archive" \
  "grep -qE '^helm upgrade --install cnpg .*/cloudnative-pg-0\.29\.1\.tgz -n cnpg-system --create-namespace\$' '$cn_tmp/calls'"
assert "the release name matches mock-cluster.sh's" "grep -q 'helm upgrade --install cnpg \"\$chart\" -n cnpg-system' scripts/mock-cluster.sh"
unset -f helm helm_pin install_pinned_cnpg
rm -rf "$cn_tmp"

echo
if [ "$fails" -eq 0 ]; then
  echo "dev-env: all checks passed"
else
  echo "dev-env: $fails check(s) FAILED" >&2
  exit 1
fi
