import json,hashlib,sqlite3,subprocess,sys,os
from pathlib import Path
from http.server import BaseHTTPRequestHandler,HTTPServer
ref=json.loads(Path('qa409-reference.json').read_text());marker=json.loads(Path('qa409-marker.json').read_text());sha=lambda b:hashlib.sha256(b).hexdigest();query='SELECT id,label,n FROM qa409 ORDER BY id'
files={p:{'bytes':len(Path(p).read_bytes()),'sha256':sha(Path(p).read_bytes())} for p in ref['files']}
assert files==ref['files'];Path('qa409.sql').write_text(ref['sql']['text'],encoding='utf8');sql=Path('qa409.sql').read_bytes();assert sha(sql)==ref['sql']['sha256']
base={'marker':marker,'files':files,'sql':{'bytes':len(sql),'sha256':sha(sql)},'python':sys.version.split()[0],'program_sha256':sha(Path(__file__).read_bytes())}
def rows(c):return [dict(zip(['id','label','n'],r)) for r in c.execute(query)]
def tool():
 try:
  r=subprocess.run(['sqlite3','-version'],capture_output=True,text=True,timeout=4);return {'status':r.returncode,'error':None,'stdout':r.stdout,'stderr':r.stderr}
 except FileNotFoundError:return {'status':None,'error':'ENOENT','stdout':'','stderr':''}
if sys.argv[1]=='before':
 c=sqlite3.connect(':memory:');c.executescript(sql.decode());v={**base,'builtin_sqlite':sqlite3.sqlite_version,'builtin_rows':rows(c),'tools':{'sqlite3':tool()}};c.close();assert v['builtin_rows']==ref['rows'];Path('qa409-before.json').write_text(json.dumps(v));print('QA409_BEFORE '+json.dumps(v),flush=True);sys.exit(0)
def proof():
 r=subprocess.run(['sqlite3','-readonly','-json','qa409.db',query],capture_output=True,text=True,timeout=4);r.check_returncode();external=json.loads(r.stdout);c=sqlite3.connect('file:qa409.db?mode=ro',uri=True);internal=rows(c);c.close();assert external==internal==ref['rows'];b=Path('qa409.db').read_bytes();assert b[:16]==b'SQLite format 3\x00';return {**base,'tools':{'sqlite3':tool()},'rows':external,'builtin_rows':internal,'builtin_sqlite':sqlite3.sqlite_version,'database':{'bytes':len(b),'sha256':sha(b),'SQLite3_header':True}}
if sys.argv[1]=='build':
 v={**proof(),'before':json.loads(Path('qa409-before.json').read_text())};Path('qa409-build.json').write_text(json.dumps(v));print('QA409_AFTER '+json.dumps(v),flush=True);sys.exit(0)
built=json.loads(Path('qa409-build.json').read_text())
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  v={'build':built,'runtime':{**proof(),'request_url':self.path}};b=json.dumps(v,ensure_ascii=False).encode();self.send_response(200);self.send_header('Content-Type','application/json; charset=utf-8');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
  if 'qa409=' in self.path:print('QA409_REQUEST '+json.dumps(v),flush=True)
HTTPServer(('0.0.0.0',int(os.environ.get('PORT','10000'))),Handler).serve_forever()
