"""Read/hash the one local OCI tar, retain small original JSON; never load/import/extract layers."""
from pathlib import Path,PurePosixPath
import hashlib,json,os,re,stat,tarfile,time
R=Path(__file__).resolve().parent;E=R/'evidence';A=E/'child.oci';OUT=E/'oci-metadata'
START=time.monotonic();CAP=4*1024**3
assert A.is_file() and not A.is_symlink() and 0<A.stat().st_size<=CAP
assert not OUT.exists();OUT.mkdir(mode=0o700)
def sha(b):return hashlib.sha256(b).hexdigest()
items={};metadata={};count=0;metabytes=0
with tarfile.open(A,'r|*') as archive:
 for m in archive:
  count+=1;assert count<=4096 and time.monotonic()-START<180
  p=PurePosixPath(m.name);assert not p.is_absolute() and '..' not in p.parts and '\\' not in m.name
  if m.isdir():continue
  assert m.isfile() and not m.issym() and not m.islnk() and m.name not in items
  assert m.name in ('index.json','oci-layout') or re.fullmatch(r'blobs/sha256/[a-f0-9]{64}',m.name),m.name
  assert 0<=m.size<=CAP
  stream=archive.extractfile(m);assert stream is not None
  h=hashlib.sha256();n=0;small=[]
  while True:
   b=stream.read(1024**2)
   if not b:break
   n+=len(b);h.update(b)
   if m.size<=16*1024**2:small.append(b)
  assert n==m.size
  digest=h.hexdigest();items[m.name]={'bytes':n,'sha256':digest}
  if m.name.startswith('blobs/'):assert m.name.split('/')[-1]==digest
  if small:
   data=b''.join(small)
   try:x=json.loads(data)
   except (ValueError,UnicodeDecodeError):continue
   if not isinstance(x,dict):continue
   metabytes+=n;assert metabytes<=32*1024**2
   metadata[m.name]=(data,x)
assert 'index.json' in metadata and 'oci-layout' in metadata and metadata['oci-layout'][1]['imageLayoutVersion']=='1.0.0'
index=metadata['index.json'][1];images=[];seen=set();graph=[]
def descriptor(d):
 assert re.fullmatch(r'sha256:[a-f0-9]{64}',d['digest'])
 key='blobs/sha256/'+d['digest'][7:];assert key in items
 assert d['size']==items[key]['bytes'] and d['digest'][7:]==items[key]['sha256']
 return key

def walk(d,depth=0):
 assert depth<5
 k=descriptor(d)
 if k in seen:return
 seen.add(k);assert k in metadata
 raw,x=metadata[k];media=x.get('mediaType','')
 if 'manifests' in x:
  assert x.get('schemaVersion')==2
  for y in x['manifests']:walk(y,depth+1)
 elif 'config' in x and 'layers' in x:
  cfg=descriptor(x['config']);assert cfg in metadata
  for y in x['layers']:descriptor(y)
  conf=metadata[cfg][1]
  if (conf.get('os'),conf.get('architecture'))==('linux','arm64'):
   images.append((d,k,x,cfg,conf))
 graph.append({'descriptor':d,'metadata_path':k})
for d in index['manifests']:walk(d)
assert len(images)==1
selected,mkey,manifest,ckey,config=images[0]
env=config.get('config',{}).get('Env',[]);labels=config.get('config',{}).get('Labels',{})
assert 'NODE_VERSION=24.22.1' in env and labels.get('co.bex.native-qualification')=='bex-pd-7cf7c0d919d6'
assert len(config['rootfs']['diff_ids'])==len(manifest['layers'])
for name,(raw,x) in metadata.items():
 # Only original JSON bytes copied; actual layer bodies remain solely in original OCI tar.
 target=OUT/('index.json' if name=='index.json' else 'oci-layout' if name=='oci-layout' else name.split('/')[-1]+'.json')
 target.write_bytes(raw)
h=hashlib.sha256()
with A.open('rb') as f:
 for b in iter(lambda:f.read(1024**2),b''):h.update(b)
result={'PASS':True,'scope':'One actual locally-built canonical arm64 compute child; archive/descriptor/config identity only, no load/import/Pod/runtime/phase/qualification/adoption claim','oci_tar':{'path':str(A),'bytes':A.stat().st_size,'sha256':h.hexdigest()},'oci_layout_index':{'sha256':sha(metadata['index.json'][0]),'bytes':len(metadata['index.json'][0])},'platform_manifest':{'digest':'sha256:'+items[mkey]['sha256'],'bytes':items[mkey]['bytes'],'platform':{'os':'linux','architecture':'arm64'}},'config':{'digest':'sha256:'+items[ckey]['sha256'],'bytes':items[ckey]['bytes']},'layers':manifest['layers'],'layer_count':len(manifest['layers']),'layer_bytes':sum(x['size'] for x in manifest['layers']),'all_blob_filenames_full_compressed_bytes_sha256_and_sizes_verified':True,'tar_members':count,'regular_file_members':len(items),'blob_custody':items,'raw_json_graph':graph,'ENV':env,'labels':labels,'working_dir':config.get('config',{}).get('WorkingDir'),'entrypoint':config.get('config',{}).get('Entrypoint'),'cmd':config.get('config',{}).get('Cmd'),'source_binding':'Actual sealed6883953 snapshot/exported canonical recipe/context + owned build command/process/artifact manifests. No Git-revision OCIlabel is invented; only actual labels above.','manual_layer_decompression_or_extraction':False,'raw_metadata_bytes':metabytes,'seconds':round(time.monotonic()-START,3)}
(E/'OCI-IDENTITY.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({k:result[k] for k in ('PASS','oci_tar','oci_layout_index','platform_manifest','config','layer_count','layer_bytes','seconds')}))
