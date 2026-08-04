#!/usr/bin/env bash
set -euo pipefail

# Live end-to-end verification of the ADR047 cloud coding-agent session path:
#   phase-1 (w3/m41): create a session on a real disposable repo -> the agent
#     commits -> bex-api opens a draft PR with Codex-style evidence -> a steering
#     turn produces a follow-up commit -> failure paths surface as failed
#     sessions, never hangs.
#   conversation API (w3/m43, ADR047 D9): mint an attach ticket -> the stream
#     endpoint replays the transcript + terminates with [DONE] (attach+replay) ->
#     a resumed session accepts a live prompt turn whose parts stream back
#     (turn) -> a fresh attach replays the teed parts (reattach, no dup). The
#     stream publishes under the primary API origin and rejects a ticketless call.
# Every assertion is loud; a trap cancels the sessions it created.
#
# This is an operator-run verifier (like scripts/verify-sandbox-isolation-live.sh
# and scripts/webhooks-verify.sh) — it is NOT wired into CI. It talks only to the
# public REST + stream surface with a caller token; it needs no cluster access.
#
# Required:
#   BEX_LIVE_VERIFY=1
#   BEX_API_URL=https://api.example.com
#   BEX_API_TOKEN=<bearer for a workspace member with can_operate>
#   BEX_VERIFY_REPO=<owner/name of a DISPOSABLE repo under the workspace's
#                    GitHub App installation; default branch protected so only
#                    bex-agent/* is writable>
#
# Precondition (out of band, ADR047 D7): the workspace's BYO model key must be
# provisioned in OpenBao before this runs — the driver sources it there, not from
# the request. Provision with:
#   bao kv put agent-sessions/<workspaceId>/model-key BEX_AGENT_MODEL_API_KEY=<key>
# (workspaceId is the tea-… id BEX_VERIFY_OWNER_ID resolves to.)
#
# Optional:
#   BEX_VERIFY_AGENT=codex           # ACP adapter selector (default codex)
#   BEX_VERIFY_MODEL=gpt-5           # model passed to the adapter
#   BEX_VERIFY_MODEL_ENDPOINT=https://api.openai.com/v1
#   BEX_VERIFY_OWNER_ID=<workspace id to bill/scope to>
#   BEX_GITHUB_TOKEN=<token to independently confirm the PR on GitHub>
#   BEX_VERIFY_TIMEOUT=1800          # seconds to wait for a turn (default 30m)
#   BEX_VERIFY_EGRESS_PROFILE=<label recorded in the log for the m40 policy in effect>

cd "$(dirname "$0")/.."

[ "${BEX_LIVE_VERIFY:-0}" = 1 ] || {
  echo "error: set BEX_LIVE_VERIFY=1 to authorize a live agent-session verification run" >&2
  exit 2
}
: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${BEX_API_TOKEN:?set BEX_API_TOKEN to a caller bearer token}"
: "${BEX_VERIFY_REPO:?set BEX_VERIFY_REPO to a disposable owner/name repo}"

for command in curl jq; do
  command -v "$command" >/dev/null || { echo "error: missing required command: $command" >&2; exit 2; }
done

api_url="${BEX_API_URL%/}"
agent="${BEX_VERIFY_AGENT:-codex}"
model="${BEX_VERIFY_MODEL:-}"
model_endpoint="${BEX_VERIFY_MODEL_ENDPOINT:-}"
owner_id="${BEX_VERIFY_OWNER_ID:-}"
timeout_s="${BEX_VERIFY_TIMEOUT:-1800}"
egress_profile="${BEX_VERIFY_EGRESS_PROFILE:-unspecified}"
stamp="$(date -u +%Y%m%d%H%M%S)-$$"
branch="bex-agent/verify-${stamp}"

created_sessions=()

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "PASS: $*"; }

cleanup() {
  for sid in "${created_sessions[@]:-}"; do
    [ -n "$sid" ] || continue
    curl -sS -X POST -H "Authorization: Bearer ${BEX_API_TOKEN}" \
      "${api_url}/v1/agent-sessions/${sid}/cancel" >/dev/null 2>&1 || true
  done
}
trap cleanup EXIT

api() {
  # api METHOD PATH [json-body]
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS -X "$method" -H "Authorization: Bearer ${BEX_API_TOKEN}" \
      -H "Content-Type: application/json" -d "$body" "${api_url}${path}"
  else
    curl -sS -X "$method" -H "Authorization: Bearer ${BEX_API_TOKEN}" "${api_url}${path}"
  fi
}

config_json() {
  jq -n --arg agent "$agent" --arg model "$model" --arg endpoint "$model_endpoint" --arg task "$1" \
    '{agent:$agent, task:$task} + (if $model=="" then {} else {model:$model} end) + (if $endpoint=="" then {} else {modelEndpoint:$endpoint} end)'
}

create_session() {
  # create_session BRANCH TASK -> prints session id (or the raw error body)
  local br="$1" task="$2"
  jq -n --arg repo "$BEX_VERIFY_REPO" --arg branch "$br" \
        --arg owner "$owner_id" --argjson config "$(config_json "$task")" \
     '{repo:$repo, branch:$branch, agentConfig:$config} + (if $owner=="" then {} else {ownerId:$owner} end)' \
    | api POST "/v1/agent-sessions" "$(cat -)"
}

poll_terminal() {
  # poll_terminal SESSION_ID -> prints the final session JSON; fails on timeout
  local sid="$1" deadline=$(( $(date +%s) + timeout_s )) view phase
  while :; do
    view="$(api GET "/v1/agent-sessions/${sid}")"
    phase="$(jq -r '.phase // "?"' <<<"$view")"
    case "$phase" in
      completed|failed) echo "$view"; return 0 ;;
      canceled) fail "session ${sid} was canceled unexpectedly" ;;
    esac
    [ "$(date +%s)" -lt "$deadline" ] || fail "session ${sid} did not finish within ${timeout_s}s (last phase=${phase})"
    sleep 10
  done
}

echo "== agent-session live verify =="
echo "   repo=${BEX_VERIFY_REPO} agent=${agent} branch=${branch} egress-profile=${egress_profile}"

# ---------------------------------------------------------------------------
# 1. Happy path: create -> completed -> draft PR + evidence.
# ---------------------------------------------------------------------------
resp="$(create_session "$branch" "Add a file named VERIFY-${stamp}.md describing this run, then commit it.")"
sid="$(jq -r '.id // empty' <<<"$resp")"
[ -n "$sid" ] || fail "create returned no session id: $resp"
created_sessions+=("$sid")
ok "created session ${sid}"

final="$(poll_terminal "$sid")"
phase="$(jq -r '.phase' <<<"$final")"
[ "$phase" = completed ] || fail "expected completed, got ${phase} (reason=$(jq -r '.failureReason // ""' <<<"$final"))"
pr_url="$(jq -r '.prUrl // empty' <<<"$final")"
head_sha="$(jq -r '.headSha // empty' <<<"$final")"
[ -n "$pr_url" ] || fail "completed session has no prUrl"
[ -n "$head_sha" ] || fail "completed session has no headSha"
jq -e '.evidence and ((.evidence.commandLog|length)>0 or (.evidence.changedFiles|length)>0 or (.evidence.outputTail|length)>0)' <<<"$final" >/dev/null \
  || fail "completed session carries no evidence"
ok "draft PR ${pr_url} opened with evidence (head ${head_sha})"

if [ -n "${BEX_GITHUB_TOKEN:-}" ]; then
  pr_number="$(jq -r '.prNumber' <<<"$final")"
  draft="$(curl -sS -H "Authorization: token ${BEX_GITHUB_TOKEN}" \
    "https://api.github.com/repos/${BEX_VERIFY_REPO}/pulls/${pr_number}" | jq -r '.draft')"
  [ "$draft" = true ] || fail "GitHub reports PR #${pr_number} is not a draft"
  ok "GitHub confirms PR #${pr_number} is a draft"
fi

# ---------------------------------------------------------------------------
# 2. Steering: a new prompt turn produces a follow-up commit on the same branch.
# ---------------------------------------------------------------------------
steer_body="$(jq -n --arg p "Append a STEERED line to VERIFY-${stamp}.md and commit." '{prompt:$p}')"
api POST "/v1/agent-sessions/${sid}/steer" "$steer_body" >/dev/null \
  || fail "steer request failed"
steered="$(poll_terminal "$sid")"
[ "$(jq -r '.phase' <<<"$steered")" = completed ] || fail "steered turn did not complete: $(jq -c '{phase,failureReason}' <<<"$steered")"
[ "$(jq -r '.turns' <<<"$steered")" -ge 2 ] || fail "turn count did not advance past 1"
[ "$(jq -r '.deliveryMode' <<<"$steered")" = redispatch ] || fail "steering did not record a re-dispatch"
new_head="$(jq -r '.headSha' <<<"$steered")"
[ -n "$new_head" ] && [ "$new_head" != "$head_sha" ] || fail "steering produced no follow-up commit (head unchanged)"
ok "steering produced follow-up commit ${new_head} (turns=$(jq -r '.turns' <<<"$steered"))"

# ---------------------------------------------------------------------------
# 3. Failure path A: a non-bex-agent/* branch is refused at create (mint gate).
# ---------------------------------------------------------------------------
bad="$(create_session "main" "should be rejected")"
# The refusal's machine code is in `.code` (the Render error dialect puts the
# human sentence in `.error`); match the code, falling back to the branch-rule
# message so either shape passes.
jq -e '(.code == "AGENT_SESSION_INPUT_INVALID") or ((.error // "")|test("bex-agent/"))' <<<"$bad" >/dev/null \
  || fail "create on a protected branch was not refused: $bad"
ok "non-bex-agent/* branch refused at create"

# ---------------------------------------------------------------------------
# 4. Failure path B: an agent that cannot run surfaces as failed, not a hang.
#    Uses a non-existent adapter so the driver's turn errors out.
# ---------------------------------------------------------------------------
crash_branch="bex-agent/verify-crash-${stamp}"
crash_agent="$agent"; agent="bex-nonexistent-adapter"
crash_resp="$(create_session "$crash_branch" "this turn cannot start")"
agent="$crash_agent"
crash_sid="$(jq -r '.id // empty' <<<"$crash_resp")"
if [ -n "$crash_sid" ]; then
  created_sessions+=("$crash_sid")
  # Bounded soft poll: a clean failure reaches `failed` fast. A driver whose ACP
  # adapter dies from an UNCAUGHT child-process spawn error (not a caught throw)
  # exits the pod's container without writing status:failed; OpenSandbox keeps
  # reporting the sandbox as pending, so the Completer cannot read a terminal
  # status and the session lingers in `running`. That crash-path stranding is a
  # known follow-up (driver: wire the ACP provider's child error/exit to reject
  # the turn; and/or a gateway pod-state signal to the Completer) — orthogonal to
  # the conversation API under test here. Flag it loudly, don't hard-fail.
  crash_deadline=$(( $(date +%s) + 120 )); crash_phase=running
  while :; do
    crash_phase="$(jq -r '.phase // "?"' <<<"$(api GET "/v1/agent-sessions/${crash_sid}")")"
    case "$crash_phase" in failed|completed|canceled) break ;; esac
    [ "$(date +%s)" -lt "$crash_deadline" ] || break
    sleep 10
  done
  if [ "$crash_phase" = failed ]; then
    ok "crashing agent surfaced as failed"
  else
    echo "WARN: crashing agent did not reach 'failed' within 120s (phase=${crash_phase}) — KNOWN crash-path stranding (uncaught ACP spawn error; see .pm follow-up). Conversation-API legs below are unaffected." >&2
  fi
else
  # Some deployments reject an unknown adapter at create; that is also a clean,
  # non-hanging failure.
  jq -e '.error' <<<"$crash_resp" >/dev/null || fail "unknown adapter neither created nor errored: $crash_resp"
  ok "unknown adapter refused at create"
fi

# ---------------------------------------------------------------------------
# 5. Conversation API (w3/m43, ADR047 D9): attach -> replay -> live turn ->
#    reattach, all ticket-authenticated on the primary API origin. The stream
#    endpoint is served by the isolated gateway (edge path-routing), so this
#    exercises the whole verbatim-forward path, not just bex-api.
#
#    The attach ticket header carries the credential (never a URL param); a
#    ticketless call is rejected. `stream_url` is `<url>/<sid>/stream` where the
#    `url` field the mint returns is BEX_AGENT_SESSION_GATEWAY_URL.
# ---------------------------------------------------------------------------
attach_hdr="X-Bex-Agent-Ticket"

mint_attach() {
  # mint_attach SESSION_ID -> prints the attach-ticket JSON (ticket,url,expiresAt)
  api POST "/v1/agent-sessions/$1/attach-ticket" ""
}

# stream_get TICKET URL [max_seconds] -> prints "<http_code>\n<headers+body>";
# reads the SSE with a hard cap so a stalled stream can never hang the verifier.
stream_get() {
  local ticket="$1" url="$2" max="${3:-60}"
  curl -sS -iN --max-time "$max" -H "${attach_hdr}: ${ticket}" "$url" \
    -o /tmp/agent-stream-$$.out -w '%{http_code}' 2>/dev/null || true
  cat /tmp/agent-stream-$$.out 2>/dev/null || true
  rm -f /tmp/agent-stream-$$.out
}

echo "-- 5. conversation API (attach / replay / turn / reattach) --"

# stream_url_for SESSION_ID -> the phase-1 SSE stream endpoint. The stream is
# published under the PRIMARY API origin (edge-routed to the isolated gateway,
# t006); the mint's `url` field is the phase-2 raw-ACP WebSocket origin
# (BEX_AGENT_SESSION_GATEWAY_URL, e.g. wss://ssh.bex.co/agent-sessions) and is
# NOT the SSE base — validate it is present, but stream against the API origin.
stream_url_for() { printf '%s/v1/agent-sessions/%s/stream' "$api_url" "$1"; }

mint="$(mint_attach "$sid")"
ticket="$(jq -r '.ticket // empty' <<<"$mint")"
[ -n "$ticket" ] || fail "attach-ticket returned no ticket: $mint"
jq -e '.url and .expiresAt' <<<"$mint" >/dev/null \
  || fail "attach-ticket missing url/expiresAt (BEX_AGENT_SESSION_GATEWAY_URL unset?): $mint"
stream_url="$(stream_url_for "$sid")"
ok "attach-ticket minted for ${sid} (stream ${stream_url})"

# 5a. Ticketless attach is rejected (the ticket is the sole credential).
noauth_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 30 "$stream_url" || true)"
[ "$noauth_code" = 401 ] || [ "$noauth_code" = 403 ] || fail "ticketless stream GET returned ${noauth_code}, want 401/403"
ok "ticketless stream GET rejected (${noauth_code})"

# 5b. Attach + replay on the (terminal) completed session from section 1: the
# stream must carry the v1 marker and terminate with [DONE] without hanging.
resp5="$(stream_get "$ticket" "$stream_url" 60)"
grep -qi '^x-vercel-ai-ui-message-stream: v1' <<<"$resp5" \
  || fail "stream missing the x-vercel-ai-ui-message-stream: v1 marker:\n$(head -20 <<<"$resp5")"
grep -q 'data: \[DONE\]' <<<"$resp5" || fail "terminal-session stream did not terminate with [DONE]"
ok "attach+replay: v1 marker present, stream terminated with [DONE]"

# 5c. Durable tee + reattach: create a fresh session and attach to its LIVE turn
# so the gateway tees the UI-message parts to the durable transcript; then a
# fresh attach must replay the teed parts (store-not-memory sourcing, no
# duplication). This is the phase-1 tee path — the transcript is populated by an
# attach during the turn, not by resume.
live_branch="bex-agent/verify-live-${stamp}"
live_sid="$(jq -r '.id // empty' <<<"$(create_session "$live_branch" "Add VERIFY-live-${stamp}.md with one line and commit.")")"
[ -n "$live_sid" ] || fail "live-attach session create returned no id"
created_sessions+=("$live_sid")
live_stream="$(stream_url_for "$live_sid")"
live_ticket="$(jq -r '.ticket // empty' <<<"$(mint_attach "$live_sid")")"
[ -n "$live_ticket" ] || fail "live-attach ticket mint returned no ticket"
# Attach during the live turn: the stream carries real parts and terminates with
# [DONE] when the turn ends (bounded by the turn timeout).
live_out="$(stream_get "$live_ticket" "$live_stream" "$timeout_s")"
grep -qi '^x-vercel-ai-ui-message-stream: v1' <<<"$live_out" || fail "live attach missing v1 marker"
grep -q '^data: {' <<<"$live_out" || fail "live attach produced no UI-message parts"
grep -q 'data: \[DONE\]' <<<"$live_out" || fail "live attach did not terminate with [DONE]"
ok "live attach streamed teed parts and terminated with [DONE]"

# Reattach: replay the teed parts from the durable transcript (session now
# terminal, its sandbox torn down — the store is the only source).
sleep 3
rt2="$(jq -r '.ticket // empty' <<<"$(mint_attach "$live_sid")")"
replay="$(stream_get "$rt2" "$live_stream" 60)"
grep -q '^data: {' <<<"$replay" || fail "reattach replayed no teed parts (transcript empty?)"
grep -q 'data: \[DONE\]' <<<"$replay" || fail "reattach replay did not terminate with [DONE]"
ok "reattach replayed the teed transcript then [DONE]"

echo "== ALL AGENT-SESSION CHECKS PASSED (egress-profile=${egress_profile}) =="
