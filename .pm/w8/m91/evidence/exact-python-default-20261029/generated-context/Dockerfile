# syntax=docker/dockerfile:1.7
FROM localhost:32828/native-python-default-qa@sha256:fe4c9d72e071edcc46dd3e3f45759bfa714dc485d6459ef97a23a55dfeab6784
COPY --link --from=python:3.14.8-bookworm@sha256:d971782704801eedcce987f5a3aa473500e255094e86400e2078a74652973b74 /usr/lib/os-release /opt/bex-primary-os-release
RUN . /opt/bex-primary-os-release && [ -n "$VERSION_CODENAME" ] && primary_os="$VERSION_CODENAME" && . /etc/os-release && [ -n "$VERSION_CODENAME" ] && { [ "$primary_os" = "$VERSION_CODENAME" ] || { echo 'exact native primary and supplied base have different Debian distributions' >&2; exit 1; }; } && rm -f /opt/bex-primary-os-release && rm -rf /usr/local && mkdir -p /usr/local
COPY --link --from=python:3.14.8-bookworm@sha256:d971782704801eedcce987f5a3aa473500e255094e86400e2078a74652973b74 /usr/local/ /usr/local/
RUN --network=none /sbin/ldconfig
ENV PATH="/usr/local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENV PYTHON_VERSION="3.14.8"
ENV PYTHON_SHA256="c2215904f02b175596dc49351585104f4bc20341e1c47378b26a2c274360ce73"
RUN --network=none /usr/bin/bex-native-python-tools
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /usr/local/bin/node /usr/local/bin/node
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
COPY --link --from=node:24-bookworm@sha256:d862bb5d082c5ae0210ac07e8c0344cb8cf7bef6d78e9b8c50c179da923108ea /opt/yarn-v1.22.22 /opt/yarn-v1.22.22
RUN ln -sf /opt/yarn-v1.22.22/bin/yarn /usr/local/bin/yarn && ln -sf /opt/yarn-v1.22.22/bin/yarnpkg /usr/local/bin/yarnpkg
ADD --link --checksum=sha256:c729f6538e005d234889910466fd1c4c44affb0b28ea86d93fcf88aa1aefa694 https://registry.npmjs.org/pnpm/-/pnpm-10.34.7.tgz /opt/bex/pnpm.tgz
RUN mkdir -p /usr/local/lib/node_modules/pnpm && tar -xzf /opt/bex/pnpm.tgz -C /usr/local/lib/node_modules/pnpm --strip-components=1 --no-same-owner && rm /opt/bex/pnpm.tgz && ln -sf ../lib/node_modules/pnpm/bin/pnpm.cjs /usr/local/bin/pnpm && ln -sf ../lib/node_modules/pnpm/bin/pnpx.cjs /usr/local/bin/pnpx
RUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install --no-cache-dir --disable-pip-version-check --no-input poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry
COPY --link --from=mwader/static-ffmpeg:9.0.2-1@sha256:f5d100184884eb3e82b0eb14bb5ebf98da99826ac824989720297fb9ef21c822 /ffmpeg /ffprobe /
RUN ln -sf /ffmpeg /usr/local/bin/ffmpeg && ln -sf /ffprobe /usr/local/bin/ffprobe
ENV GUNICORN_CMD_ARGS="--preload --access-logfile -"
ENV FORWARDED_ALLOW_IPS="*"
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\n/usr/local/bin/python3 /opt/render/project/src/check-runtime.py build"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","/usr/local/bin/python3 /opt/render/project/src/check-runtime.py runtime"]
