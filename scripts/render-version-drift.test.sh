#!/usr/bin/env bash
# Unit tests for render-version-drift.sh (w3/m637). Network-free: the docs base
# URL is a file:// URL to Render's seven version pages and a trimmed llms.txt,
# saved byte for byte (scripts/fixtures/render-version-pages/). The real pages
# must pass against bex's tables, and each kind of change Render makes must fail
# with a message naming what changed: a moved current default, a new row, a
# back-dated row, a removed page, changed markup and an eighth page. The check
# runs the operator's TestRenderVersionDrift, so it needs Go.
#
# When bex's tables follow a Render move, refresh the pages:
#   for p in bun-version elixir-erlang-versions node-version poetry-version \
#     python-version ruby-version uv-version; do
#     curl -fsSL "https://render.com/docs/$p.md" -o "scripts/fixtures/render-version-pages/$p.md"
#   done
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-version-drift.sh"
fixtures="$repo_root/scripts/fixtures/render-version-pages"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -12 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

tmp="${TMPDIR:-/tmp}"
sandbox="$(mktemp -d "${tmp%/}/bex-render-version-drift-test.XXXXXX")"
trap 'rm -rf "$sandbox"' EXIT
mkdir "$sandbox/tmp"

# run_with <pages dir> [mode]: the script against these pages, with its
# temporary directory inside the sandbox so the cleanup can be asserted.
run_with() {
  TMPDIR="$sandbox/tmp" RENDER_DOCS_URL="file://$1" "$script" "${2:-check}"
}

# pages_with <name> <file> <sed expression>: a copy of the fixture pages with
# one file edited; prints the copy's directory.
pages_with() {
  local dir="$sandbox/$1"
  cp -R "$fixtures" "$dir"
  sed -e "$3" "$fixtures/$2" >"$dir/$2"
  if cmp -s "$fixtures/$2" "$dir/$2"; then
    echo "pages_with $1: the edit changed nothing in $2" >&2
    exit 1
  fi
  echo "$dir"
}

# rows <json> <page>: "<from> <version>" per history row, then the defaults.
rows() {
  python3 -I -c '
import json, sys
page = json.load(open(sys.argv[1]))["pages"][sys.argv[2]]
for row in page["rows"]:
    print(row.get("from") or "before " + row["before"], row["version"])
print("defaults", " ".join(f"{k}={v}" for k, v in sorted(page["defaults"].items())), "since", page["since"])
' "$1" "$2"
}

# The real pages, parsed: the rows read on 2026-11-25 when this was filed.
check "the real pages parse" 0 "" run_with "$fixtures" parse
run_with "$fixtures" parse >"$sandbox/pages.json"
check "Bun 1.4.5 from 2026-11-20" 0 "2026-11-20 1.4.5" rows "$sandbox/pages.json" bun-version
check "Ruby 3.4.13 from 2026-11-19" 0 "2026-11-19 3.4.13" rows "$sandbox/pages.json" ruby-version
check "uv 0.12.24 from 2026-10-12" 0 "2026-10-12 0.12.24" rows "$sandbox/pages.json" uv-version
check "uv's oldest row is a Before row" 0 "before 2025-06-12 0.7.12" rows "$sandbox/pages.json" uv-version
check "Elixir and Erlang/OTP defaults from the header" 0 "defaults Elixir=1.18.4 Erlang/OTP=28.0.2 since 2025-06-12" \
  rows "$sandbox/pages.json" elixir-erlang-versions
check "Poetry's series rows are read" 0 "2021-04-27 1.1.x (various)" rows "$sandbox/pages.json" poetry-version

# When this fails after bex's tables followed Render, refresh the pages (above).
check "the real pages match bex's tables" 0 "every default from 2026-06-05 on matches bex's tables (7 pages)" \
  run_with "$fixtures"

# Comparison failures: the page parses, bex's table no longer matches it.
check "a moved current default fails naming the page and the table" 1 \
  "render.com/docs/node-version, Node.js current default: Render's current default moved: 24.22.2 from 2026-10-21, where exactDefaults[nativeNodeRuntime] (nativeversion.go)'s newest step is 24.22.1" \
  run_with "$(pages_with moved-default node-version.md 's/24\.22\.1/24.22.2/g')"

check "a new row fails naming the row and the table" 1 \
  "render.com/docs/ruby-version, Ruby row 2026-12-01: new row: Render gives 3.4.14 to services created from 2026-12-01; exactDefaults[nativeRubyRuntime] (nativeversion.go) has no step that day" \
  run_with "$(pages_with new-row ruby-version.md 's/`3\.4\.13`\* Services created before \*2026-11-19\*/`3.4.14`* Services created before *2026-12-01*/; s/^| 2026-11-19 and later | `3\.4\.13` |$/| 2026-12-01 and later | `3.4.14` |\
| 2026-11-19 | `3.4.13` |/')"

check "a back-dated row fails naming the row and the table" 1 \
  "render.com/docs/uv-version, uv row 2026-09-20: back-dated row: Render gives 0.12.15 to services created from 2026-09-20; uvDefaults (nativeversion.go) has no step that day" \
  run_with "$(pages_with back-dated uv-version.md 's/^| 2026-09-09 | `0\.12\.10` |$/| 2026-09-20 | `0.12.15` |\
&/')"

check "a moved Erlang/OTP default fails naming erlangDefaults" 1 \
  "Erlang/OTP current default: Render's current default is 28.1 from 2025-06-12, erlangDefaults (nativeversion.go)'s newest step is 28.0.2.0" \
  run_with "$(pages_with moved-erlang elixir-erlang-versions.md 's#\*Erlang/OTP `28\.0\.2`\*#*Erlang/OTP `28.1`*#')"

check "a row before bex's first service changes nothing" 0 "matches bex's tables" \
  run_with "$(pages_with old-row python-version.md 's/`3\.11\.11`/`3.11.12`/')"

# Parser failures: what Render published is not what the parser reads.
check "a removed page fails naming it" 1 \
  "uv-version: file://$sandbox/removed-page/llms.txt no longer lists uv-version; Render removed or renamed the page" \
  run_with "$(pages_with removed-page llms.txt '/uv-version\.md/d')"

cp -R "$fixtures" "$sandbox/unfetchable" && rm "$sandbox/unfetchable/bun-version.md"
check "a listed page that cannot be fetched fails naming it" 1 "bun-version: could not fetch file://$sandbox/unfetchable/bun-version.md" \
  run_with "$sandbox/unfetchable"

check "an eighth version page fails naming it" 1 \
  "go-version: Render lists a version page this job does not watch: file://$sandbox/eighth-page/go-version.md" \
  run_with "$(pages_with eighth-page llms.txt 's#^- \[Setting Your Node.js Version\].*#- [Setting Your Go Version](https://render.com/docs/go-version.md)\
&#')"

check "a renamed history column fails as changed markup" 1 \
  "bun-version: found 0 'Service Creation Date' tables, want 1; the markup changed" \
  run_with "$(pages_with renamed-column bun-version.md 's/^| Service Creation Date |/| Created |/')"

check "a version cell without backticks fails as changed markup" 1 \
  "node-version: history row '2026-09-17' | '24.21.0' is not '<date> | \`<version>\`'" \
  run_with "$(pages_with bare-version node-version.md 's/^| 2026-09-17 | `24\.21\.0` |$/| 2026-09-17 | 24.21.0 |/')"

check "a version the parser does not read fails as changed markup" 1 \
  "node-version: 'lts' is not a version this parser reads" \
  run_with "$(pages_with alias-version node-version.md 's/^| 2026-09-17 | `24\.21\.0` |$/| 2026-09-17 | `lts` |/')"

check "a date the parser does not read fails as changed markup" 1 \
  "python-version: history row 'Oct 12, 2026' | '\`3.14.8\`' is not '<date> | \`<version>\`'" \
  run_with "$(pages_with prose-date python-version.md 's/^| 2026-10-12 |/| Oct 12, 2026 |/')"

check "a current default the history table does not start with fails" 1 \
  "ruby-version: the current default Ruby 3.4.14 is not the history table's newest row (3.4.13)" \
  run_with "$(pages_with header-moved ruby-version.md 's/^| \*`3\.4\.13`\*/| *`3.4.14`*/')"

check "a header date the history table does not start with fails" 1 \
  "node-version: the header dates the current default from 2026-10-22, the history table from 2026-10-21" \
  run_with "$(pages_with header-date node-version.md 's/before \*2026-10-21\*/before *2026-10-22*/')"

check "rows out of date order fail" 1 "bun-version: history row 2026-10-15 is not older than the row above it (2026-10-12)" \
  run_with "$(pages_with unordered bun-version.md 's/^| 2026-09-09 |/| 2026-10-15 |/')"

check "a current default for no tool fails" 1 "elixir-erlang-versions: the current defaults cell" \
  run_with "$(pages_with unnamed elixir-erlang-versions.md 's#\*Elixir `1\.18\.4`\* \*Erlang/OTP `28\.0\.2`\*#*`1.18.4`*#')"

check "an llms.txt listing no version page fails" 1 "lists no */docs/*-version(s).md page; its markup changed" \
  run_with "$(pages_with no-links llms.txt 's#https://render\.com/docs/\([a-z-]*\)-versions*\.md#https://render.com/docs/\1.html#')"

check "an unreachable llms.txt fails" 1 "llms.txt: could not fetch file://$sandbox/absent/llms.txt" \
  run_with "$sandbox/absent"

check "an unknown mode is refused" 2 "usage:" "$script" list

# Every failure above ran with TMPDIR in the sandbox: none may leave its
# downloads behind.
if [[ -n "$(ls -A "$sandbox/tmp")" ]]; then
  echo "FAIL: the script left temporary files behind: $(ls -A "$sandbox/tmp")" >&2
  failures=$((failures + 1))
else
  echo "ok: no temporary files left behind, failures included"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "$failures render-version-drift test(s) failed" >&2
  exit 1
fi
echo "all render-version-drift tests passed"
