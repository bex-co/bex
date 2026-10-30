# syntax=docker/dockerfile:1.7
ARG NATIVE_BASE
FROM ${NATIVE_BASE}
COPY testdata/archive.dump.b64 /tmp/archive.dump.b64
COPY testdata/rows.txt /tmp/expected-rows.txt
COPY check-tools.py /tmp/check-tools.py
RUN --network=none base64 -d /tmp/archive.dump.b64 > /tmp/archive.dump \
  && echo 'a640255a7f226c765d84ad869a6a986ea4d39be7a72a68b02599dcf87721637c  /tmp/archive.dump' | sha256sum -c - \
  && pg_restore --file=/tmp/archive.sql /tmp/archive.dump \
  && awk '/^COPY public.native_archive / { copying=1; next } copying && $0=="\\." { exit } copying { print }' /tmp/archive.sql > /tmp/actual-rows.txt \
  && cmp /tmp/expected-rows.txt /tmp/actual-rows.txt \
  && psql --version && pg_dump --version \
  && ! pg_restore --file=/tmp/invalid.sql /dev/null
RUN --network=none /usr/bin/python3 /tmp/check-tools.py
