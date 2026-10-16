# syntax=docker/dockerfile:1.7
FROM python:3.12-bookworm@sha256:5560e9ab8709f459489e5b8aa696eda8a07ef821e14bb122be62d91234bfa98b
RUN /usr/local/bin/python -m venv /opt/bex/poetry && /opt/bex/poetry/bin/pip install --no-cache-dir --disable-pip-version-check --no-input poetry==2.1.3 && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\npython --version"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","python qa.py"]
