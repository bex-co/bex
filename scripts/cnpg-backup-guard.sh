#!/usr/bin/env bash
# Fail closed when a GitOps-managed CNPG Cluster is added without the supported
# Barman Cloud plugin, or when two clusters would write the same serverName into
# one ObjectStore destination. There are intentionally no current exceptions.
set -euo pipefail

cd "$(dirname "$0")/.."

PLUGIN_NAME="barman-cloud.cloudnative-pg.io"

scan() (
  local root="${1:-deploy/gitops}"
  local tmp cluster_files objectstore_files file

  if [ ! -d "$root" ]; then
    echo "FAIL: CNPG backup guard root does not exist: $root" >&2
    return 1
  fi

  tmp="$(mktemp -d)"
  # Paths originate from mktemp and are always task-scoped.
  trap 'rm -rf "$tmp"' EXIT
  : >"$tmp/clusters.tsv"
  : >"$tmp/objectstores.tsv"

  cluster_files="$(find "$root" -type f \( -name '*.yaml' -o -name '*.yml' \) \
    -exec grep -lE '^kind:[[:space:]]*Cluster[[:space:]]*$' {} + || true)"
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    yq ea -r '
      select(.apiVersion == "postgresql.cnpg.io/v1" and .kind == "Cluster") |
      [filename,
       (.metadata.namespace // "default"),
       .metadata.name,
       ([.spec.plugins[]? | select(.name == "'"$PLUGIN_NAME"'")] | length),
       ([.spec.plugins[]? | select(.name == "'"$PLUGIN_NAME"'")][0].isWALArchiver // false),
       ([.spec.plugins[]? | select(.name == "'"$PLUGIN_NAME"'")][0].parameters.barmanObjectName // ""),
       ([.spec.plugins[]? | select(.name == "'"$PLUGIN_NAME"'")][0].parameters.serverName // "")] |
      @tsv
    ' "$file" >>"$tmp/clusters.tsv" || {
      echo "FAIL: CNPG backup guard could not parse $file" >&2
      return 1
    }
  done <<<"$cluster_files"

  objectstore_files="$(find "$root" -type f \( -name '*.yaml' -o -name '*.yml' \) \
    -exec grep -lE '^kind:[[:space:]]*ObjectStore[[:space:]]*$' {} + || true)"
  while IFS= read -r file; do
    [ -n "$file" ] || continue
    yq ea -r '
      select(.apiVersion == "barmancloud.cnpg.io/v1" and .kind == "ObjectStore") |
      [(.metadata.namespace // "default"),
       .metadata.name,
       (.spec.configuration.destinationPath // "")] |
      @tsv
    ' "$file" >>"$tmp/objectstores.tsv" || {
      echo "FAIL: CNPG backup guard could not parse $file" >&2
      return 1
    }
  done <<<"$objectstore_files"

  if [ ! -s "$tmp/clusters.tsv" ]; then
    echo "FAIL: CNPG backup guard found no postgresql.cnpg.io/v1 Cluster manifests under $root" >&2
    return 1
  fi
  if [ ! -s "$tmp/objectstores.tsv" ]; then
    echo "FAIL: CNPG backup guard found no barmancloud.cnpg.io/v1 ObjectStore manifests under $root" >&2
    return 1
  fi

  awk -F '\t' '
    NR == FNR {
      stores[$1 SUBSEP $2] = $3
      next
    }
    {
      identity = $2 "/" $3
      if ($4 != "1") {
        printf "FAIL: %s (%s) needs exactly one Barman Cloud plugin block; found %s\n", $1, identity, $4 > "/dev/stderr"
        failed = 1
        next
      }
      if ($5 != "true") {
        printf "FAIL: %s (%s) must set isWALArchiver: true\n", $1, identity > "/dev/stderr"
        failed = 1
      }
      if ($6 == "" || $7 == "") {
        printf "FAIL: %s (%s) needs non-empty barmanObjectName and serverName parameters\n", $1, identity > "/dev/stderr"
        failed = 1
        next
      }
      store_key = $2 SUBSEP $6
      if (!(store_key in stores) || stores[store_key] == "") {
        printf "FAIL: %s (%s) references missing ObjectStore %s/%s\n", $1, identity, $2, $6 > "/dev/stderr"
        failed = 1
        next
      }
      archive_key = stores[store_key] SUBSEP $7
      if (archive_key in archives) {
        printf "FAIL: %s and %s share serverName %s in destination %s\n", archives[archive_key], identity, $7, stores[store_key] > "/dev/stderr"
        failed = 1
      } else {
        archives[archive_key] = identity
      }
    }
    END { exit failed ? 1 : 0 }
  ' "$tmp/objectstores.tsv" "$tmp/clusters.tsv"
)

self_test() (
  local fixture
  fixture="$(mktemp -d)"
  trap 'rm -rf "$fixture"' EXIT

  mkdir -p "$fixture/good"
  tee "$fixture/good/objectstore.yaml" >/dev/null <<'YAML'
apiVersion: barmancloud.cnpg.io/v1
kind: ObjectStore
metadata: { name: shared, namespace: auth }
spec:
  configuration: { destinationPath: s3://fixture/auth }
YAML
  tee "$fixture/good/one.yaml" >/dev/null <<'YAML'
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: { name: one, namespace: auth }
spec:
  plugins:
    - name: barman-cloud.cloudnative-pg.io
      isWALArchiver: true
      parameters: { barmanObjectName: shared, serverName: one-pg18 }
YAML
  tee "$fixture/good/two.yaml" >/dev/null <<'YAML'
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata: { name: two, namespace: auth }
spec:
  plugins:
    - name: barman-cloud.cloudnative-pg.io
      isWALArchiver: true
      parameters: { barmanObjectName: shared, serverName: two-pg18 }
YAML

  scan "$fixture/good" >/dev/null || {
    echo "FAIL: CNPG backup guard rejected the green self-test fixture" >&2
    return 1
  }

  cp -R "$fixture/good" "$fixture/missing-plugin"
  yq -i 'del(.spec.plugins)' "$fixture/missing-plugin/two.yaml"
  if scan "$fixture/missing-plugin" >/dev/null 2>&1; then
    echo "FAIL: CNPG backup guard accepted a Cluster without a plugin block" >&2
    return 1
  fi

  cp -R "$fixture/good" "$fixture/duplicate-server"
  yq -i '.spec.plugins[0].parameters.serverName = "one-pg18"' "$fixture/duplicate-server/two.yaml"
  if scan "$fixture/duplicate-server" >/dev/null 2>&1; then
    echo "FAIL: CNPG backup guard accepted duplicate serverNames in one destination" >&2
    return 1
  fi

  cp -R "$fixture/good" "$fixture/missing-store"
  yq -i '.spec.plugins[0].parameters.barmanObjectName = "absent"' "$fixture/missing-store/two.yaml"
  if scan "$fixture/missing-store" >/dev/null 2>&1; then
    echo "FAIL: CNPG backup guard accepted a missing ObjectStore reference" >&2
    return 1
  fi

  mkdir -p "$fixture/no-clusters"
  cp "$fixture/good/objectstore.yaml" "$fixture/no-clusters/objectstore.yaml"
  if scan "$fixture/no-clusters" >/dev/null 2>&1; then
    echo "FAIL: CNPG backup guard accepted an empty Cluster scan" >&2
    return 1
  fi

  echo "PASS: CNPG backup guard self-test"
)

case "${1:-}" in
  --self-test)
    self_test
    ;;
  *)
    scan "${1:-deploy/gitops}"
    echo "PASS: every GitOps CNPG Cluster has a unique Barman Cloud archive identity"
    ;;
esac
