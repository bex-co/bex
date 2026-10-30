"""One fixture only. Preparation does not authenticate or call production."""
from pathlib import Path
import argparse,datetime,fcntl,hashlib,json,os,sys,time,urllib.error,urllib.parse,urllib.request
import yaml
from naming import creation_guard,refresh as refresh_name
if not __debug__:raise RuntimeError('run with Python assertions enabled')
P=Path(__file__).resolve().parent;os.umask(0o077)
S=json.loads((P/'spec.json').read_text());L=json.loads((P/'ledger.json').read_text());WS=S['workspace'];lock=(P/'writer.lock').open('a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
class NoRedirect(urllib.request.HTTPRedirectHandler):
 def redirect_request(self,*a,**kw):raise RuntimeError('redirect refused; no credential forwarding')
OPENER=urllib.request.build_opener(NoRedirect)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def save():
 f=P/'ledger.next';f.write_text(json.dumps(L,indent=2));f.replace(P/'ledger.json')
def private_token():
 d=yaml.safe_load((P/'credentials/cli.yaml').read_text())['api'];token=d.get('key') or d.get('token') or d.get('access_token');assert token;return token
def scrub(x):
 text=json.dumps(x)
 for f in ['credentials/fixture-token']:
  secret=(P/f).read_text();text=text.replace(secret,'[OWN_FIXTURE_TOKEN_REDACTED]')
 return json.loads(text)
def request(method,path,body=None,fixture=False,timeout=35):
 token=(P/'credentials/fixture-token').read_text() if fixture else private_token()
 url=own()['serviceDetails']['url'].rstrip('/')+path if fixture else 'https://api.bex.co'+path
 u=urllib.parse.urlsplit(url);assert u.scheme=='https' and not u.username and not u.password
 if fixture:assert u.hostname==S['name']+'.onbex.co','unreviewed fixture hostname'
 else:assert u.hostname=='api.bex.co'
 data=json.dumps(body,separators=(',',':')).encode() if body is not None else None
 headers={'Authorization':'Bearer '+token,'Accept':'application/json, text/event-stream','User-Agent':'m144-owned-disk-acceptance/1','X-Bex-Workspace':WS,'MCP-Protocol-Version':'2025-03-26'}
 if fixture:headers['X-Disk-QA-Run']=S['run']
 if data is not None:headers['Content-Type']='application/json'
 rec={'at':now(),'request':{'method':method,'path':path,'body':scrub(body),'body_sha256':hashlib.sha256(data).hexdigest() if data else None,'authorization':'[REDACTED]'},'target':'owned fixture' if fixture else 'control-plane'}
 stamp=str(time.time_ns());out=P/'receipts'/(stamp+'.json');out.write_text(json.dumps({'phase':'intent before request',**rec},indent=2))
 try:
  req=urllib.request.Request(url,data=data,headers=headers,method=method)
  try:r=OPENER.open(req,timeout=timeout)
  except urllib.error.HTTPError as e:r=e
  raw=r.read(4*1024*1024+1);assert len(raw)<=4*1024*1024
  rec.update({'HTTP':r.code,'response':raw.decode(),'response_sha256':hashlib.sha256(raw).hexdigest(),'request_id':r.headers.get('X-Request-ID')})
 except Exception as e:
  rec.update({'HTTP':None,'error':type(e).__name__+': '+str(e),'ambiguous_mutation':method not in ['GET','HEAD']})
 rec=scrub(rec);out.write_text(json.dumps(rec,indent=2));return rec
def value(rec,status=200):
 assert rec['HTTP']==status,rec
 raw=rec['response']
 if raw.lstrip().startswith(('data:','event:')):
  messages=[line[5:].strip() for line in raw.splitlines() if line.startswith('data:')];assert len(messages)==1;raw=messages[0]
 return json.loads(raw)
def gate(action):
 g=json.loads((P/'rollout-ready.json').read_text());assert g['coordinated_by_root'] is True and g['workspace']==WS and g['fixture_name']==S['name'] and g['disk_name']==S['disk_name'] and g['name_date_utc']==S['name_date_utc']
 assert g['bounded_csi_image'].startswith('ghcr.io/') and '@sha256:' in g['bounded_csi_image'];assert g['operator_image'] and g['all_old_csi_node_pods_terminated'] is True and g['bounded_class_active'] is True
 assert g['reviewed_inventory_receipt'] and g['service_plan']=='starter' and g['max_disks']==g['max_services']==1 and g['max_size_gib']==5 and g['max_runtime_seconds']<=7200 and g['max_estimated_usage_usd']<=.03
 assert action in g['authorized_actions'];assert datetime.datetime.now(datetime.timezone.utc)<datetime.datetime.fromisoformat(g['expires_at'])
 if L.get('created_at'):assert time.time()-datetime.datetime.fromisoformat(L['created_at']).timestamp()<g['max_runtime_seconds'],'cost window exhausted: cleanup own resources'
 return g
def services():
 out=[];cursor=''
 for _ in range(10):
  rec=request('GET','/v1/services?ownerId='+WS+'&limit=100'+('&cursor='+urllib.parse.quote(cursor) if cursor else ''));rows=value(rec);assert isinstance(rows,list)
  out += [v.get('service',v) for v in rows]
  if len(rows)<100:break
  assert rows[-1]['cursor']!=cursor;cursor=rows[-1]['cursor']
 else:raise AssertionError('pagination bound reached')
 return out
def baseline():
 assert not L.get('create_intent');rows=services();ids=[v['id'] for v in rows];assert set(S['protected_existing_services'])<=set(ids)
 assert S['name'] not in [v['name'] for v in rows];q=value(request('GET','/v1/owners/'+WS+'/limits'));assert q['services']['used']<q['services']['limit']
 disks=value(request('GET','/v1/disks?limit=100'));assert len(disks)<100
 L.update({'baseline_ids':ids,'baseline_disks':[v['disk']['id'] for v in disks],'baseline_at':now(),'limits':q});save();return {'baseline_ids':ids,'baseline_disk_count':len(disks),'limits':q}
def own(read=True):
 x=L.get('owned');assert x and x['id'] not in L['baseline_ids'] and x['id'] not in S['protected_existing_services'] and x['id'] not in S['historical_deleted_215_ids'];proof=L['create_proof'];assert proof['HTTP']==201
 v=json.loads(proof['response'])['service'];assert v['id']==x['id'] and v['name']==S['name'] and v['ownerId']==WS and v['serviceDetails']['plan']=='starter';assert v['serviceDetails']['disk']=={'name':S['disk_name'],'mountPath':S['mount'],'sizeGB':1}
 if not read:return x
 current=value(request('GET','/v1/services/'+x['id']));assert current['id']==x['id'] and current['ownerId']==WS and current['name']==S['name'] and current['serviceDetails']['plan']=='starter'
 assert current['repo']==S['source_repository'] and current['branch']==S['branch'] and current['serviceDetails']['numInstances']==1
 disk=current['serviceDetails']['disk'];assert disk['name']==S['disk_name'] and disk['mountPath']==S['mount'] and disk['sizeGB'] in [1,5]
 return current
def disk():
 v=own();rows=value(request('GET','/v1/disks?serviceId='+v['id']));assert len(rows)==1;d=rows[0]['disk'];assert d['serviceId']==v['id'] and d['name']==S['disk_name'] and d['mountPath']==S['mount'] and d['id'] not in L['baseline_disks'] and d['sizeGB'] in [1,5]
 if L.get('disk_id'):assert L['disk_id']==d['id']
 else:L['disk_id']=d['id'];L['disk_create_binding']={'from_service_inline_create':L['create_proof']['request']['body_sha256'],'first_observed':d};save()
 return d
def observe():
 v=own();history=value(request('GET','/v1/services/'+v['id']+'/deploys?limit=100'));deploys=[r.get('deploy',r) for r in history];assert deploys and len(deploys)<100
 for d in deploys:assert d.get('commit',{}).get('id')==S['expected_source_commit'],'fixture source commit changed; stop rather than silently substituting'
 initial=next(d for d in deploys if d['id']==json.loads(L['create_proof']['response'])['deployId']);assert initial['status']=='live','first fixture deployment not Live';L['deploy_ids']=[d['id'] for d in deploys];L['source_verified_at']=now();save();return {'first_deploy':initial['id'],'source_commit':S['expected_source_commit'],'deploys':len(deploys)}
def create():
 creation_guard(S)
 gate('create');assert 'baseline_ids' in L and not L.get('create_intent') and not L.get('owned');assert time.time()-datetime.datetime.fromisoformat(L['baseline_at']).timestamp()<300
 rows=services();assert set(L['baseline_ids'])<={v['id'] for v in rows} and S['name'] not in [v['name'] for v in rows]
 cmd=(P/'start-command.txt').read_text();assert len(cmd.encode())<=4096;assert hashlib.sha256((P/'disk-fixture.py').read_bytes()).hexdigest()==S['fixture_source_sha256']
 body={'ownerId':WS,'type':'web_service','name':S['name'],'repo':S['source_repository'],'branch':S['branch'],'autoDeploy':'no','envVars':[{'key':'DISK_QA_RUN','value':S['run']},{'key':'DISK_QA_TOKEN','value':(P/'credentials/fixture-token').read_text()},{'key':'PYTHON_VERSION','value':'3.13.9'}],'serviceDetails':{'runtime':'python','plan':'starter','numInstances':1,'healthCheckPath':'/healthz','disk':{'name':S['disk_name'],'mountPath':S['mount'],'sizeGB':1},'envSpecificDetails':{'buildCommand':'true','startCommand':cmd}}}
 intent_at=creation_guard(S)
 L['create_intent']={'created_at':intent_at,'at':intent_at,'name_date_utc':S['name_date_utc'],'name':S['name'],'workspace':WS,'body_sha256':hashlib.sha256(json.dumps(body,separators=(',',':')).encode()).hexdigest()};save()
 rec=request('POST','/v1/services',body);L['create_proof']=rec;save();v=value(rec,201)['service'];L['owned']=v;L['created_at']=L['create_intent']['created_at'];L['resources']=[{'kind':'service','id':v['id']}];save();own();disk();return {'created':v['id'],'disk':L['disk_id']}
def probe(path,method='GET'):
 if method=='POST':
  gate(path.strip('/'));assert L.get('source_verified_at'),'verify original first deployment before disk writes'
 rec=request(method,path,{} if method=='POST' else None,fixture=True,timeout=155);v=value(rec);assert v.get('run',v.get('after',{}).get('run'))==S['run'];L.setdefault('probe_history',[]).append({'path':path,'method':method,'response':v,'at':now()});save();return v
def latest(path):return next(v['response'] for v in reversed(L.get('probe_history',[])) if v['path']==path)
def grow():
 gate('grow');d=disk();assert d['sizeGB']==1;full=latest('/fill');assert full['observed_enospc'] is True and full['failure']['errno']==28 and full['after']['capacity_bytes']<=int(1.01*1024**3)
 before=probe('/status');hashes=probe('/hashes');L['before_online_growth']={'status':before,'hashes':hashes};save();return value(request('PATCH','/v1/disks/'+d['id'],{'sizeGB':5}))
def check_growth():
 before=L['before_online_growth'];after=probe('/status');hashes=probe('/hashes');assert after['boot']==before['status']['boot'] and after['pid']==before['status']['pid'] and after['process_started_unix']==before['status']['process_started_unix'] and after['device']==before['status']['device'] and after['directory_inode']==before['status']['directory_inode'];assert int(4.75*1024**3)<=after['capacity_bytes']<=int(5.01*1024**3);assert hashes['hashes']==before['hashes']['hashes'];return {'online_growth_verified':True,'stable_process':after['boot']}
def check_remount():
 x=L['before_redeploy'] if L.get('last_remount')=='redeploy' else L['before_restart'];s=probe('/status');h=probe('/hashes');assert s['boot']!=x['status']['boot'] and int(4.75*1024**3)<=s['capacity_bytes']<=int(5.01*1024**3) and h['hashes']==x['hashes']['hashes'];return {'new_process_same_5Gi_filesystem_and_hashes':True}
def restart(redeploy=False):
 gate('redeploy' if redeploy else 'restart');v=own();L['before_redeploy' if redeploy else 'before_restart']={'status':probe('/status'),'hashes':probe('/hashes')};L['last_remount']='redeploy' if redeploy else 'restart';save()
 return request('POST','/v1/services/'+v['id']+('/deploys' if redeploy else '/restart'),{} if not redeploy else {'clearCache':'do_not_clear'})
def snapshots():
 d=disk();return value(request('GET','/v1/disks/'+d['id']+'/snapshots'),201)
def select_snapshot():
 # Refuse automatically selecting an older/pre-fixture or other-instance key.
 own();assert not probe('/status')['files'].get('after-snapshot.bin');hashes=probe('/hashes');rows=snapshots();minimum=datetime.datetime.fromisoformat(L['snapshot_not_before']);valid=[r for r in rows if r['instanceId']==L['owned']['id'] and datetime.datetime.fromisoformat(r['createdAt'].replace('Z','+00:00'))>=minimum];assert len(valid)==1,'wait for exactly one coordinated fresh own backup; never guess a snapshot'
 L['selected_snapshot']={'view':valid[0],'hashes':hashes,'status':probe('/status'),'disk_id':L['disk_id']};save();return {'selected_at':valid[0]['createdAt'],'own_service':valid[0]['instanceId']}
def restore():
 gate('restore');d=disk();x=L['selected_snapshot'];assert x['disk_id']==d['id'];assert 'after-snapshot.bin' in probe('/status')['files'];return value(request('POST','/v1/disks/'+d['id']+'/snapshots/restore',{'snapshotKey':x['view']['snapshotKey'],'instanceId':L['owned']['id']}))
def check_restore():
 x=L['selected_snapshot'];s=probe('/status');h=probe('/hashes');assert s['boot']!=x['status']['boot'] and int(4.75*1024**3)<=s['capacity_bytes']<=int(5.01*1024**3);assert 'after-snapshot.bin' not in s['files'] and h['hashes']==x['hashes']['hashes'];return {'restored_own_snapshot_and_5Gi_cap':True}
def rpc(tool,args):return request('POST','/mcp',{'jsonrpc':'2.0','id':1,'method':'tools/call','params':{'name':tool,'arguments':args}})
def mcp_value(rec):
 v=value(rec);assert not v.get('error') and not v['result'].get('isError');r=v['result'];return r.get('structuredContent') or json.loads(next(i['text'] for i in r['content'] if i['type']=='text'))
def series_values(series):return [p['value'] for s in series for p in s.get('values',s.get('points',[]))]
def surfaces():
 d=disk();sid=d['serviceId'];did=d['id'];q='query DiskAcceptance($d:String!,$s:String!){disk(id:$d){id name mountPath sizeGB serviceId} service(id:$s){id name plan disk{name mountPath sizeGB}}}';out={'REST_disk':request('GET','/v1/disks/'+did),'GraphQL_disk':request('POST','/graphql',{'query':q,'variables':{'d':did,'s':sid}}),'MCP_disk':rpc('get_disk',{'diskId':did})}
 for metric in ['disk','disk_capacity']:
  out['REST_'+metric]=request('GET','/v1/metrics/'+('disk-usage' if metric=='disk' else 'disk-capacity')+'?resource='+sid+'&kind=service')
  out['GQL_'+metric]=request('POST','/graphql',{'query':'query($q:DatastoreMetricsQueryInput!){datastoreMetrics(query:$q){unit labels{field value} values{time value}}}','variables':{'q':{'kind':'service','resource':sid,'name':metric.upper()}}})
 out['MCP_metrics']=rpc('get_datastore_metrics',{'resourceId':sid,'kind':'service','metricTypes':['disk','disk_capacity']})
 out['Blueprint_generate']=request('POST','/v1/blueprints/generate',{'ownerId':WS,'serviceIds':[sid]});generated=value(out['Blueprint_generate']);manifest=yaml.safe_load(generated['manifest']);entries=manifest['services'];assert len(entries)==1 and entries[0]['disk']['sizeGB']==d['sizeGB'] and entries[0]['disk']['mountPath']==S['mount']
 out['Blueprint_validate']=request('POST','/v1/blueprints/validate',{'ownerId':WS,'bexYaml':generated['manifest']});assert value(out['Blueprint_validate'])['valid'] is True
 readback={'at':now(),'size_gib':d['sizeGB'],'receipts':out};L.setdefault('surface_readbacks',[]).append(readback);save()
 assert value(out['REST_disk'])==d
 gv=value(out['GraphQL_disk']);assert not gv.get('errors');gd=gv['data']['disk'];assert all(gd[k]==d[k] for k in ['id','name','mountPath','sizeGB','serviceId']);assert gv['data']['service']['disk']['sizeGB']==d['sizeGB']
 md=mcp_value(out['MCP_disk']);md=md.get('disk',md);assert all(md[k]==d[k] for k in ['id','name','mountPath','sizeGB','serviceId'])
 expected=d['sizeGB']*1024**3
 for metric in ['disk','disk_capacity']:
  rest=series_values(value(out['REST_'+metric]));g=value(out['GQL_'+metric]);assert not g.get('errors');gql=series_values(g['data']['datastoreMetrics']);assert rest and gql,'empty metric samples are not acceptance'
  if metric=='disk_capacity':assert rest[-1]==gql[-1]==expected
  else:assert 0<rest[-1]<=expected and 0<gql[-1]<=expected
 ms=mcp_value(out['MCP_metrics'])['series'];caps=[p['value'] for s in ms if s.get('labels',{}).get('metric')=='disk_capacity' for p in s.get('points',[])];used=[p['value'] for s in ms if s.get('labels',{}).get('metric')=='disk' for p in s.get('points',[])];assert caps and caps[-1]==expected and used and 0<used[-1]<=expected
 readback['verified']=True;save();return {'verified_surfaces':list(out),'size_gib':d['sizeGB'],'qualification':'kubelet source/capacity and rendered UI still require independent root receipts'}
def cleanup():
 # Cleanup remains possible after the cost-window gate expires.
 v=own();d=disk();assert L['create_proof']['HTTP']==201 and d['id']==L['disk_id'];L['cleanup_intent']={'service':v['id'],'disk':d['id'],'at':now()};save();r=request('DELETE','/v1/services/'+v['id']);L['cleanup_response']=r;save();assert r['HTTP']==204;return {'delete_accepted':v['id'],'next':'cleanup-check until exact service/disk 404 and children/provider deletion verified'}
def cleanup_check():
 x=own(False);sid=x['id'];did=L['disk_id'];sr=request('GET','/v1/services/'+sid);dr=request('GET','/v1/disks/'+did);rows=services();assert set(L['baseline_ids'])<={v['id'] for v in rows};assert sr['HTTP']==dr['HTTP']==404 and sid not in [v['id'] for v in rows];L['cleanup_api_verified']={'at':now(),'service_404':True,'disk_404':True,'baseline_preserved':True,'remaining':'root must verify owned App/PVC/PV/LUKSSecret/Jobs and provider-volume deletion; no borrowed secret reads'};save();return L['cleanup_api_verified']
A=argparse.ArgumentParser();A.add_argument('command',choices=['refresh-name','baseline','create','observe','status','hashes','fill','grow','check-growth','growth','compact','restart','check-remount','redeploy','snapshot-ready','snapshots','select-snapshot','marker','restore','check-restore','surfaces','cleanup','cleanup-check']);a=A.parse_args();cmd=a.command
if cmd=='refresh-name':out=refresh_name(already_locked=True)
elif cmd=='baseline':out=baseline()
elif cmd=='create':out=create()
elif cmd=='observe':out=observe()
elif cmd in ['status','hashes']:out=probe('/'+cmd)
elif cmd=='fill':
 assert not any(v['path']=='/fill' for v in L.get('probe_history',[]));out=probe('/fill','POST');assert out['observed_enospc'] is True
elif cmd=='growth':
 check_growth();out=probe('/growth','POST');h=probe('/hashes');original=L['before_online_growth']['hashes']['hashes'];assert all(h['hashes'][k]==v for k,v in original.items()) and h['hashes']['fill.bin']['bytes']+h['hashes']['growth.bin']['bytes']>1024**3;L['growth_write_verified']=True;save()
elif cmd=='compact':
 assert L.get('growth_write_verified') is True;out=probe('/compact','POST');L['compacted_at']=now();save()
elif cmd=='grow':out=grow()
elif cmd=='check-growth':out=check_growth()
elif cmd=='check-remount':out=check_remount()
elif cmd in ['restart','redeploy']:out=restart(cmd=='redeploy')
elif cmd=='snapshots':out=snapshots()
elif cmd=='snapshot-ready':
 assert L.get('compacted_at') and not L.get('snapshot_not_before');L['snapshot_not_before']=now();save();out={'not_before':L['snapshot_not_before'],'next':'root coordinates existing own backup CronJob run after compact; no public snapshot-create endpoint'}
elif cmd=='select-snapshot':out=select_snapshot()
elif cmd=='marker':out=probe('/after-snapshot','POST')
elif cmd=='restore':out=restore()
elif cmd=='check-restore':out=check_restore()
elif cmd=='surfaces':out=surfaces()
elif cmd=='cleanup':out=cleanup()
else:out=cleanup_check()
print(json.dumps(scrub(out),indent=2))
