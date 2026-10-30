from pathlib import Path
import subprocess,os,json,concurrent.futures
r=Path('/tmp/loopx-w8-native-modern-review/direct-bundle-proof');tool='/private/tmp/bex7-native-acceptance-novb586u/cosign';bundle=r/'76a3c5382bb6f60b8e932bb3951bdde3f6dc17c301e9ff27c0dcc2a2c0c3067f.bundle.json';sha='4330878c280bc9748444c8e1e50ff7388df1228a'
def check(item):
 label,wanted=item;args=[tool,'verify-blob-attestation','--bundle',str(bundle),'--type','https://sigstore.dev/cosign/sign/v1','--digest','4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc','--digestAlg','sha256','--certificate-identity','https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main','--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',wanted,'--use-signed-timestamps','--timeout','5m']
 with (r/(label+'.stdout.log')).open('wb') as stdout,(r/(label+'.stderr.log')).open('wb') as stderr:p=subprocess.run(args,env=dict(os.environ,GODEBUG='http2client=0'),stdout=stdout,stderr=stderr,timeout=330)
 result={'label':label,'expected_source_sha':wanted,'exit_code':p.returncode};(r/(label+'-result.json')).write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True);return result
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:results=list(pool.map(check,[('image-positive',sha),('image-wrong-source-negative','0'*40)]))
assert results[0]['exit_code']==0 and results[1]['exit_code']!=0
