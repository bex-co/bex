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
  def refused(args,needle,data=None):
   r=subprocess.run(args,cwd=d,input=data,capture_output=True,timeout=90);assert r.returncode!=0 and needle in r.stderr,(args,r.returncode,r.stderr[-500:])
  # CLI-only CSV import, a -bail stdin seed that must stop at its first failure, and a refused readonly write.
  (d/'rows.csv').write_text('id,label,n\n1,"naïve, ""雪""",2.5\n2,plain,-7\n',encoding='utf-8')
  run(['sqlite3','-bail','import.db'],cwd=d,data='CREATE TABLE imported(id INTEGER,label TEXT,n REAL);\n.import --csv --skip 1 rows.csv imported\n'.encode())
  query='SELECT id,label,n FROM imported ORDER BY id';external=json.loads(run(['sqlite3','-readonly','-json','import.db',query],cwd=d))
  with sqlite3.connect('file:'+str(d/'import.db')+'?mode=ro',uri=True) as database:internal=database.execute(query).fetchall()
  assert external==[{'id':1,'label':'naïve, "雪"','n':2.5},{'id':2,'label':'plain','n':-7}] and internal==[(1,'naïve, "雪"',2.5),(2,'plain',-7.0)],(external,internal)
  refused(['sqlite3','-bail','seed.db'],b'UNIQUE constraint failed: seeded.id',b"CREATE TABLE seeded(id INTEGER PRIMARY KEY,label TEXT);\nINSERT INTO seeded VALUES(1,'kept');\nINSERT INTO seeded VALUES(1,'duplicate');\nINSERT INTO seeded VALUES(2,'after');\n")
  with sqlite3.connect('file:'+str(d/'seed.db')+'?mode=ro',uri=True) as database:assert database.execute('SELECT id,label FROM seeded').fetchall()==[(1,'kept')]
  stored=sha((d/'library.db').read_bytes());refused(['sqlite3','-readonly','library.db',"INSERT INTO library_rows VALUES(4,'x',1)"],b'readonly database');assert sha((d/'library.db').read_bytes())==stored
  results.append('SQLite readonly CLI JSON and independent Python library exact same-file rows; CLI CSV import; -bail seed stops at a named constraint error; readonly write refused')
  # Pandoc's Markdown reader, packaged template data and Lua filter runtime, independently parsed.
  (d/'doc.md').write_text('# Café 雪\n\n| key | value |\n|-----|-------|\n| naïve | 7 < 9 |\n\nSee [docs](https://example.invalid/a?b=1&c=2).\n',encoding='utf-8');(d/'upper.lua').write_text('function Str(el) return pandoc.Str(pandoc.text.upper(el.text)) end\n');(d/'fail.lua').write_text('function Str(el) error("bex filter refused") end\n')
  run(['pandoc','--from=markdown','--to=html5','--standalone','--metadata=title:QA 雪','--lua-filter=upper.lua','--output=doc.html','doc.md'],cwd=d)
  from html.parser import HTMLParser
  class Doc(HTMLParser):
   def __init__(self):super().__init__();self.stack=[];self.text={};self.attrs={}
   def handle_starttag(self,tag,attrs):self.stack.append(tag);self.attrs.setdefault(tag,[]).append(dict(attrs))
   def handle_endtag(self,tag):self.stack.pop()
   def handle_data(self,s):
    if self.stack and s.strip():self.text.setdefault(self.stack[-1],[]).append(s.strip())
  doc=Doc();doc.feed((d/'doc.html').read_text(encoding='utf-8'));doc.close()
  assert doc.text['title']==['QA 雪'] and doc.text['h1']==['QA 雪','CAFÉ 雪'] and doc.text['th']==['KEY','VALUE'] and doc.text['td']==['NAÏVE','7 < 9'] and doc.text['a']==['DOCS'],doc.text
  assert {'charset':'utf-8'} in doc.attrs['meta'] and doc.attrs['a']==[{'href':'https://example.invalid/a?b=1&c=2'}],doc.attrs
  run(['pandoc','--from=bex-unsupported-format','doc.md'],cwd=d,fails=True);run(['pandoc','--lua-filter=fail.lua','doc.md'],cwd=d,fails=True);results.append('Pandoc UTF-8 Markdown table/link to standalone HTML5 with packaged template + Lua filter, independent parse; unknown-format and failing-filter errors')
  # Actual libvips development closure, C headers, linker and output values.
  (d/'vips.c').write_text('#include <vips/vips.h>\nint main(void){VipsImage *a=0,*b=0;size_t n=0;if(vips_init("qa")||vips_black(&a,2,1,"bands",3,NULL)||vips_invert(a,&b,NULL))return 1;unsigned char *p=vips_image_write_to_memory(b,&n);int ok=n==6;for(size_t i=0;i<n;i++)ok=ok&&p[i]==255;g_free(p);g_object_unref(a);g_object_unref(b);vips_shutdown();return ok?0:2;}\n')
  flags=shlex.split(run(['pkg-config','--cflags','--libs','vips']).decode());run(['gcc','vips.c','-o','vips-probe',*flags],cwd=d);run([str(d/'vips-probe')]);(d/'invalid.ppm').write_bytes(b'invalid image');run(['vipsheader','invalid.ppm'],cwd=d,fails=True);results.append('libvips headers/pkg-config/C-link/pixels and invalid-image')
  # The vips tools on m95's original image: its PyVips resize, a JPEG thumbnail, lossless WebP and a BMP only the magick module loads.
  decode=runpy.run_path(str(ROOT/'png-decoder.py'))['decode_png'];vref=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==95)['validation_reference'];ppm=base64.b64decode(vref['input']['base64']);(d/'qa.ppm').write_bytes(ppm)
  run(['vips','resize','qa.ppm','quarter.png','0.25','--kernel','nearest'],cwd=d);assert decode((d/'quarter.png').read_bytes())['pixels_hex']==vref['expected_rgb_hex']
  run(['vipsthumbnail','qa.ppm','--size','4x','-o','thumb.jpg[Q=95]'],cwd=d);assert (d/'thumb.jpg').read_bytes()[:3]==b'\xff\xd8\xff' and [run(['vipsheader','-f',f,'thumb.jpg'],cwd=d).strip() for f in ('vips-loader','width','height')]==[b'jpegload',b'4',b'2']
  run(['vips','copy','thumb.jpg','thumb.png'],cwd=d);thumb=bytes.fromhex(decode((d/'thumb.png').read_bytes())['pixels_hex']);assert all(thumb[i]>200 and thumb[i+2]<60 for i in (0,3,12,15)) and all(thumb[i]<60 and thumb[i+2]>200 for i in (6,9,18,21)),thumb.hex()
  run(['vips','copy','qa.ppm','qa.webp[lossless]'],cwd=d);assert (d/'qa.webp').read_bytes()[8:12]==b'WEBP';run(['vips','copy','qa.webp','webp.png'],cwd=d);assert bytes.fromhex(decode((d/'webp.png').read_bytes())['pixels_hex'])==ppm[-96:]
  bgr=lambda h:b''.join(bytes.fromhex(h[i:i+6])[::-1] for i in range(0,len(h),6));rows=bgr('0000ffffffff')+b'\0\0'+bgr('ff000000ff00')+b'\0\0';(d/'qa.bmp').write_bytes(b'BM'+struct.pack('<IHHIIiiHHIIiiII',54+len(rows),0,0,54,40,2,2,1,24,0,len(rows),2835,2835,0,0)+rows)
  assert run(['vipsheader','-f','vips-loader','qa.bmp'],cwd=d).strip()==b'magickload';run(['vips','copy','qa.bmp','bmp.png'],cwd=d);assert decode((d/'bmp.png').read_bytes())['pixels_hex']=='ff000000ff000000ffffffff';results.append('vips tools: m95 image resize equals PyVips pixels, JPEG thumbnail geometry/colors, lossless WebP exact round trip, magick-module BMP pixels')
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
  png=decode((d/'font-color.png').read_bytes());assert (png['width'],png['height'])==(64,48);pixels=bytes.fromhex(png['pixels_hex']);pixel=lambda x,y:pixels[(y*64+x)*3:(y*64+x+1)*3];assert pixel(8,36)==b'\xff\0\0' and pixel(48,36)==b'\0\xff\0' and pixel(60,5)==b'\xff\xff\xff';assert sum(pixel(x,y)==b'\0\0\0' for y in range(1,23) for x in range(2,25))>10
  (d/'bad.pdf').write_bytes(b'not a PDF');run(['gs','-q','-dSAFER','-dBATCH','-dNOPAUSE','-sDEVICE=png16m','-sOutputFile=bad.png','bad.pdf'],cwd=d,fails=True);results.append('original owned PDF + Helvetica/color PDF exact independent RGB and render failure')
  # Ghostscript beyond the original render: m97's own PDF through its original argv, ps2pdf embedding the packaged URW font, text extraction, page selection, an authored GS_LIB, and refused programs.
  gref=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==97)['validation_reference']['input'];(d/'qa-gs.pdf').write_bytes(base64.b64decode(gref['base64']))
  gs=['gs','-q','-dBATCH','-dNOPAUSE','-sDEVICE=png16m','-r72'];page=lambda name:decode((d/name).read_bytes())
  run(gs+['-dFirstPage=1','-dLastPage=1','-sOutputFile=qa-gs.png','qa-gs.pdf'],cwd=d);original=page('qa-gs.png');assert (original['width'],original['pixels_hex'])==(2,gref['expected_RGB_hex'])
  (d/'two.ps').write_text('%!PS\n<< /PageSize [64 48] >> setpagedevice\n1 0 0 setrgbcolor 0 0 32 24 rectfill 0 0 1 setrgbcolor 32 0 32 24 rectfill 0 setgray /Helvetica findfont 14 scalefont setfont 4 32 moveto (Hi) show showpage\n0 1 0 setrgbcolor 0 0 64 48 rectfill showpage\n')
  run(['ps2pdf','-dPDFSETTINGS=/prepress','two.ps','two.pdf'],cwd=d);two=(d/'two.pdf').read_bytes();assert two[:8]==b'%PDF-1.4' and b'/Count 2' in two and b'+Helvetica' in two and b'/FontFile3 ' in two
  assert run(['gs','-q','-dBATCH','-dNOPAUSE','-sDEVICE=txtwrite','-dFirstPage=1','-dLastPage=1','-sOutputFile=-','two.pdf'],cwd=d).strip()==b'Hi'
  run(gs+['-sOutputFile=all%d.png','two.pdf'],cwd=d);run(gs+['-dFirstPage=2','-dLastPage=2','-sOutputFile=second%d.png','two.pdf'],cwd=d)
  assert sorted(p.name for p in d.glob('second*.png'))==['second1.png'] and page('second1.png')['pixels_hex']==page('all2.png')['pixels_hex']=='00ff00'*64*48
  def gs_error(args,needle):
   # Ghostscript prints the PostScript error on stdout and only its summary on stderr.
   r=subprocess.run(args,cwd=d,capture_output=True,timeout=90);assert r.returncode==1 and needle in r.stdout and b'Unrecoverable error' in r.stderr,(args,r.returncode,r.stdout[-500:],r.stderr[-300:])
  lib=d/'gslib';lib.mkdir();(lib/'qa-lib.ps').write_text('/qa-answer 6 7 mul def\n');probe=['gs','-q','-dNODISPLAY','-dBATCH','-dNOPAUSE','-c']
  assert run(probe+['(qa-lib.ps) runlibfile qa-answer =='],cwd=d,env={**os.environ,'GS_LIB':str(lib)}).strip()==b'42';gs_error(probe+['(qa-lib.ps) runlibfile qa-answer =='],b'/undefinedfilename in (qa-lib.ps)')
  gs_error(probe+['(/etc/passwd) (r) file'],b'/invalidfileaccess');gs_error(probe+['qa_undefined_op'],b'/undefined in qa_undefined_op')
  results.append('Ghostscript: m97 original PDF via its own argv, ps2pdf embedding the packaged URW Helvetica, txtwrite text, page selection, authored GS_LIB lookup, SAFER file refusal and a failing program exit 1')
  # Manifest/copy bytes, Unicode and space names, exclude and stale removal.
  source=d/'assets';source.mkdir();expected={'two words.bin':bytes(range(256))*8,'naïve.txt':'Crème 雪'.encode()};
  for n,b in expected.items():(source/n).write_bytes(b)
  (source/'skip.tmp').write_bytes(b'excluded');dest=d/'copied';dest.mkdir();(dest/'stale.txt').write_bytes(b'stale');run(['rsync','-a','--delete','--exclude=*.tmp',str(source)+'/',str(dest)+'/']);assert {f.name:f.read_bytes() for f in dest.iterdir()}==expected;run(['jq','-e','.'],data=b'{ invalid JSON',fails=True);run(['rsync','-a',str(d/'missing')+'/',str(d/'failed-copy')+'/'],fails=True);results.append('jq malformed input + rsync exact Unicode/space bytes/exclude/stale/missing-source failure')
  # jq and rsync beyond that copy: m98's own manifest and asset tree through the original argv, Oniguruma regex, safe --arg/--argjson, raw/@sh/@base64 output, slurp and exit statuses; archive attributes, symlinks, excluded files kept from --delete, itemized dry runs and trailing-slash semantics.
  aref=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==98)['validation_reference'];app=d/'asset-app';published=[a['path'] for a in aref['manifest']['assets'] if a['publish']]
  for n,s in aref['assets'].items():f=app/'qa-assets-src'/n;f.parent.mkdir(parents=True,exist_ok=True);f.write_text(s,encoding='utf-8')
  (app/'qa-assets-input.json').write_text(json.dumps(aref['manifest'],ensure_ascii=False),encoding='utf-8');(app/'dist').mkdir();(app/'dist'/'old-owned.txt').write_text('own stale output')
  jq=lambda *a,data=None:run(['jq',*a],cwd=app,data=data)
  def exit_of(args,data=None):r=subprocess.run(args,cwd=app,input=data,capture_output=True,timeout=90);return r.returncode,r.stderr
  assert json.loads(jq('-e','-c','{release:.release,paths:[.assets[]|select(.publish==true)|.path]}','qa-assets-input.json'))=={'release':aref['manifest']['release'],'paths':published}
  run(['rsync','-a','--delete','--exclude=*.tmp','qa-assets-src/','dist/'],cwd=app);assert {p.relative_to(app/'dist').as_posix():p.read_bytes() for p in (app/'dist').rglob('*') if p.is_file()}=={n:aref['assets'][n].encode() for n in published}
  assert jq('-r','.assets[]|select(.publish)|.path','qa-assets-input.json').decode().splitlines()==published and shlex.split(jq('-r','[.assets[]|select(.publish)|.path]|@sh','qa-assets-input.json').decode())==published
  assert json.loads(jq('-c',r'[.assets[].path|select(test("^assets/\\p{L}+\\.txt$"))|sub("\\.txt$";".md")]','qa-assets-input.json'))==['assets/écho.md']
  tricky='a "quoted" $(name) 雪';assert json.loads(jq('-n','-c','--arg','name',tricky,'--argjson','n','7','{name:$name,n:($n*3)}'))=={'name':tricky,'n':21}
  assert jq('-n','-r','"Crème 雪"|@base64').strip()==base64.b64encode('Crème 雪'.encode()) and jq('-n','-r','"Q3LDqG1lIOmbqg=="|@base64d').decode().strip()=='Crème 雪'
  assert json.loads(jq('-s','-c','map(.n)|add',data=b'{"n":1}\n{"n":2.5}\n{"n":-7}\n'))==-3.5
  assert [exit_of(['jq','-e',f,'qa-assets-input.json'])[0] for f in ('.release','.missing','.assets[3].publish')]==[0,1,1] and exit_of(['jq','.['],b'{}')[0]==3 and exit_of(['jq','error("bex refused")'],b'{}\n')==(5,b'jq: error (at <stdin>:1): bex refused\n')
  tree=d/'tree';(tree/'empty dir').mkdir(parents=True);(tree/'bin').mkdir();tool=tree/'bin'/'run.sh';tool.write_text('#!/bin/sh\necho ok\n');tool.chmod(0o750);(tree/'data.txt').write_text('Zoë 李\n',encoding='utf-8');os.symlink('data.txt',tree/'link to data')
  stamp=1700000000;os.utime(tool,(stamp,stamp));os.utime(tree/'data.txt',(stamp+1,stamp+1));out=d/'out';(out/'stale dir').mkdir(parents=True);(out/'stale dir'/'old.txt').write_text('stale');(out/'keep.tmp').write_text('excluded, so kept')
  rsync=lambda *a:run(['rsync',*a],cwd=d);sync=('-a','--delete','--exclude=*.tmp');rsync(*sync,'tree/','out/');copy=out/'bin'/'run.sh'
  assert (copy.stat().st_mode&0o7777,int(copy.stat().st_mtime),copy.read_bytes(),int((out/'data.txt').stat().st_mtime))==(0o750,stamp,tool.read_bytes(),stamp+1)
  assert os.readlink(out/'link to data')=='data.txt' and (out/'empty dir').is_dir() and not (out/'stale dir').exists() and (out/'keep.tmp').read_text()=='excluded, so kept'
  assert rsync(*sync,'--dry-run','--itemize-changes','tree/','out/')==b''
  (tree/'data.txt').write_text('changed\n');assert rsync(*sync,'--dry-run','--itemize-changes','tree/','out/').decode().split()==['>f.st......','data.txt'] and (out/'data.txt').read_text(encoding='utf-8')=='Zoë 李\n'
  rsync(*sync,'--delete-excluded','tree/','out/');assert not (out/'keep.tmp').exists() and (out/'data.txt').read_text()=='changed\n'
  rsync('-a','tree','nested/');assert (d/'nested'/'tree'/'bin'/'run.sh').read_bytes()==tool.read_bytes() and not (d/'nested'/'bin').exists()
  results.append('jq/rsync: m98 original manifest filter and asset copy, Oniguruma Unicode regex, --arg/--argjson, raw/@sh/@base64 output, slurp, -e/compile/runtime exit statuses; rsync -a modes/mtimes/symlinks/empty dirs, excluded files kept from --delete until --delete-excluded, itemized dry run, trailing-slash semantics')
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
