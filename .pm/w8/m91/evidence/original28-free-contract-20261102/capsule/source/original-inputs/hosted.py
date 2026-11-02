from pathlib import Path
import argparse,ast,base64,datetime,fcntl,gzip,hashlib,http.client,http.server,inspect,json,os,re,shutil,subprocess,sys,threading,time,urllib.parse,yaml
from native_naming import creation_day,creation_spec,refresh_unused
P=Path(__file__).resolve().parent;ROOT=Path('/Users/tianpan/projects/bex7');WS='tea-da2isimlm39c739m4ofg';os.umask(0o077)
LOCK=(P/'writer.lock').open('a');fcntl.flock(LOCK,fcntl.LOCK_EX|fcntl.LOCK_NB)
LEDGER=P/'ledger.json';specs=json.loads((P/'prepared-fixtures.json').read_text());c=json.loads(LEDGER.read_text()) if LEDGER.exists() else {'workspace':WS,'resources':[],'safety':'Pre-existing resources immutable; no installs/control updates; only recorded Free fixture create/delete.'}
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

def inventory(client='bex',refresh_baseline=False):
 r=cli(['services','--confirm','-o','json'],client);assert r['exit']==0;vs=[v.get('service',v.get('postgres',v.get('keyValue',v))) for v in json.loads(r['stdout']) or []]
 if 'baseline_ids' in c:
  missing=set(c['baseline_ids'])-{v['id'] for v in vs}
  if missing:
   assert refresh_baseline and not c['resources'],'pre-existing baseline IDs disappeared after preparation; never repair or silently reclassify live test state'
   records=[]
   for sid in sorted(missing):
    rec=api('GET','/v1/services/'+sid);assert rec['HTTP']=='404','missing prepared baseline requires a fresh exact404'
    records.append({'id':sid,'receipt':rec})
   c.setdefault('historical_protected_ids',[])
   c['historical_protected_ids']=sorted(set(c['historical_protected_ids'])|missing)
   c.setdefault('prepared_baseline_absences',[]).append({'at':now(),'records':records});save()
 return r,vs

def capacity(require_available=True):
 quota=api('GET','/v1/owners/'+WS+'/limits');usage=api('POST','/graphql',{'query':'query FreeCapacity($ownerId:String) { usage(ownerId:$ownerId) { workspaceId period freeInstanceHours {used included} coverage {state through degradedSources} } }','variables':{'ownerId':WS}});assert quota['HTTP']==usage['HTTP']=='200';q=decode_json(quota['response'])['services'];u=decode_json(usage['response'])['data']['usage'];hours=u['freeInstanceHours'];assert not require_available or (q['terminating']==0 and q['used']<q['limit'] and hours['included']-hours['used']>=10),'not enough verified capacity for a fresh acceptance run';return {'quota':quota,'usage':usage}

def own(x):
 assert x['id'] not in c.get('historical_protected_ids',[])
 assert x['workspace']==WS and x['id'] not in c['baseline_ids'] and x['id'] not in x['pre_ids'];r=x['create_result'];assert r['exit'] in (0,124) and len(r['captures'])==1 and r['captures'][0]['HTTP']==201;v=x['create_view'];assert v['id']==x['id'] and v['name']==x['name'] and v['ownerId']==WS and v['serviceDetails']['plan']=='free'

def history(x):
 own(x);r=cli(['deploys','list',x['id'],'--confirm','-o','json'],x['client']);assert r['exit']==0;h=json.loads(r['stdout']);x.setdefault('history_receipts',[]).append({'at':now(),'native':r});x['deploy_children']=h;x['recorded_child_ids']=sorted(set(x.get('recorded_child_ids',[]))|{d['id'] for d in h});save();return h

def captured_create(args,x):
 captures=[];capture=P/(x['name']+'-wire.json');
 def durable(v):f=capture.with_suffix('.tmp');f.write_text(json.dumps(v,indent=2));f.replace(capture)
 durable({'phase':'intent before native request','workspace':WS,'name':x['name']})
 class Handler(http.server.BaseHTTPRequestHandler):
  def log_message(self,*a):pass
  def do_POST(self):
   if self.path!='/v1/services' or captures:self.send_error(405);return
   data=self.rfile.read(int(self.headers.get('Content-Length','0')));body=json.loads(data)
   if body.get('name')!=x['name'] or body.get('ownerId')!=WS or body.get('type')!='web_service' or body.get('serviceDetails',{}).get('plan')!='free':self.send_error(405);return
   request={'method':'POST','path':self.path,'body':body,'raw_body':data.decode(),'headers':{k:v for k,v in self.headers.items() if k.lower() in ['user-agent','content-type','x-bex-workspace','x-bex-cli-version']},'authorization':'[REDACTED]'};durable({'phase':'unchanged actual native request before forward','request':request});headers={k:v for k,v in self.headers.items() if k.lower() not in ['host','connection','content-length']};headers['Host']='api.bex.co';conn=http.client.HTTPSConnection('api.bex.co',timeout=35)
   try:
    creation_day(x);conn.request('POST',self.path,body=data,headers=headers);reply=conn.getresponse();raw=reply.read();hs=reply.getheaders();decoded=gzip.decompress(raw) if reply.getheader('Content-Encoding')=='gzip' else raw;receipt={'request':request,'HTTP':reply.status,'response':decoded.decode(),'raw_response_base64':base64.b64encode(raw).decode(),'response_headers':{k:v for k,v in hs if k.lower() in ['content-type','x-request-id','cf-ray']}};captures.append(receipt);durable({'phase':'response persisted before native parse','captures':captures});self.send_response(reply.status)
    for k,v in hs:
     if k.lower() not in ['connection','transfer-encoding','content-length','keep-alive']:self.send_header(k,v)
    self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
   finally:conn.close()
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);server.daemon_threads=True;t=threading.Thread(target=server.serve_forever,daemon=True);t.start()
 try:r=cli(args,x['client'],bound=48,host='http://127.0.0.1:'+str(server.server_port)+'/v1/');r['captures']=captures;r['qualification']='Unchanged published/pinned client forwarded through owned byte-preserving relay';return r
 finally:server.shutdown();server.server_close();t.join(timeout=2)

def validators(x):
 scope={'reference':x['validation_reference'],'hashlib':hashlib,'json':json,'expected_module_sha':x.get('expected_module_sha')};exec((Path(x['fixture_dir'])/'validators.py').read_text(),scope);return scope

def objects(raw):
 decoder=json.JSONDecoder();out=[]
 while raw.strip():
  raw=raw.lstrip();v,n=decoder.raw_decode(raw);out.extend(v if isinstance(v,list) else [v]);raw=raw[n:]
 return out

def proof(x):
 h=history(x);assert x['create_result']['exit']==0;assert len(h)==1 and h[0]['status']=='live' and h[0].get('commit',{}).get('id')==x['primary_reference']['commit'],'first original deployment/source not Live';vs=validators(x);valid=vs['valid_after'];dir=Path(x['fixture_dir']);attempt={'at':now(),'history':h,'requests':[]};x.setdefault('proof_attempts',[]).append(attempt);save()
 def record(i,v):
  if i==len(attempt['requests']):attempt['requests'].append(v)
  else:attempt['requests'][i]=v
  save()
 if x['mid']==94:
  requests=[]
  for i,root in enumerate(['/rows','/','/rows']):
   path=root+'?qa409='+x['nonce']+'-'+str(i+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code}',x['create_view']['serviceDetails']['url']+path],capture_output=True,timeout=20);body,_,status=r.stdout.rpartition(b'\n');v=json.loads(body);e={'path':path,'HTTP':status.decode(),'exit':r.returncode,'body':v,'sha256':hashlib.sha256(body).hexdigest(),'pass':r.returncode==0 and status==b'200' and valid(v['build'],x) and valid(v['runtime'],x) and v['build']['database']==v['runtime']['database'] and v['runtime']['request_url']==path};record(i,e);requests.append(e)
 else:
  scope={};exec((dir/'http.py').read_text(),scope);fn=scope['exercise'];mid=x['mid'];ref=x['validation_reference']
  if mid==93:
   def healthy_before(v,x):return vs['valid_base'](v,x) and v['tools']['pandoc']['status']==0 and v['tools']['pandoc']['error'] is None
   requests=fn(x,dir,valid,healthy_before)
  elif mid in (95,):requests=fn(x,h,dir,valid,ref)
  elif mid==96:requests=fn(x,h,dir,valid,ref)
  elif mid in (98,103):requests=fn(x,dir,ref,valid,record)
  else:requests=fn(x,h,dir,valid,ref,record)
  attempt['requests']=requests;save()
 assert len(requests)==3 and all(v.get('pass') for v in requests),'independent HTTP/artifact proof failed'
 logs=cli(['logs','--resources',x['id'],'--type','build','--text',x['marker'],'--limit','100','--confirm','-o','json'],x['client']);attempt['build_logs']=logs;save();assert logs['exit']==0;messages=[v.get('message','') for v in objects(logs['stdout'])];proofs=[]
 for m in messages:
  for line in m.splitlines():
   match=re.search(r'QA[A-Za-z0-9_]* (\{.*\})',line)
   if match:
    try:v=json.loads(match[1]);proofs.append(v)
    except ValueError:pass
 assert any(valid(v,x) for v in proofs),'no native build after proof'
 app=cli(['logs','--resources',x['id'],'--type','app','--text',x['nonce'],'--limit','100','--confirm','-o','json'],x['client']);attempt['application_logs']=app;save();assert app['exit']==0;appmessages=[v.get('message','') for v in objects(app['stdout'])];assert all(any(req['path'] in message for message in appmessages) for req in requests),'native request log correlation failed';attempt['pass']=True;save();return {'first_original_deploy':h[0]['id'],'independent_HTTP_proofs':len(requests),'native_build_and_app_logs':True}

def shared(x):
 own(x);sid=x['id'];query='query Acceptance {service(id:'+json.dumps(sid)+'){id name repo branch runtime plan buildCommand startCommand preDeployCommand autoDeploy}}';checks=[]
 for surface,method,path,body in [('REST','GET','/v1/services/'+sid,None),('GraphQL','POST','/graphql',{'query':query}),('MCP','POST','/mcp',{'jsonrpc':'2.0','id':x['sweep'],'method':'tools/call','params':{'name':'get_service','arguments':{'serviceId':sid,'workspaceId':WS}}})]:
  receipt=api(method,path,body);checks.append({'surface':surface,'receipt':receipt});x['shared_readbacks']=checks;save();assert receipt['HTTP']=='200';v=decode_json(receipt['response'])
  if surface=='GraphQL':assert not v.get('errors');v=v['data']['service']
  elif surface=='MCP':
   assert not v.get('error') and not v['result'].get('isError');result=v['result'];v=result.get('structuredContent')
   if v is None:v=json.loads(next(a['text'] for a in result['content'] if a['type']=='text'))
   v=v.get('service',v)
  details=v.get('serviceDetails',{});specific=details.get('envSpecificDetails',{});build=v.get('buildCommand') if surface=='GraphQL' else specific.get('buildCommand');start=v.get('startCommand') if surface=='GraphQL' else specific.get('startCommand');assert v['id']==sid and v['name']==x['name'] and v['repo']==x['primary_reference']['repo'] and v['branch']==x['primary_reference']['branch'] and (v.get('runtime') or details.get('runtime'))==x['runtime'] and (v.get('plan') or details.get('plan'))=='free' and build==x['commands']['initial']['build'] and start==x['commands']['initial']['start'] and v['autoDeploy'] in (False,'no');checks[-1]['pass']=True;save()
 receipt=api('GET','/v1/services/'+sid+'/env-vars');x['env_readback']=receipt;save();assert receipt['HTTP']=='200';env={v['key']:v['value'] for row in decode_json(receipt['response']) for v in [row.get('envVar',row)]};assert env==x['env'];return {'exact_readbacks':['REST','GraphQL','MCP','own harmless version env']}

parser=argparse.ArgumentParser();parser.add_argument('action',choices=['baseline','create','observe','proof','shared','delete','reconcile','contract','refresh-names']);parser.add_argument('mid',type=int,nargs='?');parser.add_argument('client',choices=['bex','render'],nargs='?',default='bex');a=parser.parse_args()
if a.action=='refresh-names':
 print(json.dumps(refresh_unused(P,specs,c['resources'])));raise SystemExit
if a.action=='contract':
 receipts=[]
 for client in ['bex','render']:
  binary=P/'release/bex' if client=='bex' else P/'render-pin';dir=P/('config-'+client)
  for args in [['--version'],['--help'],['services','create','--help'],['services','delete','--help'],['deploys','list','--help'],['logs','--help']]:
   r=subprocess.run([str(binary),*args],env=clean_env(client,dir),capture_output=True,text=True,timeout=30);assert r.returncode==0;receipts.append({'client':client,'args':args,'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr})
 (P/'native-contract-receipts.json').write_text(json.dumps(receipts,indent=2));print(json.dumps({'contract_checks':len(receipts),'binary_sha256':{client:hashlib.sha256((P/('release/bex' if client=='bex' else 'render-pin')).read_bytes()).hexdigest() for client in ['bex','render']}}));raise SystemExit
if a.action=='baseline':
 assert not any('id' in x and not x['state'].startswith('deleted:') for x in c['resources']),'never reclassify this run’s live fixture as immutable baseline'
 inv,resources=inventory(refresh_baseline=True);cap=capacity();c.setdefault('baseline_history',[]).append(c.get('baseline'));c['baseline_ids']=[v['id'] for v in resources];c['baseline']={'at':now(),'inventory':inv,'capacity':cap};c['read_only_commands']=[]
 for client in ['bex','render']:
  for args in [['whoami','--confirm','-o','json'],['workspaces','--confirm','-o','json'],['workspace','set',WS,'--confirm','-o','json'],['workspace','current','--confirm','-o','json']]:
   r=cli(args,client);c['read_only_commands'].append(r);save();assert r['exit']==0
 save();print(json.dumps({'workspace':WS,'immutable_existing_service_ids':c['baseline_ids'],'native_clients':['published Bex0.3.3/embedded Render2.27','unchanged Render2.29/af9bdd180e90'],'resources_created':0}));raise SystemExit
assert a.mid is not None and 'baseline_ids' in c
if a.action=='create':
 gate=json.loads((P/'adoption-ready.json').read_text());assert gate['all_workspaces'] and gate['operator_image'] and gate['native_base_digests'] and gate['verified_at'],'actual shared-image adoption/deploy evidence required before fixtures';assert not any(x['mid']==a.mid and x['client']==a.client for x in c['resources']),'never retry a create before intent reconciliation';cap=capacity();inv,resources=inventory(a.client);x=creation_spec(P,specs,c,a.mid,a.client);assert x['name'] not in {v['name'] for v in resources};x.update(at=now(),state='create intent persisted; reconcile before retry',pre_ids=[v['id'] for v in resources],capacity=cap,pre_inventory=inv,adoption_gate=gate,recorded_child_ids=[]);c['resources'].append(x);save();r=captured_create(x['create_args'],x);x['create_result']=r;save();assert len(r['captures'])==1 and r['captures'][0]['HTTP']==201;v=json.loads(r['captures'][0]['response']);v=v.get('data',v);v=v.get('service',v);x.update(id=v['id'],create_view=v,state='created exact Free fixture with immutable wire POST201 proof; native exit recorded separately');save();own(x);h=history(x);assert len(h)==1;x['first_deploy_id']=h[0]['id'];save();assert r['exit']==0,'native timeout with POST201: fixture recorded for cleanup, no native success claim';print(json.dumps({'mid':a.mid,'client':a.client,'id':x['id'],'first_deploy':h[0]['id'],'native_exit':0,'POST':201}));raise SystemExit
x=next(x for x in c['resources'] if x['mid']==a.mid and x['client']==a.client);own(x)
if a.action=='observe':
 h=history(x);print(json.dumps({'mid':a.mid,'client':a.client,'deploys':len(h),'first_deploy':x['first_deploy_id'],'latest':h[0]['status']}))
elif a.action=='proof':print(json.dumps(proof(x)))
elif a.action=='shared':print(json.dumps(shared(x)))
elif a.action=='delete':
 assert 'delete_intent' not in x;h=history(x);assert all(v['id'] in x['recorded_child_ids'] and v['status'] in ['live','deactivated','canceled','build_failed','update_failed','pre_deploy_failed'] for v in h),'only terminal own children may be cascaded';x['delete_intent']={'at':now(),'children':h};save();x['delete_result']=cli(['services','delete',x['id'],'--confirm','-o','json'],x['client']);save();assert x['delete_result']['exit']==0;print(json.dumps({'native_delete_exit':0,'id':x['id'],'own_children':len(h)}))
elif a.action=='reconcile':
 assert 'delete_result' in x;d=api('GET','/v1/services/'+x['id']);inv,resources=inventory(x['client']);children=[api('GET','/v1/services/'+x['id']+'/deploys/'+did) for did in x['recorded_child_ids']];url=x['create_view']['serviceDetails']['url'];r=subprocess.run(['curl','--disable','-sS','--max-time','15','-o','/dev/null','-w','%{http_code}',url],capture_output=True,text=True,timeout=20);cap=capacity(require_available=False);passed=d['HTTP']=='404' and x['id'] not in {v['id'] for v in resources} and set(c['baseline_ids'])<={v['id'] for v in resources} and all(v['HTTP']=='404' for v in children) and r.returncode==0 and r.stdout=='404';x.setdefault('cleanup_receipts',[]).append({'at':now(),'detail':d,'inventory':inv,'children':children,'URL_HTTP':r.stdout,'URL_exit':r.returncode,'capacity':cap,'pass':passed});x['state']='deleted: native0/detail404/list absent/children404/runtime404/baseline preserved/quota reconciled' if passed else 'delete acknowledged; convergence pending';save();print(json.dumps({'cleanup_pass':passed,'id':x['id'],'baseline_preserved':True}));assert passed
