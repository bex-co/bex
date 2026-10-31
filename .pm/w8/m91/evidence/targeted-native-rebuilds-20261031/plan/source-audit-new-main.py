import datetime, hashlib, json, pathlib, subprocess
ROOT=pathlib.Path('/Users/tianpan/projects/bex7')
OUT=pathlib.Path('/private/tmp/bex-w8-native-targeted-rebuild-plan-20261031-0928')
CURRENT='0b138600b31ff4294651369109974a538a33310b'
FIXED='2cc8723df139bc4684d8d2a4d1eebc0c9dca004c'
OLD='83c1e30e35eeeadab499de4d8d96c1f6c0349077'
FIXES=['8af93065a6a83299e8db7977aaf6527a5bad3e5d','94165237dbe63f997b0119ce70ad243da74468fa',FIXED]
TARGETS=['python-3.11-bookworm','python-3.13-bookworm','node-26-bookworm']
def sha(b):return hashlib.sha256(b).hexdigest()
def dump(name,a):(OUT/name).write_text(json.dumps(a,indent=2,sort_keys=True)+'\n')
def git(*a):return subprocess.check_output(['git',*a],cwd=ROOT,timeout=40)
def selected(p):
 return p.startswith('deploy/native-base/') and not p.endswith('.md') or p in ['.github/workflows/native-base-image.yml','.github/native-base-recovery.json','lego/operator/internal/build/native.go','.github/scripts/native-oci-stage.py'] or p.startswith('.github/scripts/test_native_oci_') and p.endswith('.py')
def local_tree(s):
 result={}
 for item in git('ls-tree','-r','-z',s,'--','deploy/native-base','.github/workflows/native-base-image.yml','.github/native-base-recovery.json','lego/operator/internal/build/native.go','.github/scripts').split(b'\0'):
  if not item:continue
  metadata,p=item.decode().split('\t',1);mode,kind,blob=metadata.split()
  if kind=='blob' and selected(p):result[p]={'mode':mode,'git_blob_id':blob}
 return result
remote=json.loads((OUT/'new-main-source-tree.json').read_text());assert not remote['truncated']
current={x['path']:{'mode':x['mode'],'git_blob_id':x['sha']} for x in remote['tree'] if x['type']=='blob' and selected(x['path'])}
fixed=local_tree(FIXED);old=local_tree(OLD)
assert current==fixed, 'Current native input closure changed since fixed source; stop plan'
compare=json.loads((OUT/'compare-2cc-new-main.json').read_text())
assert compare['status'] in ('ahead','identical') and compare['behind_by']==0 and compare['merge_base_commit']['sha']==FIXED
for s in FIXES:subprocess.run(['git','merge-base','--is-ancestor',s,FIXED],cwd=ROOT,check=True,timeout=20)
inputs={};manifest={};assert (OUT/'current-inputs').is_dir()
for p,meta in sorted(current.items()):
 raw=git('show',FIXED+':'+p)
 assert hashlib.sha1(b'blob '+str(len(raw)).encode()+b'\0'+raw).hexdigest()==meta['git_blob_id']
 target=OUT/'current-inputs'/p;target.parent.mkdir(parents=True,exist_ok=True);assert target.read_bytes()==raw
 inputs[p]=raw;manifest[p]=dict(meta,sha256=sha(raw),size=len(raw),bytes_obtained_from_equal_local_source=FIXED)
ns={'__name__':'__reviewed_matrix_only__'}
exec(compile(inputs['deploy/native-base/matrix.py'],'reviewed-matrix.py','exec'),ns)
rows=ns['matrix'](inputs['lego/operator/internal/build/native.go'].decode())['include']
assert len(rows)==27
bases={t:next(r['base'] for r in rows if r['name']==t) for t in TARGETS}
expected={
'python-3.11-bookworm':'python:3.11-bookworm@sha256:84fa927127b46670e768371d1ad1be271d57ba93a6fbb3479045c7c6c452432f',
'python-3.13-bookworm':'python:3.13-bookworm@sha256:3856e3e7bf4d0ba061501a84ed2eb0d429663307c0f54f191dc4de03c7081c3e',
'node-26-bookworm':'node:26-bookworm@sha256:3b764117159bda01d2396391e7789b6a9106af9729cfd001ef55e98557868310'}
assert bases==expected
recovery=json.loads(inputs['.github/native-base-recovery.json']);assert all(t not in recovery['images'] for t in TARGETS)
workflow=inputs['.github/workflows/native-base-image.yml'].decode();install=inputs['deploy/native-base/install.sh'].decode()
assert 'swig zip unzip' in install and 'database_timeout=30m' in workflow and '--download-java-db-only' in workflow
assert 'timeout --kill-after=10s 21m trivy "$@"' in workflow
changed=[{'path':p,'old':old.get(p),'current':current.get(p)} for p in sorted(set(old)|set(current)) if old.get(p)!=current.get(p)]
recipechanged=[x for x in changed if x['path'].startswith('deploy/native-base/')]
assert any(x['path']=='deploy/native-base/install.sh' for x in recipechanged)
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'current_published_main':CURRENT,'native_fixed_source':FIXED,'old_failed_source':OLD,'required_fix_commits':FIXES,'fixes_ancestral_to_current':True,'current_and_fixed_complete_selected_input_closure_equal':True,'input_scope':'Every non-Markdown deploy/native-base file; native workflow; native OCI staging helper and all test_native_oci_*.py files; native.go reviewed matrix input; recovery JSON. Exact path/mode/Git blob IDs compared from untruncated remote current tree to local fixed-source tree, independently content/SHA verified. Compare API file list (300-entry cap) was not used to infer equality.','input_file_count':len(current),'current_recipe_nondoc_file_count':sum(p.startswith('deploy/native-base/') for p in current),'old_recipe_nondoc_file_count':sum(p.startswith('deploy/native-base/') for p in old),'current_native_inputs':manifest,'old_to_current_changes':changed,'old_index_recovery_eligible':False,'recovery_reason':'Explicit unzip and dependency-fetch changes alter deploy/native-base recipe; normal source rebuild is required even where an old index exists. No target has a recovery authorization.','target_bases':bases,'matrix_total_reviewed_names':27,'factory_shape':{'build_platforms':['linux/amd64','linux/arm64'],'build_runner_labels':['self-hosted','Linux','ARM64','bex-production'],'matrix_runner_labels':['self-hosted','Linux','ARM64','bex-ci'],'max_parallel_per_matrix_run':2,'per_runtime_concurrency_group':'native-base-${{ matrix.name }}','cancel_in_progress':False,'targeted_singleton_selector':'inputs.runtime exact matrix name','java_db_prefetch_budget':'30m plus10s kill grace','vulnerability_db_prefetch_budget':'21m plus10s kill grace','each_complete_scan_budget':'20m Trivy /21m wrapper plus10s kill grace','scan_attempts':3,'gates':'Both architecture runtime probes, both complete Trivy reports, both full Syft SPDX, strict signed attestations and discovery publication remain required. No old artifact is approved by this plan.'}}
dump('source-input-compatibility-current.json',result)
for t in TARGETS:dump('dispatch-'+t+'.json',{'ref':'main','inputs':{'runtime':t}})
print('Exact source compatibility PASS',len(current),'closure files;',result['current_recipe_nondoc_file_count'],'current recipe files; targets',bases)
print('Old-to-current changed paths',[x['path'] for x in changed])
