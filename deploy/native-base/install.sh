#!/bin/sh
set -eu

. /etc/os-release
case "$VERSION_CODENAME" in
  bullseye)
    pg_repo=https://apt-archive.postgresql.org/pub/repos/apt
    # Bullseye has left the live mirrors. Keep archive signature checking;
    # its intentionally frozen Release files no longer have a live deadline.
    rm -f /etc/apt/sources.list.d/debian.sources
    cat > /etc/apt/sources.list <<'SOURCES'
deb [check-valid-until=no] https://archive.debian.org/debian bullseye main
deb [check-valid-until=no] https://archive.debian.org/debian bullseye-updates main
deb [check-valid-until=no] https://archive.debian.org/debian-security bullseye-security main
SOURCES
    ;;
  bookworm|trixie) pg_repo=https://apt.postgresql.org/pub/repos/apt ;;
  *) echo "unsupported native distribution: $VERSION_CODENAME" >&2; exit 1 ;;
esac

# Authenticate the PGDG repository with its independently pinned signing key.
mkdir -p /usr/share/keyrings
fetch_pgdg_key() {
  attempt=1
  while :; do
    if curl --fail --location --connect-timeout 30 --max-time 180 "$1" -o "$2"; then
      return 0
    else
      result=$?
    fi
    # Retry transport interruptions, never HTTP or certificate refusals.
    case "$result" in
      5|6|7|18|28|35|52|55|56|92) ;;
      *) return "$result" ;;
    esac
    [ "$attempt" -lt 4 ] || return "$result"
    sleep "$attempt"
    attempt=$((attempt + 1))
  done
}
fetch_pgdg_key https://www.postgresql.org/media/keys/ACCC4CF8.asc \
  /usr/share/keyrings/bex-pgdg.asc
echo '0144068502a1eddd2a0280ede10ef607d1ec592ce819940991203941564e8e76  /usr/share/keyrings/bex-pgdg.asc' | sha256sum -c -
printf 'deb [signed-by=/usr/share/keyrings/bex-pgdg.asc] %s %s-pgdg main\n' \
  "$pg_repo" "$VERSION_CODENAME" > /etc/apt/sources.list.d/bex-pgdg.list
apt-get update
# Trusted factory recipes may add distribution-matching runtime dependencies.
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  postgresql-client-18 gettext ghostscript jq libvips-dev libvips-tools pandoc \
  pigz python3-dev python3-pip python3-setuptools rsync sqlite3 swig zip unzip \
  gnupg2 nano vim libbrotli1 libbrotli-dev "$@"

sh /tmp/bex-patch-system-brotli.sh

# The image has no server. All programs use the same packaged client/libpq
# closure, including archives produced by bex's default PostgreSQL 18 server.
psql --version
pg_dump --version
pg_restore --version
dpkg-query -W > /usr/share/bex-native-packages.txt
apt-get clean
rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/log/apt /var/log/dpkg.log
