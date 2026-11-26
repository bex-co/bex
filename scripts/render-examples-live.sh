#!/usr/bin/env bash
# Deploy one of Render's own render-examples Blueprints unchanged into a bex
# workspace, check it the way its README or Render guide says, print one
# result line, and delete everything the run created (w6/m644).
#
#   BEX_API_URL=https://api.bex.co BEX_API_TOKEN=<API key> \
#     bash scripts/render-examples-live.sh [--keep] [--timeout MINUTES] <repo>
#
# <repo> is a github.com/render-examples repository name (or its URL). The
# Blueprint is created from the repository, as Render's Deploy to Render button
# does (POST /v1/blueprints with source.repo, then approve the planned sync), and
# named render-examples-live-<repo>. BEX_OWNER_ID picks the workspace when the
# key can reach more than one. Results and the template list live in
# docs/render-artifacts/render-examples-live.md.
#
# Output is one line, `<repo> <outcome> <stage> <detail>`:
#   outcome  live | failed | timeout
#   stage    validate | plan | apply | build | pre-deploy | deploy | provision |
#            check (failed or timed out there), or - when live
#   detail   the first error line, passed through scripts/lib/public_log.py so
#            ids and onbex.co hostnames are hashed
# then cleanup lines. Exit 0 when live, 1 when failed or timed out, 2 for usage
# or credential problems, 3 when cleanup left a resource behind.
#
# The plan must create every resource it names: a plan that would adopt or
# change an existing resource is refused before approval, so the run can never
# delete something it did not create. Cleanup runs on every exit (trap) and
# deletes the Blueprint's resources, any project it created, and the Blueprint,
# then confirms each is gone. --keep leaves a failed run's resources up for
# triage and prints the ids to delete by hand. Nothing is written but stdout.
set -euo pipefail

usage() {
  sed -n '2,/^set -euo pipefail/{ /^set -euo pipefail/d; s/^# \{0,1\}//; p; }' "$0" >&2
  exit 2
}

keep=0
timeout_min="${RENDER_EXAMPLES_LIVE_TIMEOUT_MIN:-60}"
repo=""
while [ $# -gt 0 ]; do
  case "$1" in
    --keep) keep=1 ;;
    --timeout)
      [ $# -ge 2 ] && [[ "$2" =~ ^[1-9][0-9]*$ ]] || usage
      timeout_min="$2"
      shift
      ;;
    -h | --help) usage ;;
    -*) usage ;;
    *)
      [ -z "$repo" ] || usage
      repo="$1"
      ;;
  esac
  shift
done
[ -n "$repo" ] || usage
repo="${repo%.git}"
repo="${repo%/}"
repo="${repo##*/}"
[[ "$repo" =~ ^[A-Za-z0-9._-]+$ ]] || usage
: "${BEX_API_URL:?set BEX_API_URL to the bex-api base URL, e.g. https://api.bex.co}"
: "${BEX_API_TOKEN:?set BEX_API_TOKEN to an API key for the workspace}"

api_url="${BEX_API_URL%/}"
poll="${RENDER_EXAMPLES_LIVE_POLL_SECONDS:-15}"
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
name="render-examples-live-$repo"

# --- API helpers -----------------------------------------------------------
# api METHOD PATH [BODY]: the answer's body goes to $tmp/body and its HTTP code
# to $status (000 when the request failed). The key reaches curl on stdin, never
# in argv.
status=000
api() {
  local args=(--silent --show-error --max-time 60 -X "$1" -o "$tmp/body" -w '%{http_code}' -K -)
  [ $# -lt 3 ] || args+=(-H 'Content-Type: application/json' --data "$3")
  : >"$tmp/body"
  status="$(printf 'header = "Authorization: Bearer %s"\nheader = "User-Agent: render-examples-live"\n' "$BEX_API_TOKEN" |
    curl "${args[@]}" "$api_url$2" 2>"$tmp/curl.err")" || status=000
}
body() { jq -r "$@" "$tmp/body" 2>/dev/null; }
# error_text: the API's message, or the start of the body.
error_text() { body '.message // .error // empty' | head -n1 | grep . || head -c 200 "$tmp/body"; }
q() { jq -rn --arg v "$1" '$v|@uri'; }

redact() {
  # One line, no ANSI escapes, ids and tenant hostnames hashed.
  local line
  line="$(printf '%s' "$1" | sed $'s/\x1b\\[[0-9;]*[A-Za-z]//g' | tr '\r\n\t' '   ' | sed 's/  */ /g; s/^ //; s/ $//' | cut -c1-240)"
  if [ -f "$here/lib/public_log.py" ]; then
    printf '%s\n' "$line" | python3 "$here/lib/public_log.py" 2>/dev/null || printf '%s\n' "$line"
  else
    printf '%s\n' "$line"
  fi
}

outcome=""
result() {
  # result OUTCOME STAGE DETAIL: print the one result line, once.
  [ -z "$outcome" ] || return 0
  outcome="$1"
  printf '%s %s %s %s\n' "$repo" "$1" "$2" "$(redact "${3:--}")"
}
fail() { result failed "$@"; exit 1; }

now() { date +%s; }
# RENDER_EXAMPLES_LIVE_TIMEOUT_SECONDS overrides --timeout (the self-test uses it).
timeout_s="${RENDER_EXAMPLES_LIVE_TIMEOUT_SECONDS:-$((timeout_min * 60))}"
deadline=$(($(now) + timeout_s))

# --- cleanup ---------------------------------------------------------------
bp_id=""
peer_id=""      # the peer worker this run created for its peer checks
resources=()    # "<id> <type> <name>" lines from the Blueprint
projects_before=""
projects=()     # project ids this run created

refresh_resources() {
  [ -n "$bp_id" ] || return 0
  api GET "/v1/blueprints/$bp_id?ownerId=$owner"
  [ "$status" = 200 ] || return 0
  local line
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    case " ${resources[*]-} " in *" ${line%% *} "*) continue ;; esac
    resources+=("$line")
  done < <(body '.resources[]? | "\(.id) \(.type) \(.name)"')
}

resource_path() {
  case "$1" in
    srv-* | crn-*) echo "/v1/services/$1" ;;
    dpg-*) echo "/v1/postgres/$1" ;;
    red-*) echo "/v1/key-value/$1" ;;
    evg-*) echo "/v1/env-groups/$1" ;;
    prj-*) echo "/v1/projects/$1" ;;
    *) return 1 ;;
  esac
}

cleanup() {
  local rc=$? id path left=() line deadline_gone
  trap - EXIT
  set +e
  if [ "$outcome" = usage ]; then
    rm -rf "$tmp"
    exit 2
  fi
  [ -n "$outcome" ] || result failed "${stage:-validate}" "the run stopped before a result (exit $rc)"
  refresh_resources
  local ids=()
  for line in "${resources[@]-}"; do [ -n "$line" ] && ids+=("${line%% *}"); done
  [ -z "$peer_id" ] || ids+=("$peer_id")
  if [ -n "$bp_id" ] && [ "$keep" = 1 ] && [ "$outcome" != live ]; then
    echo "cleanup: kept for triage, delete by hand: Blueprint $bp_id ${ids[*]-} ${projects[*]-}"
    rm -rf "$tmp"
    exit 1
  fi
  for id in "${ids[@]-}"; do
    [ -n "$id" ] || continue
    path="$(resource_path "$id")" || continue
    api DELETE "$path"
  done
  if [ -n "$bp_id" ]; then
    api DELETE "/v1/blueprints/$bp_id?ownerId=$owner"
  fi
  # A project can be deleted once its resources are gone.
  for id in "${projects[@]-}"; do [ -n "$id" ] && api DELETE "/v1/projects/$id"; done
  deadline_gone=$(($(now) + ${RENDER_EXAMPLES_LIVE_CLEANUP_SECONDS:-600}))
  for id in "${ids[@]-}" "${projects[@]-}"; do
    [ -n "$id" ] || continue
    path="$(resource_path "$id")" || continue
    while :; do
      api GET "$path"
      [ "$status" = 404 ] && break
      if [ "$(now)" -ge "$deadline_gone" ]; then
        left+=("$id")
        break
      fi
      # A project refuses deletion while it still holds a resource; ask again.
      case "$id" in prj-*) api DELETE "$path" ;; esac
      sleep "$poll"
    done
  done
  if [ -n "$bp_id" ]; then
    api GET "/v1/blueprints/$bp_id?ownerId=$owner"
    [ "$status" = 404 ] || left+=("$bp_id")
  fi
  rm -rf "$tmp"
  if [ ${#left[@]} -gt 0 ]; then
    echo "cleanup: LEFT ${left[*]} — delete by hand"
    exit 3
  fi
  [ -z "$bp_id" ] || echo "cleanup: ok, deleted the Blueprint and ${#ids[@]} resource(s)${projects[0]:+ and ${#projects[@]} project(s)}"
  [ "$outcome" = live ] && exit 0
  exit 1
}
trap cleanup EXIT
trap 'exit 1' INT TERM

# --- the workspace ---------------------------------------------------------
owner="${BEX_OWNER_ID:-}"
if [ -z "$owner" ]; then
  api GET "/v1/owners?limit=100"
  if [ "$status" != 200 ]; then
    outcome=usage
    echo "error: GET /v1/owners answered $status: $(error_text)" >&2
    exit 2
  fi
  if [ "$(body 'length')" != 1 ]; then
    outcome=usage
    echo "error: the key reaches $(body 'length') workspaces; set BEX_OWNER_ID" >&2
    exit 2
  fi
  owner="$(body '.[0].owner.id')"
fi

# --- checks ----------------------------------------------------------------
# What each template's README or Render guide says to look for, one per line:
#   http <service> <path> [text]   GET https://<service URL><path> answers 2xx
#                                  and its body contains text
#   post <service> <path> <json> <text>
#                                  the same for a POST with a JSON body
#   log <service> <text>           the service's app log shows text
#   cron <service>                 a triggered run ends successful
#   peer <service> [<protocol>:]<port>[/<path>] [text]
#                                  a peer in the same workspace connects to the
#                                  private service's internal host on port, and
#                                  its answer contains text (any answer when
#                                  there is none). The peer is one background
#                                  worker for the run's peer checks,
#                                  render-examples-live-peer-<repo> (busybox),
#                                  deleted with the rest. It sends, by
#                                  protocol: mysql nothing (MySQL greets
#                                  first), postgres an SSLRequest, redis PING,
#                                  mongodb isMaster, http GET <path> HTTP/1.0,
#                                  and logs the answer's printable bytes. The
#                                  protocol defaults by port: 3306 mysql, 5432
#                                  postgres, 6379 redis, 27017 mongodb, any
#                                  other http
#   addr <service> <port>          the private service's internal address
#                                  names the port its first deploy detected:
#                                  only for a port whose protocol the peer
#                                  cannot open in one command
# A template not listed gets the default: every web service answers 2xx on /.
# RENDER_EXAMPLES_LIVE_CHECKS (lines in the same form) replaces the table's
# entry, to try a template with checks of your own.
checks_for() {
  case "$1" in
    express-hello-world) echo 'http express-hello-world / Hello from Render!' ;;
    fastapi) echo 'http fastapi-example / {"message":"Hello World"}' ;;
    go-web-server) echo 'http go / Hello from Render!' ;;
    django) echo 'http mysite / Hello Django on Render!' ;;
    rails-7) echo 'http myrailssite / Hello Rails on Render!' ;;
    sidekiq) printf '%s\n' 'http sinatra-web / Sinatra + Sidekiq Example' 'log sidekiq-worker Starting processing' ;;
    celery) printf '%s\n' 'http app / Asynchronous addition using Celery!' 'http flower / Flower' 'log celery-worker ready.' ;;
    ioredis) echo 'log ioredis-worker Result for key animal: cat' ;;
    webhook-playground) printf '%s\n' 'http webhook-playground / Webhook Playground' 'cron webhook-playground-cleanup' ;;
    clickhouse) echo 'peer clickhouse 8123 Ok.' ;;
    pg_pinecone_render) echo 'peer pg-pinecone 5432' ;;
    n8n) echo 'http n8n-service /healthz ok' ;;
    umami) printf '%s\n' 'http umami /api/heartbeat' 'post umami /api/auth/login {"username":"admin","password":"umami"} token' ;;
    bun-docker) echo 'http bun / Welcome to Bun!' ;;
    strapi-sqlite) echo 'http strapi /_health' ;;
    redwoodjs) printf '%s\n' 'http redwoodjs-web /' 'http redwoodjs-api /graphql/health' ;;
    vuepress) echo 'http vuepress-example / Hello Vuepress' ;;
    postgrest) echo 'http postgrest / swagger' ;;
    minio) printf '%s\n' 'http minio-server /minio/health/live' 'http minio-console / MinIO' ;;
    wordpress) echo 'http wordpress /wp-admin/install.php WordPress' ;;
    ackee) printf '%s\n' 'http ackee / <title>Ackee</title>' 'peer ackee-mongodb 27017 ismaster' ;;
    adminer) echo 'http adminer / Login - Adminer' ;;
    astro-ssr) printf '%s\n' 'http astro-ssr / It works.' 'http astro-ssr /api/hello?name=Astro Hello, Astro!' ;;
    blitzjs) echo 'http blitzapp / Your app is ready, including user sign-up and log-in.' ;;
    crawl4ai-url-to-markdown)
      printf '%s\n' 'http url-to-markdown /health "status":"healthy"' 'http url-to-markdown / <title>URL to Markdown</title>' \
        'post url-to-markdown /crawl {"url":"https://example.com"} Example Domain'
      ;;
    elasticsearch) echo 'peer elasticsearch 9200 You Know, for Search' ;;
    elysiajs-hello-world) echo 'http elysiajs-hello-world / Hello Elysia' ;;
    fastapi-uv) printf '%s\n' 'http fastapi-uv / {"message":"Hello World"}' 'http fastapi-uv /?name=Render {"message":"Hello Render"}' ;;
    fastify-hello-world) echo 'http fastify-hello-world-example / Hello from Render using Fastify!' ;;
    ghost) printf '%s\n' 'http ghost / <meta name="generator" content="Ghost 5.14"' 'peer mysql 3306 8.0.24' ;;
    gitpod-openvscode-server) echo 'http gitpod-openvscode-server-example / vscode-workbench-web-configuration' ;;
    hasura-graphql-metadata-db-only) printf '%s\n' 'http hasura /healthz OK' 'http hasura /v1/version v2.0.2' ;;
    hasura-graphql) echo 'http hasura /healthz OK' ;;
    langfuse4-template)
      printf '%s\n' 'http langfuse /api/public/health "status":"OK"' 'http langfuse-minio /minio/health/live' \
        'log langfuse-worker Listening: http://' 'peer langfuse-clickhouse 8123/ping Ok.'
      ;;
    matomo) printf '%s\n' 'http matomo / Matomo is libre software' 'peer mysql-matomo 3306 mysql_native_password' ;;
    mattermost) printf '%s\n' 'http mattermost /api/v4/system/ping "status":"OK"' 'peer mattermost-minio 9000/minio/health/live HTTP/1.1 200 OK' ;;
    mcp-server-python | mcp-server-typescript) echo 'http my-mcp-server /health {"status":"ok"}' ;;
    metabase) echo 'http metabase /api/health {"status":"ok"}' ;;
    mongodb) echo 'peer mongodb 27017 maxWireVersion' ;;
    multi-environments-blueprint)
      printf '%s\n' 'http portal-web-env-demo / "environment":"production"' 'http portal-web-env-demo-staging / "environment":"staging"' \
        'cron nightly-jobs' 'log nightly-jobs Nightly tasks completed successfully'
      ;;
    mysql) echo 'peer mysql 3306 mysql_native_password' ;;
    nextjs-hello-world) printf '%s\n' 'http next-js / Hello World.' 'http next-js /about About' ;;
    nuxtjs-hello-world) echo 'http nuxt-js / Welcome!' ;;
    paradedb) echo 'peer paradedb 5432 N' ;;
    patient-api) echo 'http patient-api / {"status":"ok"}' ;;
    pocketbase) printf '%s\n' 'http pocketbase /api/health API is healthy.' 'log pocketbase /_/#/pbinstall/' ;;
    # postgresql-missing-trigger-index declares only a database: available is its check.
    prometheus)
      printf '%s\n' 'http prometheus /graph Prometheus Time Series Collection and Processing Server' \
        'http prometheus /-/healthy Prometheus Server is Healthy.' \
        'http prometheus /api/v1/query?query=prometheus_http_requests_total "job":"prometheus"'
      ;;
    python-memray-profiling) echo 'http render-memray-example / "service":"render-memray-example"' ;;
    python-poetry) echo 'http flask-poetry / Hello, World!' ;;
    questdb) printf '%s\n' 'http questdb /index.html QuestDB · Console' 'http questdb /exec?query=SELECT%201 "dataset":[' ;;
    rabbitmq) echo 'http rabbitmq / RabbitMQ Management' ;;
    rails-6) echo 'http myrailssite / Hello Rails on Render!' ;;
    redash)
      # redash/redash:latest is still Redash 8, whose entrypoint starts Celery.
      printf '%s\n' 'http redash /ping PONG.' 'log redash-scheduler Starting scheduler and 1 workers for queues: celery' \
        'log redash-worker Starting 2 workers for queues: queries,scheduled_queries,schemas' 'peer redash-redis redis:10000 PONG'
      ;;
    redis) echo 'peer redis redis:10000 PONG' ;;
    remix-postgres) printf '%s\n' 'http remix /healthcheck OK' 'http remix / Indie Stack' ;;
    render-tanstack-demo) printf '%s\n' 'http tanstack-playground / TanStack Playground' 'http tanstack-playground /deep/link TanStack Playground' ;;
    retool) echo 'http retool /api/checkHealth "version"' ;;
    shynet) echo 'http shynet /healthz/?format=json "DatabaseBackend": "working"' ;;
    strapi-postgres) printf '%s\n' 'http strapi / The server is running successfully.' 'http strapi /_health' ;;
    svelte) printf '%s\n' 'http svelte-static / Svelte app' 'http svelte-static /build/bundle.js' ;;
    sveltekit-static) printf '%s\n' 'http sveltekit-static / id="svelte"' 'http sveltekit-static /about' ;;
    sveltekit) printf '%s\n' 'http sveltekit / SvelteKit app' 'http sveltekit /sverdle Sverdle' ;;
    tanstack-start)
      printf '%s\n' 'http tanstack-start /health {"status":"ok"}' 'http tanstack-start / TanStack Start on Render' \
        'http tanstack-start /about How it works'
      ;;
    temporal)
      # The frontend speaks gRPC, which the peer cannot open in one command.
      # shellcheck disable=SC2016 # the log line's dollar sign is literal
      printf '%s\n' 'http app-workflow-trigger / OK!' 'peer temporal-ui 8088 200 OK' \
        'peer temporal-elasticsearch 9200 missing authentication credentials' 'addr temporal-frontend 7233' \
        'http app-workflow-trigger /trigger-workflow is processing.' 'log app-worker Depositing $54.990002 into account 002-002'
      ;;
    vibe-code-demo-apps)
      printf '%s\n' 'http vibe-anurag-marin-silent-retreat-web / Stillwood' \
        'http vibe-anurag-peninsula-interiors-web / Coastal-Modern Interiors' \
        'http vibe-demo-user-aussie-shepherd-guide-web / Australian Shepherd Guide' \
        'http vibe-demo-user-bark-brigade-web / Bark Brigade Volunteer K9 Fire Corps' \
        'http vibe-demo-user-el-fuego-tacos-web / El Fuego Tacos' \
        'http vibe-demo-user-gloomy-tails-web / Gloomy Tails Wellness Center' \
        'http vibe-demo-user-ironridge-meatpacking-web / Ironridge Meat Packing Co.' \
        'http vibe-demo-user-ironvale-meats-web / Ironvale Meats' \
        'http vibe-demo-user-masa-fuego-tacos-web / Masa Fuego' \
        'http vibe-demo-user-mendocino-silent-yoga-web / Still Grove' \
        'http vibe-demo-user-synthwave-collective-web / Synthwave Collective'
      ;;
    wikijs) echo 'http wikijs / Wiki.js Setup' ;;
    wrapped-for-linkedin)
      printf '%s\n' 'http wrappedforlinkedin / Wrapped for LinkedIn' 'http wrappedforlinkedin /deep/link Wrapped for LinkedIn' \
        'http wrapped-for-linkedin-screenshots /healthz ok'
      ;;
  esac
}

# --- 1. create the Blueprint: validate and plan ----------------------------
stage=validate
create_body="$(jq -nc --arg name "$name" --arg owner "$owner" --arg repo "https://github.com/render-examples/$repo" \
  '{name: $name, ownerId: $owner, source: {repo: $repo}, autoSync: false}')"
api GET "/v1/projects?ownerId=$owner&limit=100"
projects_before="$(body '.[]?.project.id' | tr '\n' ' ')"
api POST /v1/blueprints "$create_body"
case "$status" in
  200 | 201) ;;
  *) fail validate "POST /v1/blueprints answered $status: $(error_text)" ;;
esac
bp_id="$(body '.blueprint.id // empty')"
sync_id="$(body '.sync.id // empty')"
stage=plan
if [ "$(body '.planningOutcome')" != planned ] || [ -z "$sync_id" ]; then
  fail plan "$(body '[.notices[]? | select(.severity == "error") | .message][0] // "planning outcome \(.planningOutcome)"')"
fi
not_create="$(body '[.sync.rawPlan.actions[]? | select(.operation != "create") | "\(.operation) \(.kind) \(.name)"] | join(", ")')"
[ -z "$not_create" ] || fail plan "the plan would change existing resources, so it was not approved: $not_create"
manual="$(body '[.sync.requiredManualEnvVars[]? | .key] | join(", ")')"
[ -z "$manual" ] || fail plan "the plan asks for values (sync: false): $manual"

# --- 2. approve the plan ---------------------------------------------------
stage=apply
api POST "/v1/blueprints/$bp_id/approve?ownerId=$owner" "$(jq -nc --arg s "$sync_id" '{syncId: $s}')"
apply_status="$status"
apply_error="$(error_text)"
apply_state="$(body '.state // empty')"
refresh_resources
api GET "/v1/projects?ownerId=$owner&limit=100"
for id in $(body '.[]?.project.id'); do
  case " $projects_before " in *" $id "*) continue ;; esac
  # A new project is this run's only when it holds one of the run's resources.
  api GET "/v1/environments?projectId=$id&limit=100"
  held="$(body '[.[]?.environment | (.serviceIds // []) + (.databasesIds // []) + (.redisIds // []) + (.envGroupIds // [])] | flatten | join(" ")')"
  for line in "${resources[@]-}"; do
    case " $held " in *" ${line%% *} "*) projects+=("$id") && break ;; esac
  done
done
if [ "$apply_status" != 200 ] || [ "$apply_state" != success ]; then
  fail apply "approve answered $apply_status${apply_state:+ ($apply_state)}: $apply_error"
fi
[ ${#resources[@]} -gt 0 ] || fail apply "the approved sync created no resources"

# --- 3. wait for every first deploy ----------------------------------------
first_error_line() {
  # first_error_line ID TYPE: the first line of the resource's log that reads
  # like an error, else the failure reason of its last deploy event, else the
  # log's last line.
  local line last
  api GET "/v1/logs?ownerId=$owner&resource=$1&type=$2&limit=200&direction=backward"
  line="$(body '[.logs[]? | .message] | reverse | map(select(test("error|err!|failed|fatal|panic|exception|not found|exited"; "i")))[0] // empty')"
  last="$(body '.logs[0]?.message // empty')"
  if [ -z "$line" ]; then
    api GET "/v1/services/$1/events?limit=20"
    line="$(body '[.[]?.event.details | (.failureReason // .reason) | strings][0] // empty')"
  fi
  printf '%s\n' "${line:-$last}"
}

stage=deploy
finished=" "
pending=1
while [ "$pending" = 1 ]; do
  pending=0
  for line in "${resources[@]}"; do
    read -r id type rname <<<"$line"
    case "$finished" in *" $id "*) continue ;; esac
    case "$id" in
      srv-* | crn-*)
        api GET "/v1/services/$id/deploys?limit=1"
        st="$(body '.[0].deploy.status // empty')"
        case "$st" in
          live) finished+="$id " ;;
          build_failed) fail build "$rname: $(first_error_line "$id" build)" ;;
          pre_deploy_failed) fail pre-deploy "$rname: $(first_error_line "$id" app)" ;;
          update_failed | canceled | deactivated) fail deploy "$rname ($st): $(first_error_line "$id" app)" ;;
          *) pending=1 stage_hint="$rname ${st:-no deploy yet}" ;;
        esac
        ;;
      dpg-* | red-*)
        api GET "$(resource_path "$id")"
        st="$(body '.status // empty')"
        if [ "$st" = available ]; then finished+="$id "; else pending=1 stage_hint="$rname ${st:-unknown}"; fi
        ;;
      *) finished+="$id " ;;
    esac
  done
  if [ "$pending" = 1 ]; then
    if [ "$(now)" -ge "$deadline" ]; then
      case "$stage_hint" in
        *build*) stage=build ;;
        *pre_deploy*) stage=pre-deploy ;;
        *creating* | *unknown*) stage=provision ;;
        *) stage=deploy ;;
      esac
      result timeout "$stage" "after ${timeout_s}s: $stage_hint"
      exit 1
    fi
    sleep "$poll"
  fi
done

# --- 4. run the template's checks ------------------------------------------
stage=check
service_id() {
  # The service of that name: a database may share it (rails-7's myrailssite).
  # The Blueprint lists a name longer than 30 characters by its shortened
  # resource slug (w6/190), so the service's own name is asked for too.
  local line id type rname
  for line in "${resources[@]}"; do
    read -r id type rname <<<"$line"
    case "$id" in srv-* | crn-*) [ "$rname" = "$1" ] && echo "$id" && return 0 ;; esac
  done
  for line in "${resources[@]}"; do
    id="${line%% *}"
    case "$id" in srv-* | crn-*) ;; *) continue ;; esac
    api GET "/v1/services/$id"
    [ "$(body '.name // empty')" = "$1" ] && echo "$id" && return 0
  done
  return 1
}

# retry SECONDS CMD...: run CMD until it succeeds or SECONDS pass. Fresh
# deploys can take a moment to answer through the edge.
retry() {
  local until=$(($(now) + $1))
  shift
  while ! "$@"; do
    [ "$(now)" -lt "$until" ] || return 1
    sleep "$poll"
  done
}

check_detail=""
http_check() {
  # http_check METHOD URL BODY TEXT
  local code args=(--silent --show-error --max-time 30 -X "$1" -o "$tmp/check" -w '%{http_code}')
  [ -z "$3" ] || args+=(-H 'Content-Type: application/json' --data "$3")
  code="$(curl "${args[@]}" "$2" 2>"$tmp/curl.err")" || code=000
  if [[ "$code" != 2* ]]; then
    check_detail="$1 $2 answered $code: $(head -c 160 "$tmp/check" 2>/dev/null)$(cat "$tmp/curl.err")"
    return 1
  fi
  if [ -n "$4" ] && ! grep -qF -- "$4" "$tmp/check"; then
    check_detail="$1 $2 answered $code without \"$4\""
    return 1
  fi
}

log_check() {
  # log_check ID TEXT
  api GET "/v1/logs?ownerId=$owner&resource=$1&type=app&text=$(q "$2")&limit=20&direction=backward&startTime=$(q "$started")"
  # shellcheck disable=SC2016 # $t is jq's variable
  [ "$status" = 200 ] && body --arg t "$2" '[.logs[]? | .message | select(contains($t))] | length > 0' | grep -qx true && return 0
  check_detail="the app log never showed \"$2\""
  return 1
}

peer_target() {
  # peer_target [PROTOCOL:]PORT[/PATH]: sets peer_proto, peer_port and
  # peer_path, or fails when the peer's shell could misread them.
  local target="$1"
  peer_proto=""
  case "$target" in *:*) peer_proto="${target%%:*}" target="${target#*:}" ;; esac
  peer_port="${target%%/*}"
  peer_path="${target#"$peer_port"}"
  if [ -z "$peer_proto" ]; then
    case "$peer_port" in
      3306) peer_proto=mysql ;;
      5432) peer_proto=postgres ;;
      6379) peer_proto=redis ;;
      27017) peer_proto=mongodb ;;
      *) peer_proto=http ;;
    esac
  fi
  [[ "$peer_proto" =~ ^(http|mysql|postgres|redis|mongodb)$ && "$peer_port" =~ ^[0-9]+$ &&
    "$peer_path" =~ ^(/[A-Za-z0-9._~/?=\&-]*)?$ ]]
}

peer_call() {
  # peer_call HOST: the peer script's probe of HOST at peer_target's target.
  # The probe is a printf format, with octal escapes for binary protocols.
  local probe
  case "$peer_proto" in
    mysql) probe='' ;;
    postgres) probe='\000\000\000\010\004\322\026\057' ;;
    redis) probe='PING\r\n' ;;
    mongodb) probe='\072\000\000\000\001\000\000\000\000\000\000\000\324\007\000\000\000\000\000\000admin.\044cmd\000\000\000\000\000\377\377\377\377\023\000\000\000\020isMaster\000\001\000\000\000\000' ;;
    http) probe="GET ${peer_path:-/} HTTP/1.0\\r\\nHost: $1\\r\\n\\r\\n" ;;
  esac
  printf 'p %s %s "%s" "%s"; ' "$1" "$peer_port" "$probe" "$peer_path"
}

peer_check() {
  # peer_check TARGET TEXT: the peer logged an answer from TARGET (host:port
  # and path) that contains TEXT.
  api GET "/v1/logs?ownerId=$owner&resource=$peer_id&type=app&text=$(q "peer $1 ")&limit=20&direction=backward&startTime=$(q "$started")"
  # shellcheck disable=SC2016 # $m and $t are jq's variables
  [ "$status" = 200 ] && body --arg m "peer $1 answered: " --arg t "$2" \
    '[.logs[]? | .message | select(startswith($m) and contains($t))] | length > 0' | grep -qx true && return 0
  check_detail="the peer never read ${2:+\"$2\" in }an answer from $1; its last line: $(body '.logs[0]?.message // "none"' | head -c 160)"
  return 1
}

run_until_done() {
  # run_until_done GET-PATH OK-STATUS: poll a cron run to a terminal state.
  local st
  while :; do
    api GET "$1"
    st="$(body '.status // empty')"
    case "$st" in
      "$2") return 0 ;;
      unsuccessful | failed | canceled)
        check_detail="$1 ended $st"
        return 1
        ;;
    esac
    if [ "$(now)" -ge "$deadline" ]; then
      check_detail="$1 still ${st:-unknown} at the deadline"
      return 1
    fi
    sleep "$poll"
  done
}

started="$(date -u +%Y-%m-%dT%H:%M:%SZ -d "@$(($(now) - 3600))" 2>/dev/null || date -u -r "$(($(now) - 3600))" +%Y-%m-%dT%H:%M:%SZ)"
# A fresh deploy can take a few minutes to answer through the edge or to log.
check_window="${RENDER_EXAMPLES_LIVE_CHECK_SECONDS:-300}"
checks="${RENDER_EXAMPLES_LIVE_CHECKS:-$(checks_for "$repo")}"
if [ -z "$checks" ]; then
  for line in "${resources[@]}"; do
    read -r id type rname <<<"$line"
    [ "$type" = web_service ] && checks+="http $rname /"$'\n'
  done
fi

# One peer probes every private service the checks name, every 10 seconds, and
# logs each answer as `peer <host>:<port><path> answered: <printable bytes>`.
# It joins the first one's environment, which network isolation may close to
# every service outside it, as a user would place a client there.
peer_calls=""
peer_env=""
while IFS= read -r check; do
  read -r kind svc rest <<<"$check"
  [ "$kind" = peer ] || continue
  id="$(service_id "$svc")" || fail check "the Blueprint created no resource named $svc"
  api GET "/v1/services/$id"
  url="$(body '.serviceDetails.url // empty')"
  read -r target _ <<<"$rest"
  if ! peer_target "$target" || ! [[ "$url" =~ ^[a-z0-9.-]+:[0-9]+$ ]]; then
    fail check "$svc: cannot probe ${url:-an empty internal address} on $target"
  fi
  peer_calls+="$(peer_call "${url%:*}")"
  [ -n "$peer_env" ] || peer_env="$(body '.environmentId // empty')"
done <<<"$checks"
if [ -n "$peer_calls" ]; then
  # shellcheck disable=SC2016 # the peer's shell expands these
  peer_cmd='sh -c '\''trap "exit 0" TERM; p() { a="$(printf "$3" | nc -w 10 "$1" "$2" | tr -c " -~" " " | head -c 2000)"; if [ -n "$a" ]; then echo "peer $1:$2$4 answered: $a"; else echo "peer $1:$2$4 got no answer"; fi; }; while :; do '"$peer_calls"'sleep 10; done'\'
  api POST /v1/services "$(jq -nc --arg name "render-examples-live-peer-$repo" --arg owner "$owner" --arg cmd "$peer_cmd" \
    --arg image "${RENDER_EXAMPLES_LIVE_PEER_IMAGE:-docker.io/library/busybox:1.37}" --arg env "$peer_env" \
    '{type: "background_worker", name: $name, ownerId: $owner, image: {ownerId: $owner, imagePath: $image},
      serviceDetails: {runtime: "image", plan: "starter", envSpecificDetails: {dockerCommand: $cmd}}}
      + if $env == "" then {} else {environmentId: $env} end')"
  peer_id="$(body '.service.id // empty')"
  [[ "$status" == 2* && -n "$peer_id" ]] || fail check "creating the peer answered $status: $(error_text)"
fi

while IFS= read -r check; do
  [ -n "$check" ] || continue
  read -r kind svc rest <<<"$check"
  if ! id="$(service_id "$svc")"; then
    fail check "the Blueprint created no resource named $svc"
  fi
  api GET "/v1/services/$id"
  url="$(body '.serviceDetails.url // empty')"
  case "$kind" in
    http)
      read -r path text <<<"$rest"
      retry "$check_window" http_check GET "$url$path" "" "${text-}" || fail check "$svc: $check_detail"
      ;;
    post)
      read -r path json text <<<"$rest"
      retry "$check_window" http_check POST "$url$path" "$json" "$text" || fail check "$svc: $check_detail"
      ;;
    log)
      retry "$check_window" log_check "$id" "$rest" || fail check "$svc: $check_detail"
      ;;
    cron)
      api POST "/v1/cron-jobs/$id/runs"
      run_id="$(body '.id // empty')"
      if [[ "$status" != 2* ]] || [ -z "$run_id" ]; then
        fail check "$svc: triggering a run answered $status: $(error_text)"
      fi
      run_until_done "/v1/cron-jobs/$id/runs/$run_id" successful || fail check "$svc: $check_detail"
      ;;
    peer)
      read -r target text <<<"$rest"
      peer_target "$target"
      retry "$check_window" peer_check "${url%:*}:$peer_port$peer_path" "${text-}" || fail check "$svc: $check_detail"
      ;;
    addr)
      if [ "${url##*:}" != "$rest" ]; then
        fail check "$svc: its internal address is ${url:-empty}, not on port $rest"
      fi
      ;;
  esac
done <<<"$checks"

result live - "${#resources[@]} resource(s) up and every check passed"
