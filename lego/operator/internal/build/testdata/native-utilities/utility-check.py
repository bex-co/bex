"""Offline generated-image behavior regressions; no hosting or publication claim."""
from pathlib import Path
import base64,gettext,gzip,io,json,os,runpy,shlex,shutil,struct,subprocess,sys,tempfile,zipfile,zlib,hashlib
ROOT=Path(__file__).resolve().parent
PHASE=sys.argv[1]
def sha(b):return hashlib.sha256(b).hexdigest()

def run(args,cwd=None,data=None,fails=False,env=None):
 r=subprocess.run(args,cwd=cwd,input=data,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=env or os.environ,timeout=90)
 if (r.returncode!=0)!=fails:raise RuntimeError((args,r.returncode,r.stdout.decode(errors='replace')[-1500:],r.stderr.decode(errors='replace')[-1500:]))
 return r.stdout

def extra_cases():
 results=[]
 with tempfile.TemporaryDirectory(prefix='native-extra-') as directory:
  d=Path(directory)
  # Genuine PostgreSQL18 custom archive boundary beside the original PG15 app.
  archive=base64.b64decode((ROOT/'pg18.dump.b64').read_text());assert sha(archive)=='a640255a7f226c765d84ad869a6a986ea4d39be7a72a68b02599dcf87721637c';(d/'pg18.dump').write_bytes(archive)
  run(['pg_restore','--data-only','--no-owner','--no-privileges','--file=pg18.sql','pg18.dump'],cwd=d);sql=(d/'pg18.sql').read_text();section=sql.split('COPY public.native_archive ',1)[1].split('\n',1)[1].split('\\.\n',1)[0];assert section.encode()==(ROOT/'pg18-rows.txt').read_bytes();assert '(PostgreSQL) 18.' in run(['psql','--version']).decode() and '(PostgreSQL) 18.' in run(['pg_dump','--version']).decode();run(['pg_restore','--file=invalid.sql','/dev/null'],cwd=d,fails=True);results.append('genuine PG18 archive exact three rows, PG18client programs and malformed archive')
  import sqlite3
  sql="CREATE TABLE library_rows(id INTEGER,label TEXT,n REAL); INSERT INTO library_rows VALUES(1,'naïve 雪',2.5),(2,'quote''s,comma',-7),(3,'empty',0);"
  run(['sqlite3','library.db',sql],cwd=d)
  query='SELECT id,label,n FROM library_rows ORDER BY id'
  external=json.loads(run(['sqlite3','-readonly','-json','library.db',query],cwd=d))
  with sqlite3.connect('file:'+str(d/'library.db')+'?mode=ro',uri=True) as database:
   database.row_factory=sqlite3.Row;internal=[dict(row) for row in database.execute(query)]
  assert external==internal==[{'id':1,'label':'naïve 雪','n':2.5},{'id':2,'label':"quote's,comma",'n':-7},{'id':3,'label':'empty','n':0}]
  results.append('SQLite readonly CLI JSON and independent Python library exact same-file rows')
  # Actual libvips development closure, C headers, linker and output values.
  (d/'vips.c').write_text('#include <vips/vips.h>\nint main(void){VipsImage *a=0,*b=0;size_t n=0;if(vips_init("qa")||vips_black(&a,2,1,"bands",3,NULL)||vips_invert(a,&b,NULL))return 1;unsigned char *p=vips_image_write_to_memory(b,&n);int ok=n==6;for(size_t i=0;i<n;i++)ok=ok&&p[i]==255;g_free(p);g_object_unref(a);g_object_unref(b);vips_shutdown();return ok?0:2;}\n')
  flags=shlex.split(run(['pkg-config','--cflags','--libs','vips']).decode());run(['gcc','vips.c','-o','vips-probe',*flags],cwd=d);run([str(d/'vips-probe')]);(d/'invalid.ppm').write_bytes(b'invalid image');run(['vipsheader','invalid.ppm'],cwd=d,fails=True);results.append('libvips headers/pkg-config/C-link/pixels and invalid-image')
  # All GNU gettext utility consumers, retained translations and format errors.
  ref=json.loads((ROOT/'gettext-reference.json').read_text());source_po=(ROOT/'messages.po').read_text();(d/'messages.po').write_text(source_po)
  (d/'strings.py').write_text("_('Dessert')\n_('Hello, %(name)s!')\nngettext('There is %(count)d apple.', 'There are %(count)d apples.', count)\n")
  run(['xgettext','--language=Python','--from-code=UTF-8','--keyword=_','--keyword=ngettext:1,2','--output=messages.pot','strings.py'],cwd=d)
  run(['msgmerge','--update','--backup=none','messages.po','messages.pot'],cwd=d);run(['msguniq','--use-first','--output-file=unique.po','messages.po'],cwd=d);run(['msgfmt','--check-format','--output-file=messages.mo','unique.po'],cwd=d)
  g=gettext.GNUTranslations(io.BytesIO((d/'messages.mo').read_bytes()));assert g.gettext('Dessert')==ref['expected']['dessert'];assert {str(i):g.ngettext('There is %(count)d apple.','There are %(count)d apples.',i)%{'count':i} for i in [0,1,2,5]}==ref['expected']['plurals']
  (d/'bad.po').write_text('msgid ""\nmsgstr "Content-Type: text/plain; charset=UTF-8\\n"\n\n#, c-format\nmsgid "Hello %s"\nmsgstr "Bonjour %d"\n');run(['msgfmt','--check-format','-o','bad.mo','bad.po'],cwd=d,fails=True);results.append('Django original plurals + GNU extraction/merge/unique/catalog-format failure')
  # A PDF using Base14 Helvetica plus exact color blocks, independently decoded.
  content=b'q 1 0 0 rg 0 0 32 24 re f Q q 0 1 0 rg 32 0 32 24 re f Q BT /F1 14 Tf 0 0 0 rg 4 32 Td (Hi) Tj ET\n'
  objects=[b'<< /Type /Catalog /Pages 2 0 R >>',b'<< /Type /Pages /Kids [3 0 R] /Count 1 >>',b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 64 48] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>',b'<< /Length '+str(len(content)).encode()+b' >>\nstream\n'+content+b'endstream',b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>'];pdf=b'%PDF-1.4\n';offsets=[0]
  for i,o in enumerate(objects,1):offsets.append(len(pdf));pdf+=str(i).encode()+b' 0 obj\n'+o+b'\nendobj\n'
  pos=len(pdf);pdf+=b'xref\n0 6\n0000000000 65535 f \n'+b''.join(('%010d 00000 n \n'%o).encode() for o in offsets[1:])+b'trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n'+str(pos).encode()+b'\n%%EOF\n';(d/'font-color.pdf').write_bytes(pdf)
  run(['gs','-q','-dSAFER','-dBATCH','-dNOPAUSE','-sDEVICE=png16m','-r72','-sOutputFile=font-color.png','font-color.pdf'],cwd=d)
  decode=runpy.run_path(str(ROOT/'png-decoder.py'))['decode_png'];png=decode((d/'font-color.png').read_bytes());assert (png['width'],png['height'])==(64,48);pixels=bytes.fromhex(png['pixels_hex']);pixel=lambda x,y:pixels[(y*64+x)*3:(y*64+x+1)*3];assert pixel(8,36)==b'\xff\0\0' and pixel(48,36)==b'\0\xff\0' and pixel(60,5)==b'\xff\xff\xff';assert sum(pixel(x,y)==b'\0\0\0' for y in range(1,23) for x in range(2,25))>10
  (d/'bad.pdf').write_bytes(b'not a PDF');run(['gs','-q','-dSAFER','-dBATCH','-dNOPAUSE','-sDEVICE=png16m','-sOutputFile=bad.png','bad.pdf'],cwd=d,fails=True);results.append('original owned PDF + Helvetica/color PDF exact independent RGB and render failure')
  # Manifest/copy bytes, Unicode and space names, exclude and stale removal.
  source=d/'assets';source.mkdir();expected={'two words.bin':bytes(range(256))*8,'naïve.txt':'Crème 雪'.encode()};
  for n,b in expected.items():(source/n).write_bytes(b)
  (source/'skip.tmp').write_bytes(b'excluded');dest=d/'copied';dest.mkdir();(dest/'stale.txt').write_bytes(b'stale');run(['rsync','-a','--delete','--exclude=*.tmp',str(source)+'/',str(dest)+'/']);assert {f.name:f.read_bytes() for f in dest.iterdir()}==expected;run(['jq','-e','.'],data=b'{ invalid JSON',fails=True);run(['rsync','-a',str(d/'missing')+'/',str(d/'failed-copy')+'/'],fails=True);results.append('jq malformed input + rsync exact Unicode/space bytes/exclude/stale/missing-source failure')
  # Compile actual strict interface+enum at runtime; noEmitOnError failure.
  (d/'typed.cts').write_text("enum Kind { Nice='雪' }; interface Row { count:number; kind:Kind }; const r:Row={count:7,kind:Kind.Nice};console.log(JSON.stringify({value:r.count*3,label:r.kind}));\n")
  run(['tsc','--strict','--noEmitOnError','--module','NodeNext','--target','ES2022','--outDir','typed-out','typed.cts'],cwd=d);assert json.loads(run(['node','typed-out/typed.cjs'],cwd=d))=={'value':21,'label':'雪'}
  (d/'bad.cts').write_text("const count:number='bad';\n");run(['tsc','--strict','--noEmitOnError','--module','NodeNext','--outDir','bad-out','bad.cts'],cwd=d,fails=True);assert not list((d/'bad-out').glob('*.cjs'));results.append('TypeScript actual interface/enum compilation + noEmitOnError/no-output failure')
  # Real noninteractive bundling and unresolved module failure.
  (d/'domain.mjs').write_text("export function value(){return {n:21,label:'雪'};}\n");(d/'entry.mjs').write_text("import {value} from './domain.mjs';console.log(JSON.stringify(value()));\n");stats=run(['webpack','./entry.mjs','--mode','production','--target','node','--output-path',str(d/'bundle'),'--output-filename','app.cjs','--json'],cwd=d);assert json.loads(stats)['errors']==[];assert json.loads(run(['node','bundle/app.cjs'],cwd=d))=={'n':21,'label':'雪'}
  (d/'broken.mjs').write_text("import './does-not-exist.mjs';\n");bad=run(['webpack','./broken.mjs','--mode','production','--target','node','--output-path',str(d/'bad-bundle'),'--json'],cwd=d,fails=True);assert json.loads(bad)['errors'];results.append('Webpack real two-module runtime production bundle + unresolved-module nonzero statistics')
  # SWIG language/typemap data and actual runtime wrapper compilation/import.
  (d/'runtime_bind.i').write_text('%module runtime_bind\n%inline %{\nint weighted(int a,int b){return 7*a+3*b;}\n%}\n');run(['swig','-python','runtime_bind.i'],cwd=d);includes=shlex.split(run(['python3-config','--includes']).decode());suffix=run(['python3-config','--extension-suffix']).decode().strip();run(['gcc','-shared','-fPIC',*includes,'runtime_bind_wrap.c','-o','_runtime_bind'+suffix],cwd=d);run(['python3','-c','import runtime_bind; assert [runtime_bind.weighted(17,-4),runtime_bind.weighted(-9,7)]==[107,-42]'],cwd=d)
  (d/'bad.i').write_text('%module bad\n%inline %{\nint broken( ;\n%}\n');run(['swig','-python','bad.i'],cwd=d,fails=True);results.append('SWIG runtime typemap/wrapper/C import + invalid interface failure')
  # Independent ZIP names/bytes/CRC/method checks; no unzip-as-oracle shortcut.
  zipdir=d/'zip';zipdir.mkdir();members={'empty.txt':b'','greeting.txt':('Crème brûlée & 李\n'*100).encode(),'two words.bin':bytes(range(256))*64}
  for n,b in members.items():(zipdir/n).write_bytes(b)
  for label,level in [('stored','-0'),('deflated','-9')]:
   run(['zip','-q',level,label+'.zip',*sorted(members)],cwd=zipdir);raw=(zipdir/(label+'.zip')).read_bytes()
   with zipfile.ZipFile(io.BytesIO(raw)) as z:
    assert z.namelist()==sorted(members) and z.testzip() is None
    for v in z.infolist():assert z.read(v.filename)==members[v.filename] and v.file_size==len(members[v.filename]) and v.CRC==zlib.crc32(members[v.filename])&0xffffffff
    assert all(v.compress_type==0 for v in z.infolist()) if label=='stored' else any(v.compress_type==8 for v in z.infolist())
   if PHASE=='build':shutil.copyfile(zipdir/(label+'.zip'),ROOT/'public'/(label+'.zip'))
  run(['zip','-q','bad.zip','absent.txt'],cwd=zipdir,fails=True);results.append('ZIP independent exact empty/binary/Unicode/space members+CRC32/stored+deflated methods and missing input failure')
  # Build-only pigz, independent runtime/static artifact checks require no pigz.
  compressed_dir=ROOT/'public';payloads={'empty':b'','binary':bytes(range(256))*64,'unicode':('Crème 雪\n'*100).encode()}
  for name,data in payloads.items():
   f=compressed_dir/(name+'.gz')
   if PHASE=='build':f.write_bytes(run(['pigz','-n','-p','2','-c'],data=data))
   raw=f.read_bytes();assert raw[:3]==b'\x1f\x8b\x08' and raw[3]==0 and raw[4:8]==b'\0'*4;assert struct.unpack('<II',raw[-8:])==(zlib.crc32(data)&0xffffffff,len(data));assert gzip.decompress(raw)==data
  if PHASE=='build':run(['pigz','-d','-c'],data=b'invalid gzip',fails=True)
  results.append('pigz build-only empty/binary/Unicode/error + exact gzip header/CRC32/ISIZE/independent inflation and retained artifacts')
 return results
(ROOT/'public').mkdir(exist_ok=True)
checks=extra_cases()
print(json.dumps({'phase':PHASE,'checks':checks,'pass':True},ensure_ascii=False))
