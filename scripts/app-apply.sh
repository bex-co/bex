#!/usr/bin/env bash
# Apply a project's bex.yml (render.yaml Blueprint manifest) as App + Database CRs.
# Usage: scripts/app-apply.sh <path-to-bex.yml | project-dir>
#        DRY_RUN=1 scripts/app-apply.sh ...   # print the CRs instead of applying
# Requires: yq v4, kubectl (respects $KUBECONFIG).
#
# A bex.yml declares a stack — the render.yaml Blueprint shape:
#   services:   web/pserv/worker/cron/keyvalue services (Render field names;
#               a static site is type:web + runtime:static):
#               type/runtime/plan/numInstances/domains/healthCheckPath/envVars/…).
#   databases:  managed Postgres instances (name/plan/diskSizeGB/…).
#
# Databases apply first, then services, so a service's fromDatabase env reference
# (resolved to a secretRef into the CNPG "<stable-id>-app" connection Secret) waits on
# a Database that is already provisioning. Re-applying is a server-side idempotent
# upsert (kubectl apply). bex v1 does NOT sync-delete resources absent from the
# file — a documented divergence from Render Blueprints' optional sync.
#
# Manifest semantics: `type: web` is public — <name>.<base-domain> is
# auto-assigned, `domains:` adds custom domains. `type: pserv|worker|cron` have
# no ingress. `expose` is CR-level mechanism, not a manifest key.
#
# CR generation is deliberately split: bash reads each field (simple yq lookups)
# and resolves env into a JSON array; a final yq --null-input call only assembles
# the CR from strenv values (no yq conditionals — yq v4 has none). This keeps the
# projection transparent and matches what lego/backend/internal/apps/deploy.go does.
set -euo pipefail

manifest="${1:?usage: app-apply.sh <bex.yml | project-dir>}"
[ -d "$manifest" ] && manifest="$manifest/bex.yml"
[ -f "$manifest" ] || { echo "error: $manifest not found" >&2; exit 1; }

# `services:` is the Render Blueprint key. A database-only file is also valid.
have_services=$(yq '.services | length' "$manifest" 2>/dev/null || echo 0)
if [ "$(yq 'has("apps")' "$manifest")" = "true" ]; then
  echo "error: top-level apps is retired; rename it to services" >&2; exit 1
fi
if [ "$have_services" -gt 0 ] 2>/dev/null; then svc_key="services"
else
  ndb=$(yq '.databases | length' "$manifest" 2>/dev/null || echo 0)
  [ "$ndb" -gt 0 ] 2>/dev/null || { echo "error: no services:/databases: entries in $manifest" >&2; exit 1; }
  svc_key=""
fi

if yq -e '.services[]? | has("tier") or has("replicas") or has("port") or has("imagePath") or has("publishPath")' "$manifest" >/dev/null 2>&1; then
  echo "error: a service uses a retired Blueprint field; use plan, numInstances, image.url, or staticPublishPath and omit port" >&2; exit 1
fi
if yq -e '.services[]? | select(has("image")) | .image | tag == "!!str"' "$manifest" >/dev/null 2>&1; then
  echo "error: a service uses a bare image string; use image: {url: ...}" >&2; exit 1
fi

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
[ -n "$svc_key" ] && yq ".$svc_key" "$manifest" > "$tmp/services.yml"
yq '.databases // []' "$manifest" > "$tmp/databases.yml"
printf '{}\n' > "$tmp/database-ids.yml"

apply_cr() { # reads a YAML CR on stdin, applies or print it
  if [ "${DRY_RUN:-}" = "1" ]; then echo "---"; cat
  else kubectl apply -f -; fi
}

# snapshot_svc writes services[$name] alone to $tmp/svc.yml — a tiny single-object
# doc every per-service read then queries, so each field read parses ~10 lines
# instead of re-scanning the whole stack with .[] | select(.name==…).
snapshot_svc() { yq ".[] | select(.name == \"$1\")" "$tmp/services.yml" > "$tmp/svc.yml"; }

# sf reads .$field off the current service snapshot (set by snapshot_svc).
sf() { yq ".$1" "$tmp/svc.yml"; } # $1 = field
df() { yq ".[] | select(.name == \"$1\") | .$2" "$tmp/databases.yml"; } # $1=name $2=field
db_id() { NAME="$1" yq -r '.[strenv(NAME)] // ""' "$tmp/database-ids.yml"; }

# resolve_database_id preserves an existing Database's metadata.name when its
# required mutable spec.name matches the Blueprint display name, or mints a
# canonical dpg-... id through backend/internal/id for a new Database.
resolve_database_id() { # $1 = display name
  local name=$1 matches count database_id
  if [ "${#name}" -gt 30 ] || [[ ! "$name" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]]; then
    echo "error: database '$name' must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen" >&2
    exit 1
  fi
  matches=""
  if [ "${DRY_RUN:-}" != "1" ]; then
    matches=$(DISPLAY_NAME="$name" kubectl get databases.app.bex.co -o json |
      DISPLAY_NAME="$name" yq -p=json -r '.items[] | select(.spec.name == strenv(DISPLAY_NAME)) | .metadata.name')
  fi
  count=$(printf '%s\n' "$matches" | sed '/^$/d' | wc -l | tr -d ' ')
  if [ "$count" -gt 1 ]; then
    echo "error: database display name '$name' resolves to multiple CRs; repair the violated uniqueness invariant first" >&2
    exit 1
  elif [ "$count" -eq 1 ]; then
    database_id=$matches
  else
    database_id=$(cd "$repo_root/lego/backend" && go run ./cmd/idgen postgres)
  fi
  NAME="$name" ID="$database_id" yq -i '.[strenv(NAME)] = strenv(ID)' "$tmp/database-ids.yml"
}

# service_kind maps a service to the normalized bex kind (web|private|worker|cron|
# static), folding render.yaml's runtime:static (type:web+runtime:static) and the
# pserv alias. Reads the snapshot set by snapshot_svc.
service_kind() { # reads $tmp/svc.yml
  local t rt
  t=$(yq '.type // "web"' "$tmp/svc.yml")
  rt=$(yq '.runtime // ""' "$tmp/svc.yml")
  if [ "$t" = "web" ] && [ "$rt" = "static" ]; then echo static
  elif [ "$t" = "pserv" ]; then echo private
  else echo "$t"; fi
}

# db_secret_key maps a render.yaml fromDatabase property to the CNPG "<stable-id>-app"
# Secret key (the connection vocabulary: username/password/dbname/host/port/uri).
db_secret_key() { # $1 = property
  case "$1" in
    connectionString) echo uri ;;   # render.yaml connectionString -> the ready-made uri
    host) echo host ;; port) echo port ;; user) echo username ;;
    password) echo password ;; database) echo dbname ;;
    *) echo "error: unknown fromDatabase property '$1'" >&2; exit 1 ;;
  esac
}

# build_env writes a JSON array of spec.env entries to $1 from the current
# service snapshot ($tmp/svc.yml). Literal {key,value} stays literal; fromDatabase
# {name,property} -> secretRef into the CNPG "<stable-id>-app" Secret (never a plaintext
# copy); fromService {name,property} host/port/hostport -> literal (bare <name>
# resolves in-cluster; the platform service port defaults to 3000). Written to a file (not
# echoed) because the JSON contains '"' — yq load consumes it.
build_env() { # $1 = output file
  local out=$1 nev i key fdn fdp fsn fsp refport val database_id
  nev=$(yq '.envVars | length' "$tmp/svc.yml")
  if [ "$nev" -gt 0 ] 2>/dev/null; then
    printf '[' > "$out"; i=0
    while [ "$i" -lt "$nev" ]; do
      key=$(yq ".envVars[$i].key" "$tmp/svc.yml")
      fdn=$(yq ".envVars[$i].fromDatabase.name // \"\"" "$tmp/svc.yml")
      fdp=$(yq ".envVars[$i].fromDatabase.property // \"\"" "$tmp/svc.yml")
      fsn=$(yq ".envVars[$i].fromService.name // \"\"" "$tmp/svc.yml")
      fsp=$(yq ".envVars[$i].fromService.property // \"\"" "$tmp/svc.yml")
      [ "$i" -gt 0 ] && printf ',' >> "$out"
      if [ -n "$fdn" ]; then
        database_id=$(db_id "$fdn")
        [ -n "$database_id" ] || { echo "error: fromDatabase references unknown database '$fdn'" >&2; exit 1; }
        printf '{"name":"%s","valueFrom":{"secretKeyRef":{"name":"%s-app","key":"%s"}}}' \
          "$key" "$database_id" "$(db_secret_key "$fdp")" >> "$out"
      elif [ -n "$fsn" ]; then
        case "$fsp" in
          host) val="$fsn" ;;
          port|hostport)
            refport=3000
            [ "$fsp" = "port" ] && val="$refport" || val="$fsn:$refport" ;;
          *) echo "error: fromService property '$fsp' unsupported (want host/port/hostport)" >&2; exit 1 ;;
        esac
        printf '{"name":"%s","value":"%s"}' "$key" "$val" >> "$out"
      else
        val=$(yq ".envVars[$i].value" "$tmp/svc.yml")
        printf '{"name":"%s","value":"%s"}' "$key" "$val" >> "$out"
      fi
      i=$((i + 1))
    done
    printf ']' >> "$out"
  else
    printf '[]' > "$out"
  fi
}

# strip-empty (stdin YAML CR -> stdout) drops spec entries whose value is unset
# (null/0/false/""/empty-array) so operator defaults apply.
strip_empty() {
  yq -o=yaml '.spec |= with_entries(select(.value != null and .value != 0 and .value != false and .value != "" and ((.value | kind) != "seq" or (.value | length > 0)))) | del(.. | select(tag == "!!null"))'
}

# crtype_of maps a bex kind to the App CRD serviceType.
crtype_of() { # $1 = kind
  case "$1" in
    web) echo web_service ;; private) echo private_service ;;
    worker) echo background_worker ;; cron) echo cron_job ;; static) echo static_site ;;
  esac
}

# num reads a numeric field off the snapshot, coercing null/empty to 0 (dropped
# by strip_empty).
num() { local v; v=$(sf "$1"); [ "$v" = "null" ] || [ -z "$v" ] && echo 0 || echo "$v"; }

# render_app emits one App CR for the current service snapshot ($tmp/svc.yml).
render_app() { # $1 = name (snapshot already set by the caller)
  local name=$1 kind crtype
  kind=$(service_kind); crtype=$(crtype_of "$kind")
  local repo image branch builder runtime rootdir port replicas tier hcp publish schedule host expose
  repo=$(sf 'repo // ""'); branch=$(sf 'branch // ""')
  builder=$(sf 'builder // ""'); runtime=$(sf 'runtime // ""')
  [ "$runtime" = "static" ] && runtime=""
  rootdir=$(sf 'rootDir // ""'); publish=$(sf 'staticPublishPath // ""')
  port=0; replicas=$(num numInstances); tier=$(sf 'plan // ""')
  hcp=$(sf 'healthCheckPath // ""'); schedule=$(sf 'schedule // ""')
  image=$(sf 'image.url // ""')
  host=$(yq '.domains[0] // ""' "$tmp/svc.yml")
  yq -o=json '(.domains // []) | .[1:]' "$tmp/svc.yml" > "$tmp/hosts.json"
  build_env "$tmp/env.json"
  [ "$kind" = "web" ] && expose="true" || expose="false"

  # Array values (hosts/env) load from temp JSON files (they contain '"', which
  # would break shell VAR="$val" quoting); scalars ride strenv.
  NAME="$name" CRTYPE="$crtype" REPO="$repo" IMAGE="$image" BRANCH="$branch" \
  BUILDER="$builder" RUNTIME="$runtime" ROOTDIR="$rootdir" PORT="$port" REPLICAS="$replicas" TIER="$tier" \
  HCP="$hcp" PUBLISH="$publish" SCHEDULE="$schedule" HOST="$host" EXPOSE="$expose" \
  HOSTS="$tmp/hosts.json" ENV="$tmp/env.json" \
  yq --null-input -o=yaml '
    . = {
      "apiVersion":"app.bex.co/v1alpha1","kind":"App",
      "metadata":{"name": strenv(NAME)},
      "spec":{
        "type": strenv(CRTYPE),
        "schedule": strenv(SCHEDULE),
        "repo": strenv(REPO), "image": strenv(IMAGE), "branch": strenv(BRANCH),
        "builder": strenv(BUILDER), "runtime": strenv(RUNTIME), "rootDir": strenv(ROOTDIR),
        "port": (strenv(PORT) | to_number),
        "replicas": (strenv(REPLICAS) | to_number),
        "tier": strenv(TIER), "healthCheckPath": strenv(HCP),
        "publishPath": strenv(PUBLISH),
        "host": strenv(HOST),
        "hosts": (load strenv(HOSTS)),
        "env": (load strenv(ENV)),
        "expose": (strenv(EXPOSE) | fromjson)
      }
    }' | strip_empty
}

# render_db emits one Database CR for databases[$name] on stdout.
render_db() { # $1 = display name
  local name=$1 database_id plan version storage ha v
  database_id=$(db_id "$name")
  plan=$(df "$name" plan); version=$(df "$name" postgresMajorVersion)
  v=$(df "$name" diskSizeGB); [ "$v" = "null" ] || [ -z "$v" ] && storage=0 || storage="$v"
  ha=$(df "$name" 'highAvailability.enabled // false')
  yq -o=json ".[] | select(.name == \"$name\") | (.ipAllowList // []) | map(.source)" "$tmp/databases.yml" > "$tmp/ipallow.json"
  yq -o=json ".[] | select(.name == \"$name\") | .readReplicas // []" "$tmp/databases.yml" > "$tmp/replicas.json"
  ID="$database_id" NAME="$name" PLAN="$plan" VERSION="$version" STORAGE="$storage" HA="$ha" \
  IPALLOW="$tmp/ipallow.json" REPLICAS="$tmp/replicas.json" \
  yq --null-input -o=yaml '
    . = {
      "apiVersion":"app.bex.co/v1alpha1","kind":"Database",
      "metadata":{"name": strenv(ID)},
      "spec":{
        "name": strenv(NAME),
        "plan": strenv(PLAN), "version": strenv(VERSION),
        "storageGB": (strenv(STORAGE) | to_number),
        "ipAllowList": (load strenv(IPALLOW)),
        "highAvailability": (strenv(HA) | fromjson),
        "readReplicas": (load strenv(REPLICAS))
      }
    }' | strip_empty
}

# --- databases first (a fromDatabase env ref waits on the <stable-id>-app Secret) ---
ndb=$(yq 'length' "$tmp/databases.yml")
if [ "${ndb:-0}" -gt 0 ] 2>/dev/null; then
  for i in $(seq 0 $((ndb - 1))); do
    name=$(yq ".[$i].name" "$tmp/databases.yml")
    [ "$name" != "null" ] && [ -n "$name" ] || { echo "error: databases[$i].name is required" >&2; exit 1; }
    resolve_database_id "$name"
  done
  for i in $(seq 0 $((ndb - 1))); do
    name=$(yq ".[$i].name" "$tmp/databases.yml")
    render_db "$name" | apply_cr
  done
fi

# --- services ---
if [ -n "$svc_key" ]; then
  nsvc=$(yq 'length' "$tmp/services.yml")
  if [ "${nsvc:-0}" -gt 0 ] 2>/dev/null; then
    for i in $(seq 0 $((nsvc - 1))); do
      name=$(yq ".[$i].name" "$tmp/services.yml")
      [ "$name" != "null" ] && [ -n "$name" ] || { echo "error: $svc_key[$i].name is required" >&2; exit 1; }
      snapshot_svc "$name" # snapshot once; every per-service read queries this tiny doc

      # Non-web types have no ingress; listing domains for one is a mistake.
      kind=$(service_kind)
      case "$kind" in
        web) : ;;
        private|worker|cron|static)
          ndomains=$(yq '.domains | length' "$tmp/svc.yml" 2>/dev/null || echo 0)
          if [ "$ndomains" -gt 0 ] 2>/dev/null; then
            echo "error: $name is type: $kind but lists domains — non-web services have no ingress" >&2; exit 1
          fi ;;
        *) echo "error: $name has unknown service type (web|private|worker|cron|static)" >&2; exit 1 ;;
      esac

      render_app "$name" | apply_cr
    done
  fi
fi

if [ "${DRY_RUN:-}" != "1" ]; then
  echo "watch:  kubectl get apps.app.bex.co,databases.app.bex.co -w"
  echo "serve:  curl \$(kubectl get apps.app.bex.co <name> -o jsonpath='{.status.url}')/"
fi
