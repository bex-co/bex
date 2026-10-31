# syntax=docker/dockerfile:1.7
ARG NATIVE_BASE
FROM ${NATIVE_BASE} AS native

# A patched Node can also be selected beside Python 3.7 or Ruby 3.0. Execute
# that exact binary against their oldest reviewed libc/libstdc++ baseline;
# checking only its original Bookworm primary would miss a companion failure.
FROM node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e AS node-portability
RUN --network=none --mount=type=bind,from=native,source=/,target=/candidate,readonly \
  if [ -f /candidate/usr/local/include/node/node_version.h ]; then \
    major="$(awk '$1 == "#define" && $2 == "NODE_MAJOR_VERSION" { print $3 }' /candidate/usr/local/include/node/node_version.h)"; \
    case "$major" in \
      14|16|18|20) \
        test -s /candidate/usr/share/bex-node-brotli/sources.txt \
          && /candidate/usr/local/bin/node /candidate/usr/share/bex-security/node-check.js \
          && echo 'Patched Node passed the Bullseye companion baseline' \
        ;; \
    esac; \
  fi \
  && printf 'verified\n' > /tmp/bex-node-portability-verified

FROM native
COPY --from=node-portability /tmp/bex-node-portability-verified /tmp/bex-node-portability-verified
COPY testdata/archive.dump.b64 /tmp/archive.dump.b64
COPY testdata/rows.txt /tmp/expected-rows.txt
COPY check-tools.py /tmp/check-tools.py
COPY check-brotli.py /tmp/check-brotli.py
COPY check-primary-python.py /tmp/check-primary-python.py
RUN --network=none base64 -d /tmp/archive.dump.b64 > /tmp/archive.dump \
  && echo 'a640255a7f226c765d84ad869a6a986ea4d39be7a72a68b02599dcf87721637c  /tmp/archive.dump' | sha256sum -c - \
  && pg_restore --file=/tmp/archive.sql /tmp/archive.dump \
  && awk '/^COPY public.native_archive / { copying=1; next } copying && $0=="\\." { exit } copying { print }' /tmp/archive.sql > /tmp/actual-rows.txt \
  && cmp /tmp/expected-rows.txt /tmp/actual-rows.txt \
  && psql --version && pg_dump --version \
  && ! pg_restore --file=/tmp/invalid.sql /dev/null
RUN --network=none /usr/bin/python3 /tmp/check-tools.py
RUN --network=none /usr/bin/python3 /tmp/check-brotli.py
RUN --network=none if [ -x /usr/local/bin/python3 ]; then /usr/local/bin/python3 /tmp/check-primary-python.py; fi
