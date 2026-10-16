# syntax=docker/dockerfile:1.7
FROM ruby:4.0-bookworm@sha256:7671dc16cd740b9d8d653a568d8bd2df00e24a32c53a7b7a3fc39a0a194bb9ad
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nruby --version"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","ruby qa.rb"]
