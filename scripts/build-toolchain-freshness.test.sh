#!/usr/bin/env bash
# Self-test for scripts/build-toolchain-freshness.sh. Fixture trees prove
# fail-closed inventory validation, byte-identical resolve, exact old→new
# drift, and issue open/update/close/noop — without contacting a registry
# or GitHub.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$here/build-toolchain-freshness.sh"
root="$(cd "$here/.." && pwd)"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable" >&2; exit 1; }

fails=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

DIGEST_A='sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
DIGEST_B='sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
DIGEST_C='sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'

inventory_json() {
  local committed="${1:-$DIGEST_A}"
  cat <<EOF
{
  "schema": "bex.build-toolchain-freshness/v1",
  "images": [
    {
      "id": "cnb-builder",
      "kind": "builder",
      "upstream": "docker.io/example/builder:latest",
      "committed": "example/builder@${committed}",
      "resolved_at": "2026-08-18T00:00:00Z",
      "source": "fixture",
      "files": ["pin.go"]
    }
  ]
}
EOF
}

write_tree() {
  local dir="$1" committed="${2:-$DIGEST_A}"
  mkdir -p "$dir"
  inventory_json "$committed" >"$dir/inv.json"
  printf 'image := "example/builder@%s"\n' "$committed" >"$dir/pin.go"
}

assert() {
  local label="$1" want="$2"
  shift 2
  set +e
  local err
  err="$("$@" 2>&1 >/dev/null)"
  local got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $label — exit $got, want $want" >&2
    echo "$err" >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label (exit $got)"
}

assert_stdout() {
  local label="$1" needle="$2"
  shift 2
  local out
  out="$("$@")"
  if ! printf '%s' "$out" | grep -qF "$needle"; then
    echo "FAIL: $label — stdout did not contain '$needle'" >&2
    echo "$out" >&2
    fails=$((fails + 1))
    return
  fi
  echo "ok: $label"
}

# GREEN: the committed tree validates.
assert "canonical inventory validates" 0 env -u FRESHNESS_INVENTORY -u FRESHNESS_ROOT -u FRESHNESS_PIN_SITES "$SCRIPT" validate

# Canonical discovery must cover supplied derivatives without a fixture override.
write_tree "$tmp/supplied"
python3 - "$root/scripts/lib/toolchain-freshness.py" "$tmp/supplied" "$DIGEST_C" <<'PYFIXTURE'
import importlib.util, json, sys
from pathlib import Path
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("freshness_fixture", sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
fixture = Path(sys.argv[2])
for site in module.CANONICAL_PIN_SITES:
    target = fixture / site
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("")
native_site = "lego/operator/internal/build/nativebases.go"
(fixture / native_site).write_text('package build\nvar supplied = "ghcr.io/bex-co/bex-cnb-run@' + sys.argv[3] + '"\n')
inventory = json.loads((fixture / "inv.json").read_text())
inventory["images"].append(dict(inventory["images"][0], id="native-supplier", kind="native-base", upstream="ghcr.io/bex-co/bex-cnb-run:native-fixture", committed="ghcr.io/bex-co/bex-cnb-run@" + sys.argv[3], files=[native_site]))
(fixture / "covered.json").write_text(json.dumps(inventory))
PYFIXTURE
assert "canonical supplied-base digest needs inventory coverage" 1 \
  env -u FRESHNESS_PIN_SITES FRESHNESS_ROOT="$tmp/supplied" FRESHNESS_INVENTORY="$tmp/supplied/inv.json" "$SCRIPT" validate
assert "inventoried canonical supplied-base digest validates" 0 \
  env -u FRESHNESS_PIN_SITES FRESHNESS_ROOT="$tmp/supplied" FRESHNESS_INVENTORY="$tmp/supplied/covered.json" "$SCRIPT" validate

# The single common foundation is an independent upstream input, even while
# the operator's adopted common pin is empty.
printf 'ARG BASE_IMAGE=buildpack-deps:bookworm@%s\n' "$DIGEST_B" >"$tmp/supplied/deploy/native-common/Dockerfile"
assert "canonical common source needs inventory coverage" 1 \
  env -u FRESHNESS_PIN_SITES FRESHNESS_ROOT="$tmp/supplied" FRESHNESS_INVENTORY="$tmp/supplied/covered.json" "$SCRIPT" validate
python3 - "$tmp/supplied" "$DIGEST_B" <<'PYFIXTURE'
import json, sys
from pathlib import Path
fixture = Path(sys.argv[1])
inventory = json.loads((fixture / "covered.json").read_text())
inventory["images"].append(dict(inventory["images"][0], id="native-common-source", kind="native-base", upstream="docker.io/library/buildpack-deps:bookworm", committed="buildpack-deps:bookworm@" + sys.argv[2], files=["deploy/native-common/Dockerfile"]))
(fixture / "common-covered.json").write_text(json.dumps(inventory))
PYFIXTURE
assert "inventoried canonical common source validates" 0 \
  env -u FRESHNESS_PIN_SITES FRESHNESS_ROOT="$tmp/supplied" FRESHNESS_INVENTORY="$tmp/supplied/common-covered.json" "$SCRIPT" validate

# RED: missing coverage of a pin-site digest.
write_tree "$tmp/uncovered"
printf 'extra := "other@%s"\n' "$DIGEST_C" >>"$tmp/uncovered/pin.go"
assert "uncovered pin-site digest fails" 1 \
  env FRESHNESS_ROOT="$tmp/uncovered" FRESHNESS_INVENTORY="$tmp/uncovered/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" "$SCRIPT" validate

# RED: listed file lacks the committed digest.
write_tree "$tmp/mismatch"
printf 'image := "example/builder@%s"\n' "$DIGEST_B" >"$tmp/mismatch/pin.go"
assert "committed digest missing from file fails" 1 \
  env FRESHNESS_ROOT="$tmp/mismatch" FRESHNESS_INVENTORY="$tmp/mismatch/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" "$SCRIPT" validate

# RED: malformed metadata.
printf '{"schema":"nope","images":[]}\n' >"$tmp/bad.json"
assert "malformed schema fails" 1 \
  env FRESHNESS_INVENTORY="$tmp/bad.json" FRESHNESS_ROOT="$tmp" FRESHNESS_PIN_SITES="" "$SCRIPT" validate

printf '{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/example/builder:latest","committed":"example/builder@sha256:dead","resolved_at":"2026-08-18T00:00:00Z","source":"x","files":["pin.go"]}]}\n' >"$tmp/shortdigest.json"
assert "short digest fails" 1 \
  env FRESHNESS_INVENTORY="$tmp/shortdigest.json" FRESHNESS_ROOT="$tmp" FRESHNESS_PIN_SITES="" "$SCRIPT" validate

# GREEN: resolve against a fixture map is byte-identical across runs.
write_tree "$tmp/stable"
printf '{"docker.io/example/builder:latest":"%s"}\n' "$DIGEST_A" >"$tmp/stable/digests.json"
run_resolve() {
  env FRESHNESS_ROOT="$tmp/stable" FRESHNESS_INVENTORY="$tmp/stable/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/stable/digests.json" \
      "$SCRIPT" resolve
}
run_resolve >"$tmp/r1.json"
run_resolve >"$tmp/r2.json"
if ! cmp -s "$tmp/r1.json" "$tmp/r2.json"; then
  echo "FAIL: repeated resolve is not byte-identical" >&2
  fails=$((fails + 1))
else
  echo "ok: repeated resolve is byte-identical"
fi
assert_stdout "unchanged resolve reports changed=false" '"changed": false' cat "$tmp/r1.json"

# Drift: exact old → new replacement, worktree untouched.
printf '{"docker.io/example/builder:latest":"%s"}\n' "$DIGEST_B" >"$tmp/stable/digests.json"
env FRESHNESS_ROOT="$tmp/stable" FRESHNESS_INVENTORY="$tmp/stable/inv.json" \
    FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/stable/digests.json" \
    "$SCRIPT" resolve >"$tmp/drift.json"
assert_stdout "drift names committed digest" "$DIGEST_A" cat "$tmp/drift.json"
assert_stdout "drift names observed digest" "$DIGEST_B" cat "$tmp/drift.json"
assert_stdout "drift sets changed=true" '"changed": true' cat "$tmp/drift.json"
if ! grep -q "$DIGEST_A" "$tmp/stable/pin.go"; then
  echo "FAIL: resolve mutated the pin file" >&2
  fails=$((fails + 1))
else
  echo "ok: resolve does not edit the worktree"
fi

# ghcr.io upstream (w1/m169 `cnb-run-image`): parse to the ghcr.io registry and
# resolve through its anonymous token + manifest endpoints, not Docker Hub's.
# The HTTP layer is stubbed, so this exercises the real parse/resolve path
# without contacting a registry.
if ghcr_out="$(python3 - "$here/lib/toolchain-freshness.py" <<'PY' 2>&1
import importlib.util, sys
spec = importlib.util.spec_from_file_location("freshness", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
ref = "ghcr.io/bex-co/bex-cnb-run:latest"
got = m.parse_upstream(ref)
assert got == ("ghcr.io", "bex-co/bex-cnb-run", "latest"), got
calls = []
def fake_json(url, headers=None):
    calls.append(url)
    return {"token": "anon"}
def fake_digest(url, headers):
    calls.append(url)
    assert headers.get("Authorization") == "Bearer anon", headers
    return "sha256:" + "d" * 64
m.http_json, m.http_digest = fake_json, fake_digest
assert m.resolve_upstream(ref) == "sha256:" + "d" * 64
assert calls == [
    "https://ghcr.io/token?service=ghcr.io&scope=repository%3Abex-co%2Fbex-cnb-run%3Apull",
    "https://ghcr.io/v2/bex-co/bex-cnb-run/manifests/latest",
], calls
PY
)"; then
  echo "ok: ghcr.io upstream parses and resolves via ghcr.io token + manifest"
else
  echo "FAIL: ghcr.io upstream parse/resolve — $ghcr_out" >&2
  fails=$((fails + 1))
fi

# npm-package upstream (w8/m73 `native-pnpm`): resolve the dist-tag through
# the npm registry and hash the tarball it names — never an image registry.
if npm_out="$(python3 - "$here/lib/toolchain-freshness.py" <<'PY' 2>&1
import importlib.util, sys
spec = importlib.util.spec_from_file_location("freshness", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
calls = []
def fake_json(url, headers=None):
    calls.append(url)
    return {"dist": {"tarball": "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz"}}
def fake_sha256(url):
    calls.append(url)
    return "sha256:" + "e" * 64
def no_image_registry(*args):
    raise AssertionError("npm-package resolved through an image registry")
m.http_json, m.http_sha256, m.http_digest = fake_json, fake_sha256, no_image_registry
assert m.resolve_upstream("registry.npmjs.org/pnpm:latest-10") == "sha256:" + "e" * 64
assert calls == [
    "https://registry.npmjs.org/pnpm/latest-10",
    "https://registry.npmjs.org/pnpm/-/pnpm-10.0.0.tgz",
], calls
PY
)"; then
  echo "ok: npm-package upstream resolves via npm dist-tag + tarball sha256"
else
  echo "FAIL: npm-package upstream resolve — $npm_out" >&2
  fails=$((fails + 1))
fi

# release-asset upstream (w8/m106 `native-hugo-*`): hash the GitHub release
# asset the tag serves — never an image registry.
if asset_out="$(python3 - "$here/lib/toolchain-freshness.py" <<'PY' 2>&1
import importlib.util, sys
spec = importlib.util.spec_from_file_location("freshness", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
calls = []
def fake_sha256(url):
    calls.append(url)
    return "sha256:" + "f" * 64
def no_image_registry(*args):
    raise AssertionError("release-asset resolved through an image registry")
m.http_sha256, m.http_digest, m.http_json = fake_sha256, no_image_registry, no_image_registry
ref = "github.com/gohugoio/hugo/hugo_extended_0.111.3_linux-amd64.tar.gz:v0.111.3"
assert m.resolve_upstream(ref) == "sha256:" + "f" * 64
assert calls == [
    "https://github.com/gohugoio/hugo/releases/download/v0.111.3/hugo_extended_0.111.3_linux-amd64.tar.gz",
], calls
try:
    m.resolve_upstream("github.com/gohugoio/hugo:v0.111.3")
except RuntimeError:
    pass
else:
    raise AssertionError("an asset-less release-asset ref resolved")
PY
)"; then
  echo "ok: release-asset upstream resolves via the GitHub release download sha256"
else
  echo "FAIL: release-asset upstream resolve — $asset_out" >&2
  fails=$((fails + 1))
fi

body="$(env "$SCRIPT" issue-body "$tmp/drift.json")"
if ! printf '%s' "$body" | grep -qF "$DIGEST_A" || ! printf '%s' "$body" | grep -qF "$DIGEST_B"; then
  echo "FAIL: issue body omitted exact digests" >&2
  fails=$((fails + 1))
else
  echo "ok: issue body carries exact replacements"
fi
if ! printf '%s' "$body" | grep -qF 'never edits, commits, or merges'; then
  echo "FAIL: issue body must say the workflow never applies a pin" >&2
  fails=$((fails + 1))
else
  echo "ok: issue body is a review request"
fi

action() { env EXISTING_ISSUE="${1:-}" "$SCRIPT" issue-action "$2"; }
[ "$(action '' "$tmp/drift.json")" = open ] || { echo "FAIL: new drift should open" >&2; fails=$((fails + 1)); }
[ "$(action '12' "$tmp/drift.json")" = update ] || { echo "FAIL: repeated drift should update" >&2; fails=$((fails + 1)); }
[ "$(action '' "$tmp/r1.json")" = noop ] || { echo "FAIL: no-drift without issue should noop" >&2; fails=$((fails + 1)); }
[ "$(action '12' "$tmp/r1.json")" = close ] || { echo "FAIL: no-drift with issue should close" >&2; fails=$((fails + 1)); }
echo "ok: issue open/update/close/noop decisions"

wf="$root/.github/workflows/build-toolchain-freshness.yml"
if grep -Eq '\$\{\{[[:space:]]*secrets\.|git (commit|push)|gh pr ' "$wf"; then
  echo "FAIL: freshness workflow must not edit the tree or reference production secrets" >&2
  fails=$((fails + 1))
else
  echo "ok: freshness workflow is issue-only and secret-free"
fi
if ! grep -q 'permissions:' "$wf" || ! grep -q 'contents: read' "$wf" || ! grep -q 'issues: write' "$wf"; then
  echo "FAIL: freshness workflow permissions must be contents:read + issues:write" >&2
  fails=$((fails + 1))
else
  echo "ok: freshness workflow permissions are least privilege"
fi

if ! grep -q '!operator/internal/build/toolchain-freshness.json' "$root/lego/.dockerignore"; then
  echo "FAIL: image build context must re-include the embedded freshness inventory" >&2
  fails=$((fails + 1))
else
  echo "ok: dockerignore admits the embedded freshness inventory"
fi

# Line staleness (w5/m144): a Go pin one minor behind upstream's newest line is
# drift even when its own tag's digest has not moved; a current line is not.
mkdir -p "$tmp/line"
printf '{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/example/builder:latest","committed":"example/builder@%s","resolved_at":"2026-10-20T00:00:00Z","source":"x","files":["pin.go"]},{"id":"native-go","kind":"native-base","upstream":"docker.io/library/golang:1.27-bookworm","committed":"golang:1.27-bookworm@%s","resolved_at":"2026-10-20T00:00:00Z","source":"x","files":["pin.go"]}]}\n' "$DIGEST_A" "$DIGEST_B" >"$tmp/line/inv.json"
printf 'a := "example/builder@%s"\nb := "golang:1.27-bookworm@%s"\n' "$DIGEST_A" "$DIGEST_B" >"$tmp/line/pin.go"
printf '{"docker.io/example/builder:latest":"%s","docker.io/library/golang:1.27-bookworm":"%s"}\n' "$DIGEST_A" "$DIGEST_B" >"$tmp/line/digests.json"
line_resolve() {
  printf '{"native-go":"%s"}\n' "$1" >"$tmp/line/lines.json"
  env FRESHNESS_ROOT="$tmp/line" FRESHNESS_INVENTORY="$tmp/line/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/line/digests.json" \
      FRESHNESS_LATEST_LINES="$tmp/line/lines.json" "$SCRIPT" resolve
}
line_resolve 1.28 >"$tmp/line/behind.json"
assert_stdout "a Go pin one minor behind is drift" '"newer_line": "1.28"' cat "$tmp/line/behind.json"
assert_stdout "a Go pin one minor behind sets changed=true" '"changed": true' cat "$tmp/line/behind.json"
line_resolve 1.27 >"$tmp/line/current.json"
if grep -q '"changed": true' "$tmp/line/current.json"; then
  echo "FAIL: a current Go line is not drift" >&2
  fails=$((fails + 1))
else
  echo "ok: a current Go line is not drift"
fi
# Lines compare numerically: 1.3 is older than 1.27, not newer.
line_resolve 1.3 >"$tmp/line/older.json"
if grep -q '"changed": true' "$tmp/line/older.json"; then
  echo "FAIL: an upstream line of 1.3 is not newer than a 1.27 pin" >&2
  fails=$((fails + 1))
else
  echo "ok: lines compare numerically, not as strings"
fi
# A feed that answers the wrong shape is a warning, not a failed resolve.
printf '{"native-go":["1.28"]}\n' >"$tmp/line/bad.json"
if env FRESHNESS_ROOT="$tmp/line" FRESHNESS_INVENTORY="$tmp/line/inv.json" \
    FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/line/digests.json" \
    FRESHNESS_LATEST_LINES="$tmp/line/bad.json" "$SCRIPT" resolve >/dev/null 2>"$tmp/line/bad.err"; then
  echo "ok: a malformed line feed does not fail the resolve"
else
  echo "FAIL: a malformed line feed failed the resolve: $(cat "$tmp/line/bad.err")" >&2
  fails=$((fails + 1))
fi

# Missing release lines (w8/335): Python 3.15 shipped while the pinned lines
# stopped at 3.14. A released line newer than every pinned one is flagged on
# the newest pinned entry only; an exact-patch pin is no newer line.
mkdir -p "$tmp/release"
python3 - "$tmp/release" "$DIGEST_A" "$DIGEST_B" "$DIGEST_C" <<'PYFIXTURE'
import json, sys
from pathlib import Path
root, a, b, c = Path(sys.argv[1]), *sys.argv[2:]
entries = [("cnb-builder", "builder", "docker.io/example/builder:latest", "example/builder", a),
           ("native-python", "native-base", "docker.io/library/python:3.13-bookworm", "python:3.13-bookworm", b),
           ("native-python-3-14", "native-base", "docker.io/library/python:3.14-bookworm", "python:3.14-bookworm", c),
           ("native-node-26", "native-base", "docker.io/library/node:26-bookworm", "node:26-bookworm", a),
           ("native-node-14-21-2", "native-base", "docker.io/library/node:14.21.2-bullseye", "node:14.21.2-bullseye", b)]
images = [{"id": ident, "kind": kind, "upstream": upstream, "committed": name + "@" + digest,
           "resolved_at": "2026-10-20T00:00:00Z", "source": "x", "files": ["pin.go"]}
          for ident, kind, upstream, name, digest in entries]
(root / "inv.json").write_text(json.dumps({"schema": "bex.build-toolchain-freshness/v1", "images": images}))
(root / "pin.go").write_text("".join(f'"{image["committed"]}"\n' for image in images))
(root / "digests.json").write_text(json.dumps({image["upstream"]: image["committed"].split("@")[1] for image in images}))
PYFIXTURE
release_resolve() {
  printf '{"docker.io/library/python":"%s","docker.io/library/node":"%s"}\n' "$1" "$2" >"$tmp/release/lines.json"
  env FRESHNESS_ROOT="$tmp/release" FRESHNESS_INVENTORY="$tmp/release/inv.json" \
      FRESHNESS_PIN_SITES="pin.go" FRESHNESS_DIGESTS="$tmp/release/digests.json" \
      FRESHNESS_LATEST_LINES="$tmp/release/lines.json" "$SCRIPT" resolve
}
release_resolve 3.15.2 26.13.1 >"$tmp/release/missing.json"
if python3 - "$tmp/release/missing.json" <<'PYCHECK'
import json, sys
report = {image["id"]: image for image in json.load(open(sys.argv[1]))["images"]}
flagged = {ident: image["missing_line"] for ident, image in report.items() if "missing_line" in image}
changed = {ident for ident, image in report.items() if image["changed"]}
sys.exit(0 if flagged == {"native-python-3-14": "3.15"} and changed == {"native-python-3-14"} else 1)
PYCHECK
then
  echo "ok: a released Python line newer than every pinned one is flagged on the newest pin"
else
  echo "FAIL: missing Python 3.15 line not flagged on native-python-3-14 alone: $(cat "$tmp/release/missing.json")" >&2
  fails=$((fails + 1))
fi
assert_stdout "the issue body asks for the missing line" 'released line missing: `3.15`' \
  "$SCRIPT" issue-body "$tmp/release/missing.json"
release_resolve 3.14.10 27.0.0 >"$tmp/release/node.json"
assert_stdout "a released Node major newer than every pinned one is flagged" '"missing_line": "27"' cat "$tmp/release/node.json"
# Lines compare numerically: 3.9 is older than 3.14.
release_resolve 3.9.25 26.13.1 >"$tmp/release/current.json"
if grep -q '"changed": true' "$tmp/release/current.json"; then
  echo "FAIL: no released line is newer than the pinned ones" >&2
  fails=$((fails + 1))
else
  echo "ok: released lines compare numerically, and current ones are not drift"
fi

# A reviewed line's version field must be the patch its image carries
# (w3/m635): resolve reads the patch from the committed image's config and
# reports a line whose field disagrees, in the issue too. Elixir records its
# own as v1.18.6.
mkdir -p "$tmp/version"
write_tree "$tmp/version"
python3 - "$tmp/version" "$DIGEST_B" "$DIGEST_C" <<'PYFIXTURE'
import json, sys
from pathlib import Path
fixture, ruby, elixir = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
native = fixture / "lego/operator/internal/build/native.go"
native.parent.mkdir(parents=True)
native.write_text(
    '\t{line: "3.4", version: "3.4.12", image: "ruby:3.4-bookworm@' + ruby + '"},\n'
    '\t{line: "1.18", version: "1.18.6", otp: "28", image: "elixir:1.18@' + elixir + '", debian: "trixie"},\n')
inventory = json.loads((fixture / "inv.json").read_text())
base = inventory["images"][0]
inventory["images"] += [
    dict(base, id="native-ruby", kind="native-base", upstream="docker.io/library/ruby:3.4-bookworm",
         committed="ruby:3.4-bookworm@" + ruby, files=[str(native.relative_to(fixture))]),
    dict(base, id="native-elixir", kind="native-base", upstream="docker.io/library/elixir:1.18",
         committed="elixir:1.18@" + elixir, files=[str(native.relative_to(fixture))]),
]
(fixture / "inv.json").write_text(json.dumps(inventory))
(fixture / "digests.json").write_text(json.dumps({
    "docker.io/example/builder:latest": base["committed"].split("@")[1],
    "docker.io/library/ruby:3.4-bookworm": ruby, "docker.io/library/elixir:1.18": elixir}))
PYFIXTURE
version_resolve() {
  printf '{"%s":"%s","%s":"v1.18.6"}\n' "$DIGEST_B" "$1" "$DIGEST_C" >"$tmp/version/versions.json"
  env FRESHNESS_ROOT="$tmp/version" FRESHNESS_INVENTORY="$tmp/version/inv.json" \
      FRESHNESS_PIN_SITES=$'pin.go\nlego/operator/internal/build/native.go' \
      FRESHNESS_DIGESTS="$tmp/version/digests.json" FRESHNESS_IMAGE_VERSIONS="$tmp/version/versions.json" \
      "$SCRIPT" resolve
}
version_resolve 3.4.13 >"$tmp/version/stale.json"
if python3 - "$tmp/version/stale.json" <<'PYCHECK'
import json, sys
report = {image["id"]: image for image in json.load(open(sys.argv[1]))["images"]}
assert report["native-ruby"]["changed"] is True, report["native-ruby"]
assert report["native-ruby"]["version_mismatch"] == {"field": "3.4.12", "image": "3.4.13"}, report["native-ruby"]
assert not report["native-elixir"]["changed"] and "version_mismatch" not in report["native-elixir"], report["native-elixir"]
assert not report["cnb-builder"]["changed"], report["cnb-builder"]
PYCHECK
then
  echo "ok: a version field the image does not carry is drift, and v1.18.6 is 1.18.6"
else
  echo "FAIL: a stale version field was not flagged on its line alone: $(cat "$tmp/version/stale.json")" >&2
  fails=$((fails + 1))
fi
assert_stdout "the issue body names the stale version field" 'version field `3.4.12`, but the committed image carries `3.4.13`' \
  "$SCRIPT" issue-body "$tmp/version/stale.json"
version_resolve 3.4.12 >"$tmp/version/current.json"
if grep -q '"changed": true' "$tmp/version/current.json"; then
  echo "FAIL: version fields that match their images are drift: $(cat "$tmp/version/current.json")" >&2
  fails=$((fails + 1))
else
  echo "ok: version fields that match their images are not drift"
fi
# The registry path reads the linux/amd64 config: an Env variable, or Bun's
# label; a repository that records no patch is never read.
if version_out="$(python3 - "$here/lib/toolchain-freshness.py" <<'PY' 2>&1
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("freshness", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
for key in ("FRESHNESS_DIGESTS", "FRESHNESS_IMAGE_VERSIONS"):
    os.environ.pop(key, None)
configs = {
    "ruby": {"Env": ["PATH=/usr/local/bin", "RUBY_VERSION=3.4.13"]},
    "bun": {"Env": ["PATH=/usr/local/bin"], "Labels": {"org.opencontainers.image.version": "1.4.5"}},
}
calls = []
def fake_json(url, headers=None):
    calls.append(url)
    if "/token" in url:
        return {"token": "anon"}
    name = "bun" if "oven/bun" in url else "ruby"
    if url.endswith("/manifests/sha256:" + "a" * 64):
        return {"manifests": [
            {"digest": "sha256:arm", "platform": {"os": "linux", "architecture": "arm64"}},
            {"digest": "sha256:amd", "platform": {"os": "linux", "architecture": "amd64"}}]}
    if url.endswith("/manifests/sha256:amd"):
        return {"config": {"digest": "sha256:cfg"}}
    if url.endswith("/blobs/sha256:cfg"):
        return {"config": configs[name]}
    raise AssertionError(url)
m.http_json = fake_json
digest = "sha256:" + "a" * 64
assert m.image_version("docker.io/library/ruby:3.4-bookworm", digest) == "3.4.13"
assert not any("sha256:arm" in url for url in calls), calls
assert m.image_version("docker.io/oven/bun:1.4", digest) == "1.4.5"
calls.clear()
assert m.image_version("docker.io/library/golang:1.27-bookworm", digest) is None and calls == [], calls
PY
)"; then
  echo "ok: image versions come from the linux/amd64 config's Env or label"
else
  echo "FAIL: image version read — $version_out" >&2
  fails=$((fails + 1))
fi

# The native Go line moves itself (w4/719): go-release plans a bump once
# go.dev's newest stable release is newer than the pinned image's Go and the
# official golang:<line>-bookworm index carries it on both platforms;
# go-apply rewrites native.go's Go entry and the inventory, nothing else.
DIGEST_D='sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd'
mkdir -p "$tmp/go"
python3 - "$root/scripts/lib/toolchain-freshness.py" "$tmp/go" "$root/lego/operator/internal/build/native.go" "$DIGEST_A" "$DIGEST_B" <<'PYFIXTURE'
import importlib.util, json, sys
from pathlib import Path
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("freshness_fixture", sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
root, real, a, b = Path(sys.argv[2]), Path(sys.argv[3]), *sys.argv[4:]
inv = {"schema": "bex.build-toolchain-freshness/v1", "images": [
    {"id": "cnb-builder", "kind": "builder", "upstream": "docker.io/example/builder:latest",
     "committed": "example/builder@" + a, "resolved_at": "2026-10-20T00:00:00Z", "source": "x", "files": ["pin.go"]},
    {"id": "native-go", "kind": "native-base", "upstream": "docker.io/library/golang:1.27-bookworm",
     "committed": "golang:1.27-bookworm@" + b, "resolved_at": "2026-10-20T00:00:00Z", "source": "x", "files": ["native.go"]}]}
(root / "inv.json").write_text(json.dumps(inv, indent=2) + "\n")
(root / "pin.go").write_text(f'a := "example/builder@{a}"\n')
# The real native.go, with its Go entry pointed at the fixture pin, so a
# change to that entry's shape fails here before it fails a scheduled bump.
text, count = module.GO_ENTRY_RE.subn(
    lambda m: f"{m.group(1)}1.27{m.group(2)}1.27{m.group(3)}golang:1.27-bookworm@{b}{m.group(4)}", real.read_text())
if count != 1:
    sys.exit("FAIL: native.go's Go entry is not in GO_ENTRY_RE's shape, so go-apply could not bump it")
(root / "native.go").write_text(text)
PYFIXTURE
cp "$tmp/go/native.go" "$tmp/go/native.go.orig"
cp "$tmp/go/inv.json" "$tmp/go/inv.json.orig"
printf '{"docker.io/library/golang:1.27-bookworm":"%s","docker.io/library/golang:1.28-bookworm":"%s"}\n' "$DIGEST_C" "$DIGEST_D" >"$tmp/go/digests.json"
printf '{"%s":"1.27.4","%s":"1.27.5","%s":"1.28.0"}\n' "$DIGEST_B" "$DIGEST_C" "$DIGEST_D" >"$tmp/go/versions.json"
go_env() {
  env FRESHNESS_ROOT="$tmp/go" FRESHNESS_INVENTORY="$tmp/go/inv.json" FRESHNESS_PIN_SITES="pin.go" \
      FRESHNESS_DIGESTS="$tmp/go/digests.json" FRESHNESS_GO_VERSIONS="$tmp/go/versions.json" \
      FRESHNESS_LATEST_LINES="$tmp/go/latest.json" FRESHNESS_NOW="2026-11-23T12:00:00Z" "$@"
}
go_plan() {
  printf '{"native-go":"%s"}\n' "$1" >"$tmp/go/latest.json"
  go_env "$SCRIPT" go-release
}
plan_field() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get(sys.argv[2], ""))' "$@"; }
check_plan() {
  local label="$1" latest="$2" action="$3" image="${4:-}"
  go_plan "$latest" >"$tmp/go/plan.json" 2>"$tmp/go/plan.err" || { echo "FAIL: $label — go-release failed: $(cat "$tmp/go/plan.err")" >&2; fails=$((fails + 1)); return; }
  local got_action got_image
  got_action="$(plan_field "$tmp/go/plan.json" action)"
  got_image="$(plan_field "$tmp/go/plan.json" image)"
  if [ "$got_action" != "$action" ] || [ "$got_image" != "$image" ]; then
    echo "FAIL: $label — action $got_action image '$got_image', want $action '$image'" >&2
    fails=$((fails + 1))
  else
    echo "ok: $label"
  fi
}
check_plan "the pinned image's Go is go.dev's newest: current" go1.27.4 current
check_plan "an older feed release is no bump" go1.26.11 current
check_plan "a new patch is a bump within the line" go1.27.5 bump "golang:1.27-bookworm@$DIGEST_C"
check_plan "a new minor moves to its line" go1.28.0 bump "golang:1.28-bookworm@$DIGEST_D"
# Go's name for x.y.0 before 1.21 was "go1.y"; 1.28 is 1.28.0.
check_plan "a release named without its patch is .0" go1.28 bump "golang:1.28-bookworm@$DIGEST_D"
check_plan "a release with no official image yet waits" go1.29.0 wait
printf '{"%s":"1.27.4","%s":{"linux/amd64":"1.27.5","linux/arm64":"1.27.4"}}\n' "$DIGEST_B" "$DIGEST_C" >"$tmp/go/versions.json"
check_plan "an image one platform has not caught up on waits" go1.27.5 wait
printf '{"%s":"1.27.4","%s":"1.27.5","%s":"1.28.0"}\n' "$DIGEST_B" "$DIGEST_C" "$DIGEST_D" >"$tmp/go/versions.json"
assert "a release name go.dev would never publish fails the plan" 1 go_plan go1.28rc1

go_plan go1.28.0 >"$tmp/go/bump.json"
assert "go-apply rewrites the pin to a bump plan" 0 go_env "$SCRIPT" go-apply "$tmp/go/bump.json"
python3 - "$tmp/go" "$DIGEST_B" "$DIGEST_D" <<'PYCHECK' && echo "ok: go-apply moves the line, its default, the digest and the inventory entry, nothing else" || fails=$((fails + 1))
import difflib, json, sys
from pathlib import Path
root, b, d = Path(sys.argv[1]), *sys.argv[2:]
before, after = (root / "native.go.orig").read_text(), (root / "native.go").read_text()
changed = [l for l in difflib.unified_diff(before.splitlines(), after.splitlines(), lineterm="", n=0)
           if l[:1] in "+-" and l[:3] not in ("+++", "---")]
want = ['-\t"go": {label: "Go", defaultLine: "1.27", lines: []nativeLine{',
        f'-\t\t{{line: "1.27", image: "golang:1.27-bookworm@{b}"}},',
        '+\t"go": {label: "Go", defaultLine: "1.28", lines: []nativeLine{',
        f'+\t\t{{line: "1.28", image: "golang:1.28-bookworm@{d}"}},']
if changed != want:
    sys.exit(f"FAIL: native.go diff {changed!r}")
old = json.loads((root / "inv.json.orig").read_text())["images"]
new = json.loads((root / "inv.json").read_text())["images"]
if old[0] != new[0]:
    sys.exit("FAIL: go-apply touched another inventory entry")
go = new[1]
want = {"upstream": "docker.io/library/golang:1.28-bookworm", "committed": "golang:1.28-bookworm@" + d,
        "resolved_at": "2026-11-23T12:00:00Z"}
if any(go[k] != v for k, v in want.items()) or "Go 1.28.0 (was 1.27.4)" not in go["source"]:
    sys.exit(f"FAIL: native-go entry {go!r}")
if list(go) != list(old[1]) or (root / "inv.json").read_text() != json.dumps(json.loads((root / "inv.json").read_text()), indent=2) + "\n":
    sys.exit("FAIL: the inventory lost its committed layout")
PYCHECK
assert "the bumped tree validates" 0 go_env "$SCRIPT" validate
check_plan "after the bump the pin is current" go1.28.0 current
cp "$tmp/go/native.go" "$tmp/go/native.go.bumped"
assert_stdout "the same plan again is a no-op" "noop: native Go already pins" go_env "$SCRIPT" go-apply "$tmp/go/bump.json"
assert_stdout "a plan made before main moved is a no-op" "noop: native Go moved" \
  go_env "$SCRIPT" go-apply <(sed "s/1.28-bookworm@$DIGEST_D/1.28-bookworm@$DIGEST_C/" "$tmp/go/bump.json")
cmp -s "$tmp/go/native.go" "$tmp/go/native.go.bumped" && echo "ok: a no-op leaves native.go alone" \
  || { echo "FAIL: a no-op go-apply rewrote native.go" >&2; fails=$((fails + 1)); }
cp "$tmp/go/native.go.orig" "$tmp/go/native.go"
cp "$tmp/go/inv.json.orig" "$tmp/go/inv.json"
go_plan go1.27.4 >"$tmp/go/current.json"
assert "go-apply refuses a plan that is not a bump" 1 go_env "$SCRIPT" go-apply "$tmp/go/current.json"
sed 's/"golang:1.28-bookworm@/"golang:1.28-alpine@/' "$tmp/go/bump.json" >"$tmp/go/alpine.json"
assert "go-apply refuses an image that is not the line's bookworm tag" 1 go_env "$SCRIPT" go-apply "$tmp/go/alpine.json"
sed 's/"line": "1.28"/"line": "1.29"/' "$tmp/go/bump.json" >"$tmp/go/offline.json"
assert "go-apply refuses a line its image is not tagged with" 1 go_env "$SCRIPT" go-apply "$tmp/go/offline.json"
cmp -s "$tmp/go/native.go" "$tmp/go/native.go.orig" && echo "ok: a refused plan leaves native.go alone" \
  || { echo "FAIL: a refused go-apply rewrote native.go" >&2; fails=$((fails + 1)); }

# The bump workflow is the one writer: main-only, no secret, contents:write on
# the commit job alone, and it pushes only after both smoke builds and the
# checks pass.
bump_wf="$root/.github/workflows/go-line-bump.yml"
python3 - "$bump_wf" <<'PYCHECK' && echo "ok: go-line-bump.yml writes only after its smoke builds and checks" || fails=$((fails + 1))
import re, sys
text = open(sys.argv[1]).read()
problems = []
if "secrets." in text:
    problems.append("references a secret")
if re.search(r"(?m)^permissions:\n  contents: read\n\n", text) is None:
    problems.append("workflow permissions must be contents: read")
jobs = re.split(r"(?m)^  (?=[a-z0-9-]+:\n)", text.split("\njobs:\n", 1)[1])[1:]
writers = [job.split(":", 1)[0] for job in jobs if "contents: write" in job]
if writers != ["bump"]:
    problems.append(f"contents: write on {writers}, want the bump job alone")
for job in jobs:
    name = job.split(":", 1)[0]
    if "github.ref == 'refs/heads/main'" not in job and name in ("plan", "bump"):
        problems.append(f"{name} is not main-only")
bump = next((job for job in jobs if job.startswith("bump:")), "")
if "needs: [plan, smoke-arm64, smoke-amd64]" not in bump:
    problems.append("bump must need plan and both smoke builds")
order = [bump.find(s) for s in ("go-apply", "toolchain-freshness.py validate", "image-pin-validate.sh", "go test ./internal/build/", "git push")]
if -1 in order or order != sorted(order):
    problems.append("bump must apply, validate, run the pin guard and the build tests, then push")
for name, runner in (("smoke-arm64", "ubuntu-24.04-arm"), ("smoke-amd64", "ubuntu-latest")):
    job = next((job for job in jobs if job.startswith(name + ":")), "")
    if f"runs-on: {runner}" not in job or "scripts/go-line-smoke.sh" not in job:
        problems.append(f"{name} must run go-line-smoke.sh on {runner}")
if problems:
    sys.exit("FAIL: go-line-bump.yml: " + "; ".join(problems))
PYCHECK

if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails build-toolchain freshness checks" >&2
  exit 1
fi
echo "PASS: build-toolchain-freshness.sh"
