#!/usr/bin/env bash
# Fake Go/sleep exercise the real Make recipe without downloads or compilation.
set -euo pipefail
operator=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/fakes"
cat > "$work/fakes/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = env ]; then
  case "$2" in
    GOBIN) printf '%s\n' "$FAKE_STATE/bin" ;;
    GOPATH) printf '%s\n' "$FAKE_STATE/gopath" ;;
    *) exit 98 ;;
  esac
  exit 0
fi
[ "$#" -eq 2 ] && [ "$1" = install ] \
  && [ "$2" = sigs.k8s.io/controller-tools/cmd/controller-gen@v0.20.1 ] || exit 99
count=0
[ ! -e "$FAKE_STATE/count" ] || count=$(cat "$FAKE_STATE/count")
count=$((count + 1))
printf '%s\n' "$count" > "$FAKE_STATE/count"
case "$FAKE_MODE" in
  eof-once|eof-always|integrity)
    if [ "$FAKE_MODE" != eof-once ] || [ "$count" -eq 1 ]; then
      echo 'go: sigs.k8s.io/controller-tools/cmd/controller-gen@v0.20.1: Get "https://proxy.golang.org/sigs.k8s.io/controller-tools/cmd/controller-gen/@v/v0.20.1.info": unexpected EOF' >&2
      if [ "$FAKE_MODE" = integrity ]; then echo 'SECURITY ERROR: checksum mismatch' >&2; fi
      exit 23
    fi ;;
  http503)
    if [ "$count" -eq 1 ]; then echo 'go: reading https://proxy.golang.org/example/@v/v1.0.0.zip: 503 Service Unavailable' >&2; exit 24; fi ;;
  http404) echo 'go: reading https://proxy.golang.org/example/@v/v1.0.0.zip: 404 Not Found' >&2; exit 25 ;;
  compile) echo 'controller.go:10: undefined: missingSymbol' >&2; exit 26 ;;
  version) echo 'go: invalid version: unknown revision v0.20.1' >&2; exit 27 ;;
  source-eof) echo 'controller.go:10: syntax error: unexpected EOF' >&2; exit 28 ;;
  certificate) echo 'go: Get "https://proxy.golang.org/example": tls: failed to verify certificate: x509: certificate signed by unknown authority' >&2; exit 29 ;;
  success) ;;
  *) exit 97 ;;
esac
printf '#!/usr/bin/env bash\nexit 0\n' > "$GOBIN/controller-gen"
chmod +x "$GOBIN/controller-gen"
SH
cat > "$work/fakes/sleep" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_STATE/sleeps"
SH
chmod +x "$work/fakes/"*
export PATH="$work/fakes:$PATH"

check() {
  local mode=$1 attempts=$2 result=$3 waits=$4
  export FAKE_MODE="$mode" FAKE_STATE="$work/$mode"
  mkdir -p "$FAKE_STATE/bin"
  if make --no-print-directory -s -C "$operator" controller-gen \
    LOCALBIN="$FAKE_STATE/bin" CONTROLLER_GEN="$FAKE_STATE/bin/controller-gen" \
    CONTROLLER_TOOLS_VERSION=v0.20.1 > "$FAKE_STATE/output" 2>&1; then
    [ "$result" = success ]
    [ -x "$FAKE_STATE/bin/controller-gen-v0.20.1" ]
    [ "$(readlink "$FAKE_STATE/bin/controller-gen")" = "$(realpath "$FAKE_STATE/bin/controller-gen-v0.20.1")" ]
  else
    [ "$result" = failure ]
    [ ! -e "$FAKE_STATE/bin/controller-gen-v0.20.1" ]
    [ ! -L "$FAKE_STATE/bin/controller-gen" ]
  fi
  [ "$(cat "$FAKE_STATE/count")" -eq "$attempts" ]
  [ "$(cat "$FAKE_STATE/sleeps" 2>/dev/null || true)" = "$waits" ]
  printf 'PASS: %s (%s install attempt(s), %s)\n' "$mode" "$attempts" "$result"
}

check success 1 success ''
check eof-once 2 success 2
# The existing versioned tool is reused without another install.
make --no-print-directory -s -C "$operator" controller-gen \
  LOCALBIN="$FAKE_STATE/bin" CONTROLLER_GEN="$FAKE_STATE/bin/controller-gen" \
  CONTROLLER_TOOLS_VERSION=v0.20.1
[ "$(cat "$FAKE_STATE/count")" -eq 2 ]
check eof-always 3 failure $'2\n4'
check http503 2 success 2
for mode in http404 compile version source-eof integrity certificate; do
  check "$mode" 1 failure ''
done

for pair in compile:26 eof-always:23; do
  export FAKE_MODE="${pair%:*}" FAKE_STATE="$work/status-${pair%:*}"
  mkdir -p "$FAKE_STATE/bin"
  status=0
  GOBIN="$FAKE_STATE/bin" bash "$operator/hack/go-install.sh" \
    sigs.k8s.io/controller-tools/cmd/controller-gen@v0.20.1 \
    > "$FAKE_STATE/output" 2>&1 || status=$?
  [ "$status" -eq "${pair#*:}" ]
  printf 'PASS: %s preserves exit %s\n' "$FAKE_MODE" "$status"
done
