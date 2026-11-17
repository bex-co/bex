# shellcheck shell=bash
# The agent leg's retention probe (ADR047 D12, w22/m413): a turn a person stops
# keeps its disk, and the session's next message runs in a new sandbox that
# starts from it.
#
# Sourced by scripts/verify-sandbox-isolation.sh (BEX_VERIFY_AGENT_RETENTION=1),
# which has a chat-only agent session whose turn is held in flight. The caller
# provides api_call, expect_status, fail, pass, wait_ready_pod and
# workload_container, and $fixture_dir and $run_id. No model is called: the
# file is written with kubectl, and the turn the follow-up message starts is
# held like the first.
#
# Two calls, because the caller cancels the session in between:
#
#   verify_agent_retention <auth> <session> <namespace> <pod> <container>
#     Writes a file in the running turn's working directory, stops the turn
#     (Stop turn, POST …/interrupt), waits for the turn to read
#     `retention: kept` with a `retainedUntil`, sends a message, and reads the
#     file in the new sandbox, whose image must be the retained one. Sets
#     agent_retention_sandbox (the stopped turn's sandbox) and
#     agent_retention_next_cr (the new one, for the caller's cleanup).
#
#   verify_agent_retention_released <namespace>
#     After the session is canceled: no image of this run is left in the
#     registry under the workspace's repositories, neither the retained image
#     nor the pause snapshot it was copied from.

agent_retention_sandbox=""
agent_retention_next_cr=""
agent_retention_forward_pid=""
agent_retention_zot_url=""
agent_retention_dir="/workspace"

# agent_retention_turn <auth> <session>: turn 1's retention facts, as JSON.
agent_retention_turn() {
  local out="$fixture_dir/agent-retention-transcript.json"
  [ "$(api_call "$1" GET "/v1/agent-sessions/$2/transcript" "$out")" = 200 ] || return 1
  jq -c '.turns[0] | {interrupted, retention, retainedUntil, retentionLostReason}' "$out"
}

verify_agent_retention() {
  local auth="$1" session="$2" namespace="$3" pod="$4" container="$5"
  local file="$agent_retention_dir/retention-probe-$run_id.txt" value="kept-$run_id"
  local read_out="$fixture_dir/agent-retention-session.json" turn="" state="" next="" deadline image got

  expect_status 200 "$auth" GET "/v1/agent-sessions/$session" "$read_out"
  agent_retention_sandbox="$(jq -er '.sandboxId' "$read_out")"
  kubectl -n "$namespace" exec "$pod" -c "$container" -- /bin/sh -c \
    'printf "%s\n" "$1" >"$2" && cat "$2"' sh "$value" "$file" >"$fixture_dir/agent-retention-write.txt" 2>&1 \
    || fail "could not write $file in the agent sandbox: $(head -c 300 "$fixture_dir/agent-retention-write.txt")"

  expect_status 202 "$auth" POST "/v1/agent-sessions/$session/interrupt" "$fixture_dir/agent-retention-interrupt.json"
  # The save pauses the sandbox, commits its disk and copies the image: a
  # minute on an agent image, bounded here well under the save's own deadline.
  deadline=$((SECONDS + 600))
  while ((SECONDS < deadline)); do
    turn="$(agent_retention_turn "$auth" "$session" || true)"
    state="$(jq -r '.retention // empty' <<<"${turn:-null}" 2>/dev/null || true)"
    case "$state" in
      kept | lost) break ;;
      # A settled stop that is not being saved keeps nothing: retention is off.
      none) [ "$(jq -r '.interrupted' <<<"$turn")" = true ] && break ;;
    esac
    sleep 5
  done
  [ "$state" = kept ] || fail "the stopped turn's disk was not kept: ${turn:-no transcript}"
  jq -e '.interrupted == true and (.retainedUntil | type == "string" and length > 0)' <<<"$turn" >/dev/null \
    || fail "the stopped turn does not read interrupted with a retention time: $turn"
  pass "a stopped turn reads interrupted and kept, with the time its disk is kept until"

  expect_status 200 "$auth" POST "/v1/agent-sessions/$session/steer" "$fixture_dir/agent-retention-steer.json" \
    --header 'Content-Type: application/json' --data '{"prompt":"Reply with the word ok."}'
  deadline=$((SECONDS + 300))
  while ((SECONDS < deadline)); do
    if [ "$(api_call "$auth" GET "/v1/agent-sessions/$session" "$read_out")" = 200 ]; then
      next="$(jq -r '.sandboxId // empty' "$read_out")"
      [ -n "$next" ] && [ "$next" != "$agent_retention_sandbox" ] && break
    fi
    next=""
    sleep 5
  done
  [ -n "$next" ] || fail "the follow-up message got no new sandbox: $(jq -c '{phase, state, status, sandboxId}' "$read_out" 2>/dev/null)"
  agent_retention_next_cr="$next"
  pod="$(wait_ready_pod "$namespace" "$next")" || fail "the follow-up turn's sandbox never reached Ready"
  container="$(workload_container "$namespace" "$pod")"
  image="$(kubectl -n "$namespace" get pod "$pod" -o json \
    | jq -r --arg c "$container" '.spec.containers[] | select(.name == $c) | .image')"
  case "$image" in
    */"$namespace"/images/ari-*@sha256:*) ;;
    *) fail "the follow-up turn's sandbox did not start from the retained image: $image" ;;
  esac
  got="$(kubectl -n "$namespace" exec "$pod" -c "$container" -- cat "$file" 2>"$fixture_dir/agent-retention-read.err" || true)"
  [ "$got" = "$value" ] || fail "the stopped turn's file is not in the next turn's sandbox ($file): $(head -c 300 "$fixture_dir/agent-retention-read.err")"
  turn="$(agent_retention_turn "$auth" "$session" || true)"
  [ "$(jq -r '.retention // empty' <<<"${turn:-null}" 2>/dev/null)" = continued ] \
    || fail "the stopped turn does not read continued once the next turn started from it: ${turn:-no transcript}"
  pass "the next message's sandbox starts from the retained image and holds the stopped turn's uncommitted file"
}

# agent_retention_registry: forward the registry, set agent_retention_zot_url,
# and write a curl config with the read-only meter user (never printed).
agent_retention_registry() {
  local log="$fixture_dir/agent-retention-zot-pf.log" user pass port i
  user="$(kubectl -n bex-registry get secret bex-sandbox-registry -o jsonpath='{.data.reader-username}' | base64 -d)"
  pass="$(kubectl -n bex-registry get secret bex-sandbox-registry -o jsonpath='{.data.reader-password}' | base64 -d)"
  [ -n "$user" ] && [ -n "$pass" ] || return 1
  (umask 077 && printf 'user = "%s:%s"\n' "$user" "$pass" >"$fixture_dir/agent-retention-zot.curl")
  kubectl -n bex-registry port-forward svc/zot :5000 --request-timeout=20s >"$log" 2>&1 &
  agent_retention_forward_pid=$!
  for i in $(seq 1 40); do
    port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\) .*/\1/p' "$log" 2>/dev/null | head -1 || true)"
    [ -n "$port" ] && { agent_retention_zot_url="http://127.0.0.1:$port"; return 0; }
    sleep 0.5
  done
  return 1
}

# agent_retention_stop_forward: end the registry forward; safe to call twice.
agent_retention_stop_forward() {
  [ -n "$agent_retention_forward_pid" ] && kill "$agent_retention_forward_pid" 2>/dev/null
  agent_retention_forward_pid=""
  return 0
}

verify_agent_retention_released() {
  local namespace="$1" zot_url left="" deadline=$((SECONDS + 300)) repo tags
  local catalog="$fixture_dir/agent-retention-catalog.json" tags_out="$fixture_dir/agent-retention-tags.json"
  agent_retention_registry \
    || { agent_retention_stop_forward; fail "could not reach the registry with the meter's read user"; }
  zot_url="$agent_retention_zot_url"
  while ((SECONDS < deadline)); do
    left=""
    if [ "$(curl --config "$fixture_dir/agent-retention-zot.curl" --silent --max-time 30 --output "$catalog" \
      --write-out '%{http_code}' "$zot_url/v2/_catalog?n=10000")" = 200 ]; then
      # This run's repositories: the workspace's retained images, and the
      # pause snapshot of the sandbox whose turn was stopped.
      while IFS= read -r repo; do
        [ -n "$repo" ] || continue
        tags='[]'
        if [ "$(curl --config "$fixture_dir/agent-retention-zot.curl" --silent --max-time 30 --output "$tags_out" \
          --write-out '%{http_code}' "$zot_url/v2/$repo/tags/list")" = 200 ]; then
          tags="$(jq -c '.tags // []' "$tags_out")"
        fi
        [ "$tags" = '[]' ] || left+="$repo $tags"$'\n'
      done < <(jq -r --arg p "snapshots/$namespace/" --arg s "$agent_retention_sandbox" '
        .repositories[]? | select(startswith($p)) |
        select((ltrimstr($p) | startswith("images/ari-")) or (ltrimstr($p) | startswith($s + "-")))' "$catalog")
      [ -z "$left" ] && break
    else
      left="the registry catalog could not be read"
    fi
    sleep 10
  done
  agent_retention_stop_forward
  [ -z "$left" ] || fail "the canceled session left images in the registry: $left"
  pass "after Cancel the registry holds no retained image and no pause snapshot of the stopped turn"
}
