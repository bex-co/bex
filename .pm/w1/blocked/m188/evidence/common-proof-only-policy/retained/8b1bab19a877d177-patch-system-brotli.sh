#!/bin/sh
# Backport Debian's audited CVE-2026-111497 source to the frozen Bullseye
# images. Bookworm/Trixie use their signed security packages instead.
set -eu
. /etc/os-release
case "$VERSION_CODENAME" in
  bookworm) minimum=1.0.9-2+deb12u1 ;;
  trixie) minimum=1.1.0-2+deb13u1 ;;
  bullseye)
    # Same source and patch as DLA-4855-1, compiled against Bullseye's libc.
    # nopython avoids installing a new Python interpreter or Python module.
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends build-essential cmake debhelper dpkg-dev
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT
    cd "$work"
    source_url=https://deb.debian.org/debian-security/pool/updates/main/b/brotli
    for file in brotli_1.0.9-2+deb12u1.dsc brotli_1.0.9.orig.tar.gz brotli_1.0.9-2+deb12u1.debian.tar.xz; do
      curl --fail --location --retry 3 "$source_url/$file" -o "$file"
    done
    sha256sum -c - <<'CHECKSUMS'
942ab3bccf28fa76d2c98a4e4a3a13390ee4484a2250d95a9ca46e6fbe0d6240  brotli_1.0.9-2+deb12u1.dsc
f9e8d81d0405ba66d181529af42a3354f838c939095ff99930da6aa9cdf6fe46  brotli_1.0.9.orig.tar.gz
110534b0960a4d013942f48c5dcdee34d39501fcb40cd24106e160ad233cc8be  brotli_1.0.9-2+deb12u1.debian.tar.xz
CHECKSUMS
    # The reviewed hashes above authenticate every byte independently of the
    # builder's Debian keyring (which may predate this security uploader).
    dpkg-source --no-check -x brotli_1.0.9-2+deb12u1.dsc source
    cd source
    sed -i '1s/1.0.9-2+deb12u1/1.0.9-2+deb12u1+bex11u1/' debian/changelog
    DEB_BUILD_PROFILES=nopython dpkg-buildpackage -b -us -uc -Pnopython -j2
    cd ..
    dpkg -i libbrotli1_*_*.deb libbrotli-dev_*_*.deb brotli_*_*.deb
    minimum=1.0.9-2+deb12u1+bex11u1
    mkdir -p /usr/share/bex-security
    cp brotli_1.0.9-2+deb12u1.dsc /usr/share/bex-security/brotli-source.dsc
    printf '%s\n' 'DLA-4855-1 source backport to Bullseye; version suffix +bex11u1' \
      > /usr/share/bex-security/brotli-backport.txt
    ;;
  *) echo "unsupported native distribution: $VERSION_CODENAME" >&2; exit 1 ;;
esac
# Development archives are executable inputs too: do not retain an old static
# decoder merely because the shared library has been repaired.
for package in libbrotli1 libbrotli-dev; do
  dpkg --compare-versions "$(dpkg-query -W -f='${Version}' "$package")" ge "$minimum"
done
ldconfig
