#!/usr/bin/env bash
# Fail-closed supply-chain guard: every container image bex builds from, runs,
# or ships must resolve by digest (w7/m85).
#
# This finding was deferred through six consecutive security rounds — ADR055 F7
# → ADR061 #1 → ADR063 #12 → ADR066 #7 → ADR067 #8 → ADR068 #9 — because it
# reads like an inventory sweep, and an inventory sweep is only ever true on the
# day it is done. This script is the durable half: it re-derives the inventory
# on every CI run, so a floating tag reintroduced anywhere turns the build red
# instead of waiting for round seven.
#
# Three syntaxes, one policy — an image reference must carry `@sha256:<64hex>`:
#
#   1. Dockerfile   `FROM <ref>`               (also `--from=<ref>` mounts)
#   2. Go           string literals that are image references
#   3. manifests    `image:`, kustomize `images:` edits, Helm `repository:`/`tag:`
#
# FAIL CLOSED is the whole point. Two rules keep it that way:
#
#   * A reference that cannot be classified is a FAILURE, never a skip. The
#     inventory drifted for six rounds precisely because unrecognized things
#     were quietly passed over.
#   * Exemptions are ENUMERATED file+reference pairs with a reason each (see
#     `exemptions` below), never a blanket pattern. A placeholder that moves to
#     a new file must be re-reviewed rather than silently inheriting a pass.
#
# Usage:
#   scripts/image-pin-validate.sh              # scan the tracked tree
#   scripts/image-pin-validate.sh --list       # print the classified inventory
#   scripts/image-pin-validate.sh --verify-digests
#                                              # NETWORK: prove every pinned
#                                              # digest exists in the repository
#                                              # its reference names
#
# Complementary to scripts/gitops-validate.sh, which renders the gitops tree and
# checks the EFFECTIVE references a cluster would pull (so it catches a kustomize
# pin patch being dropped). This one reads source across the whole repo, which
# that render cannot see.
#
# IMAGE_PIN_TREE overrides the scanned tree; the self-test
# (image-pin-validate.test.sh) points it at fixtures to exercise both
# directions. Only the default, override-free invocation applies the canonical
# exemption list, which pins this repo's specific placeholder sites.
set -euo pipefail
cd "$(dirname "$0")/.."

# Fail loudly on an old shell rather than quietly scanning less. macOS still
# ships bash 3.2 as /bin/bash, where the per-file process substitutions below
# crash their subshells partway through the tree — which does not stop the run,
# it just silently shrinks the inventory. A guard that reports "all pinned"
# after examining half the repo is worse than no guard. CI and Homebrew bash
# are 5.x; run this with those.
if [ "${BASH_VERSINFO[0]:-0}" -lt 4 ]; then
  echo "FAIL: this guard needs bash 4+ (found ${BASH_VERSION:-unknown});" >&2
  echo "on macOS run it with Homebrew bash, not /bin/bash." >&2
  exit 1
fi

mode="scan"
case "${1:-}" in
  "") ;;
  --list) mode="list" ;;
  --verify-digests) mode="verify" ;;
  *) echo "usage: $0 [--list|--verify-digests]" >&2; exit 2 ;;
esac

# The seam is "was an override supplied?", not a path compare, so an explicit
# IMAGE_PIN_TREE=. cannot silently drop the canonical exemptions.
if [ -n "${IMAGE_PIN_TREE:-}" ]; then
  canonical_tree=0
  tree="$IMAGE_PIN_TREE"
else
  canonical_tree=1
  tree="."
fi

# Tracked files only. Untracked worktrees (.claude/worktrees/**) and build
# output are not shipped, and scanning them would fail on other branches' code.
tracked() {
  if [ "$canonical_tree" -eq 1 ]; then
    git ls-files -z -- "$@" | tr '\0' '\n'
    return
  fi
  # Fixture trees are not git repositories: match the same basename globs by hand.
  local path base spec
  find "$tree" -type f 2>/dev/null | while IFS= read -r path; do
    base="${path##*/}"
    for spec in "$@"; do
      # shellcheck disable=SC2053
      [[ "$base" == $spec ]] && { printf '%s\n' "$path"; break; }
    done
  done
}

pinned() { [[ "$1" =~ @sha256:[0-9a-f]{64}$ ]]; }

# --- exemptions -------------------------------------------------------------
# One `<path>\t<reference>\t<reason>` row per exempt site. Enumerated, not
# pattern-blanket: each row names the exact file and the exact reference, so the
# same string appearing somewhere new is a failure until a human writes a reason
# for it. Nothing here is an image bex resolves in production — every row is a
# placeholder rewritten before it runs, or example code a tenant is meant to
# copy and own.
exemptions() {
  cat <<'EOF'
lego/operator/config/manager/manager.yaml	controller:latest	kustomize `images:` placeholder — deploy/gitops/base/bex.yaml rewrites it to the operator digest deploy.yml pushed
lego/operator/config/api/deployment.yaml	controller:latest	same manager image, /api entrypoint; rewritten by the same kustomize edit
lego/operator/config/ssh/deployment.yaml	controller:latest	same manager image, /ssh-gateway entrypoint; rewritten by the same kustomize edit
lego/operator/config/activator/deployment.yaml	controller:latest	same manager image, /activator entrypoint; rewritten by the same kustomize edit
lego/operator/config/staticserver/deployment.yaml	controller:latest	same manager image, /staticserver entrypoint; rewritten by the same kustomize edit
lego/operator/config/pg-sni-proxy/daemonset.yaml	controller:latest	same manager image, /pg-sni-proxy entrypoint; rewritten by the same kustomize edit
lego/operator/config/kv-sni-proxy/daemonset.yaml	controller:latest	same manager image, /kv-sni-proxy entrypoint; rewritten by the same kustomize edit
lego/operator/config/egress-meter/daemonset.yaml	controller:latest	same manager image, /egress-meter entrypoint; rewritten by the same kustomize edit
dashboard/deploy/deployment.yaml	dashboard:latest	kustomize `images:` placeholder — deploy/gitops/base/dashboard.yaml rewrites it to the dashboard digest deploy.yml pushed
deploy/opensandbox/server-in-cluster.yaml	opensandbox-server:0.2.2	kustomize `images:` placeholder — deploy/opensandbox/kustomization.yaml carries the digest
lego/operator/config/samples/app_v1alpha1_app.yaml	traefik/whoami	sample App CR: a TENANT's image choice in documentation, not an image bex resolves
deploy/gitops/charts/opensandbox-controller/values.yaml	sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/controller	upstream chart default; deploy/gitops/base/values/opensandbox-controller.values.yaml overrides it with a digest
deploy/gitops/charts/opensandbox-controller/values.yaml	image-committer:dev	upstream chart default for a snapshot path production leaves disabled; the same base values file overrides it with a digest
lego/operator/config/deploy/kustomization.yaml	bex-operator:dev	disposable local overlay: `make docker-build` copies this beside config/default, points it at the just-built local IMG, and deletes the copy — it never reaches a cluster
deploy/gitops/charts/barman-cloud-plugin/upstream/manifest-0.13.0.yaml	ghcr.io/cloudnative-pg/plugin-barman-cloud:v0.13.0	vendored upstream v0.13.0 release, kept byte-for-byte reproducible against the SHA-256 in its README; the EXECUTED reference is digest-pinned by the kustomize patch beside it, which scripts/gitops-validate.sh asserts on the RENDERED output
EOF
}

# Example Dockerfiles under examples/ are a tenant's own application build. They
# are documentation of what a USER writes, and pinning them would teach a
# digest-pinned base image as the shape of an ordinary app Dockerfile — so they
# are excluded from the scan rather than exempted reference by reference.
# scripts/fixtures/ is test input, built only by the test that owns it.
skip_path() {
  case "$1" in
    ./examples/* | examples/*) return 0 ;;
    ./scripts/fixtures/* | scripts/fixtures/*) return 0 ;;
  esac
  return 1
}

exempt_reason() {
  exemptions | awk -F'\t' -v p="$1" -v r="$2" '$1==p && $2==r {print $3; found=1} END{exit !found}'
}

findings=()   # "<path>\t<ref>\t<syntax>"
inventory=()  # "<status>\t<path>\t<ref>\t<syntax>"

record() {
  local path="$1" ref="$2" syntax="$3" reason
  if pinned "$ref"; then
    inventory+=("pinned	$path	$ref	$syntax")
    return
  fi
  if [ "$canonical_tree" -eq 1 ] && reason="$(exempt_reason "$path" "$ref")"; then
    inventory+=("exempt	$path	$ref	$syntax ($reason)")
    return
  fi
  inventory+=("UNPINNED	$path	$ref	$syntax")
  findings+=("$path	$ref	$syntax")
}

# --- 1. Dockerfile FROM -----------------------------------------------------
# Stage names defined earlier in the same file (`AS build`) are internal
# references, not registry pulls. `scratch` is the empty base and has no
# manifest. Everything else must be pinned. ARG-substituted refs (${FOO}) are
# NOT skipped — they are reported, because the guard cannot see what they
# resolve to and a silent skip is exactly the drift this exists to stop.
scan_dockerfiles() {
  local path stages ref
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    skip_path "$path" && continue
    stages=" $(grep -ioE '^[[:space:]]*FROM[[:space:]]+\S+[[:space:]]+AS[[:space:]]+\S+' "$path" |
      awk '{print tolower($NF)}' | tr '\n' ' ' || true)"
    while IFS= read -r ref; do
      [ -n "$ref" ] || continue
      [ "$ref" = "scratch" ] && continue
      # `tr`, not ${ref,,}: macOS ships bash 3.2 and a developer running this
      # with /bin/bash must get the same answer CI does.
      [[ "$stages " == *" $(printf '%s' "$ref" | tr '[:upper:]' '[:lower:]') "* ]] && continue
      record "$path" "$ref" "Dockerfile"
    done < <({
      grep -iE '^[[:space:]]*FROM[[:space:]]' "$path" | awk '{print $2}'
      grep -ohE '\-\-from=[^ ]+' "$path" | sed 's/--from=//'
    } || true)
  done < <(tracked '*Dockerfile*')
}

# --- 2. Go string literals --------------------------------------------------
# Comments are stripped first: prose legitimately quotes unpinned examples
# ("nginx:latest" as a counter-example in a doc comment) and those are not code.
# What survives is classified as an image reference when it is a well-formed
# ref carrying a NON-NUMERIC tag or a digest — the non-numeric rule is what
# keeps host:port strings ("zot.bex-registry.svc:5000") out — and it either
# contains a path separator or names one of the official single-word images
# this repo actually uses.
go_literals() {
  local path="$1"
  sed -e 's;^[[:space:]]*//.*$;;' -e 's;[[:space:]]//[[:space:]].*$;;' "$path" |
    grep -ohE '"[^"[:space:]]+"' | tr -d '"' || true
}

is_image_ref() {
  local ref="$1"
  [[ "$ref" =~ ^([a-z0-9][a-z0-9.-]*(:[0-9]+)?/)?[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*(:[a-zA-Z0-9][a-zA-Z0-9._-]*)?(@sha256:[0-9a-f]{64})?$ ]] || return 1
  [[ "$ref" == *@sha256:* ]] && return 0
  # A tag component that is entirely digits is a port, not a tag.
  [[ "$ref" =~ :[a-zA-Z0-9._-]*[a-zA-Z][a-zA-Z0-9._-]*$ ]] || return 1
  [[ "$ref" == */* ]] && return 0
  [[ "$ref" =~ ^(alpine|busybox|golang|node|python|redis|valkey|postgres|nginx|debian|ubuntu|traefik|mailpit)[:@] ]]
}

scan_go() {
  local path ref
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    case "$path" in *_test.go) continue ;; esac
    skip_path "$path" && continue
    while IFS= read -r ref; do
      [ -n "$ref" ] || continue
      is_image_ref "$ref" || continue
      record "$path" "$ref" "Go literal"
    done < <(go_literals "$path" | sort -u || true)
  done < <(tracked '*.go')
}

# --- 3. manifest image references -------------------------------------------
# Four shapes, all in YAML:
#
#   image: <ref>            and any *Image*: key (sidecarImage, imageCommitterImage)
#   kustomize images:       `- <name>=<ref>` edits, and name/newName/digest|newTag triples
#   Helm values             `repository:` + the `tag:` beside it
#
# A Helm TEMPLATE value ({{ ... }}) carries no reference — it is rendered from a
# values file this scan reads directly — and a shell/env expansion likewise. Both
# are dropped here rather than reported, and nothing else is: an unrecognized
# value is reported so it has to be classified by a human.
manifest_refs() {
  awk '
    function emit(v) {
      gsub(/^[ \t]+|[ \t]+$/, "", v)
      gsub(/^["'"'"']|["'"'"']$/, "", v)
      if (v == "") return
      print v
    }
    function trim(v) { gsub(/^[ \t]+|[ \t]+$/, "", v); gsub(/^["'"'"']|["'"'"']$/, "", v); return v }
    function value(line) { return trim(substr(line, index(line, ":") + 1)) }
    function indent(line,   i) { i = match(line, /[^ ]/); return i ? i - 1 : 0 }
    {
      line = $0
      sub(/[ \t]+#.*$/, "", line)          # trailing comment
    }
    line ~ /^[ \t]*#/ { next }
    line ~ /^[ \t]*$/ { next }

    # Enter/leave a kustomize `images:` block by indentation.
    line ~ /^[ \t]*images:[ \t]*$/ { inimg = 1; imgind = indent(line); pending = ""; next }
    inimg && indent(line) <= imgind { inimg = 0 }

    inimg {
      if (line ~ /^[ \t]*-[ \t]*[A-Za-z0-9_.-]+=/) { emit(substr(line, index(line, "=") + 1)); next }
      if (line ~ /^[ \t]*-?[ \t]*(name|newName):[ \t]*/) { pending = value(line); next }
      if (line ~ /^[ \t]*digest:[ \t]*/) {
        if (pending != "") emit(pending "@" value(line))
        pending = ""; next
      }
      if (line ~ /^[ \t]*newTag:[ \t]*/) {
        if (pending != "") emit(pending ":" value(line))
        pending = ""; next
      }
    }

    # Helm values: repository + the tag at the same indentation.
    line ~ /^[ \t]*repository:[ \t]*/ { repo = value(line); repoind = indent(line); next }
    repo != "" && line ~ /^[ \t]*tag:[ \t]*/ && indent(line) == repoind {
      tag = value(line)
      emit(tag == "" ? repo : repo ":" tag)
      repo = ""; next
    }

    # Any key ENDING in "image": `image:`, `imageCommitterImage:`, `sidecarImage:`.
    # The suffix rule is what keeps `imagePullPolicy: Always` and
    # `imagePullSecrets: []` — settings, not references — out of the inventory,
    # along with CAPI `imageName:`, which names a Hetzner VM snapshot rather than
    # a container image (that supply chain is w7/m75 infra pinning, not this).
    line ~ /^[ \t]*-?[ \t]*([A-Za-z0-9_]*[Ii]mage|image):[ \t]*[^ \t]/ {
      emit(value(line))
    }
  ' "$1"
}

scan_manifests() {
  local path ref
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    skip_path "$path" && continue
    while IFS= read -r ref; do
      [ -n "$ref" ] || continue
      # shellcheck disable=SC2016 # matching the literal characters, not expanding
      case "$ref" in *'{{'*|*'}}'*|*'$('*|*'${'*) continue ;; esac
      record "$path" "$ref" "manifest"
    done < <(manifest_refs "$path" | sort -u || true)
  done < <(tracked '*.yaml' '*.yml')
}

# --- 4. `docker run` in CI workflows ----------------------------------------
# Narrow by design: a workflow step that starts a container is part of the same
# supply chain as a `services:` block (backend-test.yml runs OpenFGA this way
# precisely because a service container cannot pass its `run` subcommand), so it
# gets the same rule. Developer harnesses under scripts/ are NOT covered — they
# run on a laptop against local fixtures, never in the pipeline that builds a
# shipped artifact.
scan_workflow_containers() {
  local path ref
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    case "$path" in *.github/workflows/*) ;; *) continue ;; esac
    while IFS= read -r ref; do
      [ -n "$ref" ] || continue
      case "$ref" in *'{{'*|*'$'*) continue ;; esac
      is_image_ref "$ref" || continue
      record "$path" "$ref" "workflow docker run"
    done < <(sed -e ':a' -e '/\\$/{N;s/\\\n//;ba' -e '}' "$path" |
      grep -hoE 'docker run [^|#]*' |
      tr -s ' \t' '\n' | grep -vE '^-' | grep -E ':|@sha256:' || true)
  done < <(tracked '*.yml' '*.yaml')
}

scan_dockerfiles
scan_go
scan_manifests
scan_workflow_containers

if [ "${#inventory[@]}" -eq 0 ]; then
  echo "FAIL: no image references found — the scan matched nothing, which is never true of this tree" >&2
  exit 1
fi

if [ "$mode" = "list" ]; then
  printf '%s\n' "${inventory[@]}" | sort
  exit 0
fi

if [ "$mode" = "verify" ]; then
  # Network mode. NOT a merge gate (it needs registry reachability); run it when
  # adding or bumping a pin. It answers the failure mode a tag comparison cannot:
  # a digest copy-pasted from a DIFFERENT image. It deliberately does not assert
  # that the digest is what the tag resolves to today — a moved tag is the pin
  # working, not a defect.
  rc=0
  while IFS=$'\t' read -r status path ref _; do
    [ "$status" = "pinned" ] || continue
    if scripts/lib/image-digest-exists.sh "$ref"; then
      printf 'ok       %s\t%s\n' "$path" "$ref"
    else
      printf 'MISSING  %s\t%s\n' "$path" "$ref" >&2
      rc=1
    fi
  done < <(printf '%s\n' "${inventory[@]}" | sort -u)
  [ "$rc" -eq 0 ] || echo "FAIL: a pinned digest does not exist in the repository its reference names" >&2
  exit "$rc"
fi

if [ "${#findings[@]}" -gt 0 ]; then
  echo "FAIL: image references must be digest-pinned (name:tag@sha256:...):" >&2
  printf '  %s\n' "${findings[@]}" >&2
  echo >&2
  echo "Pin it, or — if it is a placeholder or tenant example — add an enumerated" >&2
  echo "row WITH A REASON to exemptions() in scripts/image-pin-validate.sh." >&2
  exit 1
fi

echo "image pins OK: ${#inventory[@]} references, all digest-pinned or enumerated as exempt"
