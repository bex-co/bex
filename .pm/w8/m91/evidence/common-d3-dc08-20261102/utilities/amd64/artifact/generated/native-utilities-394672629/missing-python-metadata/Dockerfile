# syntax=docker/dockerfile:1.7
FROM ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0
RUN --network=none /usr/bin/python3 -c 'import setuptools' && /usr/bin/python3 -m pip --version && /usr/bin/python3-config --includes && test -f /usr/include/python3.11/Python.h && rm -r /usr/lib/python3/dist-packages/setuptools && test ! -e /usr/lib/python3/dist-packages/setuptools && /usr/bin/python3 -m pip --version && test -f /usr/include/python3.11/Python.h && echo BEX_SUPPLIER_REMOVAL_READY missing-python-metadata-1793615771938373003
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /usr/lib/os-release /opt/bex-primary-os-release
RUN . /opt/bex-primary-os-release && [ -n "$VERSION_CODENAME" ] && primary_os="$VERSION_CODENAME" && . /etc/os-release && [ -n "$VERSION_CODENAME" ] && { [ "$primary_os" = "$VERSION_CODENAME" ] || { echo 'exact native primary and supplied base have different Debian distributions' >&2; exit 1; }; } && rm -f /opt/bex-primary-os-release && rm -rf /usr/local && mkdir -p /usr/local
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /usr/local/ /usr/local/
RUN --network=none /sbin/ldconfig
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV NODE_VERSION="24.22.1"
ENV YARN_VERSION="1.22.22"
RUN --network=none [ "$(/usr/local/bin/node -p 'process.versions.node')" = 24.22.1 ]
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /opt/yarn-v1.22.22 /opt/yarn-v1.22.22
RUN ln -sf /opt/yarn-v1.22.22/bin/yarn /usr/local/bin/yarn && ln -sf /opt/yarn-v1.22.22/bin/yarnpkg /usr/local/bin/yarnpkg
ADD --link --checksum=sha256:c729f6538e005d234889910466fd1c4c44affb0b28ea86d93fcf88aa1aefa694 https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz
RUN mkdir -p /usr/local/lib/node_modules/pnpm && tar -xzf /opt/bex/pnpm.tgz -C /usr/local/lib/node_modules/pnpm --strip-components=1 --no-same-owner && rm /opt/bex/pnpm.tgz && ln -sf ../lib/node_modules/pnpm/bin/pnpm.cjs /usr/local/bin/pnpm && ln -sf ../lib/node_modules/pnpm/bin/pnpx.cjs /usr/local/bin/pnpx
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\n/usr/bin/python3 original-cases.py build node 103"]
ENV NODE_ENV=production
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c",""]
