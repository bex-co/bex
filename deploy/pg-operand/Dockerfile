# bex managed-Postgres operand image (ADR092 D1, w8/m92 t008).
#
# A reviewed CloudNativePG "system" operand plus the six extensions Render
# documents (https://render.com/docs/postgresql-extensions), on exactly the
# majors where Render offers each, and pgextwlist, the closed allowlist that
# lets a database owner CREATE EXTENSION them (ADR092 D3). Nothing is
# preinstalled into any database: activation stays per database, by the owner.
#
# The base is CNPG's bookworm system image of the same major and patch. The
# bullseye images bex pinned before carry no PGDG archive for PostGIS on 18 or
# TimescaleDB on 13, so the extension majors could not be supplied there.
#
# This recipe publishes; it never pins. The operator adopts a published digest
# in lego/operator/internal/controller/database_exports.go by a reviewed
# commit (ADR060 D7), because changing it restarts every managed cluster of
# that major once (ADR092 D4).
ARG BASE_IMAGE
# pg_duckdb has no PGDG package. Its upstream image is PGDG-on-bookworm, the
# same distribution and server packages as the base, so its shared objects
# are copied as built. Majors below 16 pass the base image here and copy
# nothing.
ARG DUCKDB_IMAGE=${BASE_IMAGE}

FROM ${DUCKDB_IMAGE} AS duckdb
ARG PG_MAJOR
USER root
RUN set -eu; mkdir -p /out; \
  lib="/usr/lib/postgresql/${PG_MAJOR}/lib"; share="/usr/share/postgresql/${PG_MAJOR}/extension"; \
  if [ -e "$lib/pg_duckdb.so" ]; then \
    mkdir -p "/out$lib" "/out$share"; \
    cp -a "$lib/pg_duckdb.so" "$lib"/libduckdb*.so* "/out$lib/"; \
    cp -a "$share"/pg_duckdb* "/out$share/"; \
    # Upstream leaves these owned by postgres; the server must not be able to
    # rewrite its own code.
    chown -R root:root /out; chmod -R go-w /out; \
  fi

FROM ${BASE_IMAGE}
ARG PG_MAJOR
# Space-separated PGDG package suffixes for this major (postgresql-<major>-<x>),
# chosen by matrix.json from ADR092 D1's table.
ARG PGDG_PACKAGES
# Exact packages from Timescale's own repository, for majors PGDG does not
# build TimescaleDB for (13: its last PG13 line, 2.15). They also carry the
# Timescale-licensed (TSL) module, which is deleted below so the Apache-only
# guarantee is structural, not just timescaledb.license.
ARG TIMESCALE_PACKAGES=
USER root
RUN set -eu; \
  export DEBIAN_FRONTEND=noninteractive; \
  dpkg-query -W -f='${binary:Package}=${Version}\n' "postgresql-${PG_MAJOR}" "postgresql-client-${PG_MAJOR}" libpq5 | sort > /tmp/pg-base-versions; \
  cat /tmp/pg-base-versions; \
  # Hold the server so an extension dependency can never move the patch the
  # base image was reviewed at.
  apt-mark hold "postgresql-${PG_MAJOR}" "postgresql-client-${PG_MAJOR}" libpq5 >/dev/null; \
  apt-get update; \
  pkgs=""; for p in ${PGDG_PACKAGES}; do pkgs="$pkgs postgresql-${PG_MAJOR}-$p"; done; \
  if [ -n "${TIMESCALE_PACKAGES}" ]; then \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg; \
    curl -fsSL https://packagecloud.io/timescale/timescaledb/gpgkey -o /tmp/timescale.key; \
    # SHA-256 of Timescale's packagecloud signing key, fetched 2026-10-31.
    echo "eb53bd3dc31e06fbf1ed769139f12b1fffaf1958a96c9b4f9727c19aada83321  /tmp/timescale.key" | sha256sum -c -; \
    gpg --dearmor -o /usr/share/keyrings/timescale.gpg /tmp/timescale.key; \
    echo "deb [signed-by=/usr/share/keyrings/timescale.gpg] https://packagecloud.io/timescale/timescaledb/debian/ bookworm main" \
      > /etc/apt/sources.list.d/timescale.list; \
    apt-get update; \
    pkgs="$pkgs ${TIMESCALE_PACKAGES}"; \
  fi; \
  apt-get install -y --no-install-recommends $pkgs; \
  if [ -n "${TIMESCALE_PACKAGES}" ]; then \
    rm -f "/usr/lib/postgresql/${PG_MAJOR}/lib"/timescaledb-tsl-*.so /etc/apt/sources.list.d/timescale.list; \
    apt-get purge -y --auto-remove curl gnupg; \
  fi; \
  dpkg-query -W -f='${binary:Package}=${Version}\n' "postgresql-${PG_MAJOR}" "postgresql-client-${PG_MAJOR}" libpq5 | sort > /tmp/pg-after-versions; \
  cmp /tmp/pg-base-versions /tmp/pg-after-versions; \
  cat /tmp/pg-after-versions; \
  if ls "/usr/lib/postgresql/${PG_MAJOR}/lib"/timescaledb-tsl-*.so >/dev/null 2>&1; then \
    echo "TimescaleDB TSL module present" >&2; exit 1; \
  fi; \
  # pgextwlist loads through local_preload_libraries, which only reads
  # $libdir/plugins.
  test -e "/usr/lib/postgresql/${PG_MAJOR}/lib/plugins/pgextwlist.so"; \
  ldd "/usr/lib/postgresql/${PG_MAJOR}/lib"/*.so | { ! grep 'not found'; }; \
  apt-get clean; \
  rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/log/apt /var/log/dpkg.log /tmp/*

COPY --from=duckdb /out/ /
# pgextwlist's custom scripts (extwlist.custom_path): the grants that let the
# owner use what a whitelisted extension created as superuser.
COPY extwlist/ /usr/share/bex/extwlist/
COPY export.sh /usr/local/bin/bex-postgres-export
COPY dump-snapshot.sh /usr/local/bin/bex-postgres-dump-snapshot
COPY restore.sh /usr/local/bin/bex-postgres-restore
RUN chmod 755 /usr/local/bin/bex-postgres-export /usr/local/bin/bex-postgres-dump-snapshot /usr/local/bin/bex-postgres-restore

USER 26
