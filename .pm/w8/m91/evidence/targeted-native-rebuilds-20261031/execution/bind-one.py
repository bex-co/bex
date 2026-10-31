import datetime,json,os,pathlib,subprocess,sys,time
root=pathlib.Path('/private/tmp/bex-w8-native-targeted-dispatch-20261031-0952');name=sys.argv[1]
assert name in ['python-3.11-bookworm','python-3.13-bookworm','node-26-bookworm']
case=root/name;result=json.loads((case/'result.json').read_text());rid=result['run_id']
for attempt in range(1,13):
 label='native-binding-'+str(attempt);assert not (case/(label+'.stdout')).exists()
 started=datetime.datetime.now(datetime.timezone.utc).isoformat()
 p=subprocess.run(['gh','api','--method','GET','-H','X-GitHub-Api-Version: 2022-11-28','repos/bex-co/bex/actions/runs/'+str(rid)+'/jobs?per_page=100'],env=os.environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=80)
 (case/(label+'.stdout')).write_bytes(p.stdout);(case/(label+'.stderr')).write_bytes(p.stderr)
 assert p.returncode==0
 a=json.loads(p.stdout);assert a['total_count']==len(a['jobs'])
 print(datetime.datetime.now(datetime.timezone.utc).isoformat(),[(j['id'],j['name'],j['status'],j['conclusion']) for j in a['jobs']],flush=True)
 native=[j for j in a['jobs'] if j['name'].startswith('build (')]
 if native:
  assert len(native)==1 and a['total_count']==2 and native[0]['name'].startswith('build ('+name+', ')
  assert any(j['name']=='matrix' and j['conclusion']=='success' for j in a['jobs'])
  result.update({'native_job_id':native[0]['id'],'native_job_runtime_bound':True,'latest_actual_jobs':[{'id':j['id'],'name':j['name'],'status':j['status'],'conclusion':j['conclusion']} for j in a['jobs']],'binding_read_started_at':started,'binding_observed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'binding_raw_file':label+'.stdout'})
  (case/'bound-result.json').write_text(json.dumps(result,indent=2,sort_keys=True)+'\n');break
 if any(j['name']=='matrix' and j['status']=='completed' and j['conclusion']!='success' for j in a['jobs']):raise RuntimeError('matrix failed; no native job binding')
 if attempt<12:time.sleep(15)
else:raise RuntimeError('bounded materialization read wait ended; no further POST')
