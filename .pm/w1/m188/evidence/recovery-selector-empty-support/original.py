from pathlib import Path
import tempfile,subprocess,os,json,datetime
lines=Path('.github/workflows/native-base-image.yml').read_text().splitlines()
step=next(i for i,x in enumerate(lines) if "name: select a reviewed recovery artifact" in x)
start=next(i for i in range(step,len(lines)) if lines[i].strip()=="python3 - <<'PY'")+1
end=next(i for i in range(start,len(lines)) if lines[i].strip()=='PY')
script='\n'.join(x[10:] for x in lines[start:end])+'\n';compile(script,'workflow-recovery','exec')
auth=json.loads(Path('.github/native-base-recovery.json').read_text()); results=[]
with tempfile.TemporaryDirectory(prefix='bex-m188-recovery-test-') as d:
 root=Path(d);(root/'deploy/native-base').mkdir(parents=True);(root/'.github').mkdir()
 def git(*args):return subprocess.check_output(['git','-C',d,*args],text=True,stderr=subprocess.DEVNULL).strip()
 git('init','--quiet');git('config','user.name','Recovery test');git('config','user.email','recovery-test@example.invalid')
 payload=root/'deploy/native-base/payload';payload.write_text('reviewed artifact recipe\n')
 git('add','deploy/native-base/payload');git('commit','--quiet','-m','fixture: reviewed source');producer=git('rev-parse','HEAD')
 for entry in auth['images'].values():entry['producer']=producer
 path=root/'.github/native-base-recovery.json';path.write_text(json.dumps(auth))
 primary='node-20-bookworm' if 'node-20-bookworm' in auth['images'] else next(iter(auth['images']))
 def run(name,matrix=None,base=None,repo='ghcr.io/bex-co/bex-cnb-run',expected='recover'):
  matrix=matrix or primary
  out=root/'outputs';out.write_text('')
  env=dict(os.environ,MATRIX_NAME=matrix,BASE_IMAGE=base or auth['images'].get(matrix,auth['images'][primary])['base'],NATIVE_IMAGE=repo,GITHUB_OUTPUT=str(out))
  p=subprocess.run(['python3','-c',script],cwd=d,env=env,capture_output=True,text=True)
  data=out.read_text();kind='reject' if p.returncode else ('recover' if 'digest=sha256:' in data else 'build')
  assert kind==expected,(name,kind,p.stdout,p.stderr,data)
  if kind=='recover':assert 'digest='+auth['images'][matrix]['ref'].split('@')[1] in data and 'producer='+producer in data
  results.append({'case':name,'expected':expected,'observed':kind,'exit':p.returncode})
 for retired,base in [('node-14-bullseye','node:14-bullseye'),('node-16-bookworm','node:16-bookworm'),('python-3.7-bullseye','python:3.7-bullseye'),('python-3.8-bookworm','python:3.8-bookworm'),('python-3.9-bookworm','python:3.9-bookworm'),('python-3.10-bookworm','python:3.10-bookworm'),('node-18-bookworm','node:18-bookworm'),('node-20-bookworm','node:20-bookworm')]:
  if retired not in auth['images']:run('verified '+retired+' authorization retired',matrix=retired,base=base+'@sha256:'+'c'*64,expected='build')
 for matrix in auth['images']:run('authorized '+matrix+' same recipe',matrix=matrix)
 run('unlisted runtime',matrix='ruby-3.0-bullseye',base='ruby:3.0-bullseye@sha256:'+'b'*64,expected='build')
 run('changed upstream base',base='node:20-bookworm@sha256:'+'a'*64,expected='build')
 run('different package rejected',repo='ghcr.io/other/package',expected='reject')
 (root/'deploy/native-base/README.md').write_text('documentation only\n');git('add','deploy/native-base/README.md');git('commit','--quiet','-m','fixture: documentation')
 run('documentation-only delta remains eligible')
 git('mv','deploy/native-base/payload','deploy/native-base/payload.md');git('commit','--quiet','-m','fixture: rename recipe as documentation')
 run('nonMD to MD rename invalidates recovery',expected='build')
 path.unlink();run('removed authorization rebuilds',expected='build')
Path('/tmp/bex-m188-recovery-selection-tests.json').write_text(json.dumps({'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'method':'Actual workflow embedded selection script against isolated temporary Git commits; no Docker, signing, network, or production mutation','results':results},indent=2)+'\n')
print(json.dumps(results,indent=2))
