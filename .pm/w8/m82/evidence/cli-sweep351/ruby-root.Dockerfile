# syntax=docker/dockerfile:1.7
FROM ruby:3.4-bookworm@sha256:80259ba83f866aaa43c689e11569dc950b2e508effca0799d50608f7c4ca6172
COPY --link --from=node:24-bookworm@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0 /usr/local/bin/node /usr/local/bin/node
COPY --link --from=node:24-bookworm@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0 /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --link --from=node:24-bookworm@sha256:3d27e5c11e5786e309ec3e03f93ae536eb36e6e5eb3714d5eb3300a36157add0 /opt/yarn-v1.22.22 /opt/yarn-v1.22.22
RUN ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && ln -sf ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && ln -sf /opt/yarn-v1.22.22/bin/yarn /usr/local/bin/yarn && ln -sf /opt/yarn-v1.22.22/bin/yarnpkg /usr/local/bin/yarnpkg
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
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env ["/bin/bash","-c",". /opt/bex/load-env\nset -e\ncat > qa351-probe.py <<'QA351_PYTHON_PROBE'\nMARKER = \"QA351_c3b88d_bex\"\nREFERENCE = {'pydub/__init__.py': {'bytes': 39, 'sha256': 'c355efd5ac1b691ddf3214cd135fa0ae77ecc200114cd42b2a90737d9fbe5410'}, 'pydub/audio_segment.py': {'bytes': 49233, 'sha256': '9f418d5215743ca48cfd2d77ba4a8e355ac9b04ae3dccba4baa5fa917deee1b4'}}\nimport os,sys,json,platform,hashlib,subprocess,wave\nfrom pathlib import Path\ndef tool(name):\n try:\n  r=subprocess.run([name,'-version'] if name in ('ffmpeg','ffprobe') else [name,'--version'],text=True,capture_output=True,timeout=20);return {'status':r.returncode,'stdout':r.stdout,'stderr':r.stderr,'error':None}\n except FileNotFoundError:return {'status':None,'stdout':'','stderr':'','error':'ENOENT'}\n except subprocess.TimeoutExpired:return {'status':None,'stdout':'','stderr':'','error':'ETIMEDOUT'}\ndef snapshot():\n audio=Path('qa351-input.wav').read_bytes()\n with wave.open('qa351-input.wav','rb') as wav:shape={'channels':wav.getnchannels(),'sample_width':wav.getsampwidth(),'rate':wav.getframerate(),'frames':wav.getnframes(),'silence':wav.readframes(wav.getnframes())==b'\\0\\0'*800}\n return {'marker':MARKER,'python':platform.python_version(),'architecture':platform.machine(),'requested_python':os.environ.get('PYTHON_VERSION'),'files':{n:{'bytes':len(Path(n).read_bytes()),'sha256':hashlib.sha256(Path(n).read_bytes()).hexdigest()} for n in ('pydub/__init__.py','pydub/audio_segment.py')},'wav':{'bytes':len(audio),'sha256':hashlib.sha256(audio).hexdigest(),'shape':shape},'tools':{n:tool(n) for n in ('ffmpeg','ffprobe','python','pip','poetry')}}\nif '--prepare' in sys.argv:\n expected=REFERENCE\n assert all({'bytes':len(Path(n).read_bytes()),'sha256':hashlib.sha256(Path(n).read_bytes()).hexdigest()}==v for n,v in expected.items())\n with wave.open('qa351-input.wav','wb') as wav:wav.setnchannels(1);wav.setsampwidth(2);wav.setframerate(8000);wav.writeframes(b'\\0\\0'*800)\n from pydub import AudioSegment\n assert len(AudioSegment.from_wav('qa351-input.wav'))==100\n print(MARKER+'_SOURCE_AND_WAV_CONFIRMED',flush=True)\nelif '--build' in sys.argv:\n value=snapshot();Path('qa351-built.json').write_text(json.dumps(value,ensure_ascii=False));print(json.dumps(value,ensure_ascii=False),flush=True)\nelse:\n from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer\n built=json.loads(Path('/opt/render/project/src/qa351-built.json').read_text());value=snapshot();value['built']=built;body=json.dumps(value,ensure_ascii=False).encode()\n class Handler(BaseHTTPRequestHandler):\n  def log_message(self,*a):pass\n  def do_GET(self):self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)\n ThreadingHTTPServer(('0.0.0.0',int(os.environ.get('PORT','3000'))),Handler).serve_forever()\n\nQA351_PYTHON_PROBE\npython qa351-probe.py --prepare\npython qa351-probe.py --build"]
ENV PORT=10000
ENTRYPOINT []
CMD ["/bin/bash","-c","python -c 'from pydub import AudioSegment; audio=AudioSegment.from_wav('\"'\"'qa351-input.wav'\"'\"'); assert len(audio)==100; audio.export('\"'\"'qa351-output.mp3'\"'\"',format='\"'\"'mp3'\"'\"')'"]
