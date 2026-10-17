import base64,hashlib,importlib,json,os,pathlib,shutil,subprocess,sys
from http.server import BaseHTTPRequestHandler,HTTPServer
ROOT=pathlib.Path(__file__).resolve().parent
ref=json.loads((ROOT/'qa-swig-reference.json').read_text());marker=json.loads((ROOT/'qa-swig-marker.json').read_text());mode=sys.argv[1]
def sha(b):return hashlib.sha256(b).hexdigest()
def observed():
 files={n:{'bytes':len((b:=(ROOT/n).read_bytes())),'sha256':sha(b)} for n in ref['files']};assert files==ref['files'];i=ROOT/'qa_swig.i';assert sha(i.read_bytes())==ref['interface_sha256'];v=subprocess.run(['swig','-version'],capture_output=True,text=True) if shutil.which('swig') else None
 return {'marker':marker,'python':sys.version.split()[0],'files':files,'program_sha256':sha(pathlib.Path(__file__).read_bytes()),'interface_sha256':sha(i.read_bytes()),'swig_path':shutil.which('swig'),'swig_version':v.stdout.strip() if v and v.returncode==0 else None}
def calculate():
 import qa_swig,_qa_swig
 result=[{'a':a,'b':b,'value':qa_swig.weighted_sum(a,b)} for a,b in ref['pairs']];assert result==ref['expected'];unicode=qa_swig.dessert();assert unicode==ref['unicode'];out=observed();out.update(result=result,unicode=unicode,extension={'file':pathlib.Path(_qa_swig.__file__).name,'bytes':pathlib.Path(_qa_swig.__file__).stat().st_size,'sha256':sha(pathlib.Path(_qa_swig.__file__).read_bytes())},generated_wrapper_sha256=sha((ROOT/'qa_swig_wrap.c').read_bytes()),generated_python_sha256=sha((ROOT/'qa_swig.py').read_bytes()));return out
if mode=='before':print('QA_SWIG_BEFORE '+json.dumps(observed(),ensure_ascii=False),flush=True)
elif mode=='build':
 proof=calculate();(ROOT/'qa-swig-build.json').write_text(json.dumps(proof,ensure_ascii=False));print('QA_SWIG_AFTER '+json.dumps(proof,ensure_ascii=False),flush=True)
elif mode=='serve':
 build=json.loads((ROOT/'qa-swig-build.json').read_text())
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   runtime=calculate();runtime['request_path']=self.path;body=json.dumps({'build':build,'runtime':runtime},ensure_ascii=False).encode();self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body);print('QA_SWIG_HTTP '+json.dumps({'marker':marker,'path':self.path,'result':runtime['result'],'unicode':runtime['unicode'],'extension_sha256':runtime['extension']['sha256']},ensure_ascii=False),flush=True)
 HTTPServer(('0.0.0.0',int(os.environ.get('PORT','3000'))),Handler).serve_forever()
else:raise ValueError(mode)
