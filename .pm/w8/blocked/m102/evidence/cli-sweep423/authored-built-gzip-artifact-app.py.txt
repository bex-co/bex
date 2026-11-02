import base64,gzip,hashlib,json,os,pathlib,shutil,struct,subprocess,sys,zlib
from http.server import BaseHTTPRequestHandler,HTTPServer
ROOT=pathlib.Path(__file__).resolve().parent;ref=json.loads((ROOT/'qa-pigz-reference.json').read_text());marker=json.loads((ROOT/'qa-pigz-marker.json').read_text());mode=sys.argv[1];data=(ref['input']['unit']*ref['input']['repetitions']).encode()
def sha(b):return hashlib.sha256(b).hexdigest()
assert len(data)==ref['input']['bytes'] and sha(data)==ref['input']['sha256']
def base():
 files={n:{'bytes':len((b:=(ROOT/n).read_bytes())),'sha256':sha(b)} for n in ref['files']};assert files==ref['files'];assert sha(pathlib.Path(__file__).read_bytes())==ref['program_sha256'];return {'marker':marker,'python':sys.version.split()[0],'files':files,'program_sha256':ref['program_sha256'],'input':{'bytes':len(data),'sha256':sha(data)}}
def proof():
 v=base();assert (ROOT/'qa-pigz-input.txt').read_bytes()==data;b=(ROOT/'qa-pigz-output.gz').read_bytes();actual=gzip.decompress(b);assert actual==data;crc,size=struct.unpack('<II',b[-8:]);assert crc==zlib.crc32(data) and size==len(data);v['gzip']={'bytes':len(b),'sha256':sha(b),'header_hex':b[:10].hex(),'CRC32':crc,'ISIZE':size,'inflated_bytes':len(actual),'inflated_sha256':sha(actual)};return v
if mode=='before':
 (ROOT/'qa-pigz-input.txt').write_bytes(data);v=base();v['pigz_path']=shutil.which('pigz');print('QA_PIGZ_BEFORE '+json.dumps(v),flush=True)
elif mode=='build':
 v=proof();r=subprocess.run(['pigz','--version'],capture_output=True,text=True,check=True);v['build_compressor_version']=(r.stdout+r.stderr).strip();(ROOT/'qa-pigz-build.json').write_text(json.dumps(v));print('QA_PIGZ_AFTER '+json.dumps(v),flush=True)
elif mode=='serve':
 build=json.loads((ROOT/'qa-pigz-build.json').read_text())
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   v=proof();v['request_path']=self.path;raw=self.path.split('?',1)[0]=='/asset.gz';b=(ROOT/'qa-pigz-output.gz').read_bytes() if raw else json.dumps({'build':build,'runtime':v}).encode();self.send_response(200);self.send_header('Content-Type','application/gzip' if raw else 'application/json');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b);print('QA_PIGZ_HTTP '+json.dumps({'marker':marker,'path':self.path,'gzip_sha256':v['gzip']['sha256'],'inflated_sha256':v['gzip']['inflated_sha256']}),flush=True)
 HTTPServer(('0.0.0.0',int(os.environ.get('PORT','3000'))),Handler).serve_forever()
else:raise ValueError(mode)
