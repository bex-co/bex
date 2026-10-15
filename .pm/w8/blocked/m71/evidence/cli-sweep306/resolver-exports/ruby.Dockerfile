# syntax=docker/dockerfile:1.7
FROM ruby:3.4-bookworm@sha256:80259ba83f866aaa43c689e11569dc950b2e508effca0799d50608f7c4ca6172
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
  export "$key=${value%.}"
done < /run/secrets/render-env
BEX_LOAD_ENV
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nprintf '%s\\n' QA306_SOURCE"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","printf '%s\\n' QA306_START"]
