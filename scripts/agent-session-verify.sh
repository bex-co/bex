#!/usr/bin/env bash
set -euo pipefail

# Live end-to-end verification of the ADR047 cloud coding-agent session path:
#   phase-1 (w3/m41): create a session on a real disposable repo -> the agent
#     commits -> bex-api opens a draft PR with Codex-style evidence -> a steering
#     turn produces a follow-up commit -> failure paths surface as failed
#     sessions, never hangs.
#   conversation API (w3/m43, ADR047 D9): mint an attach ticket -> the stream
#     endpoint replays the transcript + terminates with [DONE] (attach+replay);
#     the fire-and-forget session's replay is NON-EMPTY (ADR051 headless recorder,
#     w3/m77 — no browser attached during its run, yet the conversation persists) ->
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
# Preconditions (out of band, ADR047 D7): BEX_VERIFY_AGENT must name the provider
# whose BYO model key is provisioned for this workspace; a Claude credential
# cannot authenticate a Codex/OpenAI verification (and vice versa). The driver
# sources the key from OpenBao, not from the request. Provision with:
#   bao kv put agent-sessions/<workspaceId>/model-key BEX_AGENT_MODEL_API_KEY=<key>
# (workspaceId is the tea-… id BEX_VERIFY_OWNER_ID resolves to.)
#
# Required:
#   BEX_VERIFY_AGENT=claude          # ACP adapter matching the provisioned key
#
# Optional:
#   BEX_VERIFY_MODEL=gpt-5           # model passed to the adapter
#   BEX_VERIFY_MODEL_ENDPOINT=https://api.openai.com/v1
#   BEX_VERIFY_OWNER_ID=<workspace id to bill/scope to>
#   BEX_VERIFY_STREAM_URL=http://localhost:62030 # local gateway; defaults to API origin
#   BEX_GITHUB_TOKEN=<token to independently confirm the PR on GitHub>
#   BEX_VERIFY_TIMEOUT=1800          # seconds to wait for a turn (default 30m)
#   BEX_VERIFY_CRASH_TIMEOUT=180      # terminal convergence bound for crash leg
#   BEX_VERIFY_EGRESS_PROFILE=<label recorded in the log for the m40 policy in effect>
#   BEX_VERIFY_KUBE_CONTEXT=<context> # additionally assert prod logs/reclamation

cd "$(dirname "$0")/.."

[ "${BEX_LIVE_VERIFY:-0}" = 1 ] || {
  echo "error: set BEX_LIVE_VERIFY=1 to authorize a live agent-session verification run" >&2
  exit 2
}
: "${BEX_API_URL:?set BEX_API_URL to the bex-api origin}"
: "${BEX_API_TOKEN:?set BEX_API_TOKEN to a caller bearer token}"
: "${BEX_VERIFY_REPO:?set BEX_VERIFY_REPO to a disposable owner/name repo}"
: "${BEX_VERIFY_AGENT:?set BEX_VERIFY_AGENT to the adapter matching the workspace model-key provider}"

for command in curl jq; do
  command -v "$command" >/dev/null || { echo "error: missing required command: $command" >&2; exit 2; }
done

api_url="${BEX_API_URL%/}"
stream_api_url="${BEX_VERIFY_STREAM_URL:-$api_url}"
stream_api_url="${stream_api_url%/}"
agent="${BEX_VERIFY_AGENT}"
model="${BEX_VERIFY_MODEL:-}"
model_endpoint="${BEX_VERIFY_MODEL_ENDPOINT:-}"
owner_id="${BEX_VERIFY_OWNER_ID:-}"
timeout_s="${BEX_VERIFY_TIMEOUT:-1800}"
egress_profile="${BEX_VERIFY_EGRESS_PROFILE:-unspecified}"
crash_timeout_s="${BEX_VERIFY_CRASH_TIMEOUT:-180}"
kube_context="${BEX_VERIFY_KUBE_CONTEXT:-}"
verify_started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
stamp="$(date -u +%Y%m%d%H%M%S)-$$"
branch="bex-agent/verify-${stamp}"

created_sessions=()
stream_output="$(mktemp)"
api_log_file="$(mktemp)"
gateway_log_file="$(mktemp)"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok()   { echo "PASS: $*"; }

cleanup() {
  for sid in "${created_sessions[@]:-}"; do
    [ -n "$sid" ] || continue
    curl -sS -X POST -H "Authorization: Bearer ${BEX_API_TOKEN}" \
      "${api_url}/v1/agent-sessions/${sid}/cancel" >/dev/null 2>&1 || true
  done
  rm -f "$stream_output" "$api_log_file" "$gateway_log_file"
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
  # config_json TASK -> the agentConfig object.
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
jq -e '.evidence and ((.evidence.commandLog|length)>0 or (.evidence.testOutput|length)>0 or (.evidence.changedFiles|length)>0 or (.evidence.outputTail|length)>0 or (.evidence.commits // 0)>0)' <<<"$final" >/dev/null \
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
if [ -z "$new_head" ] || [ "$new_head" = "$head_sha" ]; then
  fail "steering produced no follow-up commit (head unchanged)"
fi
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
# Render-compatible error bodies also carry an `id` (for example
# `id:"bad_request"`). Only the opaque agent-session kind denotes a created
# session; otherwise this leg is the clean create-time refusal handled below.
crash_sid="$(jq -r 'if ((.id // "") | startswith("ags-")) then .id else empty end' <<<"$crash_resp")"
if [ -n "$crash_sid" ]; then
  created_sessions+=("$crash_sid")
  crash_view="$(api GET "/v1/agent-sessions/${crash_sid}")"
  crash_sandbox="$(jq -r '.sandboxId // empty' <<<"$crash_view")"
  # This is a hard regression leg: ACP/setup failure, child-Pod failure, and a
  # stale BatchSandbox must all converge to one failed row within the bound.
  crash_deadline=$(( $(date +%s) + crash_timeout_s )); crash_phase=running
  while :; do
    crash_view="$(api GET "/v1/agent-sessions/${crash_sid}")"
    crash_phase="$(jq -r '.phase // "?"' <<<"$crash_view")"
    if [ -z "$crash_sandbox" ]; then
      crash_sandbox="$(jq -r '.sandboxId // empty' <<<"$crash_view")"
    fi
    case "$crash_phase" in failed|completed|canceled) break ;; esac
    [ "$(date +%s)" -lt "$crash_deadline" ] || break
    sleep 10
  done
  if [ "$crash_phase" = failed ]; then
    [ -n "$(jq -r '.failureReason // empty' <<<"$crash_view")" ] \
      || fail "crashing agent failed without a readable failureReason"
    if [ -n "$kube_context" ]; then
      [ -z "$(jq -r '.sandboxId // empty' <<<"$crash_view")" ] \
        || fail "crashing agent retained sandboxId after terminal convergence"
    fi
    ok "crashing agent surfaced as failed and converged within ${crash_timeout_s}s"
  else
    fail "crashing agent did not reach failed within ${crash_timeout_s}s (phase=${crash_phase})"
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

wait_attach() {
  # wait_attach SESSION_ID -> waits for asynchronous sandbox provisioning, then
  # prints the attach-ticket JSON. Any non-retryable mint error stays loud.
  local sid="$1" deadline=$(( $(date +%s) + timeout_s )) mint code
  while :; do
    mint="$(mint_attach "$sid")"
    if [ -n "$(jq -r '.ticket // empty' <<<"$mint")" ]; then
      printf '%s\n' "$mint"
      return 0
    fi
    code="$(jq -r '.code // empty' <<<"$mint")"
    [ "$code" = AGENT_SESSION_NOT_ATTACHABLE ] \
      || fail "attach-ticket failed for ${sid}: $(jq -c '{code,error}' <<<"$mint")"
    [ "$(date +%s)" -lt "$deadline" ] \
      || fail "session ${sid} did not become attachable within ${timeout_s}s"
    sleep 5
  done
}

# stream_get TICKET URL [max_seconds] -> prints "<http_code>\n<headers+body>";
# reads the SSE with a hard cap so a stalled stream can never hang the verifier.
stream_get() {
  local ticket="$1" url="$2" max="${3:-60}"
  curl -sS -iN --max-time "$max" -H "${attach_hdr}: ${ticket}" "$url" \
    -o "$stream_output" -w '%{http_code}' 2>/dev/null || true
  cat "$stream_output" 2>/dev/null || true
  : > "$stream_output"
}

echo "-- 5. conversation API (attach / replay / turn / reattach) --"

# stream_url_for SESSION_ID -> the phase-1 SSE stream endpoint. In production
# this is the primary API origin (edge-routed to the isolated gateway). Local
# dev has no edge route, so BEX_VERIFY_STREAM_URL points at the :8083 forward.
# The mint's `url` field is the phase-2 raw-ACP WebSocket origin
# (BEX_AGENT_SESSION_GATEWAY_URL, e.g. wss://ssh.bex.co/agent-sessions) and is
# NOT the SSE base — validate it is present, but stream against the configured
# production or local SSE origin.
stream_url_for() { printf '%s/v1/agent-sessions/%s/stream' "$stream_api_url" "$1"; }

mint="$(mint_attach "$sid")"
ticket="$(jq -r '.ticket // empty' <<<"$mint")"
[ -n "$ticket" ] || fail "attach-ticket returned no ticket: $mint"
jq -e '.url and .expiresAt' <<<"$mint" >/dev/null \
  || fail "attach-ticket missing url/expiresAt (BEX_AGENT_SESSION_GATEWAY_URL unset?): $mint"
stream_url="$(stream_url_for "$sid")"
# w10/m9 t003 (w3/013): the mint now also returns streamUrl — the same address
# this script derives independently. Present ⇒ it must agree (catches drift
# between BEX_API_PUBLIC_URL and this API origin); absent is fine (the backend
# has no BEX_API_PUBLIC_URL configured, same as before this field existed).
minted_stream_url="$(jq -r '.streamUrl // empty' <<<"$mint")"
if [ -n "$minted_stream_url" ] && [ "$minted_stream_url" != "$stream_url" ]; then
  fail "mint streamUrl (${minted_stream_url}) disagrees with the derived stream endpoint (${stream_url})"
fi
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

# 5b'. Headless recorder (ADR051): section 1's session ran fire-and-forget — NO
# browser attached during its turn — yet its replay must be NON-EMPTY, because
# the Completer recorded the driver's conversation into the durable transcript
# before teardown. This is the exact regression that showed "No conversation
# yet." for every completed session; pre-ADR051 this replay was empty and 5b
# still passed (marker + [DONE] only). Assert real parts here.
grep -q '^data: {' <<<"$resp5" \
  || fail "fire-and-forget session replayed NO parts — headless recorder did not persist the transcript (ADR051):\n$(head -20 <<<"$resp5")"
ok "headless recorder: fire-and-forget transcript is non-empty on replay (ADR051)"

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
live_ticket="$(jq -r '.ticket // empty' <<<"$(wait_attach "$live_sid")")"
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

# 5d. Optional cluster-backed assertions. Public API tests prove the installed
# role can execute AgentSessionTurns; these log checks additionally prove the
# forced crash did not fall back to repeated dead-container exec and that no
# replay permission denial occurred during this verifier window.
if [ -n "$kube_context" ]; then
  command -v kubectl >/dev/null || fail "BEX_VERIFY_KUBE_CONTEXT requires kubectl"
  kubectl --context "$kube_context" -n bex-system logs \
    -l app.kubernetes.io/name=bex-api --all-containers=true --prefix --since-time="$verify_started" \
    >"$api_log_file" 2>&1 || fail "could not read bex-api logs for verification"
  kubectl --context "$kube_context" -n bex-system logs \
    -l app.kubernetes.io/name=bex-ssh-gateway --all-containers=true --prefix --since-time="$verify_started" \
    >"$gateway_log_file" 2>&1 || fail "could not read gateway logs for verification"
  ! grep -Fq 'SQLSTATE 42501' "$gateway_log_file" \
    || fail "gateway emitted SQLSTATE 42501 during authenticated replay"
  if grep -F "$crash_sid" "$api_log_file" | grep -Fq 'read status failed'; then
    fail "Completer retried the forced-crash status read after terminal signaling"
  fi
  if [ -n "${crash_sandbox:-}" ]; then
    if grep -F "$crash_sandbox" "$gateway_log_file" | grep -Fq 'container not found'; then
      fail "gateway polled a nonexistent crash sandbox container"
    fi
  fi
  ok "cluster logs show no replay denial or dead-container retry storm"
fi

echo "== ALL AGENT-SESSION CHECKS PASSED (egress-profile=${egress_profile}) =="
