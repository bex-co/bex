#!/usr/bin/env bash
# Red/green self-test for scripts/image-pin-validate.sh (w7/m85 t005).
#
# A pinning guard whose RED case was never exercised is decoration: it passes
# every run, including the run where someone reintroduced `node:22-alpine`. So
# every check below asserts BOTH directions on a fixture tree — the same
# discipline that has kept 42 SHA-pinned GitHub Actions pinned since w7/m65.
#
# Hermetic: fixtures are built in a temp dir and the guard is pointed at them
# with IMAGE_PIN_TREE. The canonical tree is checked by the guard itself in CI,
# not here.
set -euo pipefail
cd "$(dirname "$0")/.."

guard="$PWD/scripts/image-pin-validate.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0
DIGEST="sha256:$(printf 'a%.0s' {1..64})"

# run <tree> — the guard against a fixture tree, output captured, rc returned.
run() {
  local out rc=0
  out="$(IMAGE_PIN_TREE="$1" bash "$guard" 2>&1)" || rc=$?
  printf '%s' "$out"
  return "$rc"
}

expect_green() {
  local name="$1" tree="$2" out rc=0
  out="$(run "$tree")" || rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "FAIL [$name]: pinned tree must pass, got rc=$rc:" >&2
    printf '%s\n' "$out" >&2
    failures=$((failures + 1))
  fi
}

# expect_red <name> <tree> <substring the failure must name>
expect_red() {
  local name="$1" tree="$2" needle="$3" out rc=0
  out="$(run "$tree")" || rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "FAIL [$name]: unpinned tree passed the guard — the guard is not fail-closed" >&2
    failures=$((failures + 1))
    return
  fi
  if [[ "$out" != *"$needle"* ]]; then
    echo "FAIL [$name]: failure did not name '$needle':" >&2
    printf '%s\n' "$out" >&2
    failures=$((failures + 1))
  fi
}

# --- fixture -----------------------------------------------------------------
# One tree carrying all four syntaxes, every reference pinned. Each red case
# below copies it and de-pins exactly one line, so a failure is attributable.
green="$tmp/green"
mkdir -p "$green/.github/workflows" "$green/svc"

cat >"$green/Dockerfile" <<EOF
FROM node:22-alpine@$DIGEST AS builder
RUN true
FROM node:22-alpine@$DIGEST
COPY --from=builder /app /app
EOF

cat >"$green/svc/images.go" <<EOF
package svc

const defaultRuntimeImage = "valkey/valkey:8-alpine@$DIGEST"

var versionImages = map[string]string{
	"7": "valkey/valkey:7-alpine@$DIGEST",
}

// A doc comment may quote a counter-example like "nginx:latest" without it
// becoming a reference the guard has to accept or reject.
func image() string { return defaultRuntimeImage }
EOF

cat >"$green/svc/deployment.yaml" <<EOF
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/example/app:v1.2.3@$DIGEST
          imagePullPolicy: Always
EOF

cat >"$green/.github/workflows/test.yml" <<EOF
jobs:
  test:
    services:
      postgres:
        image: postgres:17@$DIGEST
    steps:
      - name: start openfga
        run: |
          docker run -d --name openfga \\
            -p 8080:8080 \\
            openfga/openfga:latest@$DIGEST run
EOF

expect_green "pinned tree" "$green"

# --- red: one syntax at a time ----------------------------------------------
# depin <name> <file> <literal-to-remove> <replacement> — copy the green tree to
# $tmp/red-<name> and de-pin exactly one line of it. A fixture whose target line
# has moved aborts the suite: a red case that silently edits nothing would
# "prove" the guard red while testing the untouched green tree.
depin() {
  local name="$1" file="$2" from="$3" to="$4" tree="$tmp/red-$1"
  rm -rf "$tree"
  cp -R "$green" "$tree"
  if ! grep -qF -- "$from" "$tree/$file"; then
    echo "FAIL [$name]: fixture no longer contains '$from'" >&2
    exit 1
  fi
  FROM_LITERAL="$from" TO_LITERAL="$to" perl -pi \
    -e 'BEGIN { $f = $ENV{FROM_LITERAL}; $t = $ENV{TO_LITERAL} } s/\Q$f\E/$t/' "$tree/$file"
}

depin dockerfile Dockerfile "node:22-alpine@$DIGEST AS builder" "node:22-alpine AS builder"
expect_red "Dockerfile FROM floating tag" "$tmp/red-dockerfile" "node:22-alpine"

depin go svc/images.go "valkey/valkey:8-alpine@$DIGEST" "valkey/valkey:8-alpine"
expect_red "Go constant floating tag" "$tmp/red-go" "valkey/valkey:8-alpine"

# The map VALUE carries no "image" in its own text — only the enclosing
# declaration does. It must still be caught, or de-pinning one entry of a
# version map is invisible.
depin gomap svc/images.go "valkey/valkey:7-alpine@$DIGEST" "valkey/valkey:7-alpine"
expect_red "Go map value floating tag" "$tmp/red-gomap" "valkey/valkey:7-alpine"

depin manifest svc/deployment.yaml "ghcr.io/example/app:v1.2.3@$DIGEST" "ghcr.io/example/app:v1.2.3"
expect_red "manifest version-only tag" "$tmp/red-manifest" "ghcr.io/example/app:v1.2.3"

depin service .github/workflows/test.yml "postgres:17@$DIGEST" "postgres:17"
expect_red "CI service container version-only tag" "$tmp/red-service" "postgres:17"

depin dockerrun .github/workflows/test.yml "openfga/openfga:latest@$DIGEST" "openfga/openfga:latest"
expect_red "CI docker run floating tag" "$tmp/red-dockerrun" "openfga/openfga:latest"

# --- red: the fail-closed properties ----------------------------------------

# An unresolvable reference must FAIL, not be skipped. A build ARG is the
# canonical case: the guard cannot see what ${BASE} expands to, and "cannot see"
# is exactly how the inventory drifted through six review rounds.
tree="$tmp/red-arg"; rm -rf "$tree"; cp -R "$green" "$tree"
# shellcheck disable=SC2016 # a literal build ARG reference is the point
printf 'ARG BASE\nFROM ${BASE}\n' >>"$tree/Dockerfile"
expect_red "unparseable FROM is not skipped" "$tree" 'BASE'

# Exemptions are canonical-tree-only. A fixture that spells `controller:latest`
# must fail even though the real tree exempts that exact string — otherwise an
# exemption written for one placeholder silently covers every other file.
tree="$tmp/red-exempt"; rm -rf "$tree"; cp -R "$green" "$tree"
FROM_LITERAL="ghcr.io/example/app:v1.2.3@$DIGEST" perl -pi \
  -e 'BEGIN { $f = $ENV{FROM_LITERAL} } s/\Q$f\E/controller:latest/' "$tree/svc/deployment.yaml"
expect_red "exemptions do not leak into other trees" "$tree" "controller:latest"

# A scan that matches nothing is a broken scan, not a clean tree.
mkdir -p "$tmp/empty"
expect_red "empty scan fails rather than passing" "$tmp/empty" "no image references found"

# --- the canonical exemption list stays reviewable ---------------------------
# Every exemption row must carry a reason. A row with an empty third field would
# pass the guard while documenting nothing, which is how blanket exemptions
# start.
if awk -F'\t' 'NF < 3 || $3 == "" { bad = 1 } END { exit !bad }' \
    < <(sed -n '/^exemptions() {/,/^EOF$/p' scripts/image-pin-validate.sh | sed '1,2d; $d'); then
  echo "FAIL: an exemption row has no reason" >&2
  failures=$((failures + 1))
fi

if [ "$failures" -ne 0 ]; then
  echo "image-pin-validate self-test: $failures failure(s)" >&2
  exit 1
fi
echo "image-pin-validate self-test: green tree passes, every syntax fails red"
