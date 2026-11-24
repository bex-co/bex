#!/usr/bin/env bash
# Production proof leg for w22/m443: sandbox API calls are rate limited per user
# and per operation at Render's documented rates (render.com/docs/sandboxes-limits),
# not by bex-api's general budget.
#
#   1. 600 execs in one sandbox inside a minute, through the public path the
#      Render CLI and SDK use (mint a run connect token, redeem it): none is
#      rate limited. Before w22/m443 the mints shared the general 500 a minute
#      and the redeems an IP budget of 60 a minute.
#   2. A burst of creates inside a few seconds, alternating between two API
#      keys of one user: the first 100 are not rate limited, and creates past
#      the user's bucket are refused with the rate limiter's 429
#      (`id: rate_limited`, with Retry-After), on both keys. The class is a
#      token bucket of 100 that refills at 100 a minute, and each bex-api
#      replica keeps its own (ADR006: the limiters are in-process), so the leg
#      does not expect exactly 100: it sends 100 more creates than every
#      replica's bucket holds together, and bounds what is admitted by the
#      buckets plus their refill over the seconds the burst took.
#
# The creates start no sandbox: the group is first held at a concurrencyLimit
# of 1 by the leg's one running sandbox, so every create the rate limiter
# admits is refused by the group with its own 429 (its message names the group's
# concurrency limit; REST carries only Render's codes, w8/411). The rate limit
# is charged before that check, which is what makes the two 429s
# distinguishable here.
#
# Run by sandbox-measure.yml (leg `rate`) through
# scripts/verify-sandbox-isolation-live.sh with BEX_VERIFY_SHARED_USER_KEYS=1,
# which makes workspace A's owner one user with two API keys. One sandbox,
# everything printed as `MEASURE <name> <json>`.

cd "$(dirname "$0")/.."
# shellcheck source=lib/sandbox-measure.sh
. scripts/lib/sandbox-measure.sh

: "${BEX_WS_A_OWNER_SECOND_TOKEN:?set BEX_WS_A_OWNER_SECOND_TOKEN (run the live wrapper with BEX_VERIFY_SHARED_USER_KEYS=1)}"
printf 'header = "Authorization: Bearer %s"\n' "$BEX_WS_A_OWNER_SECOND_TOKEN" >"$fixture_dir/owner2.curl"

execs="${BEX_MEASURE_RATE_EXECS:-600}"
# How many execs the client keeps in flight, as an agent running commands in
# parallel does. Waves of 20 sent 600 in 88 s on production (run 45408380111:
# 30 waves of about 2.9 s, every exec answered); 40 in flight fit a minute.
exec_parallel="${BEX_MEASURE_RATE_EXEC_PARALLEL:-40}"
# One bucket per bex-api replica, and one more for a rollout's surge pod.
replicas="${BEX_MEASURE_RATE_REPLICAS:-$(kubectl -n "${BEX_API_NAMESPACE:-bex-system}" get deployment bex-api -o jsonpath='{.spec.replicas}' 2>/dev/null || true)}"
[[ "$replicas" =~ ^[1-9][0-9]*$ ]] || replicas=1
buckets=$((replicas + 1))
creates="${BEX_MEASURE_RATE_CREATES:-$((buckets * 100 + 100))}"
create_parallel="${BEX_MEASURE_RATE_CREATE_PARALLEL:-30}"

# rate_limited <status> <body file>: true when the answer is the rate limiter's
# 429, which carries `id: rate_limited` and no domain code.
rate_limited() {
  [ "$1" = 429 ] && [ "$(jq -r '.id // empty' "$2" 2>/dev/null)" = rate_limited ]
}

echo "==> one running sandbox"
sandbox_id="$(create_sandbox "$fixture_dir/sandbox.json" '{"type":"deny-all"}')"
wait_status "$sandbox_id" running 300
group_id="$(jq -er '.sandboxGroupId' "$fixture_dir/sandbox.json")"
sandbox_exec "$sandbox_id" true "$fixture_dir/warm.txt" || fail "the first exec in sandbox $sandbox_id did not complete: $(head -c 300 "$fixture_dir/warm.txt.sse" 2>/dev/null)"

echo "==> $execs execs inside a minute, $exec_parallel in flight"
mkdir "$fixture_dir/exec"
# one_exec <n>: mint and redeem one `true`, and write "<mint status> <redeem
# status> <limited|ok|failed>" to exec/<n>.result.
one_exec() {
  trap - ERR # a failed request is a recorded result here, not an error to report
  local out="$fixture_dir/exec/$1" mint redeem=000 verdict=failed
  mint="$(curl --config "$fixture_dir/owner.curl" --silent --show-error --max-time 60 --request POST \
    --header 'Content-Type: application/json' --data '{"command":"true"}' \
    --output "$out.mint" --write-out '%{http_code}' \
    "$api_url/v1/sandboxes/$sandbox_id/runs/stream/token?ownerId=$BEX_WORKSPACE_A" 2>/dev/null)" || mint=000
  if rate_limited "$mint" "$out.mint"; then
    verdict=limited
  elif [ "$mint" = 201 ]; then
    printf 'header = "Authorization: Bearer %s"\n' "$(jq -r '.token' "$out.mint")" >"$out.curl"
    redeem="$(curl --config "$out.curl" --silent --show-error --max-time 60 --request POST \
      --header 'Accept: text/event-stream' --header 'Content-Type: application/json' --data '{"command":"true"}' \
      --output "$out.sse" --write-out '%{http_code}' "$(jq -r '.uri' "$out.mint")" 2>/dev/null)" || redeem=000
    if rate_limited "$redeem" "$out.sse"; then
      verdict=limited
    elif [ "$redeem" = 200 ] && grep -q '"exit_code":0' "$out.sse"; then
      verdict=ok
    fi
  fi
  printf '%s %s %s\n' "$mint" "$redeem" "$verdict" >"$out.result"
}
exec_started="$SECONDS"
in_flight "$execs" "$exec_parallel" one_exec
exec_seconds=$((SECONDS - exec_started))
exec_ok="$(cat "$fixture_dir"/exec/*.result | grep -c ' ok$' || true)"
exec_limited="$(cat "$fixture_dir"/exec/*.result | grep -c ' limited$' || true)"
measure execs "$(jq -cn --argjson sent "$execs" --argjson ok "$exec_ok" --argjson limited "$exec_limited" --argjson s "$exec_seconds" \
  --arg statuses "$(cat "$fixture_dir"/exec/*.result | awk '{print $1 "/" $2}' | sort | uniq -c | awk '{printf "%s x%s ", $2, $1}')" \
  '{sent: $sent, completed: $ok, rateLimited: $limited, seconds: $s, mintAndRedeemStatuses: $statuses}')"
check "the $execs execs were sent inside a minute" "$([ "$exec_seconds" -le 60 ] && echo true || echo false)" "they took $exec_seconds s; raise BEX_MEASURE_RATE_EXEC_PARALLEL"
check "none of the $execs execs was rate limited" "$([ "$exec_limited" = 0 ] && echo true || echo false)" "$exec_limited were"
check "at least 95% of the execs completed with exit code 0" "$([ "$exec_ok" -ge $((execs * 95 / 100)) ] && echo true || echo false)" "$exec_ok of $execs"

echo "==> hold the group at one slot, then $creates creates across the user's two API keys ($replicas bex-api replicas)"
expect_status 200 PATCH "/v1/sandbox-groups/$group_id" "$fixture_dir/group.json" \
  --header 'Content-Type: application/json' --data '{"concurrencyLimit":1}'
mkdir "$fixture_dir/create"
# one_create <n>: one create with the first key (odd n) or the second (even n);
# writes "<status> <limited|group|created|other>" to create/<n>.result.
one_create() {
  trap - ERR
  local out="$fixture_dir/create/$1" auth="$fixture_dir/owner.curl" status verdict=other
  [ $(($1 % 2)) = 0 ] && auth="$fixture_dir/owner2.curl"
  status="$(curl --config "$auth" --silent --show-error --max-time 60 --request POST \
    --header 'Content-Type: application/json' \
    --data "{\"ownerId\":\"$BEX_WORKSPACE_A\",\"networkPolicy\":{\"type\":\"deny-all\"},\"sandboxGroupId\":\"$group_id\"}" \
    --dump-header "$out.headers" --output "$out.json" --write-out '%{http_code}' "$api_url/v1/sandboxes" 2>/dev/null)" || status=000
  if rate_limited "$status" "$out.json"; then
    verdict=limited
  elif [ "$status" = 429 ] && jq -e '.message | test("^sandbox group \".*\" is at its concurrency limit")' "$out.json" >/dev/null 2>&1; then
    verdict=group
  elif [ "$status" = 201 ]; then
    verdict=created
  fi
  printf '%s %s %s\n' "$status" "$verdict" "$(($1 % 2))" >"$out.result"
}
create_started="$(date +%s.%N)"
in_flight "$creates" "$create_parallel" one_create
create_seconds="$(awk -v a="$create_started" -v b="$(date +%s.%N)" 'BEGIN { printf "%.1f", b - a }')"
# A create the group admitted after all started a sandbox: end it on exit.
for created in "$fixture_dir"/create/*.json; do
  [ "$(awk '{print $2}' "${created%.json}.result")" = created ] && measure_ids+=("$(jq -r '.id' "$created")")
done
count() { cat "$fixture_dir"/create/*.result | awk -v v="$1" '$2 == v' | wc -l | tr -d ' '; }
limited="$(count limited)"
# Admitted by the rate limiter: refused by the group's limit afterwards, or created.
admitted=$(($(count group) + $(count created)))
limited_first_key="$(cat "$fixture_dir"/create/*.result | awk '$2 == "limited" && $3 == 1' | wc -l | tr -d ' ')"
limited_second_key="$(cat "$fixture_dir"/create/*.result | awk '$2 == "limited" && $3 == 0' | wc -l | tr -d ' ')"
# What that many buckets of 100, each refilling at 100 a minute, can admit in that long.
ceiling="$(awk -v s="$create_seconds" -v b="$buckets" 'BEGIN { printf "%d", b * (100 + s * 100 / 60) + 2 }')"
retry_after=""
for result in "$fixture_dir"/create/*.result; do
  if [ "$(awk '{print $2}' "$result")" = limited ]; then
    retry_after="$(tr -d '\r' <"${result%.result}.headers" | awk 'tolower($1) == "retry-after:" {print $2}' | head -1)"
    break
  fi
done
measure creates "$(jq -cn --argjson sent "$creates" --argjson limited "$limited" --argjson group "$(count group)" --argjson created "$(count created)" \
  --argjson other "$(count other)" --arg s "$create_seconds" --argjson ceiling "$ceiling" --argjson k1 "$limited_first_key" --argjson k2 "$limited_second_key" \
  --argjson replicas "$replicas" --arg retry "$retry_after" --arg statuses "$(cat "$fixture_dir"/create/*.result | awk '{print $1 "/" $2}' | sort | uniq -c | awk '{printf "%s x%s ", $2, $1}')" \
  '{sent: $sent, rateLimited: $limited, rateLimitedByKey: [$k1, $k2], refusedByGroupLimit: $group, created: $created, other: $other,
    seconds: ($s | tonumber), apiReplicas: $replicas, admittedCeiling: $ceiling, retryAfter: $retry, statuses: $statuses}')"
check "the first 100 creates were not rate limited" "$([ "$admitted" -ge 100 ] && echo true || echo false)" "only $admitted of $creates were admitted by the rate limiter"
check "creates past the user's bucket were refused with the rate limiter's 429" "$([ "$limited" -ge 1 ] && echo true || echo false)" "none of $creates creates in $create_seconds s was rate limited: two keys of one user do not share a bucket, or the limit is not Render's 100 for each of $replicas replicas"
check "no more creates were admitted than $buckets buckets of 100 hold over $create_seconds s" "$([ "$admitted" -le "$ceiling" ] && echo true || echo false)" "$admitted admitted, ceiling $ceiling"
check "both API keys were refused by the one bucket" "$([ "$limited_first_key" -ge 1 ] && [ "$limited_second_key" -ge 1 ] && echo true || echo false)" "rate limited: first key $limited_first_key, second key $limited_second_key"
check "the rate limiter's 429 carries Retry-After" "$([ -n "$retry_after" ] && [ "$retry_after" -ge 1 ] 2>/dev/null && echo true || echo false)" "Retry-After '$retry_after'"
check "every create was answered by the rate limiter, the group limit or a 201" "$([ "$(count other)" = 0 ] && echo true || echo false)" "$(count other) other answers"

echo "==> the leg leaves no sandbox behind"
expect_status 200 GET "/v1/sandboxes?ownerId=$BEX_WORKSPACE_A&limit=100" "$fixture_dir/list.json"
listed="$(jq -c --arg keep "$sandbox_id" '[.[].sandbox | select(.id != $keep) | {id, status}]' "$fixture_dir/list.json")"
unregistered=0
for id in $(jq -r '.[].id' <<<"$listed"); do
  case " ${measure_ids[*]} " in *" $id "*) ;; *) unregistered=$((unregistered + 1)) ;; esac
done
measure leftovers "$(jq -cn --argjson l "$listed" --argjson u "$unregistered" '{otherSandboxes: $l, notRegisteredForCleanup: $u}')"
check "every sandbox the leg started is registered for termination on exit" "$([ "$unregistered" = 0 ] && echo true || echo false)" "$unregistered are not"

[ "$failures" = 0 ] || fail "$failures rate-limit check(s) failed (see the lines above)"
pass "rate leg: $execs execs in $exec_seconds s with none rate limited; of $creates creates in $create_seconds s, $admitted were admitted and $limited refused by the rate limiter"
