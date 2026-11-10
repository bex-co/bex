#!/usr/bin/env bash
# Docker and Docker Compose inside a gVisor sandbox: the repeatable checks behind
# ADR104 D6 (w22/m312 t006). One PASS/FAIL line per check, so the spike can be
# re-run after a runsc, Docker or image-pin bump.
#
#   bash scripts/sandbox-docker-spike.sh --dry-run                    # print every check, touch nothing
#   bash scripts/sandbox-docker-spike.sh --docker-context colima-gvisor-spike
#   bash scripts/sandbox-docker-spike.sh --docker-context CTX --docker-versions "29 28 27"
#   bash scripts/sandbox-docker-spike.sh --k8s-namespace NS --k8s-pod POD  # an existing pod running dockerd
#
# WHAT IT DOES. For each Docker version it starts a sandbox on the target daemon:
# the stock `docker:<v>-dind` image under `--runtime <RUNTIME>` (default
# `runsc-docker`, a runsc runtime registered with `--net-raw` and, for Docker 28
# and later, `--allow-packet-socket-write`), with the capabilities the gVisor
# tutorial uses, and bootstraps dockerd inside it the way the tutorial's
# start-dockerd.sh does (tmpfs at /var/lib/docker, SNAT, --iptables=false). Then
# it runs the functional checks through `docker exec` and deletes the sandbox.
# https://gvisor.dev/docs/tutorials/docker-in-gvisor/
#
# SETUP IT DOES NOT DO. The runtime named by --runtime must already exist on the
# target daemon. On the local colima VM of the m312 spike, in that VM's
# /etc/docker/daemon.json (never the default runtime, never a production node):
#   "runtimes": {
#     "runsc": {"path": "/usr/local/bin/runsc"},
#     "runsc-docker": {"path": "/usr/local/bin/runsc",
#                      "runtimeArgs": ["--net-raw", "--allow-packet-socket-write"]}
#   }
# with runsc and containerd-shim-runsc-v1 from gVisor's release bucket
# (https://gvisor.dev/docs/user_guide/install/, sha512 checked).
#
# WHERE IT MAY RUN. Only on a throwaway node. `--net-raw` and
# `--allow-packet-socket-write` are node-level runsc settings that apply to every
# pod on the node; they must never be set on the production sandbox pool. The
# script REFUSES (exit 3) when the kubectl context it can see, an explicit
# `--target-node`, the docker endpoint or this machine's hostname resolves to a
# node labelled `bex.co/pool=sandbox`, unless `--i-know-this-is-production` is
# passed. With no reachable kubectl context there is nothing to compare against;
# the script says so and goes on (a local VM is the intended case).
#
# ISOLATION CHECKS (`--isolation`) are OPT-IN, never run by default, and have
# NOT been run on a Cilium node: they were written from the ADR104 D6 spec and
# only their packet-crafting mechanics were exercised on a local VM
# (`--lab-no-cilium`). They need a Kubernetes pod and a namespace that carries a
# Cilium policy, and exit 4 with a message when no Cilium policy is present.
# Under `--lab-no-cilium` they say REACHED or BLOCKED about what the sandbox can
# do; those lines are not isolation results.
#
# Exit codes: 0 every check passed (or --dry-run), 1 a check failed, 2 usage,
# 3 refused by the production-pool guard, 4 isolation preconditions missing.
set -uo pipefail

RUNTIME="runsc-docker"
DOCKER_CTX="${DOCKER_CONTEXT:-}"
VERSIONS="29"
CAPS="tutorial"
CAP_ADD=""
CAP_DROP_ALL=0
TMPFS_SIZE="2G"
NO_OVERLAY=0
DRY_RUN=0
KEEP=0
TARGET_NODE=""
KUBE_CONTEXT=""
NO_KUBECTL=0
I_KNOW_PROD=0
K8S_NS=""
K8S_POD=""
K8S_CONTAINER=""
ISOLATION=0
LAB_NO_CILIUM=0
RAW_IMAGE="python:3-alpine"
CURL_IMAGE="curlimages/curl"
ALLOWED_TARGET=""
ALLOWED_FQDN=""
DENIED_FQDN=""
SPOOF_SRC=""
DENIED_TARGETS=()
EGRESS_TIMEOUT=120
ONLY=""
GUARD_ONLY=0

usage() {
  cat >&2 <<'EOF'
usage: bash scripts/sandbox-docker-spike.sh [options]

target
  --docker-context CTX      docker context to run on (default: $DOCKER_CONTEXT, else the current one)
  --runtime NAME            docker runtime for the sandbox (default runsc-docker)
  --docker-versions "29 28" docker:<v>-dind majors to test, one sandbox each (default 29)
  --caps tutorial|all|strict
                            tutorial: --cap-add sys_admin,net_admin,net_raw,setuid on top of the runtime's default set (default)
                            all: --cap-add all (what the gVisor tutorial runs)
                            strict: --cap-drop all, then add only those four (measured: dockerd fails, it needs Docker's default 14 too)
  --cap-drop-all --cap-add LIST
                            custom set: drop ALL, then add the comma-separated LIST (overrides --caps)
  --tmpfs-size SIZE         size of the tmpfs at /var/lib/docker (default 2G)
  --no-overlay              no tmpfs at /var/lib/docker; Docker 29 gets --feature containerd-snapshotter=false
  --k8s-namespace NS --k8s-pod POD [--k8s-container C]
                            use an existing pod that already runs dockerd instead of starting a sandbox
  --only ID[,ID]            run only these functional checks (dockerd_up always runs first)
  --keep                    leave the sandbox container running afterwards
  --dry-run                 print every check; touch no docker daemon and no cluster
  --guard-only              run only the production-pool guard (exit 0 when it passes, 3 when it refuses)

production guard
  --target-node NODE        the node this runs on; refused when labelled bex.co/pool=sandbox
  --kube-context CTX        kubectl context to read node labels from (default: the current one)
  --no-kubectl              assert there is no cluster to check (skips the guard's kubectl reads)
  --i-know-this-is-production
                            run even when the guard matches. Never set the runsc flags on the production pool.

isolation checks (opt-in, NOT run by default, never validated on a Cilium node)
  --isolation               run them; needs --k8s-namespace, --k8s-pod and a Cilium policy in that namespace
  --lab-no-cilium           run them against the docker sandbox with no Cilium: REACHED/BLOCKED, not PASS/FAIL
  --denied-target HOST:PORT a platform-floor or denied target (repeatable); the metadata endpoint is always probed
  --allowed-target HOST:PORT an allowed destination (control for the raw-packet probe; must answer a TCP SYN)
  --allowed-fqdn F --denied-fqdn F   one allow-listed and one denied name for the FQDN/SNI probe
  --spoof-src IP            source address to forge in the spoofing probes (default 10.255.255.254)
  --raw-image IMG           nested image with python3 (default python:3-alpine)
  --curl-image IMG          nested image with curl (default curlimages/curl)
EOF
  exit 2
}

while [ $# -gt 0 ]; do
  case "$1" in
    --docker-context) DOCKER_CTX="${2:?}"; shift 2 ;;
    --runtime) RUNTIME="${2:?}"; shift 2 ;;
    --docker-versions) VERSIONS="${2:?}"; shift 2 ;;
    --caps) CAPS="${2:?}"; shift 2 ;;
    --cap-drop-all) CAPS="custom"; CAP_DROP_ALL=1; shift ;;
    --cap-add) CAPS="custom"; CAP_ADD="${2:?}"; shift 2 ;;
    --tmpfs-size) TMPFS_SIZE="${2:?}"; shift 2 ;;
    --no-overlay) NO_OVERLAY=1; shift ;;
    --k8s-namespace) K8S_NS="${2:?}"; shift 2 ;;
    --k8s-pod) K8S_POD="${2:?}"; shift 2 ;;
    --k8s-container) K8S_CONTAINER="${2:?}"; shift 2 ;;
    --only) ONLY=",${2:?},"; shift 2 ;;
    --keep) KEEP=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --guard-only) GUARD_ONLY=1; shift ;;
    --target-node) TARGET_NODE="${2:?}"; shift 2 ;;
    --kube-context) KUBE_CONTEXT="${2:?}"; shift 2 ;;
    --no-kubectl) NO_KUBECTL=1; shift ;;
    --i-know-this-is-production) I_KNOW_PROD=1; shift ;;
    --isolation) ISOLATION=1; shift ;;
    --lab-no-cilium) ISOLATION=1; LAB_NO_CILIUM=1; shift ;;
    --denied-target) DENIED_TARGETS+=("${2:?}"); shift 2 ;;
    --allowed-target) ALLOWED_TARGET="${2:?}"; shift 2 ;;
    --allowed-fqdn) ALLOWED_FQDN="${2:?}"; shift 2 ;;
    --denied-fqdn) DENIED_FQDN="${2:?}"; shift 2 ;;
    --spoof-src) SPOOF_SRC="${2:?}"; shift 2 ;;
    --raw-image) RAW_IMAGE="${2:?}"; shift 2 ;;
    --curl-image) CURL_IMAGE="${2:?}"; shift 2 ;;
    -h | --help) usage ;;
    *) echo "unknown option: $1" >&2; usage ;;
  esac
done

case "$CAPS" in tutorial | all | strict | custom) ;; *) echo "--caps must be tutorial, all or strict" >&2; exit 2 ;; esac
if [ -n "$K8S_POD" ] && [ -z "$K8S_NS" ]; then echo "--k8s-pod needs --k8s-namespace" >&2; exit 2; fi
if [ "$ISOLATION" = 1 ] && [ "$LAB_NO_CILIUM" = 0 ] && { [ -z "$K8S_POD" ] || [ -z "$K8S_NS" ]; }; then
  echo "--isolation needs --k8s-namespace and --k8s-pod (a pod on a throwaway Cilium node); use --lab-no-cilium to exercise the probes locally" >&2
  exit 2
fi

say() { printf '%s\n' "$*"; }
note() { printf 'NOTE %s\n' "$*" >&2; }

docker_cmd() { docker ${DOCKER_CTX:+--context "$DOCKER_CTX"} "$@"; }
kctl() { kubectl ${KUBE_CONTEXT:+--context "$KUBE_CONTEXT"} --request-timeout=10s "$@"; }

# --- production-pool guard ---------------------------------------------------
# A node is "production sandbox pool" when it carries bex.co/pool=sandbox
# (deploy/opensandbox/gvisor-runtimeclass.yaml). The guard compares what this run
# would touch (explicit node, docker endpoint, this hostname) with the names and
# addresses of every such node the current kubectl context can list.
guard_refuse() {
  echo "REFUSED: $1" >&2
  echo "  The runsc flags this spike needs (--net-raw, --allow-packet-socket-write) are node-level and must never be set on the production sandbox pool." >&2
  echo "  Use a throwaway node, name it with --target-node, or pass --i-know-this-is-production if you are certain." >&2
  exit 3
}

docker_endpoint_host() {
  local ctx host
  ctx="${DOCKER_CTX:-$(docker context show 2>/dev/null)}"
  host="$(docker context inspect "$ctx" --format '{{.Endpoints.docker.Host}}' 2>/dev/null)" || return 0
  case "$host" in
    tcp://* | ssh://* | http://* | https://*)
      host="${host#*://}"
      host="${host#*@}"
      host="${host%%/*}"
      printf '%s\n' "${host%:*}"
      ;;
  esac
}

production_guard() {
  if [ "$I_KNOW_PROD" = 1 ]; then
    note "--i-know-this-is-production: the production-pool guard is OFF"
    return 0
  fi
  if [ "$NO_KUBECTL" = 1 ]; then
    note "--no-kubectl: asserted that there is no cluster to compare against; guard skipped"
    return 0
  fi
  if ! command -v kubectl >/dev/null 2>&1; then
    note "kubectl is not installed: the production-pool guard has nothing to compare against"
    return 0
  fi
  if ! kubectl ${KUBE_CONTEXT:+--context "$KUBE_CONTEXT"} config current-context >/dev/null 2>&1; then
    note "kubectl has no current context: the production-pool guard has nothing to compare against"
    return 0
  fi
  local pool
  if ! pool="$(kctl get nodes -l bex.co/pool=sandbox \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .status.addresses[*]}{.address}{" "}{end}{"\n"}{end}' 2>/dev/null)"; then
    guard_refuse "a kubectl context exists but its nodes cannot be listed, so the sandbox pool cannot be ruled out (pass --no-kubectl to assert there is no cluster)"
  fi
  local candidates
  candidates="$TARGET_NODE $(docker_endpoint_host) $(hostname 2>/dev/null)"
  if [ -n "$K8S_POD" ]; then
    candidates="$candidates $(kctl -n "$K8S_NS" get pod "$K8S_POD" -o jsonpath='{.spec.nodeName}' 2>/dev/null)"
  fi
  local line word cand
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    for cand in $candidates; do
      for word in $line; do
        [ "$word" = "$cand" ] && guard_refuse "$cand is a node of the production sandbox pool (bex.co/pool=sandbox): $line"
      done
    done
  done <<EOF
$pool
EOF
  if [ -n "$pool" ] && [ -z "$TARGET_NODE" ] && [ -z "$K8S_POD" ]; then
    guard_refuse "the current kubectl context has a production sandbox pool (bex.co/pool=sandbox) and no --target-node says this run is somewhere else"
  fi
  note "production-pool guard passed (candidates: ${candidates:-none})"
}

# --- sandbox lifecycle -------------------------------------------------------
SANDBOX=""
cap_flags() {
  local c
  case "$CAPS" in
    all) printf '%s\n' "--cap-add all" ;;
    tutorial) printf '%s\n' "--cap-add sys_admin --cap-add net_admin --cap-add net_raw --cap-add setuid" ;;
    strict) printf '%s\n' "--cap-drop all --cap-add sys_admin --cap-add net_admin --cap-add net_raw --cap-add setuid" ;;
    custom)
      [ "$CAP_DROP_ALL" = 1 ] && printf '%s ' "--cap-drop all"
      for c in $(printf '%s' "$CAP_ADD" | tr ',' ' '); do printf '%s ' "--cap-add $c"; done
      printf '\n'
      ;;
  esac
}

# Derived from gVisor's images/basic/docker/start-dockerd.sh: a tmpfs under
# /var/lib/docker (gVisor needs tmpfs as the overlay upper layer), SNAT for the
# nested bridge, the outer MTU, and dockerd with iptables off.
bootstrap() {
  cat <<'EOF'
set -eu
if [ "$NO_OVERLAY" != 1 ]; then
  mkdir -p /var/lib/docker
  [ "$(stat -f -c %T /var/lib/docker)" = tmpfs ] || mount -t tmpfs -o "size=$TMPFS_SIZE" tmpfs /var/lib/docker
fi
# busybox ip ignores the "default" filter, so match the default route by hand
dev=$(ip route | sed -n 's/^default .* dev \([^ ]*\).*/\1/p' | head -1)
[ -n "$dev" ] && [ "$dev" != lo ] || { echo "no default-route device: $(ip route)" >&2; exit 1; }
addr=$(ip -4 addr show dev "$dev" | sed -n 's/.*inet \([0-9.]*\)\/.*/\1/p' | head -1)
mtu=$(ip link show dev "$dev" | sed -n 's/.*mtu \([0-9]*\).*/\1/p')
echo 1 > /proc/sys/net/ipv4/ip_forward
ipt=iptables
command -v iptables-legacy >/dev/null 2>&1 && ipt=iptables-legacy
$ipt -t nat -A POSTROUTING -o "$dev" -j SNAT --to-source "$addr" -p tcp
$ipt -t nat -A POSTROUTING -o "$dev" -j SNAT --to-source "$addr" -p udp
extra=""
if [ "$NO_OVERLAY" = 1 ] && dockerd --version | grep -q 'version 29'; then
  extra="--feature containerd-snapshotter=false"
fi
# shellcheck disable=SC2086
exec dockerd-entrypoint.sh --iptables=false --ip6tables=false ${mtu:+--mtu="$mtu"} $extra
EOF
}

start_sandbox() {
  local v="$1" caps
  SANDBOX="spike-dind-$v-$$"
  caps="$(cap_flags)"
  # shellcheck disable=SC2086
  docker_cmd run -d --runtime "$RUNTIME" $caps --name "$SANDBOX" \
    -e "NO_OVERLAY=$NO_OVERLAY" -e "TMPFS_SIZE=$TMPFS_SIZE" \
    --entrypoint sh "docker:$v-dind" -c "$(bootstrap)" >/dev/null
}

stop_sandbox() {
  [ -n "$SANDBOX" ] || return 0
  if [ "$KEEP" = 1 ]; then
    note "--keep: sandbox $SANDBOX left running on ${DOCKER_CTX:-the current docker context}"
  else
    docker_cmd rm -f "$SANDBOX" >/dev/null 2>&1 || true
  fi
  SANDBOX=""
}
trap 'stop_sandbox' EXIT

# sbx runs one sh script inside the sandbox: through docker exec, or kubectl exec
# when --k8s-pod names an existing pod.
sbx() {
  if [ -n "$K8S_POD" ]; then
    kctl -n "$K8S_NS" exec "$K8S_POD" ${K8S_CONTAINER:+-c "$K8S_CONTAINER"} -- sh -c "$1"
  else
    docker_cmd exec "$SANDBOX" sh -c "$1"
  fi
}

# --- checks ------------------------------------------------------------------
# A check is a pair of functions, desc_<id> and body_<id>; body_<id> prints a
# POSIX sh script that runs inside the sandbox and exits 0 on pass. Output of a
# failing script is printed verbatim under its FAIL line.
FUNCTIONAL="dockerd_up hello_world build_multilayer compose_internal_call compose_published_port published_port_from_outside compose_network_mode_host user_defined_bridge_dns docker_run_publish docker_run_publish_all default_bridge_link egress_https_nested_bridge egress_https_nested_host"
ISOLATION_CHECKS="floor_direct_tcp raw_syn_denied spoof_src_ip spoof_arp spoof_ndp fqdn_sni_bypass"

desc_dockerd_up() { echo "dockerd answers docker info inside the sandbox"; }
body_dockerd_up() {
  cat <<'EOF'
for i in $(seq 1 90); do docker info >/dev/null 2>&1 && break; sleep 1; done
docker info >/dev/null 2>&1 && exit 0
docker info 2>&1 | tail -8
exit 1
EOF
}

desc_hello_world() { echo "docker run hello-world"; }
body_hello_world() {
  cat <<'EOF'
out=$(docker run --rm hello-world 2>&1) || { echo "$out"; exit 1; }
case "$out" in *"Hello from Docker!"*) ;; *) echo "$out"; exit 1 ;; esac
EOF
}

desc_build_multilayer() { echo "docker build of a multi-layer image (RUN, COPY, RUN) and run it"; }
body_build_multilayer() {
  cat <<'EOF'
set -eu
d=$(mktemp -d); cd "$d"
printf 'FROM busybox:1.36\nRUN echo layer1 > /l1\nCOPY app.txt /app.txt\nRUN echo layer3 > /l3\nCMD ["sh","-c","cat /l1 /app.txt /l3"]\n' > Dockerfile
echo hello-app > app.txt
docker build -t spike-build:1 . > build.log 2>&1 || { tail -25 build.log; exit 1; }
out=$(docker run --rm spike-build:1 2>&1 | tr '\n' ' ')
[ "$out" = "layer1 hello-app layer3 " ] || { echo "unexpected output: $out"; exit 1; }
EOF
}

desc_compose_internal_call() { echo "compose: two services, client calls web by service name on the default compose network"; }
body_compose_internal_call() {
  cat <<'EOF'
d=$(mktemp -d); cd "$d"
cat > compose.yaml <<'YAML'
services:
  web:
    image: busybox:1.36
    command: sh -c "mkdir -p /www && echo hello-from-web > /www/index.html && exec httpd -f -p 8080 -h /www"
  client:
    image: busybox:1.36
    depends_on: [web]
    command: sh -c "sleep 3; wget -q -T 10 -O- http://web:8080/ && echo CLIENT_CALL_OK"
YAML
trap 'docker compose down -v >/dev/null 2>&1 || true' EXIT
docker compose up --abort-on-container-exit --exit-code-from client > up.log 2>&1 || { tail -25 up.log; exit 1; }
grep -q CLIENT_CALL_OK up.log || { tail -25 up.log; exit 1; }
EOF
}

desc_compose_published_port() { echo "compose: ports: 8081:8080 answers on 127.0.0.1 inside the sandbox"; }
body_compose_published_port() {
  cat <<'EOF'
d=$(mktemp -d); cd "$d"
cat > compose.yaml <<'YAML'
services:
  web:
    image: busybox:1.36
    command: sh -c "mkdir -p /www && echo hello-from-web > /www/index.html && exec httpd -f -p 8080 -h /www"
    ports:
      - "8081:8080"
YAML
trap 'docker compose down -v >/dev/null 2>&1 || true' EXIT
docker compose up -d > up.log 2>&1 || { tail -25 up.log; exit 1; }
for i in 1 2 3 4 5 6 7 8 9 10; do
  out=$(wget -q -T 3 -O- http://127.0.0.1:8081/ 2>&1) && [ "$out" = hello-from-web ] && exit 0
  sleep 1
done
echo "last wget output: $out"; docker ps --format '{{.Names}} {{.Ports}}'; exit 1
EOF
}

# Needs the outer docker daemon: a runc sibling container reaches the sandbox's
# eth0 address. Not applicable to --k8s-pod (use a peer pod there).
desc_published_port_from_outside() { echo "compose published port answers from outside the sandbox (a sibling container on the outer daemon)"; }
body_published_port_from_outside() { echo "# runs on the outer docker daemon, see run_check"; }

desc_compose_network_mode_host() { echo "compose: network_mode: host, client reaches web on 127.0.0.1"; }
body_compose_network_mode_host() {
  cat <<'EOF'
d=$(mktemp -d); cd "$d"
cat > compose.yaml <<'YAML'
services:
  web:
    image: busybox:1.36
    network_mode: host
    command: sh -c "mkdir -p /www && echo hello-from-host-web > /www/index.html && exec httpd -f -p 8091 -h /www"
  client:
    image: busybox:1.36
    network_mode: host
    depends_on: [web]
    command: sh -c "sleep 3; wget -q -T 10 -O- http://127.0.0.1:8091/ && echo CLIENT_CALL_OK"
YAML
trap 'docker compose down -v >/dev/null 2>&1 || true' EXIT
docker compose up --abort-on-container-exit --exit-code-from client > up.log 2>&1 || { tail -25 up.log; exit 1; }
grep -q CLIENT_CALL_OK up.log || { tail -25 up.log; exit 1; }
EOF
}

desc_user_defined_bridge_dns() { echo "user-defined bridge network: a container resolves another by name (embedded DNS 127.0.0.11)"; }
body_user_defined_bridge_dns() {
  cat <<'EOF'
trap 'docker rm -f udb-web >/dev/null 2>&1; docker network rm spike-net >/dev/null 2>&1' EXIT
docker network create spike-net >/dev/null || exit 1
docker run -d --rm --name udb-web --network spike-net busybox:1.36 sh -c 'mkdir -p /www && echo hello-udb > /www/index.html && exec httpd -f -p 8080 -h /www' >/dev/null || exit 1
sleep 2
out=$(docker run --rm --network spike-net busybox:1.36 sh -c 'grep nameserver /etc/resolv.conf; wget -q -T 5 -O- http://udb-web:8080/' 2>&1) || { echo "$out"; exit 1; }
case "$out" in *hello-udb*) ;; *) echo "$out"; exit 1 ;; esac
EOF
}

desc_docker_run_publish() { echo "docker run -p 8082:8080 answers on 127.0.0.1 inside the sandbox"; }
body_docker_run_publish() {
  cat <<'EOF'
trap 'docker rm -f pub >/dev/null 2>&1' EXIT
docker run -d --rm --name pub -p 8082:8080 busybox:1.36 sh -c 'mkdir -p /www && echo hello-pub > /www/index.html && exec httpd -f -p 8080 -h /www' >/dev/null || exit 1
for i in 1 2 3 4 5 6 7 8 9 10; do
  out=$(wget -q -T 3 -O- http://127.0.0.1:8082/ 2>&1) && [ "$out" = hello-pub ] && exit 0
  sleep 1
done
echo "last wget output: $out"; docker ps --format '{{.Names}} {{.Ports}}'; exit 1
EOF
}

desc_docker_run_publish_all() { echo "docker run -P (publish all EXPOSEd ports) answers on the assigned port"; }
body_docker_run_publish_all() {
  cat <<'EOF'
trap 'docker rm -f puball >/dev/null 2>&1' EXIT
d=$(mktemp -d); cd "$d"
printf 'FROM busybox:1.36\nEXPOSE 8080\nCMD ["sh","-c","mkdir -p /www && echo hello-P > /www/index.html && exec httpd -f -p 8080 -h /www"]\n' > Dockerfile
docker build -q -t spike-expose:1 . >/dev/null || exit 1
docker run -d --rm --name puball -P spike-expose:1 >/dev/null || exit 1
sleep 2
port=$(docker port puball 8080/tcp | head -1 | sed 's/.*://')
[ -n "$port" ] || { echo "no host port assigned"; docker port puball; exit 1; }
out=$(wget -q -T 5 -O- "http://127.0.0.1:$port/" 2>&1) || { echo "port $port: $out"; exit 1; }
[ "$out" = hello-P ] || { echo "$out"; exit 1; }
EOF
}

desc_default_bridge_link() { echo "default bridge: --link name resolution (the gVisor FAQ workaround)"; }
body_default_bridge_link() {
  cat <<'EOF'
trap 'docker rm -f lk-web >/dev/null 2>&1' EXIT
docker run -d --rm --name lk-web busybox:1.36 sh -c 'mkdir -p /www && echo hello-link > /www/index.html && exec httpd -f -p 8080 -h /www' >/dev/null || exit 1
sleep 2
out=$(docker run --rm --link lk-web busybox:1.36 wget -q -T 5 -O- http://lk-web:8080/ 2>&1) || { echo "$out"; exit 1; }
case "$out" in *hello-link*) ;; *) echo "$out"; exit 1 ;; esac
EOF
}

# A nested container fetches ~1.5 MB over HTTPS. The elapsed time is the finding:
# on the bridge it was 15-20x slower than on the host network in the first run.
desc_egress_https_nested_bridge() { echo "HTTPS fetch from a nested container on the default bridge (timing in the result line)"; }
body_egress_https_nested_bridge() {
  cat <<EOF
docker run --rm busybox:1.36 sh -c 'wget -q -T $EGRESS_TIMEOUT -O /dev/null "https://dl-cdn.alpinelinux.org/alpine/v3.20/main/\$(uname -m)/APKINDEX.tar.gz"' 2>&1
EOF
}
desc_egress_https_nested_host() { echo "HTTPS fetch from a nested container with --network host (comparison)"; }
body_egress_https_nested_host() {
  cat <<EOF
docker run --rm --network host busybox:1.36 sh -c 'wget -q -T $EGRESS_TIMEOUT -O /dev/null "https://dl-cdn.alpinelinux.org/alpine/v3.20/main/\$(uname -m)/APKINDEX.tar.gz"' 2>&1
EOF
}

# --- isolation probes (opt-in; see the header) --------------------------------
# Python raw-packet helpers run in a nested container with NET_RAW. They avoid
# backslashes, dollar signs and backticks so they survive heredoc expansion.
py_syn() {
  cat <<'EOF'
import random, socket, struct, sys, time

def csum(data):
    if len(data) % 2:
        data += bytes(1)
    s = sum(struct.unpack("!%dH" % (len(data) // 2), data))
    s = (s >> 16) + (s & 0xFFFF)
    s += s >> 16
    return ~s & 0xFFFF

dst, dport = sys.argv[1], int(sys.argv[2])
forged = sys.argv[3] if len(sys.argv) > 3 else ""
probe = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
probe.connect((dst, 9))
src = forged or probe.getsockname()[0]
probe.close()
sport = random.randint(20000, 60000)
tcp = struct.pack("!HHLLBBHHH", sport, dport, 1000, 0, 5 << 4, 0x02, 8192, 0, 0)
pseudo = socket.inet_aton(src) + socket.inet_aton(dst) + struct.pack("!BBH", 0, 6, len(tcp))
tcp = tcp[:16] + struct.pack("!H", csum(pseudo + tcp)) + tcp[18:]
ip = struct.pack("!BBHHHBBH4s4s", 0x45, 0, 20 + len(tcp), random.randint(1, 65535), 0, 64, 6, 0, socket.inet_aton(src), socket.inet_aton(dst))
ip = ip[:10] + struct.pack("!H", csum(ip)) + ip[12:]
# receive on an AF_PACKET socket: gVisor's NAT and conntrack handling can swallow a
# SYN-ACK before an AF_INET raw socket sees it, which made the control fail in a nested container
rx = socket.socket(socket.AF_PACKET, socket.SOCK_RAW, socket.ntohs(0x0800))
rx.settimeout(0.5)
tx = socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.IPPROTO_RAW)
tx.setsockopt(socket.IPPROTO_IP, socket.IP_HDRINCL, 1)
try:
    tx.sendto(ip + tcp, (dst, 0))
except OSError as e:
    print("SEND FAILED: %s" % e)
    sys.exit(3)
print("sent SYN src=%s dst=%s:%d" % (src, dst, dport))
deadline = time.time() + 4
while time.time() < deadline:
    try:
        pkt = rx.recv(2048)
    except socket.timeout:
        continue
    pkt = pkt[14:]
    if len(pkt) < 34 or pkt[9] != 6:
        continue
    ihl = (pkt[0] & 15) * 4
    if socket.inet_ntoa(pkt[12:16]) != dst:
        continue
    sp, dp, _, _, _, fl = struct.unpack("!HHLLBB", pkt[ihl:ihl + 14])
    if sp == dport and dp == sport and (fl & 0x04 or fl & 0x12 == 0x12):
        print("REPLY flags=0x%02x" % fl)
        sys.exit(0)
print("NO REPLY")
sys.exit(1)
EOF
}

py_l2() {
  cat <<'EOF'
import fcntl, socket, struct, sys

mode, forged_ip, victim_ip = sys.argv[1], sys.argv[2], sys.argv[3]
iface = sys.argv[4] if len(sys.argv) > 4 else "eth0"
s = socket.socket(socket.AF_PACKET, socket.SOCK_RAW)
s.bind((iface, 0))
mac = fcntl.ioctl(s.fileno(), 0x8927, struct.pack("256s", iface.encode()[:15]))[18:24]
bcast = bytes([255] * 6)
if mode == "arp":
    frame = bcast + mac + struct.pack("!H", 0x0806)
    frame += struct.pack("!HHBBH", 1, 0x0800, 6, 4, 2) + mac + socket.inet_aton(forged_ip) + bcast + socket.inet_aton(victim_ip)
else:
    # Neighbor Advertisement for forged_ip (an IPv6 address) to ff02::1, override flag set
    dst = socket.inet_pton(socket.AF_INET6, "ff02::1")
    tgt = socket.inet_pton(socket.AF_INET6, forged_ip)
    src = socket.inet_pton(socket.AF_INET6, victim_ip)
    na = struct.pack("!BBHI", 136, 0, 0, 0x20000000) + tgt + bytes([2, 1]) + mac
    pseudo = src + dst + struct.pack("!IBBBB", len(na), 0, 0, 0, 58)
    data = pseudo + na
    if len(data) % 2:
        data += bytes(1)
    t = sum(struct.unpack("!%dH" % (len(data) // 2), data))
    t = (t >> 16) + (t & 0xFFFF)
    t += t >> 16
    na = na[:2] + struct.pack("!H", ~t & 0xFFFF) + na[4:]
    ipv6 = struct.pack("!IHBB", 0x60000000, len(na), 58, 255) + src + dst
    frame = bytes([0x33, 0x33, 0, 0, 0, 1]) + mac + struct.pack("!H", 0x86DD) + ipv6 + na
try:
    n = s.send(frame)
    print("SENT %d bytes (%s, claiming %s)" % (n, mode, forged_ip))
except OSError as e:
    print("SEND FAILED: %s" % e)
    sys.exit(3)
EOF
}

# probe_write <script-fn> <file> writes a python helper into the sandbox;
# probe_run <file> <image> <extra-docker-flags> <args> runs it in a nested
# container that shares the sandbox's network.
probe_write() {
  cat <<EOF
# the sandbox holds CAP_NET_ADMIN, so an attacker can delete the SNAT rules the bootstrap
# installed; left in place they swallow locally originated raw IP packets under gVisor
iptables-legacy -t nat -F POSTROUTING >/dev/null 2>&1 || true
cat > /tmp/$2 <<'PYEOF'
$("$1")
PYEOF
EOF
}

probe_run() {
  printf 'docker run --rm --network host --cap-add NET_RAW %s -v /tmp/%s:/%s:ro %s python /%s %s 2>&1\n' "$3" "$1" "$1" "$2" "$1" "$4"
}

floor_targets() {
  local t
  say "169.254.169.254:80"
  for t in ${DENIED_TARGETS[@]+"${DENIED_TARGETS[@]}"}; do say "$t"; done
}

desc_floor_direct_tcp() { echo "platform floor: TCP connect to each denied target from the pod, a bridge container and a host-network container"; }
body_floor_direct_tcp() {
  local t h p
  echo 'rc=0'
  for t in $(floor_targets); do
    h="${t%:*}"; p="${t##*:}"
    cat <<EOF
nc -z -w 3 $h $p 2>/dev/null && { echo "REACHED $h:$p from the sandbox shell"; rc=1; } || echo "blocked $h:$p from the sandbox shell"
docker run --rm busybox:1.36 nc -z -w 3 $h $p 2>/dev/null && { echo "REACHED $h:$p from a bridge container"; rc=1; } || echo "blocked $h:$p from a bridge container"
docker run --rm --network host busybox:1.36 nc -z -w 3 $h $p 2>/dev/null && { echo "REACHED $h:$p from a host-network container"; rc=1; } || echo "blocked $h:$p from a host-network container"
EOF
  done
  echo "exit \$rc"
}

desc_raw_syn_denied() { echo "raw-packet reach: a crafted TCP SYN to each denied target must get no reply (control: the allowed target must)"; }
body_raw_syn_denied() {
  local t h p
  echo 'rc=0'
  probe_write py_syn syn.py
  if [ -n "$ALLOWED_TARGET" ]; then
    h="${ALLOWED_TARGET%:*}"; p="${ALLOWED_TARGET##*:}"
    probe_run syn.py "$RAW_IMAGE" "" "$h $p"
    echo 'if [ $? -ne 0 ]; then echo "CONTROL FAILED: the raw SYN probe got no reply from the allowed target, so silence below proves nothing"; exit 2; fi'
  else
    echo 'echo "NO CONTROL: pass --allowed-target HOST:PORT so silence is distinguishable from a broken probe"'
  fi
  for t in $(floor_targets); do
    h="${t%:*}"; p="${t##*:}"
    probe_run syn.py "$RAW_IMAGE" "" "$h $p"
    cat <<EOF
case \$? in
  0) echo "REACHED $h:$p by raw SYN"; rc=1 ;;
  1 | 3) echo "blocked $h:$p by raw SYN" ;;
  *) echo "probe error for $h:$p"; exit 1 ;;
esac
EOF
  done
  echo "exit \$rc"
}

default_spoof() { printf '%s\n' "${SPOOF_SRC:-10.255.255.254}"; }

# The spoofing probes only SEND. Whether a forged packet or frame leaves the pod
# is judged by an observer on the receiving side, so an accepted send is MANUAL
# (exit 2) and a refused send is blocked.
spoof_verdict() { # what the observer should run
  cat <<EOF
echo "\$out"
case "\$out" in
  *"SEND FAILED"*) exit 0 ;;
  *"sent SYN"* | *SENT*) echo "MANUAL: the send was accepted. Observer: $1"; exit 2 ;;
  *) echo "probe did not run"; exit 1 ;;
esac
EOF
}

# spoof_body <script-fn> <file> <flags> <args> <observer hint>
spoof_body() {
  probe_write "$1" "$2"
  echo "out=\$($(probe_run "$2" "$RAW_IMAGE" "$3" "$4"))"
  spoof_verdict "$5"
}

desc_spoof_src_ip() { echo "source-IP spoofing: a SYN with a forged source toward the allowed target (an observer must judge)"; }
body_spoof_src_ip() {
  local h p
  h="${ALLOWED_TARGET%:*}"; p="${ALLOWED_TARGET##*:}"
  spoof_body py_syn syn.py "" "${h:-169.254.169.254} ${p:-80} $(default_spoof)" \
    "on the receiving host run tcpdump -ni any 'tcp and src host $(default_spoof)'; a packet there means the forged source left the pod"
}

desc_spoof_arp() { echo "ARP spoofing: a gratuitous ARP reply written to an AF_PACKET socket (needs --allow-packet-socket-write; an observer must judge)"; }
body_spoof_arp() {
  spoof_body py_l2 l2.py "--cap-add NET_ADMIN" "arp $(default_spoof) 10.255.255.253 eth0" \
    "on a peer pod or the node run ip neigh show and tcpdump -eni any arp"
}

desc_spoof_ndp() { echo "NDP spoofing: an unsolicited Neighbor Advertisement for a forged IPv6 address (an observer must judge)"; }
body_spoof_ndp() {
  spoof_body py_l2 l2.py "--cap-add NET_ADMIN" "ndp fd00::dead fe80::1 eth0" \
    "on a peer pod or the node run ip -6 neigh show and tcpdump -eni any icmp6"
}

desc_fqdn_sni_bypass() { echo "FQDN/SNI allow-list: a nested container's own DNS must not widen what the policy allows"; }
body_fqdn_sni_bypass() {
  if [ -z "$ALLOWED_FQDN" ] || [ -z "$DENIED_FQDN" ]; then
    echo 'echo "needs --allowed-fqdn and --denied-fqdn"; exit 2'
    return
  fi
  cat <<EOF
run() { docker run --rm \$1 $CURL_IMAGE -sS -o /dev/null -m 10 -w '%{http_code}' "\$2" 2>&1; }
rc=0
echo "control (allowed name, bridge DNS): \$(run '' https://$ALLOWED_FQDN/)"
for net in "" "--network host"; do
  out=\$(run "\$net" https://$DENIED_FQDN/) && { echo "REACHED $DENIED_FQDN (\$net) -> \$out"; rc=1; } || echo "blocked $DENIED_FQDN (\$net)"
done
ip=\$(docker run --rm $CURL_IMAGE getent hosts $ALLOWED_FQDN 2>/dev/null | head -1 | cut -d' ' -f1)
if [ -n "\$ip" ]; then
  out=\$(docker run --rm $CURL_IMAGE -sS -o /dev/null -m 10 -w '%{http_code}' --resolve $DENIED_FQDN:443:\$ip https://$DENIED_FQDN/ 2>&1) \
    && { echo "REACHED the allowed IP with a denied SNI ($DENIED_FQDN) -> \$out"; rc=1; } || echo "blocked: allowed IP with denied SNI"
fi
exit \$rc
EOF
}

# --- running -----------------------------------------------------------------
PASSED=0
FAILED=0
SKIPPED=0
LAST_STATUS=""

report() { # status id desc [seconds]
  LAST_STATUS="$1"
  say "$(printf '%-7s %-28s %s%s' "$1" "$2" "$3" "${4:+ (${4}s)}")"
}

run_check() { # id [mode]   mode: functional | isolation
  local id="$1" mode="${2:-functional}" desc body out rc start
  desc="$("desc_$id")"
  if [ "$DRY_RUN" = 1 ]; then
    say "PLAN  $id: $desc"
    if [ "$id" = published_port_from_outside ]; then
      say "      # on the OUTER daemon: docker run --rm busybox:1.36 wget -q -T 5 -O- http://<sandbox eth0 address>:8081/ (after the check starts its own publisher inside the sandbox)"
    else
      "body_$id" | sed 's/^/      | /'
    fi
    return 0
  fi
  start=$SECONDS
  if [ "$id" = published_port_from_outside ]; then
    out="$(published_port_from_outside 2>&1)"; rc=$?
  else
    body="$("body_$id")"
    out="$(sbx "$body" 2>&1)"; rc=$?
  fi
  local took=$((SECONDS - start))
  case "$mode:$rc" in
    functional:0) PASSED=$((PASSED + 1)); report PASS "$id" "$desc" "$took" ;;
    functional:*) FAILED=$((FAILED + 1)); report FAIL "$id" "$desc" "$took"; printf '%s\n' "$out" | tail -15 | sed 's/^/      | /' ;;
    isolation:0) if [ "$LAB_NO_CILIUM" = 1 ]; then report BLOCKED "$id" "$desc" "$took"; else PASSED=$((PASSED + 1)); report PASS "$id" "$desc" "$took"; fi ;;
    isolation:2) SKIPPED=$((SKIPPED + 1)); report MANUAL "$id" "$desc" "$took"; printf '%s\n' "$out" | tail -15 | sed 's/^/      | /' ;;
    isolation:*) if [ "$LAB_NO_CILIUM" = 1 ]; then report REACHED "$id" "$desc" "$took"; else FAILED=$((FAILED + 1)); report FAIL "$id" "$desc" "$took"; fi; printf '%s\n' "$out" | tail -15 | sed 's/^/      | /' ;;
  esac
}

published_port_from_outside() {
  # The compose stack of compose_published_port was torn down by its trap, so
  # bring up a one-container publisher, then call it from a sibling on the outer daemon.
  [ -z "$K8S_POD" ] || { echo "not applicable to --k8s-pod: call the published port from a peer pod"; return 1; }
  sbx "docker rm -f outside-web >/dev/null 2>&1; docker run -d --rm --name outside-web -p 8083:8080 busybox:1.36 sh -c 'mkdir -p /www && echo hello-outside > /www/index.html && exec httpd -f -p 8080 -h /www' >/dev/null" || return 1
  sleep 2
  local ip out
  ip="$(docker_cmd inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$SANDBOX")"
  [ -n "$ip" ] || { echo "no address for $SANDBOX"; return 1; }
  out="$(docker_cmd run --rm busybox:1.36 wget -q -T 5 -O- "http://$ip:8083/" 2>&1)"; local rc=$?
  sbx "docker rm -f outside-web >/dev/null 2>&1" || true
  [ $rc -eq 0 ] && [ "$out" = hello-outside ] && return 0
  echo "wget http://$ip:8083/ -> $out"
  return 1
}

versions_line() {
  sbx 'dockerd --version; docker compose version 2>&1; docker info --format "storage-driver={{.Driver}} driver-status={{json .DriverStatus}}" 2>&1; uname -a' 2>&1 | sed 's/^/      | /'
  if [ -z "$K8S_POD" ]; then
    docker_cmd inspect -f '      | runtime={{.HostConfig.Runtime}} image={{.Config.Image}}' "$SANDBOX" 2>/dev/null
  fi
}

cilium_preflight() {
  [ "$LAB_NO_CILIUM" = 1 ] && { say "LAB MODE: no Cilium check. REACHED/BLOCKED below describe what the sandbox can do; they are NOT isolation results."; return 0; }
  command -v kubectl >/dev/null 2>&1 || { echo "isolation checks need kubectl and a Cilium cluster; kubectl is not installed" >&2; exit 4; }
  local cnp
  cnp="$(kctl -n "$K8S_NS" get ciliumnetworkpolicies.cilium.io -o name 2>&1)" || {
    echo "isolation checks need a Cilium policy in namespace $K8S_NS; the CiliumNetworkPolicy API answered: $cnp" >&2
    exit 4
  }
  [ -n "$cnp" ] || {
    echo "isolation checks need bex's allow-list policy: namespace $K8S_NS has no CiliumNetworkPolicy. Nothing to test the platform floor against; not running." >&2
    exit 4
  }
  say "Cilium policies in $K8S_NS: $(printf '%s' "$cnp" | tr '\n' ' ')"
}

print_header() {
  say "sandbox-docker-spike: runtime=$RUNTIME caps=$CAPS docker-context=${DOCKER_CTX:-current} versions=\"$VERSIONS\" tmpfs=$([ "$NO_OVERLAY" = 1 ] && echo none || echo "$TMPFS_SIZE")"
  say "NOTE: whatever node this runs on is NOT necessarily a bex node. Results are about gVisor and Docker on THAT node (runsc, kernel, arch as printed below)."
}

main() {
  print_header
  if [ "$DRY_RUN" = 1 ]; then
    say "DRY RUN: nothing below touches a docker daemon or a cluster."
    say "GUARD: would refuse (exit 3) when kubectl lists a node labelled bex.co/pool=sandbox that matches --target-node, the docker endpoint host or this hostname, or when a sandbox pool exists and --target-node is not given."
    say "SANDBOX: docker ${DOCKER_CTX:+--context $DOCKER_CTX }run -d --runtime $RUNTIME $(cap_flags) docker:<v>-dind with this bootstrap:"
    bootstrap | sed 's/^/      | /'
    local id
    for id in $FUNCTIONAL; do run_check "$id"; done
    if [ "$ISOLATION" = 1 ]; then
      say "ISOLATION (opt-in; needs a Cilium namespace; exits 4 without one):"
      for id in $ISOLATION_CHECKS; do run_check "$id" isolation; done
    else
      say "ISOLATION checks are off (--isolation): spoof_src_ip, spoof_arp, spoof_ndp, raw_syn_denied, fqdn_sni_bypass, floor_direct_tcp; --dry-run --isolation prints them."
    fi
    return 0
  fi

  production_guard
  if [ "$GUARD_ONLY" = 1 ]; then
    say "guard: passed"
    return 0
  fi
  [ "$ISOLATION" = 1 ] && cilium_preflight
  local v id
  if [ -n "$K8S_POD" ]; then
    VERSIONS="pod"
  fi
  for v in $VERSIONS; do
    say "== Docker $v"
    if [ "$v" != pod ]; then
      if ! start_sandbox "$v" 2>&1; then
        FAILED=$((FAILED + 1)); report FAIL "start_sandbox" "docker run --runtime $RUNTIME docker:$v-dind"
        continue
      fi
    fi
    run_check dockerd_up
    if [ "$LAST_STATUS" = FAIL ]; then
      [ -z "$K8S_POD" ] && docker_cmd logs "$SANDBOX" 2>&1 | tail -15 | sed 's/^/      | log: /'
    else
      versions_line
      for id in $FUNCTIONAL; do
        [ "$id" = dockerd_up ] && continue
        [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) continue ;; esac
        run_check "$id"
      done
      if [ "$ISOLATION" = 1 ]; then
        for id in $ISOLATION_CHECKS; do run_check "$id" isolation; done
      fi
    fi
    [ "$v" != pod ] && stop_sandbox
  done
  say "== summary: $PASSED passed, $FAILED failed, $SKIPPED manual"
  [ "$FAILED" -eq 0 ]
}

main
