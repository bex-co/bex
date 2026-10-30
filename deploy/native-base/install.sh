#!/bin/sh
set -eu

. /etc/os-release
case "$VERSION_CODENAME" in
  bullseye) pg_repo=https://apt-archive.postgresql.org/pub/repos/apt ;;
  bookworm|trixie) pg_repo=https://apt.postgresql.org/pub/repos/apt ;;
  *) echo "unsupported native distribution: $VERSION_CODENAME" >&2; exit 1 ;;
esac

# Authenticate the PGDG repository with its independently pinned signing key.
mkdir -p /usr/share/keyrings
curl --fail --location --retry 3 https://www.postgresql.org/media/keys/ACCC4CF8.asc \
  -o /usr/share/keyrings/bex-pgdg.asc
echo '0144068502a1eddd2a0280ede10ef607d1ec592ce819940991203941564e8e76  /usr/share/keyrings/bex-pgdg.asc' | sha256sum -c -
printf 'deb [signed-by=/usr/share/keyrings/bex-pgdg.asc] %s %s-pgdg main\n' \
  "$pg_repo" "$VERSION_CODENAME" > /etc/apt/sources.list.d/bex-pgdg.list
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends postgresql-client-18

# The image has no server. All programs use the same packaged client/libpq
# closure, including archives produced by bex's default PostgreSQL 18 server.
psql --version
pg_dump --version
pg_restore --version
dpkg-query -W > /usr/share/bex-native-packages.txt
apt-get clean
rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/log/apt /var/log/dpkg.log
