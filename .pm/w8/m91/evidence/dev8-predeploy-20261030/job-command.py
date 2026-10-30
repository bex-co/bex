import base64,contextlib,hashlib,io,json,os,runpy,subprocess,sys
from pathlib import Path
r=Path('/opt/render/project/src');os.chdir(r)
literal='literal $(QA_SECRET_MARKER) $HOME; "雪"\ntrailing\n'
assert os.environ['QA_LITERAL']==literal
assert os.environ['QA_EMPTY']==''
assert os.environ['QA_SECRET_MARKER']=='own-synthetic-only'
assert os.environ['QA_OVERRIDE']=='literal-wins'
assert os.environ['PORT']=='10000'
assert not Path('/var/run/secrets/kubernetes.io/serviceaccount/token').exists()
payload=Path('/etc/secrets/proof.txt').read_bytes()
assert payload==b'own-dev8-file\x00\xff\n'
try:Path('/etc/secrets/proof.txt').write_bytes(b'bad')
except OSError:pass
else:raise AssertionError('secret-file mount writable')
assert not Path('/etc/secrets/.dockerconfigjson').exists()
assert all('dockerconfigjson' not in k.lower() for k in os.environ)
print('BEX_DEV8_ENV_FILE '+json.dumps({'pass':True,'literal_sha256':hashlib.sha256(literal.encode()).hexdigest(),'secret_file_sha256':hashlib.sha256(payload).hexdigest(),'empty_preserved':True,'envFrom':True,'env_precedence':True,'port_precedence':True,'read_only_secret_file':True,'serviceaccount_token_absent':True,'pull_credential_not_projected':True}),flush=True)
# Authored driver is unchanged. Its historic Docker-network field is explicitly
# removed from the current Job receipt; it is not evidence for this execution.
cap=io.StringIO()
with contextlib.redirect_stdout(cap):
 sys.argv=['extended-checks.py','predeploy']
 runpy.run_path(str(r/'extended-checks.py'),run_name='__main__')
summary=json.loads(cap.getvalue());summary.pop('network',None);summary.pop('scope',None)
summary['actual_execution_scope']='Kubernetes dev-8 Job; canonical userns/hardening; own deny-egress NetworkPolicy; loopback consumers; companion Python3.11; no Docker-network-none claim'
print('BEX_DEV8_UTILITIES '+json.dumps(summary,ensure_ascii=False),flush=True)
proofs={}
for f in sorted((r/'qa-output').glob('runtime-*-predeploy.json')):
 proofs[f.name]=json.loads(f.read_text())
assert len(proofs)==13 and all(len(x)==3 and all(q['pass'] for q in x) for x in proofs.values())
print('BEX_DEV8_HTTP_PROOFS '+json.dumps(proofs,ensure_ascii=False),flush=True)
identities={}
for f in sorted((r/'fixtures').rglob('*')):
 if f.is_file() and '__pycache__' not in f.parts:
  b=f.read_bytes();identities[str(f.relative_to(r))]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
for n in ['extended-checks.py','local-specs.json','pg18.dump.b64','pg18-rows.txt','png-decoder.py']:
 b=(r/n).read_bytes();identities[n]={'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
print('BEX_DEV8_IDENTITIES '+json.dumps(identities,sort_keys=True),flush=True)
print('BEX_DEV8_PASS',flush=True)
