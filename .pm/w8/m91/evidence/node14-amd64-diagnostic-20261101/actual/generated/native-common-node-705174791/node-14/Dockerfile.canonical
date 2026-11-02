# syntax=docker/dockerfile:1.7
FROM scratch AS bex-pnpm-amd64
ADD --checksum=sha256:0ccd4ab9ffee62e9905396dfd9f9c9d67737f58563a4e9199811cf4665965062 https://registry.npmjs.org/@pnpm/linux-x64/-/linux-x64-10.34.7.tgz /pnpm.tgz
FROM scratch AS bex-pnpm-arm64
ADD --checksum=sha256:e5f004bc5e0b1acab7b6fda01a7d41f77670f07f0ea45ce9e56878938d898292 https://registry.npmjs.org/@pnpm/linux-arm64/-/linux-arm64-10.34.7.tgz /pnpm.tgz
FROM bex-pnpm-${TARGETARCH} AS bex-pnpm-tarball
FROM busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e AS bex-pnpm
RUN --mount=type=bind,from=bex-pnpm-tarball,target=/in mkdir /out && tar -xzof /in/pnpm.tgz -C /out package/pnpm

FROM ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0
COPY --link --from=ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc /usr/lib/os-release /opt/bex-primary-os-release
RUN . /opt/bex-primary-os-release && [ -n "$VERSION_CODENAME" ] && primary_os="$VERSION_CODENAME" && . /etc/os-release && [ -n "$VERSION_CODENAME" ] && { [ "$primary_os" = "$VERSION_CODENAME" ] || { [ "$primary_os" = bullseye ] && [ "$VERSION_CODENAME" = bookworm ]; } || { echo 'exact native primary and supplied base have different Debian distributions' >&2; exit 1; }; } && rm -f /opt/bex-primary-os-release && rm -rf /usr/local && mkdir -p /usr/local
COPY --link --from=ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc /usr/local/ /usr/local/
COPY --link --from=ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc /usr/share/bex-node-brotli /usr/share/bex-node-brotli
RUN --network=none /sbin/ldconfig
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV NODE_VERSION="14.21.3"
ENV YARN_VERSION="1.22.19"
RUN --network=none [ "$(/usr/local/bin/node -p 'process.versions.node')" = 14.21.3 ]
RUN --network=none /usr/local/bin/node /usr/share/bex-security/node-check.js
COPY --link --from=python:3.9-bookworm@sha256:ff12c273af1e1814efcd8dfdefe16a70a4d901afe94dce721ffed8a34176f285 /usr/local/ /opt/bex/native-node-python/
RUN --network=none /opt/bex/native-node-python/bin/python3.9 -I /usr/libexec/bex-native/prepare-node14-tools.py 3.9.25
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /opt/yarn-v1.22.22 /opt/yarn-v1.22.22
RUN ln -sf /opt/yarn-v1.22.22/bin/yarn /usr/local/bin/yarn && ln -sf /opt/yarn-v1.22.22/bin/yarnpkg /usr/local/bin/yarnpkg
COPY --link --from=bex-pnpm /out/package/pnpm /opt/bex/pnpm/bin/pnpm
COPY --chmod=755 <<'BEX_PNPM' /usr/local/bin/pnpm
#!/bin/sh
if [ -z "${NODE+x}" ] && node=$(command -v node); then export NODE="$node"; fi
PATH="/opt/bex/pnpm/node-gyp-bin:$PATH" exec /opt/bex/pnpm/bin/pnpm "$@"
BEX_PNPM
COPY --chmod=755 <<'BEX_PNPX' /usr/local/bin/pnpx
#!/bin/sh
exec /usr/local/bin/pnpm dlx "$@"
BEX_PNPX
COPY --chmod=755 <<'BEX_NODE_GYP' /opt/bex/pnpm/node-gyp-bin/node-gyp
#!/bin/sh
if [ -n "$npm_config_node_gyp" ] && [ -f "$npm_config_node_gyp" ]; then exec "$npm_config_node_gyp" "$@"; fi
exec node /usr/local/lib/node_modules/npm/node_modules/node-gyp/bin/node-gyp.js "$@"
BEX_NODE_GYP
COPY --link --from=mwader/static-ffmpeg:9.0.2-1@sha256:f5d100184884eb3e82b0eb14bb5ebf98da99826ac824989720297fb9ef21c822 /ffmpeg /ffprobe /
RUN ln -sf /ffmpeg /usr/local/bin/ffmpeg && ln -sf /ffprobe /usr/local/bin/ffprobe
WORKDIR /opt/render/project/src
COPY . .
COPY <<'BEX_LOAD_ENV' /opt/bex/load-env
: bex-native-env-rev=none
decoder="$(command -v base64)" || exit 1
eval "bex_decode() { '$decoder' -d; }"
while IFS= read -r record; do
  [ -n "$record" ] || continue
  key=${record%%=*}
  encoded=${record#*=}
  value="$(printf '%s' "$encoded" | bex_decode 2>/dev/null && printf '.')" || exit 1
  case $key in ''|[0-9]*|*[!A-Za-z0-9_]*) continue ;; esac # a dotted name reaches the container, not the shell
  export "$key=${value%.}"
done < /run/secrets/render-env
BEX_LOAD_ENV
ARG RENDER_GIT_COMMIT
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\n/usr/bin/python3 check.py build"]
ENV NODE_ENV=production
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","/usr/bin/python3 check.py runtime"]
