from pathlib import Path
import hashlib,json,os,shutil,subprocess
os.umask(0o077);P=Path(__file__).resolve().parent;d=P/'sweep394-public-reference';assert not d.exists();repo='https://github.com/git-fixtures/basic';r=subprocess.run(['git','-c','credential.helper=','clone','--depth','1','--branch','master',repo,str(d)],text=True,capture_output=True,timeout=60);assert r.returncode==0,r.stderr
commit=subprocess.check_output(['git','-C',str(d),'rev-parse','HEAD'],text=True).strip();files={}
for f in ['LICENSE','README.md']:
 b=(d/f).read_bytes();files[f]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
ref={'repo':repo,'branch':'master','commit':commit,'files':files,'qualification':'public Git carrier only; qa394 Gemfile, stale lock, observer and TCP server are explicitly authored QA fixtures'};(P/'sweep394-public-reference.json').write_text(json.dumps(ref,indent=2));shutil.rmtree(d)
s=(P/'sweep393-common.py').read_text().replace('393','394');(P/'sweep394-common.py').write_text(s)
for name in ['final-inventory','cleanup','wait','progress','shared-state','contract']:
 f=P/('sweep393-'+name+'.py');s=f.read_text().replace('393','394').replace('native-submodule','native-bundler');s=s.replace("runtime=='node'","runtime=='ruby'").replace("{'NODE_VERSION':'24'}","{'RUBY_VERSION':'3.4'}");(P/('sweep394-'+name+'.py')).write_text(s)
f=P/'sweep394-contract.py';s=f.read_text();s=s[:s.index('ref=json.loads')]+"ref=json.loads((P/'sweep394-public-reference.json').read_text());remote=subprocess.check_output(['git','-c','credential.helper=','ls-remote',ref['repo'],'refs/heads/'+ref['branch']],text=True,timeout=30);assert remote.split()[0]==ref['commit'];(P/'sweep394-help-and-source-contract.json').write_text(json.dumps({'source_head':c['head'],'serving_revision':'unknown','native_help':receipts,'anonymous_public_branch':remote,'primary_contract':'https://render.com/docs/environment-variables','scope':'native Ruby stale dependency lock functional QA; source-pinned Ruby3.4/Bundler2.6.9'},indent=2));print(json.dumps({'actual_native_helps':len(receipts),'source_commit':ref['commit']}))\n";f.write_text(s)
print(json.dumps({'source_commit':commit,'source_files':files,'own_reference_checkout_removed':not d.exists(),'helpers_prepared':True}))
