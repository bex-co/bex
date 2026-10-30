from pathlib import Path
import ast,copy,datetime,json,subprocess,tempfile
P=Path(__file__).resolve().parent;S=json.loads((P/'spec.json').read_text());script=P/'hosted-disk.py';py='/private/tmp/bex7-native-acceptance-novb586u/venv/bin/python';before=(P/'ledger.json').read_bytes();checks={}
for name,args in [('create_gate',[py,str(script),'create']),('cleanup_without_proof',[py,str(script),'cleanup']),('optimization_guard',[py,'-O',str(script),'--help'])]:
 r=subprocess.run(args,capture_output=True,text=True);assert r.returncode!=0;checks[name]={'refused':True,'exit':r.returncode}
tree=ast.parse(script.read_text());functions=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name in ['own','gate']];ns={'S':S,'json':json,'datetime':datetime,'time':__import__('time')}
exec(compile(ast.Module(body=functions,type_ignores=[]),str(script),'exec'),ns)
sid='srv-synthetic-owned-preparation-only';v={'id':sid,'ownerId':S['workspace'],'name':S['name'],'serviceDetails':{'plan':'starter','disk':{'name':S['disk_name'],'mountPath':'/var/data','sizeGB':1}}}
ledger={'baseline_ids':S['protected_existing_services'],'owned':{'id':sid},'create_proof':{'HTTP':201,'response':json.dumps({'service':v})}}
ns['L']=copy.deepcopy(ledger);ns['WS']=S['workspace'];assert ns['own'](False)['id']==sid
for name,change in [('old_service_prefix',lambda x:x['service'].update(name='qa-m144-'+S['run'])),('old_disk_name',lambda x:x['service']['serviceDetails']['disk'].update(name='m144-'+S['run'])),('foreign_workspace',lambda x:x['service'].update(ownerId='tea-foreign'))]:
 bad={'service':copy.deepcopy(v)};change(bad);ns['L']=copy.deepcopy(ledger);ns['L']['create_proof']['response']=json.dumps(bad)
 try:ns['own'](False);raise RuntimeError('ownership guard did not refuse '+name)
 except AssertionError:checks[name]={'refused':True}
ns['L']=copy.deepcopy(ledger);ns['L']['baseline_ids'].append(sid)
try:ns['own'](False);raise RuntimeError('baseline guard did not refuse')
except AssertionError:checks['preexisting_id']={'refused':True}
with tempfile.TemporaryDirectory(prefix='m144-synthetic-gate-') as t:
 ns['P']=Path(t);ns['L']={};g={'coordinated_by_root':True,'workspace':S['workspace'],'fixture_name':S['name'],'disk_name':S['disk_name'],'name_date_utc':S['name_date_utc'],'bounded_csi_image':'ghcr.io/test@sha256:'+('a'*64),'operator_image':'synthetic-test-only','all_old_csi_node_pods_terminated':True,'bounded_class_active':True,'reviewed_inventory_receipt':'synthetic-test-only','service_plan':'starter','max_disks':1,'max_services':1,'max_size_gib':5,'max_runtime_seconds':7200,'max_estimated_usage_usd':.03,'authorized_actions':['create'],'expires_at':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=5)).isoformat()}
 f=Path(t)/'rollout-ready.json';f.write_text(json.dumps(g));assert ns['gate']('create')==g
 for name,key,value in [('wrong_gate_service','fixture_name','qa-m144-'+S['run']),('wrong_gate_disk','disk_name','m144-'+S['run']),('wrong_gate_UTC_date','name_date_utc','20200101'),('expired_gate','expires_at','2020-01-01T00:00:00+00:00')]:
  bad=copy.deepcopy(g);bad[key]=value;f.write_text(json.dumps(bad))
  try:ns['gate']('create');raise RuntimeError('gate did not refuse '+name)
  except AssertionError:checks[name]={'refused':True}
assert (P/'ledger.json').read_bytes()==before and not list((P/'receipts').iterdir()) and not (P/'credentials/cli.yaml').exists() and not (P/'rollout-ready.json').exists()
checks.update({'dated_service_name':S['name'],'dated_disk_name':S['disk_name'],'ledger_unchanged':True,'synthetic_tests_only':True,'HTTP_requests_made':0,'authentication_sessions_created':0,'hosted_resources_created':0})
(P/'preparation-guard-results.json').write_text(json.dumps(checks,indent=2));print(json.dumps(checks))
