#!/usr/bin/env python3
"""Fresh read-only legacy-primary/JS-companion census. No no-pod safety inference or rebuilds."""
import argparse,datetime,hashlib,json,re,subprocess,sys
from pathlib import Path
sha=lambda b:hashlib.sha256(b).hexdigest()
def source_scope(revision):
 def read(path):
  p=subprocess.run(['git','show',revision+':'+path],capture_output=True,check=True);return p.stdout
 native=read('lego/operator/internal/build/native.go');resolve=read('lego/operator/internal/build/nativeresolve.go');constants=dict(re.findall(r'\b(native\w+Runtime)\s*=\s*"([^"\n]+)"',native.decode()));m=re.search(r'var nativeJSCompanionRuntimes = map\[string\]bool\{([^}]+)\}',resolve.decode());assert m,'JS companion declaration changed; review before census';values=[]
 for entry in m[1].split(','):
  k,v=entry.strip().split(':',1);assert v.strip() in ('true','false');name=json.loads(k) if k.startswith('"') else constants[k]
  if v.strip()=='true':values.append(name)
 return set(values),{'source_sha':revision,'native.go_sha256':sha(native),'nativeresolve.go_sha256':sha(resolve),'JS_companion_primaries':sorted(values),'primary_node_also_included':True}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source-sha',required=True);p.add_argument('--kubeconfig');p.add_argument('--output',type=Path);p.add_argument('--describe-scope',action='store_true');a=p.parse_args();assert re.fullmatch(r'[0-9a-f]{40}',a.source_sha),'Exact deployed source SHA required';companions,scope=source_scope(a.source_sha)
 if a.describe_scope:print(json.dumps(scope,indent=2));return
 assert a.kubeconfig and a.output,'Explicit existing private kubeconfig and NEW/tmp output required';out=a.output.resolve();assert out.is_relative_to(Path('/tmp').resolve()) and not out.exists();k=['kubectl','--kubeconfig',a.kubeconfig,'--request-timeout=30s']
 def run(args):return subprocess.run(k+args,capture_output=True,text=True,timeout=45)
 def get(kind):r=run(['get',kind,'-A','-o','json']);r.check_returncode();return json.loads(r.stdout)['items']
 apps=get('apps.app.bex.co');pods=get('pods');workloads=get('deployments.apps,cronjobs.batch');rows=[]
 for app in apps:
  s=app.get('spec',{});st=app.get('status',{});m=app['metadata'];rt=s.get('runtime','').strip().lower();builder=s.get('builder','').strip().lower();typ=s.get('type','web_service');ns=m['namespace'];name=m['name'];own=[x for x in pods if x['metadata']['namespace']==ns and x['metadata'].get('labels',{}).get('app.bex.co/app')==name];templates=[]
  for x in workloads:
   if x['metadata']['namespace']!=ns or x['metadata'].get('labels',{}).get('app.bex.co/app')!=name:continue
   podspec=x['spec']['jobTemplate']['spec']['template']['spec'] if x['kind']=='CronJob' else x['spec']['template']['spec'];templates.append({'kind':x['kind'],'name':x['metadata']['name'],'suspended':x['spec'].get('suspend'),'replicas':x['spec'].get('replicas'),'images':[c['image'] for c in podspec.get('containers',[])],'lastSuccessfulTime':x.get('status',{}).get('lastSuccessfulTime')})
  row={'namespace':ns,'app':name,'runtime':rt,'builder':builder,'type':typ,'phase':st.get('phase'),'suspended':s.get('suspended',False),'deleting':bool(m.get('deletionTimestamp')),'activeRevision':st.get('activeRevision'),'active_status_image':st.get('image'),'candidate_artifact_image':st.get('artifactImage'),'workload_templates':templates,'pods':[],'affected_signals':[],'unresolved':[]}
  explicit_customer=(rt in ('docker','image')) or (not rt and builder=='dockerfile')
  row['classification']='customer_image_or_explicit_Dockerfile_excluded' if explicit_customer else 'static_build_only_no_serving_toolchain_claim' if typ=='static_site' else 'candidate'
  candidate=not explicit_customer and typ!='static_site' and (rt in companions|{'node','cnb'} or builder in ('buildpack','cnb') or (builder=='native' and rt in ('','auto')) or (not rt and builder in ('','auto') and bool(s.get('repo'))))
  if candidate:
   if builder=='native' and rt in ('','auto'):row['unresolved'].append('Native builder with unspecified/auto runtime requires actual resolved primary/JS-companion evidence; it is not outside census scope.')
   if not rt and builder in ('','auto'):row['unresolved'].append('Legacy auto strategy requires actual serving-artifact/build-strategy review; auto alone proves neither CNB nor Dockerfile.')
   for pod in own:
    pm=pod['metadata'];ps=pod.get('status',{});spec=pod['spec'];container=next((c for c in spec['containers'] if c['name']=='app'),spec['containers'][0]);status=next((c for c in ps.get('containerStatuses',[]) if c['name']==container['name']),{});entry={'pod':pm['name'],'deleted_at':pm.get('deletionTimestamp'),'phase':ps.get('phase'),'container':container['name'],'image':container['image'],'imageID':status.get('imageID'),'container_ready':status.get('ready'),'Ready':[c for c in ps.get('conditions',[]) if c['type']=='Ready']}
    if container['name']!='app':
     row['unresolved'].append('Expected app container missing; first container is not accepted as primary: '+pm['name']);row['pods'].append(entry);continue
    if entry['phase']=='Running' and not entry['deleted_at']:
     # Reuse historical bounded exec census pattern, sanitizing preload options and retaining only parsed public version/OS fields.
     script='if command -v node >/dev/null 2>&1; then printf "NODE="; env -u NODE_OPTIONS node -p \'JSON.stringify({node:process.versions.node,brotli:process.versions.brotli})\'; fi; '
     if rt=='python':script+='printf "PYTHON="; python -I --version; '
     if rt=='ruby':script+='printf "RUBY="; env -u RUBYOPT ruby --version; '
     script+='if [ -r /etc/os-release ]; then sed -n \'/^ID=/p; /^VERSION_ID=/p\' /etc/os-release; fi; if command -v dpkg-query >/dev/null 2>&1; then printf "SYSTEM_BROTLI="; dpkg-query -W -f=\'${Version}\\n\' libbrotli1; fi'
     result=run(['exec','-n',ns,pm['name'],'-c',container['name'],'--','sh','-c',script]);entry['probe_exit']=result.returncode;entry['stderr_bytes']=len(result.stderr.encode());entry['stderr_sha256']=sha(result.stderr.encode());version={}
     for line in result.stdout.splitlines():
      if line.startswith('NODE='):
       try:
        v=json.loads(line[5:]);version.update({k:v[k] for k in ('node','brotli') if isinstance(v.get(k),str) and re.fullmatch(r'\d+\.\d+\.\d+',v[k])})
       except (ValueError,TypeError,KeyError):pass
      for label,pat in (('python',r'^PYTHON=Python (\d+\.\d+\.\d+)'),('ruby',r'^RUBY=ruby (\d+\.\d+\.\d+)')):
       match=re.match(pat,line)
       if match:version[label]=match[1]
      match=re.fullmatch(r'(ID|VERSION_ID)=["\']?([a-zA-Z0-9._+-]+)["\']?',line)
      if match:version['os_'+match[1].lower()]=match[2]
      match=re.fullmatch(r'SYSTEM_BROTLI=([a-zA-Z0-9.+:~_-]+)',line)
      if match:version['system_brotli_package']=match[1]
     entry['versions']=version
     if version.get('node','').split('.')[0] in ('14','16','18','20'):row['affected_signals'].append({'pod':pm['name'],'kind':'legacy_primary_or_companion_Node','version':version['node']})
     if rt=='python' and '.'.join(version.get('python','').split('.')[:2]) in ('3.7','3.8','3.9','3.10'):row['affected_signals'].append({'pod':pm['name'],'kind':'legacy_Python_primary','version':version['python']})
     if rt=='ruby' and '.'.join(version.get('ruby','').split('.')[:2]) in ('3.0','3.1','3.2'):row['affected_signals'].append({'pod':pm['name'],'kind':'legacy_Ruby_primary','version':version['ruby']})
     if result.returncode or not version.get('node') or rt in ('python','ruby') and not version.get(rt):row['unresolved'].append('Actual running pod version probe incomplete: '+pm['name'])
    row['pods'].append(entry)
   live=[x for x in row['pods'] if x['phase']=='Running' and not x['deleted_at']]
   if not live:
    active_images=[st.get('image'),*[i for t in templates for i in t['images']]];active_images=sorted({i for i in active_images if i});row['fallback_active_image_candidates']=active_images
    if active_images:row['unresolved'].append('No running pod; resolve actual active status/CronJob/scale-zero template immutable image and primary+companion versions via existing release/registry metadata. No-pod is not safe.')
    else:row['unresolved'].append('No running pod or active-image pointer; establish current active-release/no-live-deploy evidence. Failed/canceled phase alone is not exclusion.')
  elif row['classification']=='candidate':row['classification']='not_in_native_JS_or_buildpack_scope_requires_no_safety_claim'
  rows.append(row)
 affected=[{'namespace':r['namespace'],'app':r['app'],'signals':r['affected_signals']} for r in rows if r['affected_signals']];unresolved=[{'namespace':r['namespace'],'app':r['app'],'reasons':r['unresolved'],'active_status_image':r['active_status_image'],'workload_templates':r['workload_templates'],'fallback_active_image_candidates':r.get('fallback_active_image_candidates',[])} for r in rows if r['unresolved']]
 report={'schema':'bex.final-legacy-runtime-census/v1','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_scope':scope,'total_apps':len(apps),'rows':rows,'affected_signals':affected,'unresolved':unresolved,'zero_affected_conclusion_allowed':not affected and not unresolved,'no_apply_performed':True,'limits':'Read-only version/OS/image-pointer census, not decoder/security acceptance. Static/build-only and customer Dockerfiles are explicitly scoped out, not declared patched. No-pod active cron/scale-zero releases require immutable-image/release evidence. Any unresolved eligible version blocks zero-target conclusion; root can resolve exact rows without broad fleet apply.'};out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'output':str(out),'sha256':sha(out.read_bytes()),'affected_apps':len(affected),'unresolved_apps':len(unresolved),'zero_affected_conclusion_allowed':report['zero_affected_conclusion_allowed']},indent=2))
if __name__=='__main__':main()
