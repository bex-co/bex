#!/usr/bin/env bash
# Unit tests for render-dashboard-ops.sh (w1/m645). Network-free: the dashboard
# page is a file:// URL to a synthetic bundle shaped like Render's
# (scripts/fixtures/render-dashboard-ops/), so what is under test is the
# script's own logic: the crawl, the operation and flag parsers, the ledger's
# rules, and that a bundle it cannot read fails with a message instead of
# passing. The last check runs the committed ledger through the same rules.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/render-dashboard-ops.sh"
fixtures="$repo_root/scripts/fixtures/render-dashboard-ops"
committed_ledger="$repo_root/docs/render-artifacts/dashboard-operations.md"

failures=0
check() { # check <name> <expected exit> <expected text> <command...>
  local name="$1" expected_exit="$2" expected_text="$3" actual_exit=0 output
  shift 3
  output="$("$@" 2>&1)" || actual_exit=$?
  if [[ "$actual_exit" != "$expected_exit" ]]; then
    echo "FAIL: $name — exit $actual_exit, want $expected_exit" >&2
    echo "$output" | head -8 >&2
    failures=$((failures + 1))
    return
  fi
  if [[ -n "$expected_text" ]] && ! grep -qF -- "$expected_text" <<<"$output"; then
    echo "FAIL: $name — output missing '$expected_text'" >&2
    echo "$output" | head -8 >&2
    failures=$((failures + 1))
    return
  fi
  echo "ok: $name"
}

tmp="${TMPDIR:-/tmp}"
sandbox="$(mktemp -d "${tmp%/}/bex-dashboard-ops-test.XXXXXX")"
trap 'rm -rf "$sandbox"' EXIT

# bundle_with <name> <file> <sed expression>: a copy of the fixture bundle with
# one file edited; prints the copy's page path.
bundle_with() {
  cp -R "$fixtures" "$sandbox/$1"
  if [[ -n "$2" ]]; then
    sed -e "$3" "$fixtures/$2" >"$sandbox/$1/$2"
  fi
  echo "$sandbox/$1/index.html"
}

# ledger_with <name> <sed expression>: the fixture ledger with one edit.
ledger_with() {
  sed -e "$2" "$fixtures/ledger.md" >"$sandbox/$1.md"
  echo "$sandbox/$1.md"
}

run() { # run <page> <ledger> [mode]
  RENDER_DASHBOARD_URL="file://$1" RENDER_DASHBOARD_OPS_LEDGER="$2" "$script" "${3:-check}"
}

page="$fixtures/index.html"
ledger="$fixtures/ledger.md"

check "the fixture bundle matches its ledger" 0 "8 operations and 6 flags in 7 chunks (entry index-Ent1y-_A.js)" \
  run "$page" "$ledger"

# The crawl: a chunk reached only through two dynamic imports (entry ->
# Settings -> Rollback) and one named only in a preload list (Billing).
# A list that fails leaves these checks to fail rather than ending the run.
inventory="$(run "$page" "$ledger" list 2>&1)" || true
check "a chunk two dynamic imports deep is crawled" 0 '"name": "rollbackComparison"' echo "$inventory"
check "a chunk named only in a preload list is crawled" 0 '"name": "addPromotion"' echo "$inventory"
# Root fields: an alias names its field, a fragment spread and the arguments'
# braces are skipped, an inline fragment's fields count, and a selection built
# at run time is marked.
fields() { python3 -I -c 'import json,sys; print([o["fields"] for o in json.load(sys.stdin)["operations"] if o["name"]==sys.argv[1]][0])' "$1" <<<"$inventory"; }
check "an aliased root field is read by its field name" 0 "['deploy', 'updateServerV2']" fields updateServer
check "an inline fragment's root fields count" 0 "['currentLiveDeploy', 'deploy', 'serverPlans']" fields rollbackComparison
check "root fields chosen at run time are marked" 0 "['\${...}', '__typename']" fields statusQuery

# The four drift failures the job exists for.
check "a new operation fails and is named" 1 "no row: mutation brandNewThing (root fields thing; in Billing)" \
  run "$(bundle_with new-op assets/Billing-B1ll-ng0.js 's#^export{A};#var B=new Qe(V`mutation brandNewThing { thing }`);export{A,B};#')" "$ledger"
check "a removed operation fails and is named" 1 "subscription serviceEvents is no longer in Render's dashboard bundle" \
  run "$(bundle_with removed-op assets/index-Ent1y-_A.js 's#subscription serviceEvents#fragment serviceEvents on Service#')" "$ledger"
check "a new flag gate fails and is named" 1 "no row: flag brand-new-experiment (called from Settings)" \
  run "$(bundle_with new-flag assets/Settings-SeTT1ngs.js 's#Zq9(`okta-app-available`)#Zq9(`okta-app-available`)\&\&Zq9(`brand-new-experiment`)#')" "$ledger"
check "a removed flag gate fails and names the item to reopen" 1 \
  "flag gate postgres-parameter-overrides is gone from Render's dashboard bundle: Render made the feature GA or dropped it; reopen Reopen \`w2/066\` D2=A" \
  run "$(bundle_with removed-flag assets/index-Ent1y-_A.js 's#jx(`postgres-parameter-overrides`)#!0#')" "$ledger"

# A flag whose call sites moved to another chunk family is a notice, not a
# failure: Rollup re-splits chunks between Render deploys.
moved="$(bundle_with moved-flag assets/index-Ent1y-_A.js 's#kx(`skip-banner`)#!0#')"
sed -e 's#Zq9(`okta-app-available`)#Zq9(`okta-app-available`)\&\&Zq9(`skip-banner`)#' \
  "$fixtures/assets/Settings-SeTT1ngs.js" >"$sandbox/moved-flag/assets/Settings-SeTT1ngs.js"
check "a flag that moved to another chunk family passes with a notice" 0 \
  "flag skip-banner is now called from Settings (recorded: index)" run "$moved" "$ledger"

# Render's second flag mechanism: hooks from the frontendFlag chunk, whose
# values Render's server evaluates (allFrontendFlags). Each flag records which
# chunk's hooks read it; a route loader's flag counts like a hook call (w1/148).
hooks() { python3 -I -c 'import json,sys; print([f["hooks"] for f in json.load(sys.stdin)["flags"] if f["name"]==sys.argv[1]][0])' "$1" <<<"$inventory"; }
check "a frontend flag is read through the frontendFlag chunk" 0 "['frontendFlag']" hooks oauth_proxy
check "a frontend flag on a route loader is found" 0 "['frontendFlag']" hooks sandbox_shell
check "a featureflags flag is read through the featureflags chunk" 0 "['featureflags']" hooks okta-app-available
check "a new frontend flag gate fails and is named" 1 "no row: flag brand_new_flag (called from Settings)" \
  run "$(bundle_with new-frontend-flag assets/Settings-SeTT1ngs.js 's#So(`oauth_proxy`)#So(`oauth_proxy`)\&\&So(`brand_new_flag`)#')" "$ledger"
check "a removed frontend flag gate fails and names the item to reopen" 1 \
  "flag gate sandbox_shell is gone from Render's dashboard bundle: Render made the feature GA or dropped it; reopen A frontend flag on a route loader: compare \`w1/152\`" \
  run "$(bundle_with removed-frontend-flag assets/index-Ent1y-_A.js 's#Co(`sandbox_shell`)#null#')" "$ledger"

# Flag calls are found under whatever local name the caller imports a hook as.
check "a flag call under another local import name is found" 0 "every operation and flag has a row" \
  run "$(bundle_with renamed-local assets/Settings-SeTT1ngs.js 's#Zq9#$q_0#g')" "$ledger"
check "a flag call through a namespace import is found" 0 "every operation and flag has a row" \
  run "$(bundle_with namespace assets/Settings-SeTT1ngs.js 's#import{r as Zq9}from#import*as FF from#; s#Zq9(#FF.r(#')" "$ledger"

# What the parser no longer recognizes fails with a message naming it.
check "a page with no entry script fails" 1 "no <script src=…assets/index-*.js>" \
  run "$(bundle_with no-entry index.html 's#assets/index-Ent1y-_A.js#assets/main.js#')" "$ledger"
check "an entry script that cannot be fetched fails" 1 "could not fetch file://$sandbox/missing-entry/assets/index-Ent1y-_A.js" \
  run "$(bundle_with missing-entry '' '' && rm "$sandbox/missing-entry/assets/index-Ent1y-_A.js")" "$ledger"
check "an unreachable page fails" 1 "could not fetch file://$sandbox/absent.html" run "$sandbox/absent.html" "$ledger"
check "a renamed featureflags chunk fails closed" 1 "want one featureflags-*.js chunk in the import graph, found none" \
  run "$(cp -R "$fixtures" "$sandbox/renamed-chunk" && cd "$sandbox/renamed-chunk/assets" &&
    mv featureflags-F1agsAbC.js flaghooks-F1agsAbC.js && sed -i.bak 's#featureflags-F1agsAbC#flaghooks-F1agsAbC#g' ./*.js &&
    echo "$sandbox/renamed-chunk/index.html")" "$ledger"
check "a featureflags export shape this parser cannot read fails closed" 1 "its export shape changed" \
  run "$(bundle_with export-shape assets/featureflags-F1agsAbC.js 's#export{g as a,p as i,h as n,d as o,f as r,m as t};#export default {g,p,h,d,f,m};#')" "$ledger"
check "a renamed frontendFlag chunk fails closed" 1 "want one frontendFlag-*.js chunk in the import graph, found none" \
  run "$(cp -R "$fixtures" "$sandbox/renamed-frontend" && cd "$sandbox/renamed-frontend/assets" &&
    mv frontendFlag-Fr0ntF1g.js uiFlags-Fr0ntF1g.js && sed -i.bak 's#frontendFlag-Fr0ntF1g#uiFlags-Fr0ntF1g#g' ./*.js &&
    echo "$sandbox/renamed-frontend/index.html")" "$ledger"
check "a frontendFlag chunk that no longer reads allFrontendFlags fails closed" 1 \
  "frontendFlag has no export{…} clause over allFrontendFlags" \
  run "$(bundle_with frontend-marker assets/frontendFlag-Fr0ntF1g.js 's#allFrontendFlags#frontendSettings#g')" "$ledger"
check "no frontend flag call fails closed" 1 "no chunk calls a frontendFlag hook with a flag name" \
  run "$(cp -R "$fixtures" "$sandbox/no-frontend-calls" && sed -E -i.bak 's#(So|Co)\(`[a-z_]*`#\1(e#g' "$sandbox"/no-frontend-calls/assets/*.js &&
    echo "$sandbox/no-frontend-calls/index.html")" "$ledger"
check "no flag call at all fails closed" 1 "no chunk calls a featureflags hook with a flag name" \
  run "$(cp -R "$fixtures" "$sandbox/no-calls" && sed -E -i.bak 's#(jx|Mx|kx|Zq9)\(`[a-z-]*`#\1(e#g' "$sandbox"/no-calls/assets/*.js &&
    echo "$sandbox/no-calls/index.html")" "$ledger"
check "an anonymous operation fails and names its chunk" 1 "anonymous query in Billing" \
  run "$(bundle_with anonymous-op assets/Billing-B1ll-ng0.js 's#^export{A};#var B=new Qe(V`query { anonThing }`);export{A,B};#')" "$ledger"
check "an anonymous operation with variables fails" 1 "anonymous subscription in index" \
  run "$(bundle_with anonymous-sub assets/index-Ent1y-_A.js 's#query server#subscription($x: String!) { anonSub } query server#')" "$ledger"
check "a nested subscription field is not an anonymous operation" 1 "no row: query nestedOp" \
  run "$(bundle_with nested-sub assets/Billing-B1ll-ng0.js 's#^export{A};#var B=new Qe(V`query nestedOp { subscription { x } }`);export{A,B};#')" "$ledger"
check "a JS call shaped like an operation is not one" 0 "every operation and flag has a row" \
  run "$(bundle_with js-call assets/Billing-B1ll-ng0.js 's#^export{A};#var q=query({a:1});export{A};#')" "$ledger"
check "an operation whose selection set does not parse fails" 1 "query server in index: no selection set" \
  run "$(bundle_with bad-op assets/index-Ent1y-_A.js 's#query server($id: String!) {#query server($id: String!) [#')" "$ledger"
check "a selection set that ends early fails" 1 "query server in index: unexpected" \
  run "$(bundle_with open-op assets/index-Ent1y-_A.js 's#^    server(id: $id) {#    server(id: $id) {{#')" "$ledger"

# The ledger's rules.
check "a gap row with no board item fails" 1 "gap row names no board item" \
  run "$page" "$(ledger_with gap-unowned 's#| gap | `w1/m646` |#| gap | nobody yet |#')"
check "a placeholder pointer fails" 1 "matched row has no pointer" \
  run "$page" "$(ledger_with placeholder 's#| `server` | query | matched | .* |#| `server` | query | matched | TBD |#')"
check "a non-goal row must cite DO_NOT_DO or ADR018" 1 "non-goal row cites neither" \
  run "$page" "$(ledger_with non-goal 's#DO_NOT_DO: ADR071 tenant billing credits#we decided#')"
check "a partial row must name what owns the rest" 1 "partial row names neither" \
  run "$page" "$(ledger_with partial 's#| `addPromotion` | mutation | non-goal | .* |#| `addPromotion` | mutation | partial | some of it |#')"
check "a flagged operation row must name a flag from the Flag table" 1 "flagged row names no flag from the Flag table" \
  run "$page" "$(ledger_with flagged-op 's#| `rollbackComparison` | query | gap | `w1/m646` |#| `rollbackComparison` | query | flagged | `some-other-flag` |#')"
check "a flag row that is not flagged fails" 1 "flag skip-banner has disposition 'matched', want flagged" \
  run "$page" "$(ledger_with flag-matched 's#| `skip-banner` | `index` | flagged |#| `skip-banner` | `index` | matched |#')"
check "a flag row must name what to reopen" 1 "flag skip-banner names no board item or ADR018 row to reopen" \
  run "$page" "$(ledger_with flag-pointer "s#Compare bex's banner (ADR018 § Billing \& invoicing: Unpaid invoice, suspension and recovery)#compare the banner#")"
check "an unknown disposition fails" 1 "disposition 'done' is not one of" \
  run "$page" "$(ledger_with done-word 's#| `server` | query | matched |#| `server` | query | done |#')"
check "a wrong kind fails" 1 "want \`name\` | query, mutation or subscription" \
  run "$page" "$(ledger_with kind 's#| `server` | query |#| `server` | mutations |#')"
check "a duplicate row fails" 1 "second row for query server" \
  run "$page" "$(ledger_with duplicate 's#^| `statusQuery` |#| `server` | query | matched | ADR018 § Services \& lifecycle: Get service |\
| `statusQuery` |#')"
check "a row with a missing cell fails" 1 "3 cells, want 4" \
  run "$page" "$(ledger_with short-row 's#| `server` | query | matched |#| `server` | matched |#')"
check "a ledger with no Flag table fails" 1 "found no Operation or no Flag rows" \
  run "$page" "$(grep -v '^| `[a-z0-9_-]*` | `' "$ledger" >"$sandbox/no-flags.md"; echo "$sandbox/no-flags.md")"
RENDER_DASHBOARD_OPS_LEDGER="$sandbox/absent.md" check "a missing ledger fails" 1 "does not exist" "$script"

# The committed ledger itself, offline: every row is well formed, and a bundle
# built from its rows (each operation in the entry chunk, each flag called
# there through both flag chunks' hooks) reads back as all dispositioned. The
# live comparison is the weekly job's; this catches a bad row on the push that
# adds it.
committed="$sandbox/committed"
mkdir -p "$committed/assets"
cp "$fixtures/index.html" "$committed/index.html"
cp "$fixtures/assets/hooks-HooK5x9Q.js" "$fixtures/assets/featureflags-F1agsAbC.js" "$fixtures/assets/frontendFlag-Fr0ntF1g.js" \
  "$committed/assets/"
{
  echo 'import{n as Qe,y as V}from"./hooks-HooK5x9Q.js";import{t as Mx}from"./featureflags-F1agsAbC.js";import{o as So}from"./frontendFlag-Fr0ntF1g.js";'
  awk -F'|' '/^\| `[A-Za-z_][A-Za-z0-9_]*` \| (query|mutation|subscription) \|/ {
    gsub(/[` ]/, "", $2); gsub(/ /, "", $3)
    printf "new Qe(V`\n  %s %s {\n    x\n  }\n`);\n", $3, $2
  }' "$committed_ledger"
  awk -F'|' '/^\| `[a-z0-9_-]+` \| `/ { gsub(/[` ]/, "", $2); printf "Mx(`%s`,e);So(`%s`);\n", $2, $2 }' "$committed_ledger"
} >"$committed/assets/index-Ent1y-_A.js"
check "the committed ledger is well formed" 0 "every operation and flag has a row" run "$committed/index.html" "$committed_ledger"

if [[ "$failures" -gt 0 ]]; then
  echo "$failures render-dashboard-ops test(s) failed" >&2
  exit 1
fi
echo "all render-dashboard-ops tests passed"
