#!/usr/bin/env bash
# oidc-issuer-verify.sh — check the Managed OIDC issuer as a relying party
# would (docs/ADR116-managed-oidc.md, w2/m476 t006).
#
#   KUBECONFIG=<app cluster kubeconfig> bash scripts/oidc-issuer-verify.sh <tea-…> <srv-…>
#
# 1. Mints a token for the service inside bex-api (`/api oidc-test-token`,
#    audience bex-oidc-verify, which no provider accepts).
# 2. Verifies it with go-oidc (lego/backend/cmd/oidc-verify) given only
#    <issuer base>/<tea>: discovery, JWKS, the ES256 signature, iss, aud and
#    exp. Then checks Render's sub template, exp = iat + 3600, and that every
#    published kid is its key's RFC 7638 thumbprint.
# 3. Verifies the same token against another workspace's issuer and requires
#    the refusal: each workspace is its own identity provider.
# 4. Compares the discovery document, the JWKS members and both documents'
#    headers with Render's live ones.
# 5. Requires Go's 404 for malformed workspace ids and the host root.
#
# Exits 0 only when every check passes. Environment:
#   OIDC_ISSUER_BASE   the issuer origin (default https://oidc.bex.co)
#   RENDER_OIDC_BASE   Render's issuer origin (default https://oidc.render.com)
#   BEX_API_NAMESPACE  where bex-api runs (default bex-system)
#   OIDC_VERIFY_BIN    the verifier (default: go run ./cmd/oidc-verify)
set -euo pipefail

usage() {
  echo "usage: KUBECONFIG=<kubeconfig> bash scripts/oidc-issuer-verify.sh <tea-…> <srv-…>" >&2
  exit 2
}
[ "$#" -eq 2 ] || usage
workspace=$1
service=$2
case "$workspace" in tea-????????????????????) ;; *) usage ;; esac
case "$service" in srv-????????????????????) ;; *) usage ;; esac

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
base=${OIDC_ISSUER_BASE:-https://oidc.bex.co}
render=${RENDER_OIDC_BASE:-https://oidc.render.com}
namespace=${BEX_API_NAMESPACE:-bex-system}
audience=bex-oidc-verify
issuer="$base/$workspace"
# Another well-formed workspace id: its issuer must refuse this workspace's token.
other=tea-c0000000000000000000
[ "$other" != "$workspace" ] || other=tea-c00000000000000000g0
# Render answers any well-formed id; this one is in its own documentation probes.
render_workspace=tea-c0000000000000000000

failures=0
pass() { echo "PASS: $*"; }
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

verify() { # verify <issuer> < token
  if [ -n "${OIDC_VERIFY_BIN:-}" ]; then
    "$OIDC_VERIFY_BIN" --issuer "$1" --audience "$audience"
  else
    (cd "$root/lego/backend" && go run ./cmd/oidc-verify --issuer "$1" --audience "$audience")
  fi
}

# fetch <url> <body file> prints "<status> <content-type> <cache-control>".
fetch() {
  local headers
  headers=$(curl -sS -D - -o "$2" "$1") || {
    echo "000 - -"
    return
  }
  awk 'BEGIN { s = "000"; t = "-"; c = "-" }
    { sub(/\r$/, "") }
    toupper($1) ~ /^HTTP\// { s = $2 }
    tolower($1) == "content-type:" { $1 = ""; t = substr($0, 2) }
    tolower($1) == "cache-control:" { $1 = ""; c = substr($0, 2) }
    END { print s " " t " " c }' <<<"$headers"
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# 1. Mint.
if ! kubectl -n "$namespace" exec deploy/bex-api -c api -- /api oidc-test-token "$workspace" "$service" >"$tmp/token" 2>"$tmp/mint.err"; then
  echo "FAIL: minting a token inside bex-api: $(cat "$tmp/mint.err")" >&2
  exit 1
fi
[ -s "$tmp/token" ] || {
  echo "FAIL: bex-api printed no token" >&2
  exit 1
}

# 2. Verify at the workspace's own issuer.
if verify "$issuer" <"$tmp/token" >"$tmp/claims.json" 2>"$tmp/verify.err"; then
  want_sub="workspace:$workspace:environment:default:service:$service"
  if jq -e --arg iss "$issuer" --arg sub "$want_sub" --arg aud "$audience" \
    '.iss == $iss and .sub == $sub and .aud == [$aud] and .exp - .iat == 3600 and (.kid as $k | .jwksKids | index($k)) != null' \
    "$tmp/claims.json" >/dev/null; then
    pass "a token for $service verifies through $issuer alone (kid $(jq -r .kid "$tmp/claims.json"), $(jq -r '.jwksKids | length' "$tmp/claims.json") published key(s), RFC 7638 kids)"
  else
    fail "the verified claims are not Render's: $(cat "$tmp/claims.json")"
  fi
else
  fail "the token does not verify through $issuer: $(cat "$tmp/verify.err")"
fi

# 3. Another workspace's issuer refuses it.
if verify "$base/$other" <"$tmp/token" >/dev/null 2>"$tmp/other.err"; then
  fail "$base/$other accepted a token issued for $workspace"
else
  pass "$base/$other refuses the token ($(tr '\n' ' ' <"$tmp/other.err"))"
fi

# 4. Render's shape.
bex_disc=$(fetch "$issuer/.well-known/openid-configuration" "$tmp/bex-disc.json")
ren_disc=$(fetch "$render/$render_workspace/.well-known/openid-configuration" "$tmp/ren-disc.json")
bex_jwks=$(fetch "$issuer/.well-known/jwks.json" "$tmp/bex-jwks.json")
ren_jwks=$(fetch "$render/$render_workspace/.well-known/jwks.json" "$tmp/ren-jwks.json")
if [ "$bex_disc" = "$ren_disc" ] && [ "$bex_jwks" = "$ren_jwks" ]; then
  pass "status, Content-Type and Cache-Control match Render's ($bex_disc)"
else
  fail "headers differ from Render's: discovery $bex_disc vs $ren_disc, jwks $bex_jwks vs $ren_jwks"
fi
shape='{keys: keys_unsorted, response_types_supported, subject_types_supported, id_token_signing_alg_values_supported}'
if [ "$(jq -cS "$shape" "$tmp/bex-disc.json" 2>/dev/null)" = "$(jq -cS "$shape" "$tmp/ren-disc.json" 2>/dev/null)" ] &&
  jq -e --arg iss "$issuer" '.issuer == $iss and .jwks_uri == $iss + "/.well-known/jwks.json"' "$tmp/bex-disc.json" >/dev/null 2>&1; then
  pass "discovery has Render's fields in Render's order and names $issuer"
else
  fail "discovery differs from Render's: $(cat "$tmp/bex-disc.json") vs $(cat "$tmp/ren-disc.json")"
fi
members='[.keys[] | keys_unsorted] | unique'
if [ "$(jq -c "$members" "$tmp/bex-jwks.json" 2>/dev/null)" = "$(jq -c "$members" "$tmp/ren-jwks.json" 2>/dev/null)" ] &&
  [ "$(jq -c '[.keys[] | {use, kty, crv, alg}] | unique' "$tmp/bex-jwks.json")" = '[{"use":"sig","kty":"EC","crv":"P-256","alg":"ES256"}]' ]; then
  pass "the JWKS keys have Render's members and are ES256 P-256 signing keys"
else
  fail "the JWKS differs from Render's: $(cat "$tmp/bex-jwks.json")"
fi

# 5. Refusals.
for path in /tea-abc/.well-known/openid-configuration /tea-d0000000000000000001/.well-known/jwks.json / "/$workspace"; do
  got=$(fetch "$base$path" "$tmp/refusal")
  if [ "${got%% *}" = 404 ] && [ "$(cat "$tmp/refusal")" = "404 page not found" ]; then
    pass "$path answers 404 page not found"
  else
    fail "$path answers $got: $(head -c 200 "$tmp/refusal")"
  fi
done

if [ "$failures" -gt 0 ]; then
  echo "oidc-issuer-verify: $failures check(s) failed" >&2
  exit 1
fi
echo "oidc-issuer-verify: every check passed for $issuer"
