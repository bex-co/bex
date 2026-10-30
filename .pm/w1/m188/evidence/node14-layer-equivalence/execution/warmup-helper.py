# Reuse the identical bounded proof function; only parallelize independent pending immutable blobs.
from pathlib import Path
source=Path('/tmp/loopx-w8-native-prove-docker-sbom-roots.py').read_text();prefix=source.split('\nurl,headers=v.public_registry',1)[0];exec(compile(prefix,'same-reviewed-layer-proof-functions','exec'))
work=[]
for arch in ['amd64','arm64']:
 manifest=json.loads((root/(arch+'-manifest.json')).read_text());config=json.loads((root/(arch+'-config.json')).read_text())
 for i,(desc,diff) in enumerate(zip(manifest['layers'],config['rootfs']['diff_ids'])):
  if i>0 and not (cache/(desc['digest'].split(':')[1]+'.json')).exists():work.append((desc,diff,arch,i))
work.sort(key=lambda x:x[0]['size'],reverse=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:list(pool.map(lambda x:layer(*x),work))
emit('parallel-pending-layer-proof-complete',tasks=len(work))
