# syntax=docker/dockerfile:1.7
FROM python:3.12-bookworm@sha256:5560e9ab8709f459489e5b8aa696eda8a07ef821e14bb122be62d91234bfa98b
COPY --link --from=node:24-bookworm@sha256:14078204b6bb268c107ceff6d3c43f45a888105e5c2b18b14c32b3409d93a1b0 /usr/local/bin/node /usr/local/bin/node
COPY --link --from=node:24-bookworm@sha256:14078204b6bb268c107ceff6d3c43f45a888105e5c2b18b14c32b3409d93a1b0 /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
COPY --link --from=node:24-bookworm@sha256:14078204b6bb268c107ceff6d3c43f45a888105e5c2b18b14c32b3409d93a1b0 /opt/yarn-v1.22.22 /opt/yarn-v1.22.22
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
: bex-native-env-rev=qa452-web-empty
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nset -e\ncat > qa362-observer.py <<'QA362_OBSERVER_SOURCE'\nimport hashlib,json,os,platform,sys\nfrom pathlib import Path\nfrom importlib.metadata import version\nfrom gunicorn.app.wsgiapp import WSGIApplication\nreference=json.loads(Path('qa362-reference.json').read_text()); marker=json.loads(Path('qa362-marker.json').read_text())\nfiles={n:{'bytes':len((b:=Path(n).read_bytes())),'sha256':hashlib.sha256(b).hexdigest()} for n in reference['files']}\nassert files==reference['files']\nphase=sys.argv[1];sys.argv=['gunicorn','app:app'];a=WSGIApplication()\nv={'marker':marker,'phase':phase,'python':platform.python_version(),'gunicorn':version('Gunicorn'),'flask':version('Flask'),'files':files,'gunicorn_cmd_args_present':'GUNICORN_CMD_ARGS' in os.environ,'gunicorn_cmd_args':os.environ.get('GUNICORN_CMD_ARGS'),'accesslog':a.cfg.accesslog,'preload_app':a.cfg.preload_app,'bind':a.cfg.bind}\nPath('qa362-'+phase+'.json').write_text(json.dumps(v)); print('QA362_OBSERVER '+json.dumps(v,separators=(',',':')),flush=True)\n\nQA362_OBSERVER_SOURCE\ncat > qa362-reference.json <<'QA362_REFERENCE'\n{\"repo\": \"https://github.com/render-examples/flask-hello-world\", \"branch\": \"main\", \"commit\": \"facfc73c44b35d00bb1f72e8986f0ea47cab0755\", \"files\": {\"app.py\": {\"bytes\": 109, \"sha256\": \"51102f66cefb99c52afdbf9b49b239f6a119e6fb9bfe7a6eba78f6e069934db3\"}, \"requirements.txt\": {\"bytes\": 15, \"sha256\": \"16ae3400d1e55added27ddc3382c7397563e15e6bbdb9783bd439c3859c23560\"}}}\nQA362_REFERENCE\ncat > qa362-marker.json <<'QA362_MARKER'\n\"QA452_web-empty\"\nQA362_MARKER\npip install --disable-pip-version-check -r requirements.txt\npython qa362-observer.py build"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","python qa362-observer.py runtime && exec gunicorn app:app"]
