# syntax=docker/dockerfile:1.7
# Regression gate for deploy/hugo (never pushed): the published image's /hugo
# runs on a Debian 12 toolchain base, as on every native static build, and is
# the extended release it was pinned as.
ARG HUGO_IMAGE
ARG HUGO_VERSION=0.111.3
FROM ${HUGO_IMAGE} AS hugo
FROM buildpack-deps:bookworm@sha256:db73e309a26ddb362c11c0ee35b3d957c3a8fd2ef6a9d6c2cc76438fae2c0988
ARG HUGO_VERSION
COPY --from=hugo /hugo /usr/local/bin/hugo
RUN hugo version | tee /dev/stderr | grep -q "^hugo v${HUGO_VERSION}-[0-9a-f]*+extended linux/$(dpkg --print-architecture) "
