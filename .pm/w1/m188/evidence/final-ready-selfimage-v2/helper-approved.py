#!/usr/bin/env python3
"""Read-only whitelisted operator rollout snapshot. Source/signature evidence stays a separate prerequisite."""
import argparse,datetime,hashlib,json,re,subprocess
from pathlib import Path
sha=lambda b:hashlib.sha256(b).hexdigest()
def selfimage_env_binding(container):
 env=container.get('env',[]);envfrom=container.get('envFrom',[]);checks={};overrides=sum(e.get('name')=='BEX_BACKUP_HELPER_IMAGE' for e in env)
 for name,path in (('POD_NAME','metadata.name'),('POD_NAMESPACE','metadata.namespace')):
  entries=[e for e in env if e.get('name')==name];entry=entries[0] if len(entries)==1 else {};valuefrom=entry.get('valueFrom',{});field=valuefrom.get('fieldRef',{}) if isinstance(valuefrom,dict) else {};kind='missing' if not entries else 'ambiguous' if len(entries)!=1 else 'literal' if 'value' in entry else next(iter(valuefrom)) if isinstance(valuefrom,dict) and len(valuefrom)==1 and next(iter(valuefrom)) in ('fieldRef','secretKeyRef','configMapKeyRef','resourceFieldRef') else 'unknown'
  valid=len(entries)==1 and 'value' not in entry and isinstance(valuefrom,dict) and set(valuefrom)=={'fieldRef'} and isinstance(field,dict) and set(field)<={'fieldPath','apiVersion'} and field.get('fieldPath')==path and field.get('apiVersion','v1')=='v1'
  checks[name]={'entry_count':len(entries),'source_kind':kind,'fieldPath':field.get('fieldPath') if isinstance(field,dict) and field.get('fieldPath') in ('metadata.name','metadata.namespace') else None,'matches_expected_downward_reference':valid}
 return {'valid':not overrides and not envfrom and all(c['matches_expected_downward_reference'] for c in checks.values()),'BEX_BACKUP_HELPER_IMAGE_entry_count':overrides,'envFrom_source_count':len(envfrom),'identity_sources':checks}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--kubeconfig',required=True);p.add_argument('--expected-image',required=True);p.add_argument('--root-manifest',required=True,type=Path);p.add_argument('--amd64-manifest',type=Path);p.add_argument('--output',required=True,type=Path);a=p.parse_args();assert re.fullmatch(r'ghcr.io/bex-co/bex-operator@sha256:[a-f0-9]{64}',a.expected_image);out=a.output.resolve();assert out.is_relative_to(Path('/tmp').resolve()) and not out.exists();raw=a.root_manifest.read_bytes();root='sha256:'+sha(raw);assert a.expected_image.endswith('@'+root);manifest=json.loads(raw);allowed={root};metadata={'root':{'sha256':sha(raw),'bytes':len(raw)}}
 if 'manifests' in manifest:
  children=[x for x in manifest['manifests'] if x.get('platform')=={'architecture':'amd64','os':'linux'}];assert len(children)==1 and a.amd64_manifest,'Need exact linux/amd64 child rawbytes';desc=children[0];cr=a.amd64_manifest.read_bytes();assert len(cr)==desc['size'] and 'sha256:'+sha(cr)==desc['digest'];assert 'config' in json.loads(cr);allowed.add(desc['digest']);metadata['amd64_child']={'sha256':sha(cr),'bytes':len(cr)}
 else:assert 'config' in manifest,'Unknown image root format'
 k=['kubectl','--kubeconfig',a.kubeconfig,'--request-timeout=30s']
 def get(args):return json.loads(subprocess.run(k+args+['-o','json'],capture_output=True,text=True,check=True,timeout=45).stdout)
 dep=get(['get','deployment','bex-controller-manager','-n','bex-system']);m=dep['metadata'];spec=dep['spec'];status=dep.get('status',{});manager=next(c for c in spec['template']['spec']['containers'] if c['name']=='manager');selector=','.join(key+'='+value for key,value in spec['selector']['matchLabels'].items());rs=get(['get','replicasets','-n','bex-system','-l',selector])['items'];owned={x['metadata']['uid'] for x in rs if any(o.get('kind')=='Deployment' and o.get('uid')==m['uid'] and o.get('controller') for o in x['metadata'].get('ownerReferences',[]))};pods=get(['get','pods','-n','bex-system','-l',selector])['items'];app=get(['get','applications.argoproj.io','bex-operator','-n','argocd']);rows=[];reasons=[]
 desired=spec.get('replicas',1);overrides=[e for e in manager.get('env',[]) if e['name']=='BEX_BACKUP_HELPER_IMAGE'];envfrom=manager.get('envFrom',[])
 if manager['image']!=a.expected_image:reasons.append('Deployment manager image is not expected immutable image')
 if overrides or envfrom:reasons.append('Manager resolver override/envFrom needs fresh explicit review; values are deliberately not exposed')
 template_binding=selfimage_env_binding(manager)
 if not template_binding['valid']:reasons.append('Deployment manager selfimage identity/override/envFrom sources are not uniquely bound downward references')
 if desired<1 or status.get('observedGeneration')!=m['generation'] or status.get('updatedReplicas',0)!=desired or status.get('readyReplicas',0)<desired:reasons.append('Deployment generation/new-ready replica counts have not converged')
 for pod in pods:
  pm=pod['metadata'];ps=pod.get('status',{});ownedpod=any(o.get('kind')=='ReplicaSet' and o.get('uid') in owned and o.get('controller') for o in pm.get('ownerReferences',[]))
  if not ownedpod:continue
  ctr=next((x for x in pod['spec']['containers'] if x['name']=='manager'),{});cs=next((x for x in ps.get('containerStatuses',[]) if x['name']=='manager'),{});ready=next((x for x in ps.get('conditions',[]) if x['type']=='Ready'),{});idmatch=re.search(r'(sha256:[a-f0-9]{64})$',cs.get('imageID',''));row={'pod':pm['name'],'uid':pm['uid'],'deleted_at':pm.get('deletionTimestamp'),'phase':ps.get('phase'),'manager_spec_image':ctr.get('image'),'manager_status_image':cs.get('image'),'manager_imageID':cs.get('imageID'),'manager_ready':cs.get('ready'),'Ready':ready,'imageID_matches_authenticated_root_or_amd64_child':bool(idmatch and idmatch[1] in allowed)};row['selfimage_env_binding']=selfimage_env_binding(ctr);rows.append(row)
  if not pm.get('deletionTimestamp') and not row['selfimage_env_binding']['valid']:reasons.append('Manager Pod selfimage identity/override/envFrom sources need fresh explicit review: '+pm['name'])
 active=[x for x in rows if not x['deleted_at']];valid=[x for x in active if x['manager_spec_image']==a.expected_image and x['phase']=='Running' and x['manager_ready'] and x['Ready'].get('status')=='True' and x['Ready'].get('lastTransitionTime') and x['imageID_matches_authenticated_root_or_amd64_child']]
 if len(valid)<desired or len(valid)!=len(active):reasons.append('Some non-deleting owned manager pods are old, not Ready, or have unbound runtime imageIDs')
 since=None
 if not reasons:
  latest=max(datetime.datetime.fromisoformat(x['Ready']['lastTransitionTime'].replace('Z','+00:00')) for x in valid);since=latest.isoformat().replace('+00:00','Z')
 images=app.get('spec',{}).get('source',{}).get('kustomize',{}).get('images',[])
 if 'controller='+a.expected_image not in images:reasons.append('Live GitOps Application does not select expected operator image')
 if app.get('status',{}).get('sync',{}).get('status')!='Synced' or app.get('status',{}).get('health',{}).get('status')!='Healthy':reasons.append('GitOps Application is not Synced/Healthy')
 if reasons:since=None
 report={'schema':'bex.final-operator-ready-snapshot/v1','checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'expected_image':a.expected_image,'authenticated_manifest_metadata':metadata,'allowed_imageID_digests':sorted(allowed),'deployment':{'name':m['name'],'uid':m['uid'],'generation':m['generation'],'observedGeneration':status.get('observedGeneration'),'desiredReplicas':desired,'updatedReplicas':status.get('updatedReplicas'),'readyReplicas':status.get('readyReplicas'),'manager_image':manager['image'],'BEX_BACKUP_HELPER_IMAGE_entry_count':len(overrides),'envFrom_source_count':len(envfrom),'selfimage_env_binding':template_binding},'gitops':{'Application':'argocd/bex-operator','operator_image_entries':[i for i in images if i.startswith('controller=')],'sync':app.get('status',{}).get('sync'),'health':app.get('status',{}).get('health')},'owned_manager_pods':rows,'rollout_ready':not reasons,'rollout_since_latest_expected_image_Pod_Ready':since,'Available_transition_not_used':True,'unresolved':reasons,'scope':'Whitelisted read-only rollout identity/Ready evidence only. Does not verify source/adoption ancestry or signatures. No envvalues,secrets,kubeconfigbytes copied. No restart,apply,rebuild or fleet action.'};out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'output':str(out),'sha256':sha(out.read_bytes()),'rollout_ready':not reasons,'since':since,'unresolved':reasons},indent=2))
if __name__=='__main__':main()
