#!/usr/bin/env bash
set -euo pipefail

suffix="$$"
network="bex-m38-verify-${suffix}"
fixture_container="bex-m38-fixture-${suffix}"
agent_container="bex-m38-agent-${suffix}"
resumed_container="bex-m38-resumed-${suffix}"
agent_image="bex-agent-credential:verify-${suffix}"
fixture_image="bex-agent-credential-fixture:verify-${suffix}"
resumed_image="bex-agent-credential-resumed:verify-${suffix}"

cleanup() {
  docker rm -f "$resumed_container" "$agent_container" "$fixture_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  docker image rm "$resumed_image" "$fixture_image" "$agent_image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repo_root"

docker build -f lego/agent-image/Dockerfile -t "$agent_image" lego >/dev/null
docker build -f lego/agent-image/verify.Dockerfile -t "$fixture_image" lego >/dev/null
docker run --rm --entrypoint /bin/sh "$agent_image" -ec '
  test "$(id -u)" != 0
  command -v git-credential-bex
  command -v bex-pre-snapshot
  test "$(git config --system credential.helper)" = bex
  test "$(git config --system credential.useHttpPath)" = true
'

docker network create "$network" >/dev/null
docker run -d --name "$fixture_container" --network "$network" \
  --network-alias github.com \
  --network-alias bex-ssh-gateway.bex-system.svc.cluster.local \
  "$fixture_image" >/dev/null

ready=0
for _ in $(seq 1 50); do
  if docker logs "$fixture_container" 2>&1 | grep -q 'fixture ready'; then
    ready=1
    break
  fi
  sleep 0.1
done
if [ "$ready" -ne 1 ]; then
  docker logs "$fixture_container" >&2
  echo "credential fixture did not become ready" >&2
  exit 1
fi

session_env=(
  --env BEX_SANDBOX_NAMESPACE=tea-a-sandbox
  --env BEX_AGENT_SESSION_ID=ags-verify
  --env BEX_AGENT_REPOSITORY=octo/repo
  --env BEX_AGENT_BRANCH=bex-agent/verify
  --env GIT_SSL_NO_VERIFY=1
)

docker run --name "$agent_container" --network "$network" "${session_env[@]}" \
  --entrypoint /bin/sh "$agent_image" -ec '
    bex-agent-driver >/tmp/bex-agent-driver.log 2>&1 &
    driver_pid=$!
    trap "kill $driver_pid >/dev/null 2>&1 || true" EXIT
    ready=0
    for _ in $(seq 1 50); do
      if curl -fsS http://127.0.0.1:8787/healthz >/dev/null 2>&1; then
        ready=1
        break
      fi
      sleep 0.1
    done
    test "$ready" = 1
    git clone https://github.com/octo/repo.git work
    cd work
    git config user.name "bex agent"
    git config user.email agent@bex.invalid
    printf "first agent change\n" >>README.md
    git add README.md
    git commit -m "agent change one"
    git push origin HEAD:refs/heads/bex-agent/verify
    git fetch origin bex-agent/verify
    if grep -R -I -q "ghs_verify_" /home/bex /workspace /tmp 2>/dev/null; then
      echo "credential material reached the live agent rootfs" >&2
      exit 1
    fi
    printf "rootfs-survives\n" >/home/bex/resume-state
    /usr/local/bin/bex-pre-snapshot
  '

docker commit "$agent_container" "$resumed_image" >/dev/null
docker kill --signal USR1 "$fixture_container" >/dev/null

advanced=0
for _ in $(seq 1 50); do
  if docker logs "$fixture_container" 2>&1 | grep -q 'logical clock advanced beyond token TTL'; then
    advanced=1
    break
  fi
  sleep 0.1
done
if [ "$advanced" -ne 1 ]; then
  echo "fixture clock did not advance" >&2
  exit 1
fi

docker run --name "$resumed_container" --network "$network" "${session_env[@]}" \
  --entrypoint /bin/sh "$resumed_image" -ec '
    test "$(cat /home/bex/resume-state)" = rootfs-survives
    cd /workspace/work
    printf "second agent change after token expiry and resume\n" >>README.md
    git add README.md
    git commit -m "agent change two after resume"
    git push origin HEAD:refs/heads/bex-agent/verify
    git fetch origin bex-agent/verify
    if grep -R -I -q "ghs_verify_" /home/bex /workspace /tmp 2>/dev/null; then
      echo "credential material reached the resumed rootfs" >&2
      exit 1
    fi
  '

commit_count="$(docker exec "$fixture_container" git --git-dir=/srv/git/octo/repo.git rev-list --count refs/heads/bex-agent/verify)"
status="$(docker exec "$fixture_container" cat /tmp/bex-agent-fixture-status)"
mint_count="$(printf '%s\n' "$status" | awk -F= '$1 == "mint_count" { print $2 }')"
logical_advances="$(printf '%s\n' "$status" | awk -F= '$1 == "logical_advances" { print $2 }')"
authenticated_after_advance="$(printf '%s\n' "$status" | awk -F= '$1 == "authenticated_after_advance" { print $2 }')"

if [ "$commit_count" -ne 3 ] || [ "$mint_count" -lt 5 ] || [ "$logical_advances" -ne 1 ] || [ "$authenticated_after_advance" -lt 1 ]; then
  echo "unexpected e2e state: commits=$commit_count mints=$mint_count advances=$logical_advances post_advance_auth=$authenticated_after_advance" >&2
  exit 1
fi

echo "agent credential live-rootfs e2e passed: clone/push/fetch, >1h refresh, snapshot/resume, no token at rest"
