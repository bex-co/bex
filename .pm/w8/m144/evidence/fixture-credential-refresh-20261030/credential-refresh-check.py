"""Synthetic helper policy checks plus a real published CLI against localhost only."""
import ast, datetime, hashlib, http.server, json, os, subprocess, tempfile, threading, time, types
from pathlib import Path
import yaml
P=Path(__file__).resolve().parent
before=(P/'ledger.json').read_bytes()
tree=ast.parse((P/'hosted-disk.py').read_text())
node=next(n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='private_token')
checks={}
with tempfile.TemporaryDirectory(prefix='m144-synthetic-refresh-') as tmp:
 q=Path(tmp);(q/'credentials').mkdir();cfg=q/'credentials/cli.yaml';calls=[];ledger={};saves=[]
 def write(api):cfg.write_text(yaml.safe_dump({'api':api}))
 def run(args,**kw):
  assert args==[str(q/'release/bex'),'workspaces','-o','json']
  assert kw['env']['BEX_CLI_CONFIG_PATH']==str(cfg) and kw['env']['BEX_HOST']=='https://api.bex.co/v1/'
  assert kw['stdin']==kw['stdout']==kw['stderr']==subprocess.DEVNULL
  calls.append('read-only command')
  write({'key':'synthetic-rotated','expires_at':int(time.time())+900,'refreshtoken':'synthetic-new-refresh'})
  return types.SimpleNamespace(returncode=0)
 ns={'P':q,'yaml':yaml,'time':time,'os':os,'subprocess':types.SimpleNamespace(run=run,DEVNULL=subprocess.DEVNULL),'L':ledger,'now':lambda:'synthetic-time','save':lambda:saves.append(True)}
 exec(compile(ast.Module(body=[node],type_ignores=[]),'hosted-disk.py','exec'),ns)
 write({'key':'synthetic-fresh','expires_at':int(time.time())+900,'refreshtoken':'synthetic-refresh'})
 assert ns['private_token']()=='synthetic-fresh' and not calls
 checks['fresh_token_no_command']=True
 write({'key':'synthetic-expiring','expires_at':int(time.time())+30,'refreshtoken':'synthetic-refresh'})
 assert ns['private_token']()=='synthetic-rotated' and len(calls)==1 and len(saves)==1
 assert 'synthetic-rotated' not in json.dumps(ledger) and 'synthetic-new-refresh' not in json.dumps(ledger)
 checks['expiring_token_reads_rotated_config_without_secret_receipt']=True
 write({'key':'synthetic-machine-key'})
 assert ns['private_token']()=='synthetic-machine-key' and len(calls)==1
 checks['nonexpiring_machine_key_not_refreshed']=True
 write({'key':'synthetic-expired','expires_at':1,'refreshtoken':'synthetic-refresh'})
 ns['subprocess'].run=lambda *a,**kw:types.SimpleNamespace(returncode=1)
 try:ns['private_token']();raise RuntimeError('failed refresh was accepted')
 except AssertionError:checks['failed_refresh_refused_without_resource_retry']=True
paths=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*a):pass
 def reply(self,body):
  raw=json.dumps(body).encode();self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
 def do_POST(self):
  path=self.path.split('?',1)[0];paths.append(['POST',path]);self.rfile.read(int(self.headers.get('Content-Length','0')))
  if path!='/v1/token/refresh/':self.send_error(404);return
  self.reply({'access_token':'localhost-synthetic-new-access','refresh_token':'localhost-synthetic-new-refresh','token_type':'Bearer','expires_in':900})
 def do_GET(self):
  path=self.path.split('?',1)[0];paths.append(['GET',path])
  if path!='/v1/owners' or self.headers.get('Authorization')!='Bearer localhost-synthetic-new-access':self.send_error(403);return
  self.reply([{'owner':{'id':'tea-localhost-synthetic','name':'localhost synthetic only','email':'qa@example.invalid','type':'team'}}])
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
try:
 with tempfile.TemporaryDirectory(prefix='m144-published-cli-localhost-') as tmp:
  q=Path(tmp);cfg=q/'cli.yaml';host='http://127.0.0.1:'+str(server.server_port)+'/v1/'
  cfg.write_text(yaml.safe_dump({'version':2,'api':{'key':'localhost-synthetic-old-access','expires_at':1,'host':host,'refreshtoken':'localhost-synthetic-old-refresh'}}));cfg.chmod(0o600)
  e={k:v for k,v in os.environ.items() if not k.startswith(('BEX_','RENDER_','QA_'))};e.update({'DO_NOT_TRACK':'1','BROWSER':'/usr/bin/true','BEX_NO_UPDATE_NOTIFIER':'1','BEX_HOST':host,'BEX_CLI_CONFIG_PATH':str(cfg),'BEX_CLI_CONFIG_DIR':str(q),'BEX_CLI_DISABLE_ANALYTICS':'1'})
  r=subprocess.run([str(P/'release/bex'),'workspaces','-o','json'],env=e,stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=60)
  assert r.returncode==0, 'published CLI localhost exit '+str(r.returncode)
  api=yaml.safe_load(cfg.read_text())['api'];assert api['key']=='localhost-synthetic-new-access' and api['expires_at']>time.time()+120
  assert 1<=len(paths[:-1])<=3 and all(p==['POST','/v1/token/refresh/'] for p in paths[:-1]) and paths[-1]==['GET','/v1/owners'],paths
  checks['real_published_cli_rotates_persists_and_reads_on_localhost']=True
finally:server.shutdown();server.server_close();thread.join()
assert (P/'ledger.json').read_bytes()==before and not (P/'credentials/cli.yaml').exists()
receipt={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':checks,'real_binary_sha256':hashlib.sha256((P/'release/bex').read_bytes()).hexdigest(),'real_cli_http_paths':paths,'scope':'All credentials synthetic; HTTP server bound only127.0.0.1. Actual production token refresh not yet exercised. No production login/request/resource/ledger mutation.'}
(P/'credential-refresh-results.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
