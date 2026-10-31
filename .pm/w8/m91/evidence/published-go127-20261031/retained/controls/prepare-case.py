from pathlib import Path
import datetime,difflib,hashlib,importlib.util,json,os,shutil,subprocess
ROOT=Path('/Users/tianpan/projects/bex7');S='8af93065a6a83299e8db7977aaf6527a5bad3e5d';OLD='83c1e30e35eeeadab499de4d8d96c1f6c0349077';NAME='golang-1.27-bookworm'
OUT=Path('/tmp/loopx-w8-native-review-go127-v11-8af9306-20261031');CACHE=Path('/tmp/loopx-w8-native-cache-go127-v11-8af9306-20261031');CTL=Path(__file__).parent
assert not OUT.exists() and not CACHE.exists()
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def h(b):return hashlib.sha256(b).hexdigest()
def dump(n,a):(CTL/n).write_text(json.dumps(a,indent=2,sort_keys=True)+'\n')
dump('stopped-source-preflight.json',{'at':now(),'scope':'Setup only; V11 was not invoked and no candidate/image body was read. Initial fresh three roots were asserted absent, then only this controls directory was created.','actual_error':'required reviewed source file missing: .github/scripts/native-oci-stage.py','cause':'Source-audit enumeration assumed later OCI-staging helper files existed in both earlier commits. Both commits actually omit those paths.','resolution':'Compare optional file absence symmetrically; require all actual present source bytes unchanged. No candidate, tool, policy or source edits; this is not a real verification retry.'})
expected={'/tmp/loopx-w8-native-review-v11.py':'ea03d2a9c7a21fde50c0d55f6ead84910c2c431f8c7b52c490624fc5ff8bff85','/tmp/loopx-w8-verify-native-artifact-ranged.py':'ca4a765d547b92f567f7e08149183a8cb094249d544ae594655c92b93d09ae15','/tmp/loopx-w8-verify-native-artifact.py':'cf5932b2c5598bc9140a4bf9b99ab8c6ae0aa9613e73ec88140f492b086f1bb2'}
for p,want in expected.items():assert h(Path(p).read_bytes())==want
spec=importlib.util.spec_from_file_location('v','/tmp/loopx-w8-verify-native-artifact-ranged.py');v=importlib.util.module_from_spec(spec);spec.loader.exec_module(v)
base=v.matrix_base(S,NAME);assert base==v.matrix_base(OLD,NAME)
before=v.recipe_tree(OLD);after=v.recipe_tree(S);assert set(before)==set(after) and len(after)==22
assert [p for p in sorted(before) if before[p]!=after[p]]==['deploy/native-base/install.sh']
old=v.source_bytes(OLD,'deploy/native-base/install.sh');new=v.source_bytes(S,'deploy/native-base/install.sh')
assert old.replace(b'swig zip \\\n',b'swig zip unzip \\\n',1)==new
paths=['.github/workflows/native-base-image.yml','.github/native-base-recovery.json','.github/scripts/native-oci-stage.py','.github/scripts/test_native_oci_stage.py','.github/scripts/test_native_oci_workflow.py','lego/operator/internal/build/native.go']
nonrecipe={}
for p in paths:
 a=v.source_bytes(OLD,p,optional=True);b=v.source_bytes(S,p,optional=True);assert a==b
 nonrecipe[p]={'present':b is not None,'sha256':h(b) if b is not None else None,'bytes':len(b) if b is not None else 0}
assert nonrecipe['.github/workflows/native-base-image.yml']['present'] and nonrecipe['lego/operator/internal/build/native.go']['present']
manifest={}
for p,metadata in after.items():
 b=v.source_bytes(S,p);manifest[p]=dict(metadata,sha256=h(b),bytes=len(b))
(CTL/'explicit-unzip-only.diff').write_text(''.join(difflib.unified_diff(old.decode().splitlines(True),new.decode().splitlines(True),fromfile=OLD+'/deploy/native-base/install.sh',tofile=S+'/deploy/native-base/install.sh')))
recovery=json.loads(v.source_bytes(S,'.github/native-base-recovery.json'));assert NAME not in recovery['images']
p=subprocess.run(['gh','run','view','41142870861','--repo','bex-co/bex','--json','databaseId,headSha,status,conclusion,event,url,createdAt,updatedAt,jobs'],env=os.environ|{'GODEBUG':'http2client=0'},capture_output=True,timeout=80)
(CTL/'fresh-complete-ci.json').write_bytes(p.stdout);(CTL/'fresh-complete-ci.stderr').write_bytes(p.stderr);assert p.returncode==0
ci=json.loads(p.stdout);assert ci['databaseId']==41142870861 and ci['headSha']==S and ci['status']=='completed' and ci['conclusion']=='success' and ci['event']=='workflow_dispatch'
j=next(x for x in ci['jobs'] if x['databaseId']==124361678277);assert j['status']=='completed' and j['conclusion']=='success' and j['name'].startswith('build ('+NAME+', ')
required=['Run docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a','verify real native tool transformations without network on both architectures','scan amd64 candidate','scan arm64 candidate','sign and attest the immutable candidate']
steps={x['name']:x for x in j['steps']};assert all(steps[x]['conclusion']=='success' for x in required)
dump('fresh-source-recipe-context.json',{'at':now(),'source':S,'reviewed_baseline':OLD,'mode':'Fresh normal build; never failed99f recovery','official_base':base,'native_nondoc_recipe_file_count':len(after),'recipe_file_manifest':manifest,'only_recipe_delta':'One explicit unzip package token, source bytes checked against exact replacement; no other recipe path/mode changed.','unchanged_other_native_inputs':nonrecipe,'fresh_CI':{'run':41142870861,'job':124361678277,'complete_success':True,'required_successful_steps':required},'old99f_failed_history':'Retained and excluded; no artifact bytes or receipts reused from it.'})
deps=[];priorroot=Path('/tmp/loopx-w8-native-controls-node22-v11-83c1e30-20261031')
prior=json.loads((priorroot/'observed-local-dependency-custody.json').read_text())
for row in prior['dependencies']:
 p=Path(row['path']);b=p.read_bytes();assert h(b)==row['sha256'];deps.append({'path':str(p),'sha256':h(b),'bytes':len(b),'mtime_ns':p.stat().st_mtime_ns})
dump('observed-local-dependency-custody.json',{'at':now(),'scope':'Local dependency bytes match root-approved Node22 proof; canonical policy independently checks official Cosign hash. No modifications.','dependencies':deps})
ns=['reviewed-v11-strict-guards.py','reviewed-v11-strict-guards-result.json','reviewed-v11-strict-guards.log']
for n in ns:shutil.copyfile(priorroot/n,CTL/n)
dump('prior-reviewed-guard-suite-custody.json',{'at':now(),'scope':'Unchanged approved V11/ranged policy uses prior reviewed43-guard result. Copies were not re-executed or relabeled as new tests. Actual source/subject controls execute freshly in this case.','source':str(priorroot),'files':{n:h((CTL/n).read_bytes()) for n in ns}})
free=shutil.disk_usage(CTL).free;assert free>4*(1<<30)
dump('preflight.json',{'at':now(),'scope':'ROOT assigned ONE complete Go1.27 supplier validation, first case attempt. Setup assumption failure is separately retained; no earlier V11 execution. Actual proof/root artifact approval remain pending.','expected_source_hashes':expected,'initially_absent_paths':[str(x) for x in [OUT,CACHE,CTL]],'run':'41142870861','job_id':124361678277,'name':NAME,'sha':S,'official_base':base,'free_bytes':free,'new_full_image_blob_workers':2,'reuse_policy':'NONE: initially empty cache, no receipt seeding or prior bundle reuse. Every selected image blob must be freshly anonymously fully read and hashed.','TUF':'Unique fresh per-Cosign TUF_ROOTs, default production trust/current refresh and strict flags unchanged.','source_eligibility':'83c reviewed native closure plus explicit unzip only; full source/delta custody retained separately.'})
s=Path('/tmp/loopx-w8-native-controls-python314-v11-83c1e30-20261031/run-once.py').read_text().replace('python314-v11-83c1e30-20261031','go127-v11-8af9306-20261031').replace('python-3.14-bookworm',NAME).replace(OLD,S).replace('41129176570','41142870861')
(CTL/'run-once.py').write_text(s)
print('Preflight/source/CI PASS; official base',base,'free',free,'source',S,'supervisor',h(s.encode()))
