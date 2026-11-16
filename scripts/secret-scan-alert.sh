#!/usr/bin/env bash
# secret-scan-alert.sh — list a failed secret scan of main on the one
# `secret-scan` tracking issue (ADR019 §6). Each run stays listed until a
# person handles it and closes the issue.
#
#   bash scripts/secret-scan-alert.sh leak|error RUN_URL
set -euo pipefail

case "${1:-}" in
  leak) what="possible credential pushed" ;;
  error) what="the scan did not finish, so commits are unscanned" ;;
  *)
    echo "usage: $0 leak|error RUN_URL" >&2
    exit 2
    ;;
esac
run_url=${2:?usage: $0 leak|error RUN_URL}

header=$(mktemp)
trap 'rm -f "$header"' EXIT
cat >"$header" <<'EOF'
A secret scan of main failed. Each run's log names every finding's commit, file and rule, with the value redacted.

- A real credential: rotate it (ADR019 §Decision 6). main is public, so removing it from history does not help.
- A shape that is never a credential: allowlist that shape in `.gitleaks.toml` with its reason.
- A scan that did not finish: re-run it.

Close this issue once every run below is handled.

EOF
bash "$(dirname "$0")/lib/tracking-issue.sh" append --label secret-scan \
  --description "A secret scan of main failed" \
  --title "Secret scan: a push to main may carry a credential" \
  --body-file "$header" --line "- $(date -u +%FT%TZ): $what, $run_url"
