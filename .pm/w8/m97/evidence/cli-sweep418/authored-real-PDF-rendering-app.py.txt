import base64,hashlib,json,os,platform,shutil,struct,subprocess,sys
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
ref=json.loads(Path('qa-gs-reference.json').read_text());marker=json.loads(Path('qa-gs-marker.json').read_text());phase=sys.argv[1];program_sha=hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
if phase=='before':Path('qa-gs.pdf').write_bytes(base64.b64decode(ref['input']['base64']))
def digest(path):
 b=Path(path).read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
def before():
 files={n:digest(n) for n in ref['files']};pdf=digest('qa-gs.pdf');assert files==ref['files'] and pdf=={k:ref['input'][k] for k in ['bytes','sha256']}
 return {'marker':marker,'python':platform.python_version(),'files':files,'pdf':pdf,'program_sha256':program_sha,'gs_path':shutil.which('gs')}
if phase=='before':print('QA_GS_BEFORE '+json.dumps(before(),separators=(',',':')),flush=True);raise SystemExit(0)
def prove(png):
 assert png[:8]==bytes.fromhex('89504e470d0a1a0a') and struct.unpack('>IIBBBBB',png[16:29])==(2,1,8,2,0,0,0);r=subprocess.run(['gs','--version'],capture_output=True,text=True,timeout=5);assert r.returncode==0
 return {**before(),'gs_version':r.stdout.strip(),'png':{'bytes':len(png),'sha256':hashlib.sha256(png).hexdigest(),'base64':base64.b64encode(png).decode()}}
def render():
 r=subprocess.run(['gs','-q','-dBATCH','-dNOPAUSE','-sDEVICE=png16m','-r72','-dFirstPage=1','-dLastPage=1','-sOutputFile=-','qa-gs.pdf'],capture_output=True,timeout=12);assert r.returncode==0,r.stderr.decode(errors='replace');return prove(r.stdout),r.stdout
if phase=='build':
 v=prove(Path('qa-gs-build.png').read_bytes());Path('qa-gs-build.json').write_text(json.dumps(v));print('QA_GS_AFTER '+json.dumps(v,separators=(',',':')),flush=True)
else:
 build=json.loads(Path('qa-gs-build.json').read_text())
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   info,png=render();info['request_url']=self.path;path=self.path.split('?',1)[0]
   if path not in ['/','/render','/png']:self.send_error(404);return
   data=png if path=='/png' else json.dumps({'build':build,'runtime':info}).encode();self.send_response(200);self.send_header('Content-Type','image/png' if path=='/png' else 'application/json');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data);print('QA_GS_HTTP '+json.dumps({'marker':marker,'path':self.path,'png_sha256':info['png']['sha256']},separators=(',',':')),flush=True)
 ThreadingHTTPServer(('0.0.0.0',int(os.environ.get('PORT','10000'))),Handler).serve_forever()
