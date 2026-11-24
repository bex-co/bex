# bex's default sandbox image: what a sandbox created with no image runs
# (ADR104 D2 "Image", w22/m447).
#
# Render's three first-party agent templates (Claude Managed Agents, Cursor
# Cloud Agents, OpenAI Agents API) create a sandbox with no image, upload an
# install script and run it unchanged. Read 2026-11-23, they need of the image:
#   - glibc: Cursor's agent bundles a Node binary linked against glibc 2.28
#     (x64) / 2.34 (arm64) and libstdc++ (the Alpine/musl image fails with
#     "cannot execute: required file not found");
#   - Node.js and npm: the OpenAI template installs `@openai/codex` with npm;
#   - curl, GNU `sha256sum --check`, GNU `find -printf`, `base64 -w0`, tar
#     with gzip, bash at /bin/bash and /usr/bin/env bash.
# Node 24 on Debian 12 is the pairing Render's own sandbox images use
# (render-examples/sandbox-images, `node:24-bookworm`), and this base digest
# is the one lego/agent-image already pins for its Node 24 stage.
#
# wget and busybox stay because sandboxes had them before (Alpine, w32/m348)
# and the isolation matrix runs `/bin/busybox` and `wget` inside a sandbox.
# Debian's GNU tools come first on PATH; a busybox applet with no Debian
# command of the same name (httpd, nc, vi, ps, ping, unzip, ...) is linked
# into /opt/busybox/bin, the last PATH entry, so nothing it links can shadow a
# command installed later. git is there because coding agents clone with it.
#
# The user stays root and the home directory is the base template's
# /home/render, created by its entrypoint (lego/backend/cmd/api/main.go).
# check.sh is the publication gate: every tool above must answer in the
# pushed image.
FROM node:24.22.2-bookworm-slim@sha256:01d566e8d94d8c619591d78652babfccd3c6992a3dffa9ed5ab39e11cf6c70e9

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    busybox ca-certificates curl git wget \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*.deb /var/log/apt /var/log/dpkg.log \
  && mkdir -p /opt/busybox/bin \
  && for applet in $(busybox --list); do \
    command -v "$applet" >/dev/null 2>&1 || ln -s /bin/busybox "/opt/busybox/bin/$applet"; \
  done

ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/busybox/bin

# The node image starts `docker-entrypoint.sh node`; a sandbox's template sets
# its own command, and a plain `docker run` opens bash, as the bash image did.
ENTRYPOINT []
CMD ["bash"]
