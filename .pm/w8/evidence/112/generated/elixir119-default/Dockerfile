# syntax=docker/dockerfile:1.7
FROM elixir:1.19@sha256:ead2215da1c6756a4cef5526e55a967e355c345ab3c6c03722618143bc9faa6a
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nmix deps.get --only prod && mix compile"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","mix run --no-halt"]
