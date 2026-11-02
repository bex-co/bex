"""Root-reviewed fixed publication view. No repository writes or validator execution."""
from pathlib import Path
import hashlib, json, os, shutil, stat, subprocess, time

R=Path(__file__).resolve().parent
VIEW=R/'clean-git301-view'
REPO=Path('/Users/tianpan/projects/bex7')
FLOOR=8<<30
MARGIN=32<<20

def need(ok,why):
    if not ok:raise AssertionError(why)
def free():
    value=shutil.disk_usage(R).free
    need(value>FLOOR+MARGIN,'physical8GiB+32MiB')
    return value
def destination(name):
    p=Path(name)
    need(not p.is_absolute() and '..' not in p.parts and '.' not in p.parts,'relative-only view path')
    target=VIEW/p;target.parent.mkdir(parents=True,exist_ok=True)
    return target
def filehash(p):
    h=hashlib.sha256()
    with p.open('rb') as f:
        for chunk in iter(lambda:f.read(1<<20),b''):h.update(chunk)
    return h.hexdigest()

started=time.monotonic();minimum=free();need(not VIEW.exists(),'fresh owned view required')
sources=json.loads((R/'CLEAN-GIT301-SOURCE-FILES.json').read_bytes())['files']
existing=json.loads((R/'UNCHANGED0941-VALIDATION-CLOSURE.json').read_bytes())['files']
publication=json.loads((R/'PUBLICATION-FILES.json').read_bytes())['files']
need(len(sources)==301,'exact source301');need(len({x['destination'] for x in sources+existing+publication})==len(sources+existing+publication),'closed view destination bijection')
source_bytes=sum(x['bytes'] for x in sources);need(shutil.disk_usage(R).free>FLOOR+MARGIN+source_bytes,'materialized301 fit')
VIEW.mkdir(mode=0o700)
record={'complete':False,'view':str(VIEW),'source_files':301,'existing_evidence_links':0,'publication_links':0,'materialized_source_bytes':0,'allocated_strategy':'hardlink exact frozen evidence; new301Git source only. Never chmod/write a linked source; no full469MB reservation or bodycopy.'}
try:
    argv=['git','cat-file','--batch']
    result=subprocess.run(argv,cwd=REPO,input=''.join(x['source_Git_blob']+'\n' for x in sources).encode(),capture_output=True,timeout=30,check=True)
    need(len(result.stdout)<source_bytes+65536 and len(result.stderr)<1<<20,'bounded local Git output')
    raw=result.stdout;cursor=0
    for row in sources:
        minimum=min(minimum,free());end=raw.index(b'\n',cursor);oid,kind,size=raw[cursor:end].decode().split();size=int(size);body=raw[end+1:end+1+size];cursor=end+1+size
        need(raw[cursor:cursor+1]==b'\n' and kind=='blob' and oid==row['source_Git_blob'],'Git object framing');cursor+=1
        need(len(body)==row['bytes'] and hashlib.sha256(body).hexdigest()==row['sha256'],'Git source bytes')
        p=destination(row['destination'])
        with p.open('xb') as f:need(f.write(body)==len(body),'source write')
        p.chmod(int(row['mode'][-3:],8));record['materialized_source_bytes']+=len(body)
    need(cursor==len(raw),'complete Git framing')
    for label,rows in [('existing_evidence_links',existing),('publication_links',publication)]:
        for row in rows:
            minimum=min(minimum,free());need(time.monotonic()-started<300,'stage300s')
            src=Path(row['source']);before=src.lstat();need(stat.S_ISREG(before.st_mode) and not src.is_symlink() and before.st_size==row['bytes'],'exact regular source size')
            need(before.st_dev==VIEW.stat().st_dev,'same-filesystem hardlink required; no silent bulk-copy fallback')
            dst=destination(row['destination']);os.link(src,dst,follow_symlinks=False)
            after=dst.lstat();need(after.st_ino==before.st_ino and after.st_dev==before.st_dev and stat.S_IMODE(after.st_mode)==stat.S_IMODE(before.st_mode),'exact readonly alias')
            # No chmod/write after link. Unchanged validator will freshly hash every evidence byte.
            record[label]+=1
    record['complete']=True
finally:
    record.update(minimum_free=minimum,final_free=shutil.disk_usage(R).free,seconds=time.monotonic()-started)
    (R/'STAGE-RESULT.json').write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps(record))
