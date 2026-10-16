# syntax=docker/dockerfile:1.7
FROM elixir:1.17@sha256:0bdcc080e296103b68fd5eaf0b39aa947b5c53433894c2a1af6467f9f32245c0
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nmix deps.get --only prod"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","mix --version"]
