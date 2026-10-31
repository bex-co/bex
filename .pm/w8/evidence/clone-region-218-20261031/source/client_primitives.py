from pathlib import Path
import datetime,json,os,subprocess,time,yaml
P=Path(__file__).resolve().parent
ROOT=Path('/Users/tianpan/projects/bex7')
WS='tea-da2isimlm39c739m4ofg'
LEDGER=P/'ledger.json'
c={}
def configure(root,workspace,ledger):
 global P,WS,LEDGER,c
 P=Path(root);WS=workspace;LEDGER=P/'ledger.json';c=ledger

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()

def save():
 f=LEDGER.with_suffix('.tmp');f.write_text(json.dumps(c,indent=2,ensure_ascii=False));f.chmod(0o600);f.replace(LEDGER)

def clean_env(client,dir,host='https://api.bex.co/v1/'):
 d={k:v for k,v in os.environ.items() if not k.startswith(('BEX_','RENDER_','QA_'))};prefix='BEX' if client=='bex' else 'RENDER';d.update({'PATH':str(P/'bin')+':'+os.environ['PATH'],'DO_NOT_TRACK':'1','BROWSER':'/usr/bin/true','BEX_NO_UPDATE_NOTIFIER':'1',prefix+'_HOST':host,prefix+'_CLI_CONFIG_PATH':str(dir/'cli.yaml'),prefix+'_CLI_CONFIG_DIR':str(dir),prefix+'_WORKSPACE':WS,prefix+'_CLI_DISABLE_ANALYTICS':'1'});return d

def cli(args,client='bex',bound=45,host='https://api.bex.co/v1/'):
 dir=P/('config-'+client);dir.mkdir(exist_ok=True,mode=0o700);master=P/'credentials/cli.yaml';cfg=dir/'cli.yaml';doc=yaml.safe_load(master.read_text());tokens=[v for k,v in doc.get('api',{}).items() if isinstance(v,str) and any(n in k.lower() for n in ['key','token'])];doc['api']['host']=host;cfg.write_text(yaml.safe_dump(doc));cfg.chmod(0o600);binary=P/'release/bex' if client=='bex' else P/'render-pin';start=time.monotonic()
 try:
  try:r=subprocess.run([str(binary),*args],env=clean_env(client,dir,host),capture_output=True,text=True,timeout=bound);result={'client':client,'command':[client,*args],'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr,'elapsed':round(time.monotonic()-start,3)}
  except subprocess.TimeoutExpired:result={'client':client,'command':[client,*args],'exit':124,'stdout':'','stderr':'Bounded timeout: reconcile recorded create intent before any retry','elapsed':round(time.monotonic()-start,3)}
  for k in ['stdout','stderr']:
   for token in tokens:
    if token:result[k]=result[k].replace(token,'[REDACTED]')
  return result
 finally:
  if cfg.exists():
   fresh=yaml.safe_load(cfg.read_text());fresh['api']['host']='https://api.bex.co/v1/';master.write_text(yaml.safe_dump(fresh));master.chmod(0o600);cfg.unlink()

def decode_json(raw):
 if not raw.lstrip().startswith(('{','[')):
  messages=[s[5:].strip() for s in raw.splitlines() if s.startswith('data:')];assert len(messages)==1;raw=messages[0]
 return json.loads(raw)

def private_token():
 cfg=P/'credentials/cli.yaml';d=yaml.safe_load(cfg.read_text())['api']
 expiry=int(d.get('expires_at',0))
 if expiry and expiry <= time.time()+120:
  # Use the published client's own rotation/persistence; never replay a resource mutation.
  e={k:v for k,v in os.environ.items() if not k.startswith(('BEX_','RENDER_','QA_'))}
  e.update({'PATH':str(P/'bin')+':'+os.environ['PATH'],'DO_NOT_TRACK':'1','BROWSER':'/usr/bin/true','BEX_NO_UPDATE_NOTIFIER':'1','BEX_HOST':'https://api.bex.co/v1/','BEX_CLI_CONFIG_PATH':str(cfg),'BEX_CLI_CONFIG_DIR':str(P/'credentials'),'BEX_CLI_DISABLE_ANALYTICS':'1'})
  r=subprocess.run([str(P/'release/bex'),'workspaces','-o','json'],env=e,stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=60)
  assert r.returncode==0,'own CLI refresh/read failed; preserve its private config for reconciliation'
  d=yaml.safe_load(cfg.read_text())['api'];assert int(d['expires_at'])>time.time()+120,'CLI did not persist a fresh own access token'
  c.setdefault('credential_refreshes',[]).append({'at':now(),'command':'published bex workspaces -o json','exit':0,'credentials_recorded':False});save()
 token=d.get('key') or d.get('token') or d.get('access_token');assert token;return token

def api(method,path,body=None):
 assert method=='GET' or (method=='POST' and path=='/graphql' and body['query'].lstrip().startswith('query')) or (method=='POST' and path=='/mcp' and body['method']=='tools/call' and body['params']['name']=='get_service')
 token=private_token();auth='header = "Authorization: Bearer '+token+'"\nheader = "User-Agent: render-cli/2.29.0 (macOS - 26.0.1)"\nheader = "X-Bex-Workspace: '+WS+'"\n';requestfile=P/'API-body.json';args=['curl','--disable','-sS','--max-time','30','--config','-','-X',method,'-H','Accept: application/json, text/event-stream','-H','MCP-Protocol-Version: 2025-03-26']
 if body is not None:requestfile.write_text(json.dumps(body));args+=['-H','Content-Type: application/json','--data-binary','@'+str(requestfile)]
 try:r=subprocess.run([*args,'-w','\n%{http_code}','https://api.bex.co'+path],input=auth,capture_output=True,text=True,timeout=35)
 finally:requestfile.unlink(missing_ok=True)
 raw,_,status=r.stdout.rpartition('\n');out={'at':now(),'API_diagnostic_only':True,'request':{'method':method,'path':path,'body':body,'authorization':'[REDACTED]'},'HTTP':status,'exit':r.returncode,'response':raw,'stderr':r.stderr};assert token not in json.dumps(out);return out
