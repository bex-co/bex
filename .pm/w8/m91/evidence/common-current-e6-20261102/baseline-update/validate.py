"""Existing helper positive validation in an exact private Git-source view only."""
from pathlib import Path
import hashlib,json,os,shutil,stat,subprocess,tempfile,types
from unittest.mock import patch
R=Path(__file__).resolve().parent
REPO=Path('/Users/tianpan/projects/bex7')
SOURCE='e6a2386101a78d679f729b7ec0d251373c0aa678'
def digest(p):
 h=hashlib.sha256();size=0
 with Path(p).open('rb') as stream:
  for chunk in iter(lambda:stream.read(1<<20),b''):h.update(chunk);size+=len(chunk)
 return {'sha256':h.hexdigest(),'bytes':size}
def load(p):return json.loads(Path(p).read_bytes())
proposal=load(R/'candidate-proposal.json');candidate=proposal['candidate']
plan=load(R/'COPY-PLAN.json')
expected=load(R.parent/'amd64/extracted/artifact/qualification-source.json')
assert expected['source']==SOURCE and len(expected['files'])==300
view=Path(tempfile.mkdtemp(prefix='clean-validation-',dir=R))
receipt={'scope':'Pure local unchanged-helper validation. No CI invocation, signature rerun, API/auth/Docker/Go/production or repository mutation. GITHUB_SHA is an explicit validator control input, not a fabricated actual-CI process receipt.','source':SOURCE,'source_paths':300,'private_view':str(view),'result':'UNFINISHED'}
try:
 tree={}
 raw=subprocess.check_output(['git','ls-tree','-r','-z',SOURCE],cwd=REPO)
 for row in raw.split(b'\0'):
  if row:
   info,name=row.split(b'\t',1);mode,kind,blob=info.decode().split();tree[name.decode()]=(mode,kind,blob)
 names=sorted(expected['files'])
 objects=subprocess.check_output(['git','cat-file','--batch'],input=''.join(tree[n][2]+'\n' for n in names).encode(),cwd=REPO)
 assert len(objects)<16*1024*1024
 cursor=0
 for name in names:
  end=objects.index(b'\n',cursor);oid,kind,count=objects[cursor:end].decode().split();count=int(count)
  body=objects[end+1:end+1+count];cursor=end+1+count
  assert objects[cursor:cursor+1]==b'\n';cursor+=1
  row=expected['files'][name];mode=tree[name][0]
  assert kind=='blob' and oid==tree[name][2] and hashlib.sha1(b'blob '+str(len(body)).encode()+b'\0'+body).hexdigest()==oid
  assert hashlib.sha256(body).hexdigest()==row['sha256'] and len(body)==row['bytes'] and mode==row['mode'] and row['type']=='regular'
  target=view/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(body);target.chmod(row['permissions'])
 assert cursor==len(objects)
 receipt['tracked_source_bytes']=sum(row['bytes'] for row in expected['files'].values())
 linked=0;copied=0
 def retain(original,name,want):
  global linked,copied
  original=Path(original);assert stat.S_ISREG(original.lstat().st_mode)
  assert digest(original)=={k:want[k] for k in ('sha256','bytes')}
  target=view/name;target.parent.mkdir(parents=True,exist_ok=True)
  if target.exists():assert digest(target)==digest(original);return
  try:os.link(original,target);linked+=1
  except OSError:shutil.copyfile(original,target);copied+=1
 # Exact old image approval/custody remains in place; no crypto is rerun.
 descs=[]
 def descriptors(obj):
  if isinstance(obj,dict):
   if set(obj)=={'path','sha256','bytes'}:descs.append(obj)
   else:
    for v in obj.values():descriptors(v)
  elif isinstance(obj,list):
   for v in obj:descriptors(v)
 for key in ('publication','approval'):descriptors(candidate[key])
 custody=load(REPO/candidate['approval']['custody']['path']);descs.extend(custody['files'].values())
 for row in descs:retain(REPO/row['path'],row['path'],row)
 for row in plan['files']:retain(row['source'],row['path'],row)
 cp=view/'.github/native-common-test-candidate.json';cp.parent.mkdir(parents=True,exist_ok=True);cp.write_bytes((R/'candidate-proposal.json').read_bytes())
 helperpath=view/'.github/scripts/native-common-qualify.py';helper=types.ModuleType('private_current_baseline_validation');helper.__file__=str(helperpath)
 exec(compile(helperpath.read_bytes(),str(helperpath),'exec'),helper.__dict__)
 generator=subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()
 with patch.dict(os.environ,{'GITHUB_SHA':generator}),patch.object(helper.subprocess,'run',side_effect=AssertionError('unexpected validator subprocess')),patch.object(helper.subprocess,'Popen',side_effect=AssertionError('unexpected validator Popen')):
  image=helper.validate_image_candidate(view)
  full=helper.validate_test_candidate(view)
 receipt.update(result='IMAGE_AND_FULL_CURRENT_BASELINE_PASS',local_validator_GITHUB_SHA=generator,source_inventory_equal=helper.source_inventory(view)==expected['files'],image_input_count=len(helper.image_source_inventory(view)),retained_private_evidence_links=linked,retained_private_evidence_copies=copied,validation={'ref':full['ref'],'producer':full['producer'],'verification':full['verification'],'generator_source':full['generator_source'],'baseline_status':full['baseline_status'],'utility_source':full['source']['sha'],'architectures':list(full['utilities'])},helper_sha256=digest(helperpath)['sha256'],candidate_sha256=digest(cp)['sha256'])
 print(json.dumps({'result':receipt['result'],'source':SOURCE,'candidate_sha256':receipt['candidate_sha256']},indent=2))
finally:
 shutil.rmtree(view)
 receipt['owned_validation_view_removed']=not view.exists()
 (R/'VALIDATION.json').write_text(json.dumps(receipt,indent=2)+'\n')
