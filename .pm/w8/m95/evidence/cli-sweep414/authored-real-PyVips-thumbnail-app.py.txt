import base64,ctypes.util,hashlib,json,os,platform,sys
from pathlib import Path
from importlib.metadata import version
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
reference=json.loads(Path('qa-vips-reference.json').read_text());marker=json.loads(Path('qa-vips-marker.json').read_text());program_sha=hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
def before():
 files={n:{'bytes':len((b:=Path(n).read_bytes())),'sha256':hashlib.sha256(b).hexdigest()} for n in reference['files']};assert files==reference['files']
 return {'marker':marker,'python':platform.python_version(),'installed_pyvips':version('pyvips'),'shared_library':ctypes.util.find_library('vips'),'files':files,'program_sha256':program_sha}
if sys.argv[1]=='build':print('QA_VIPS_BEFORE '+json.dumps(before(),separators=(',',':')),flush=True)
import pyvips

def transform():
 raw=base64.b64decode(reference['input']['base64']);assert len(raw)==reference['input']['bytes'] and hashlib.sha256(raw).hexdigest()==reference['input']['sha256'];Path('qa-vips-input.ppm').write_bytes(raw)
 image=pyvips.Image.new_from_file('qa-vips-input.ppm',access='sequential').resize(0.25,kernel='nearest');pixels=bytes(image.write_to_memory());assert (image.width,image.height,image.bands,image.format)==(2,1,3,'uchar') and pixels.hex()==reference['expected_rgb_hex'];png=image.pngsave_buffer(strip=True,compression=0)
 info={**before(),'libvips':'.'.join(str(pyvips.version(i)) for i in range(3)),'input':{k:reference['input'][k] for k in ['bytes','sha256']},'image':{'width':image.width,'height':image.height,'bands':image.bands,'format':image.format,'pixels_hex':pixels.hex()},'png':{'bytes':len(png),'sha256':hashlib.sha256(png).hexdigest()}}
 return info,png
if sys.argv[1]=='build':
 info,png=transform();Path('qa-vips-build.json').write_text(json.dumps(info));Path('qa-vips-build.png').write_bytes(png);print('QA_VIPS_AFTER '+json.dumps(info,separators=(',',':')),flush=True)
else:
 build=json.loads(Path('qa-vips-build.json').read_text())
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   info,png=transform();info['request_url']=self.path;path=self.path.split('?',1)[0]
   if path not in ['/','/render','/png']:self.send_error(404);return
   data=png if path=='/png' else json.dumps({'build':build,'runtime':info}).encode();self.send_response(200);self.send_header('Content-Type','image/png' if path=='/png' else 'application/json');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data);print('QA_VIPS_HTTP '+json.dumps({'marker':marker,'path':self.path,'png_sha256':info['png']['sha256']},separators=(',',':')),flush=True)
 ThreadingHTTPServer(('0.0.0.0',int(os.environ.get('PORT','10000'))),Handler).serve_forever()
