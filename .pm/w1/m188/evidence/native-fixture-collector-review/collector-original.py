#!/usr/bin/env python3
"""Collect one completed manual six-case native acceptance attempt; never dispatch or mutate jobs."""
import argparse,base64,concurrent.futures,datetime,gzip,hashlib,json,re,subprocess,sys
from pathlib import Path
REPO='bex-co/bex';WORKFLOW='.github/workflows/native-backport-acceptance.yml';PACKAGE='ghcr.io/bex-co/bex-cnb-run'
LINES={'node':{'14':'node-14-bullseye','16':'node-16-bookworm','18':'node-18-bookworm','20':'node-20-bookworm'},'python':{'3.7':'python-3.7-bullseye','3.8':'python-3.8-bookworm','3.9':'python-3.9-bookworm','3.10':'python-3.10-bookworm'},'ruby':{'3.0':'ruby-3.0-bullseye','3.1':'ruby-3.1-bookworm','3.2':'ruby-3.2-bookworm'}}
CASES={'Node '+n:('TestNativeBackportsBuildAndStart/node'+n,'node',n) for n in ('14','16','18','20')}
CASES.update({'Python 3.7 with Node 20':('TestNativePythonBackportBuildAndStart','python','3.7'),'Ruby 3.0 with Node 20':('TestNativeRubyBackportBuildAndStart','ruby','3.0')})
sha=lambda b:hashlib.sha256(b).hexdigest()
def need(value,message):
 if not value:raise ValueError(message)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def write(path,data):path.write_text(json.dumps(data,indent=2)+'\n')
def gh(endpoint,path):
 p=subprocess.run(['gh','api','repos/'+REPO+'/'+endpoint],capture_output=True,timeout=120);need(p.returncode==0,'GitHub read failed: '+endpoint+'; '+p.stderr.decode(errors='replace')[:300]);need(len(p.stdout)<=64<<20,'GitHub response exceeds64MiB evidence bound');path.write_bytes(p.stdout);return json.loads(p.stdout)
def source(head,name,out):
 # Local immutable object if present; public GitHub source read otherwise. No fetch or branch change.
 p=subprocess.run(['git','show',head+':'+name],capture_output=True)
 if p.returncode==0:raw=p.stdout
 else:
  data=gh('contents/'+name+'?ref='+head,out.with_suffix(out.suffix+'.api.json'));need(data.get('encoding')=='base64','GitHub source encoding differs');raw=base64.b64decode(data['content'],validate=False)
 out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(raw);return raw.decode()
def adopted_pins(manifest,native):
 expected={tag for values in LINES.values() for tag in values.values()};need(manifest.get('state')=='complete' and not manifest.get('remaining') and not manifest.get('failures'),'11-pin manifest is incomplete');verified=manifest.get('verified',{});need(set(verified)==expected,'Manifest must contain exactly11 reviewed legacy lines');selected={}
 for runtime,lines in LINES.items():
  constant={'node':'nativeNodeRuntime','python':'nativePythonRuntime','ruby':'nativeRubyRuntime'}[runtime]
  block=re.search(r'\b'+constant+r':\s*\{[^\n]*lines:\s*\[\]nativeLine\{(.*?)\n\t\}\},',native,re.S);need(block,'Cannot parse reviewed '+runtime+' source table')
  for line,tag in lines.items():
   rows=re.findall(r'\{\s*line:\s*"'+re.escape(line)+r'"[^{}]*\}',block[1]);need(len(rows)==1,'Missing/ambiguous source line '+tag);row=rows[0]
   fields={k:re.findall(r'\b'+k+r':\s*"([^"\n]+)"',row) for k in ('version','image','upstream')};need(all(len(v)==1 for v in fields.values()),'Adopted source must retain version/image/upstream: '+tag)
   value=verified[tag];digest=value.get('digest');need(value.get('candidate_state')=='verified' and re.fullmatch(r'sha256:[0-9a-f]{64}',digest or '') and value.get('ref')==PACKAGE+'@'+digest,'Manifest immutable identity is invalid: '+tag)
   image=PACKAGE+':native-'+tag+'@'+digest;need(fields['image'][0]==image and fields['version'][0]==value.get('version') and value.get('runtime')==runtime,'Dispatched source differs from accepted pin manifest: '+tag);need(re.fullmatch(runtime+r':[a-z0-9.-]+@sha256:[0-9a-f]{64}',fields['upstream'][0]),'Official provenance missing: '+tag)
   selected[tag]={'runtime':runtime,'version':value['version'],'image':image,'upstream':fields['upstream'][0]}
 return selected
def check_log(name,text,head,pins,job):
 test,runtime,line=CASES[name];plain=re.sub(r'\x1b\[[0-9;]*m','',text);plain='\n'.join(re.sub(r'^\d{4}-\d\d-\d\dT\S+Z\s?','',s) for s in plain.splitlines());need(re.search(r'^'+re.escape(head)+r'$',plain,re.M),'Checkout HEAD missing from actual metadata log');need(re.search(r'--- PASS:\s+'+re.escape(test)+r'\s+\(',plain),'Expected fixture did not PASS: '+test);need(not re.search(r'--- SKIP:',plain),'A fixture SKIPPED; cannot claim acceptance')
 steps={s['name']:s for s in job['steps']}
 for n in ('record source, existing builder and disk usage','build and start the selected native fixture','record disk usage after fixture cleanup'):need(steps.get(n,{}).get('conclusion')=='success','Required actual fixture step failed/missing: '+n)
 primary=pins[LINES[runtime][line]];node=pins[LINES['node']['20']] if runtime!='node' else None
 if runtime=='node':message='selected Node '+primary['version']+' image: '+primary['image']
 else:message='selected '+runtime.title()+' '+primary['version']+' image: '+primary['image']+'; Node '+node['version']+' companion image: '+node['image']
 need(message in plain,'Selected image log differs from adopted manifest: '+name);need('build and HTTP start preserved ' in plain,'Missing actual build+HTTP runtime success summary')
 for value in ('DOCKER_DEFAULT_PLATFORM: linux/amd64','BUILDX_BUILDER: default','BEX_BUILDKIT_INTEGRATION: 1','GOWORK: off'):need(value in plain,'Missing requested fixture environment evidence: '+value)
 daemons=re.findall(r'daemon architecture=(aarch64|arm64|x86_64|amd64) operating-system=(linux) version=(\S+)',plain);need(len(daemons)==1 and daemons[0][1]=='linux','Missing/ambiguous actual Docker daemon metrics');arch=daemons[0][0];normal={'aarch64':'arm64','arm64':'arm64','x86_64':'amd64','amd64':'amd64'}.get(arch);need(normal,'Unknown daemon architecture; review before acceptance')
 return {'test':test,'primary':primary,'node20_companion':node if runtime!='node' else None,'actual_selected_image_line':message,'requested_platform':'linux/amd64','daemon_architecture':arch,'daemon_version':daemons[0][2],'execution_mode':'amd64 on ARM64 daemon; cross-architecture emulation, emulator implementation not separately identified' if normal=='arm64' else 'amd64 on amd64 daemon','test_passed_without_skip':True,'no_native_arm64_acceptance_claim':True}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',required=True,type=int);p.add_argument('--attempt',type=int,default=1);p.add_argument('--head',required=True);p.add_argument('--adoption-sha',required=True);p.add_argument('--pin-manifest',required=True,type=Path);p.add_argument('--output',required=True,type=Path);a=p.parse_args();need(all(re.fullmatch(r'[0-9a-f]{40}',v) for v in (a.head,a.adoption_sha)) and a.run>0 and a.attempt>0,'Exact40-hex source/adoption SHA and positive run/attempt required');out=a.output.resolve();need(out.is_relative_to(Path('/tmp').resolve()) and not out.exists(),'Output must be a NEW owned/tmp directory');out.mkdir(parents=True)
 receipt={'schema':'bex.actual-six-native-fixture-acceptance/v1','at':now(),'complete':False,'run_id':a.run,'attempt':a.attempt,'expected_head':a.head,'adoption_sha':a.adoption_sha,'helper_sha256':sha(Path(__file__).read_bytes()),'remote_mutations':False,'no_runtime_architecture_beyond_logged_target_claimed':True}
 try:
  raw=a.pin_manifest.read_bytes();(out/'11-pin-manifest.json').write_bytes(raw);receipt['pin_manifest_sha256']=sha(raw)
  run=gh('actions/runs/'+str(a.run)+'/attempts/'+str(a.attempt),out/'run-attempt.json');need(run['id']==a.run and run['run_attempt']==a.attempt and run['head_sha']==a.head and run['event']=='workflow_dispatch' and run['head_branch']=='main' and run['path']==WORKFLOW,'Wrong dispatch/run/head/branch/workflow identity');need(run['status']=='completed','Run not complete; retry collector after completion, without polling/rerunning jobs')
  comparison=gh('compare/'+a.adoption_sha+'...'+a.head,out/'adoption-ancestry.json');need(comparison['status'] in ('ahead','identical') and comparison['merge_base_commit']['sha']==a.adoption_sha,'Dispatch HEAD does not contain exact adoption commit')
  src=out/'source';src.mkdir();native=source(a.head,'lego/operator/internal/build/native.go',src/'native.go');workflow=source(a.head,WORKFLOW,src/'native-backport-acceptance.yml');pins=adopted_pins(json.loads(raw),native);receipt['adopted_11_pins']=pins
  for n in ('nativebackport_test.go','nativebackport_python_test.go','nativebackport_ruby_test.go'):source(a.head,'lego/operator/internal/build/'+n,src/n)
  need('DOCKER_DEFAULT_PLATFORM: linux/amd64' in workflow and 'max-parallel: 1' in workflow and 'timeout-minutes: 45' in workflow,'Unexpected fixture workflow; review actual source before collecting')
  data=gh('actions/runs/'+str(a.run)+'/attempts/'+str(a.attempt)+'/jobs?per_page=100',out/'jobs.json');need(data['total_count']==len(data['jobs'])==6 and {j['name'] for j in data['jobs']}==set(CASES),'Need all six exact fixture jobs in ONE attempt; never merge latest/mixed attempts');need(all(j['status']=='completed' for j in data['jobs']),'Some fixture jobs remain incomplete')
  def one(job):
   folder=out/('job-'+str(job['id']));folder.mkdir();j=gh('actions/jobs/'+str(job['id']),folder/'job.json');need(j['run_id']==a.run and j['run_attempt']==a.attempt and j['head_sha']==a.head and j['name']==job['name'],'Job metadata identity differs from exact run attempt');q=subprocess.run(['gh','api','repos/'+REPO+'/actions/jobs/'+str(j['id'])+'/logs'],capture_output=True,timeout=120);need(q.returncode==0 and 0<len(q.stdout)<=64<<20,'Completed job log fetch failed or exceeded64MiB bound');(folder/'job.log').write_bytes(q.stdout);gz=gzip.compress(q.stdout,mtime=0);(folder/'job.log.gz').write_bytes(gz);record={'name':j['name'],'job_id':j['id'],'job_url':j['html_url'],'job_api_sha256':sha((folder/'job.json').read_bytes()),'raw_job_log_sha256':sha(q.stdout),'gzip_job_log_sha256':sha(gz),'raw_job_log_bytes':len(q.stdout),'conclusion':j['conclusion'],'validated':False}
   try:need(j['conclusion']=='success','Actual fixture job did not succeed');record.update(check_log(j['name'],q.stdout.decode(),a.head,pins,j),validated=True)
   except ValueError as e:record['failure']=str(e)
   write(folder/'check.json',record);return record
  with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:receipt['cases']=list(pool.map(one,data['jobs']))
  receipt['complete']=run['conclusion']=='success' and len(receipt['cases'])==6 and all(x['validated'] for x in receipt['cases']);need(receipt['complete'],'At least one real fixture failed/skipped or did not bind expected source/images; see retained evidence')
 except Exception as e:receipt['error']=str(e)
 finally:
  receipt['files']={str(f.relative_to(out)):{'bytes':f.stat().st_size,'sha256':sha(f.read_bytes())} for f in sorted(out.rglob('*')) if f.is_file()};receipt['scope']='Actual manual build+HTTP+decoder fixture evidence only, on requestedlinux/amd64. Does not replace supplier crypto/scans, native-arm64 runtime acceptance, deployed operator rollout or hosted tool acceptance.';write(out/'receipt.json',receipt);print(json.dumps({'receipt':str(out/'receipt.json'),'complete':receipt['complete'],'error':receipt.get('error')},indent=2))
 return 0 if receipt['complete'] else 1
if __name__=='__main__':
 try:raise SystemExit(main())
 except ValueError as error:print(str(error),file=sys.stderr);raise SystemExit(1)
