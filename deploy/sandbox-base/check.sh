#!/bin/sh
# Publication gate for the default sandbox image (ADR104 D2 "Image",
# w22/m447): run inside the pushed image, it fails unless every tool ADR104's
# contents list names answers, in the dialect Render's agent templates call
# it with. sandbox-base-image.yml runs it on each platform's digest before
# the image is signed; run it locally with
#   docker run --rm -v "$PWD/deploy/sandbox-base/check.sh:/check.sh:ro" <image> /bin/sh /check.sh
set -u
failures=0
ok() { printf 'ok   %s\n' "$1"; }
bad() {
  printf 'FAIL %s\n' "$1" >&2
  failures=$((failures + 1))
}
check() {
  label=$1
  shift
  if "$@" >/dev/null 2>&1; then ok "$label"; else bad "$label"; fi
}

# glibc, at least the 2.34 Cursor's arm64 agent links against (x64: 2.28).
libc=$(getconf GNU_LIBC_VERSION 2>/dev/null)
case $libc in
  "glibc 2."*)
    minor=${libc#glibc 2.}
    minor=${minor%%.*}
    if [ "$minor" -ge 34 ]; then ok "$libc"; else bad "$libc is older than glibc 2.34"; fi
    ;;
  *) bad "no glibc (getconf GNU_LIBC_VERSION: '${libc}')" ;;
esac
# A dynamically linked glibc program starts: node is one.
check "node starts ($(node --version 2>/dev/null))" node --version

# Shells: exec runs `bash -c`; scripts start with #!/bin/bash or #!/usr/bin/env bash.
check "/bin/bash" test -x /bin/bash
check "/usr/bin/env bash" /usr/bin/env bash -c 'test "${BASH_VERSINFO[0]}" -ge 5'
check "/bin/sh" /bin/sh -c 'exit 0'

# What the three install scripts and their output collection run.
for tool in curl wget git npm tar gzip mktemp uname ln install grep chmod mkdir rm base64 find sha256sum getent awk stat wc tr date mount; do
  if command -v "$tool" >/dev/null 2>&1; then ok "$tool"; else bad "$tool is missing"; fi
done
check "curl speaks https" sh -c 'curl --version | grep -qw https'
check "GNU wget" sh -c 'wget --version | grep -q "GNU Wget"'
dir=$(mktemp -d)
printf 'sandbox\n' >"$dir/f"
check "sha256sum --check -" sh -c 'printf "%s  %s\n" "$(sha256sum "$1" | cut -d" " -f1)" "$1" | sha256sum --check -' sh "$dir/f"
check "find -printf" sh -c 'test "$(find -P "$1" -type f -printf "%P")" = f' sh "$dir"
check "find -print0 | base64 -w0" sh -c 'find "$1" -type f -print0 | base64 -w0 | grep -q .' sh "$dir"
check "tar -xzf --strip-components" sh -c 'cd "$1" && mkdir -p a/b && echo x >a/b/y && tar -czf t.tgz a && mkdir out && tar --strip-components=1 -xzf t.tgz -C out && test -f out/b/y' sh "$dir"
check "mv -T" sh -c 'mkdir "$1/m" && mv -T "$1/m" "$1/n" && test -d "$1/n"' sh "$dir"
rm -rf "$dir"

# busybox: /bin/busybox (the isolation matrix's probes) and its applets on
# PATH where Debian has no command of that name.
check "/bin/busybox" /bin/busybox true
check "busybox httpd on PATH" sh -c 'test "$(command -v httpd)" = /opt/busybox/bin/httpd'
check "GNU tools first on PATH" sh -c 'test "$(command -v sha256sum)" = /usr/bin/sha256sum && test "$(command -v wget)" = /usr/bin/wget'
check "TLS roots" test -s /etc/ssl/certs/ca-certificates.crt

if [ "$failures" -ne 0 ]; then
  printf '%s check(s) failed\n' "$failures" >&2
  exit 1
fi
echo "all checks passed"
