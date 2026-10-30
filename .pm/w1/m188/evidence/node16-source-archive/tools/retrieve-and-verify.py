#!/usr/bin/env python3
"""Retrieve complete public predicates to scratch; verify immutable bytes and strict keyless bundles."""
import argparse,base64,hashlib,json,os,re,subprocess,tempfile,urllib.parse,urllib.request
from pathlib import Path
HERE=Path(__file__).resolve().parent.parent
DIGEST=re.compile(r'sha256:[a-f0-9]{64}')
MAX_BUNDLE=256<<20

def require(ok,message):
 if not ok:raise RuntimeError(message)

def get(url,headers,limit):
 with urllib.request.urlopen(urllib.request.Request(url,headers=headers),timeout=60) as response:raw=response.read(limit+1)
 require(len(raw)<=limit,'bounded metadata read exceeded');return raw

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--cosign',required=True);p.add_argument('--expected-cosign-sha256',default='9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4');p.add_argument('--output',help='fresh scratch directory outside the repository');args=p.parse_args()
 tool=Path(args.cosign).resolve();require(hashlib.sha256(tool.read_bytes()).hexdigest()==args.expected_cosign_sha256,'Cosign binary hash differs from the independently verified tool')
 output=Path(args.output).resolve() if args.output else Path(tempfile.mkdtemp(prefix='bex-native-full-predicates-'))
 for parent in HERE.parents:
  if (parent/'.git').exists():require(output!=parent and parent not in output.parents,'Full predicates must be written outside this repository');break
 if args.output:output.mkdir(parents=True,exist_ok=False)
 data=json.loads((HERE/'full-predicate-retrieval.json').read_text());policy=data['policy'];repository=data['repository'];query=urllib.parse.urlencode({'service':'ghcr.io','scope':'repository:'+repository+':pull'});token=json.loads(get('https://ghcr.io/token?'+query,{},128<<10))['token'];headers={'Authorization':'Bearer '+token,'Accept':'application/vnd.oci.image.manifest.v1+json,application/vnd.oci.image.index.v1+json'};base='https://ghcr.io/v2/'+repository;results=[]
 for row in data['entries']:
  md,bd=row['manifest_digest'],row['bundle_digest'];require(DIGEST.fullmatch(md) and DIGEST.fullmatch(bd),'Invalid immutable digest')
  manifest_raw=get(base+'/manifests/'+md,headers,8<<20);require('sha256:'+hashlib.sha256(manifest_raw).hexdigest()==md,'Manifest bytes differ from immutable digest');manifest=json.loads(manifest_raw);require(manifest.get('subject',{}).get('digest')=='sha256:'+policy['subject_digest'],'Manifest names a different supplier index');require(len(manifest['layers'])==1 and manifest['layers'][0]['digest']==bd and manifest['layers'][0]['size']==row['bundle_bytes'],'Manifest bundle descriptor differs')
  expected=row['bundle_bytes'];require(type(expected) is int and 0<=expected<=MAX_BUNDLE,'Bundle exceeds the read bound');path=output/(bd.split(':')[1]+'.bundle.json');hashed=hashlib.sha256();total=0
  with urllib.request.urlopen(urllib.request.Request(base+'/blobs/'+bd,headers=headers),timeout=60) as response,path.open('wb') as file:
   while True:
    chunk=response.read(1<<20)
    if not chunk:break
    total+=len(chunk);require(total<=expected,'Bundle exceeds immutable descriptor size');hashed.update(chunk);file.write(chunk)
  require(total==expected and 'sha256:'+hashed.hexdigest()==bd,'Bundle bytes differ from immutable descriptor')
  command=[str(tool),'verify-blob-attestation','--bundle',str(path),'--type',row['predicate_type'],'--digest',policy['subject_digest'],'--digestAlg','sha256','--certificate-identity',policy['certificate_identity'],'--certificate-oidc-issuer',policy['certificate_oidc_issuer'],'--certificate-github-workflow-sha',policy['verification_sha'],'--use-signed-timestamps','--timeout','5m']
  with path.with_suffix('.verify.stdout.log').open('wb') as stdout,path.with_suffix('.verify.stderr.log').open('wb') as stderr:run=subprocess.run(command,env=dict(os.environ,GODEBUG='http2client=0'),stdout=stdout,stderr=stderr,timeout=330)
  require(run.returncode==0,'Strict keyless verification failed; inspect its scratch stderr')
  bundle=json.loads(path.read_text());payload=base64.b64decode(bundle['dsseEnvelope']['payload'],validate=True);require(hashlib.sha256(payload).hexdigest()==row['signed_statement_sha256'],'Signed statement hash differs');statement=json.loads(payload);require(statement['predicateType']==row['predicate_type'],'Signed predicate type differs');canonical=json.dumps(statement['predicate'],sort_keys=True,separators=(',',':'),ensure_ascii=False,allow_nan=False).encode();require(hashlib.sha256(canonical).hexdigest()==row['decoded_predicate_sorted_json_sha256'],'Complete decoded predicate differs');target=output/(row['kind']+'-'+str(row['architecture'])+'-'+bd.split(':')[1][:12]+'.predicate.json');target.write_bytes(canonical)
  results.append({'bundle_digest':bd,'signed_statement_sha256':row['signed_statement_sha256'],'complete_predicate_sha256':row['decoded_predicate_sorted_json_sha256'],'predicate_file':str(target),'strict_crypto_exit':run.returncode});print('Verified',row['kind'],row['architecture'],target)
 (output/'retrieval-verification-receipt.json').write_text(json.dumps({'scope':'Full predicate retrieval and strict bundle verification only; this does not repeat image/runtime/hosted acceptance.','supplier_ref':policy['ref'],'tool_sha256':args.expected_cosign_sha256,'results':results},indent=2)+'\n')
 print('Complete evidence:',output)
if __name__=='__main__':main()
