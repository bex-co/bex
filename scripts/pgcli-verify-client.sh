#!/usr/bin/env bash
set -euo pipefail

# Redaction-safe pgcli contract shim. The official Render CLI passes the
# credential-bearing URI as argv[1]; this process validates it in memory and
# records only allow-listed, non-secret facts.

reject() {
  echo "BEX_PGCLI_SHIM_REJECTED"
  exit 64
}

: "${BEX_PGCLI_EXPECT_HOST:?missing expected host}"
: "${BEX_PGCLI_EXPECT_DATABASE:?missing expected database}"
: "${BEX_PGCLI_EXPECT_ID:?missing expected database id}"
: "${BEX_PGCLI_EXPECT_LABEL:?missing invocation label}"
: "${BEX_PGCLI_SHIM_RECORD:?missing safe record path}"

[[ "$#" == "3" ]] || reject
[[ "$2" == "--csv" && "$3" == "-q" ]] || reject

uri="$1"
if ! safe_record="$({
  printf '%s' "$uri" | python3 -c '
import json
import os
import sys
import urllib.parse

uri = sys.stdin.read()
parsed = urllib.parse.urlsplit(uri)
query = urllib.parse.parse_qs(parsed.query, keep_blank_values=True)
database = urllib.parse.unquote(parsed.path.removeprefix("/"))
if parsed.scheme not in {"postgres", "postgresql"}:
    raise SystemExit(1)
if not parsed.hostname or not parsed.username or parsed.password is None or not database:
    raise SystemExit(1)
if parsed.hostname != os.environ["BEX_PGCLI_EXPECT_HOST"]:
    raise SystemExit(1)
if database != os.environ["BEX_PGCLI_EXPECT_DATABASE"]:
    raise SystemExit(1)
if query.get("sslmode") != ["require"]:
    raise SystemExit(1)
print(json.dumps({
    "database": database,
    "flags": ["--csv", "-q"],
    "host": parsed.hostname,
    "id": os.environ["BEX_PGCLI_EXPECT_ID"],
    "label": os.environ["BEX_PGCLI_EXPECT_LABEL"],
    "scheme": parsed.scheme,
    "tls": "require",
}, sort_keys=True, separators=(",", ":")))
' 2>/dev/null
})"; then
  uri=""
  reject
fi
uri=""

umask 077
printf '%s\n' "$safe_record" >>"$BEX_PGCLI_SHIM_RECORD"
safe_record=""
echo "BEX_PGCLI_SHIM_OK"
