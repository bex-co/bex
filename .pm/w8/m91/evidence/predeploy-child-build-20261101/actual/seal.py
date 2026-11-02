"""Separate final diagnostic disposition; preserve the original controller's false result."""
from pathlib import Path
import datetime,hashlib,json,re,stat
R=Path(__file__).resolve().parent;E=R/'evidence';SRC=Path('/private/tmp/bex-w8-common-predeploy-build-source-20261101-wp_wtewn')
def sha(p):
 h=hashlib.sha256()
 with p.open('rb') as f:
  for b in iter(lambda:f.read(1024**2),b''):h.update(b)
 return h.hexdigest()
def desc(p):
 s=p.lstat();assert stat.S_ISREG(s.st_mode) and not p.is_symlink()
 return {'path':str(p),'bytes':s.st_size,'sha256':sha(p),'mode':format(stat.S_IMODE(s.st_mode),'04o')}
def put(n,x):(R/n).write_text(json.dumps(x,indent=2)+'\n')
original=desc(E/'RESULT.json');result=json.loads((E/'RESULT.json').read_bytes());commands=json.loads((E/'commands.json').read_bytes());cmd={x['label']:x for x in commands};plan=json.loads((SRC/'PLAN.json').read_bytes());owner=plan['owner']
assert result['PASS'] is False and result['build_exit0'] is True and result['failure'] is None and result['cleanup_errors']==['existing cleanup helper nonzero'] and result['pending_pgids']==[] and result['source_unchanged']
assert all(x['group_absent'] and x['direct_child_reaped'] and x['bound'] is None for x in commands)
helper=json.loads((E/'helper-cleanup-command.json').read_bytes());assert helper['exit_code']==1 and 'refusing cleanup without an exact qualification owner' in helper['stderr']
assert cmd['case']['exit_code']==0 and cmd['case']['bound'] is None
for label in ('runner-remove','builder-remove','volume-remove','container-owned-absence','image-owned-absence'):assert cmd[label]['exit_code']==0
for label in ('runner-cleanup-after','builder-cleanup-after','volume-cleanup-after'):
 assert cmd[label]['exit_code']==1 and (E/cmd[label]['stdout']).read_bytes().strip() in (b'',b'[]')
for label in ('container-owned-absence','image-owned-absence'):assert (E/cmd[label]['stdout']).read_bytes().strip()==b''
assert not (R/(owner+'-docker-config')).exists()
runner=json.loads((E/'runner-cleanup-before.stdout').read_bytes())[0];builder=json.loads((E/'builder-cleanup-before.stdout').read_bytes())[0];volume=json.loads((E/'volume-cleanup-before.stdout').read_bytes())[0]
assert runner['Name']=='/'+owner+'-runner' and runner['Config']['Labels']['co.bex.native-qualification']==owner and runner['Id']==result['runner_cid']
assert builder['Name']=='/buildx_buildkit_'+owner+'0' and builder['Config']['Image']==plan['buildkit_image']
assert volume['Name']=='buildx_buildkit_'+owner+'0_state'
assert [x['Name'] for x in builder['Mounts'] if x['Destination']=='/var/lib/buildkit']==[volume['Name']]
reconciliation={'state':'EXACT_OWNED_CLEANUP_RECONCILED_WITH_ORIGINAL_HELPER_ERROR_RETAINED','owner':owner,'runner':{'name':runner['Name'],'id':runner['Id'],'label':owner,'removed_exit':cmd['runner-remove']['exit_code'],'canonical_absence':desc(E/'runner-cleanup-after.stderr')},'builder':{'name':builder['Name'],'id':builder['Id'],'requested_image':builder['Config']['Image'],'removed_exit':cmd['builder-remove']['exit_code'],'canonical_absence':desc(E/'builder-cleanup-after.stderr')},'state_volume':{'name':volume['Name'],'removed_exit':cmd['volume-remove']['exit_code'],'canonical_absence':desc(E/'volume-cleanup-after.stderr')},'own_image_label_absent':True,'own_container_label_absent':True,'own_config_absent':True,'all_client_groups_reaped':True,'original_helper_error':helper,'original_controller_result':original,'no_foreign_prune_or_default_context_change':True,'shared_immutable_input_cache_preserved':True,'future_owner_rule':'Future local plans must start with the existing strict helper-compatible bex-nq +24hex owner; this approved local owner and original errors are never rewritten.'}
put('CLEANUP-RECONCILIATION.json',reconciliation)
identity=json.loads((E/'OCI-IDENTITY.json').read_bytes());metadata=json.loads((E/'build-metadata.json').read_bytes());assert identity['PASS'] and metadata['containerimage.digest']==identity['platform_manifest']['digest'] and metadata['containerimage.config.digest']==identity['config']['digest'] and metadata['containerimage.descriptor']['platform']=={'architecture':'arm64','os':'linux'}
assert all(x['mediaType']=='application/vnd.oci.image.layer.v1.tar+gzip' for x in identity['layers'])
assert identity['labels']['org.opencontainers.image.revision']=='1feba939b4697894e800ed0abb4f25a0bf415641'
text=(E/'case.stderr').read_text();proof=[]
for line in text.splitlines():
 if line.startswith('#') and '{"phase": "build"' in line:
  start=line.index('{');x=json.loads(line[start:]);assert x['pass'] is True;proof.append({'payload':x,'exact_raw_log_line_sha256':hashlib.sha256(line.encode()).hexdigest()})
assert len(proof)==2 and len(proof[0]['payload']['checks'])==11
seven=proof[1]['payload']['original_cases'];assert len(seven)==7 and {x['mid'] for x in seven}=={91,93,98,99,101,103,107} and all(x['original_authored_build'] and x['source_and_data_proof'] for x in seven)
put('ACTUAL-BUILD-PROOFS.json',{'scope':'Actual canonical authored build phase only, networknone; no CMD/runtime/HTTP/predeploy program execution','proofs':proof,'full_log':desc(E/'case.stderr'),'build_client':cmd['case']})
# Recheck immutable template/complete source, no recompilation/OCI/CI execution.
for n,w in json.loads((SRC/'SOURCE-MANIFEST.json').read_bytes()).items():assert sha(SRC/n)==w['sha256'] and (SRC/n).stat().st_size==w['bytes']
source=json.loads(Path(plan['source_manifest']).read_bytes())
for n,w in source['files'].items():
 p=Path(plan['source_root'])/n;assert p.stat().st_size==w['bytes'] and sha(p)==w['sha256'] and stat.S_IMODE(p.stat().st_mode)==w['permissions']
for n,w in json.loads(Path(plan['context_manifest']).read_bytes()).items():
 p=Path(plan['context_manifest']).parent/n;assert p.stat().st_size==w['bytes'] and sha(p)==w['sha256']
put('RECEIPT.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'state':'BUILD_STEP_SUCCESS_CLEANUP_RECONCILED','original_controller_PASS':False,'original_controller_exit':1,'original_controller_result':original,'build_step_exit':0,'actual_OCI_export':True,'local_diagnostic_only':True,'source_sha':plan['source_sha'],'source_manifest':desc(Path(plan['source_manifest'])),'build_plan':desc(SRC/'PLAN.json'),'controller_source_manifest':desc(SRC/'SOURCE-MANIFEST.json'),'canonical_recipe_sha256':plan['dockerfile_sha256'],'context_manifest':desc(Path(plan['context_manifest'])),'actual_build_proofs':desc(R/'ACTUAL-BUILD-PROOFS.json'),'cleanup_reconciliation':desc(R/'CLEANUP-RECONCILIATION.json'),'OCI_identity':desc(E/'OCI-IDENTITY.json'),'OCI_tar':identity['oci_tar'],'OCI_layout_index':identity['oci_layout_index'],'platform_manifest':identity['platform_manifest'],'config':identity['config'],'layer_count':identity['layer_count'],'compressed_layer_bytes':identity['layer_bytes'],'actual_labels':identity['labels'],'actual_ENV':identity['ENV'],'actual_CMD':identity['cmd'],'source_label_limit':'The revision OCIlabel is inherited common-producer1feba, not688 generator. External immutable snapshot/context/command proof binds actual688 code. No replacement or source-image revision inference.','bounds':plan['bounds'],'observed_resources':{k:result[k] for k in ('elapsed_seconds','peak_combined_observed_bytes','minimum_physical_free_bytes','maximum_oci_observed_bytes','final_log_bytes')},'limits':['ONE actual arm64 canonical compute-child diagnostic build, not image publication/adoption/botharch/wholefamily/defaultPython/hosted/native-init/predeploy completion.','No image load/import, no runtime CMD execution or dev8 Job, no cluster writes/CI action.','Both actual current utility CI arches still required before Job; current snapshot688 is historical to upcoming init test-only oracle change; exact B recipe/context unchanged as recorded.','Original CI helper refusal and false controller result remain intact. All exact own resources reconciled; no automatic rebuild.','Original13/28 programs/data/commands untouched. Seven Node programs executed only their authored build phase.','No raw layer decompression/extraction; full raw compressedblob SHA/size and OCI descriptors/config checked locally.','Future actual App/Job requires freshly journaled true owned anchor UID and actual child manifest/config/platform. Fake A cache/UID remains source-only.']})
# Preserve original controller MANIFEST; this separate final manifest includes later local identity/proof records.
files={}
for p in sorted(R.rglob('*')):
 if p.is_dir() or p.name=='FINAL-MANIFEST.json':continue
 d=desc(p);d.pop('path');files[str(p.relative_to(R))]=d
put('FINAL-MANIFEST.json',{'files':files,'controller_result_preserved':True,'no_blob_copies':True})
assert desc(E/'RESULT.json')==original
print(json.dumps({'root':str(R),'receipt':desc(R/'RECEIPT.json'),'manifest':desc(R/'FINAL-MANIFEST.json'),'files':len(files),'bytes':sum(x['bytes'] for x in files.values()),'state':'BUILD_STEP_SUCCESS_CLEANUP_RECONCILED','original_controller_PASS':False}))
