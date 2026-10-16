# syntax=docker/dockerfile:1.7
FROM node:22-bookworm@sha256:0e5f906573693feaa1e21057ebdcfdb5bd5021f050b2dc7c9deceb629c7da2a8
ADD --link --checksum=sha256:44d7db90fcbb2315b581f85989a913466765421fc656c28e80fac1b7ab5be456 https://registry.npmjs.org/pnpm/-/pnpm-10.34.6.tgz /opt/bex/pnpm.tgz
RUN mkdir -p /usr/local/lib/node_modules/pnpm && tar -xzf /opt/bex/pnpm.tgz -C /usr/local/lib/node_modules/pnpm --strip-components=1 --no-same-owner && rm /opt/bex/pnpm.tgz && ln -sf ../lib/node_modules/pnpm/bin/pnpm.cjs /usr/local/bin/pnpm && ln -sf ../lib/node_modules/pnpm/bin/pnpx.cjs /usr/local/bin/pnpx
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nnode -e 'console.log(JSON.stringify({phase:\"build\",present:Object.hasOwn(process.env,\"NODE_ENV\"),value:process.env.NODE_ENV??null}))'"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","node -e 'console.log(JSON.stringify({phase:\"runtime\",present:Object.hasOwn(process.env,\"NODE_ENV\"),value:process.env.NODE_ENV??null}))'"]
