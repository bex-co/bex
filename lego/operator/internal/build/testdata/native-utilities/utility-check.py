"""Offline generated-image behavior regressions; no hosting or publication claim."""
from pathlib import Path
import base64,email.parser,gettext,gzip,http.client,importlib.metadata,io,json,lzma,os,platform,runpy,shlex,shutil,socket,stat,struct,subprocess,sys,sysconfig,tarfile,tempfile,time,zipfile,zlib,hashlib
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
  # GNU gettext on m96's original PO: statistics and an exact msgunfmt round trip, packaged ITS data, msginit's plural table, envsubst, C-library lookup by LANGUAGE, and named syntax/python-format errors.
  po=(ROOT/'messages.po').read_bytes();assert {'bytes':len(po),'sha256':sha(po)}==ref['po'];mo=d/'locale/fr/LC_MESSAGES/django.mo';mo.parent.mkdir(parents=True);(d/'django.po').write_bytes(po)
  r=subprocess.run(['msgfmt','--check','--statistics','-o',str(mo),'django.po'],cwd=d,capture_output=True,timeout=90);assert r.returncode==0 and r.stderr.endswith(b'\n3 translated messages.\n'),r.stderr
  run(['msgunfmt','--output-file=back.po',str(mo)],cwd=d);run(['msgcmp','django.po','back.po'],cwd=d);assert run(['msgfmt','--output-file=-','back.po'],cwd=d)==mo.read_bytes()
  (d/'qa.metainfo.xml').write_text('<?xml version="1.0" encoding="UTF-8"?>\n<component type="desktop-application">\n <id>co.bex.Qa</id>\n <name>Crème QA</name>\n <summary>Translate the dessert menu</summary>\n</component>\n',encoding='utf-8');run(['xgettext','--from-code=UTF-8','--output=meta.pot','qa.metainfo.xml'],cwd=d);meta=(d/'meta.pot').read_text(encoding='utf-8');assert 'msgid "Crème QA"' in meta and 'msgid "Translate the dessert menu"' in meta and 'co.bex.Qa' not in meta,meta
  run(['msginit','--no-translator','--locale=fr','--input=messages.pot','--output-file=fr-init.po'],cwd=d);assert '"Plural-Forms: nplurals=2; plural=(n > 1);\\n"' in (d/'fr-init.po').read_text()
  assert run(['envsubst','$NAME $COUNT'],data=b'Bonjour, $NAME ! $COUNT pommes, $KEEP\n',env={**os.environ,'NAME':'Zoë & 李','COUNT':'5','KEEP':'x'})=='Bonjour, Zoë & 李 ! 5 pommes, $KEEP\n'.encode()
  lookup={**{k:v for k,v in os.environ.items() if not k.startswith('LC_') and k not in ('LANG','LANGUAGE')},'TEXTDOMAINDIR':str(d/'locale'),'LANGUAGE':'fr'};utf8={**lookup,'LC_ALL':'C.UTF-8'}
  assert run(['gettext','-d','django','Dessert'],env=utf8).decode()==ref['expected']['dessert'] and run(['ngettext','-d','django','There is %(count)d apple.','There are %(count)d apples.','5'],env=utf8)==b'Il y a %(count)d pommes.' and run(['gettext','-d','django','Dessert'],env={**lookup,'LC_ALL':'C'})==b'Dessert'
  (d/'broken.po').write_text('msgid "Dessert"\nmsgstr "Crème\n',encoding='utf-8');refused(['msgfmt','--output-file=broken.mo','broken.po'],b'broken.po:3: end-of-line within string')
  (d/'named.po').write_text('msgid ""\nmsgstr "Content-Type: text/plain; charset=UTF-8\\n"\n\n#, python-format\nmsgid "Hello, %(name)s!"\nmsgstr "Bonjour, %(nom)s !"\n',encoding='utf-8');refused(['msgfmt','--check-format','--output-file=named.mo','named.po'],b"argument 'name' doesn't exist in 'msgstr'")
  results.append('GNU gettext on the m96 PO: statistics, exact msgunfmt/msgcmp round trip, ITS metainfo extraction, msginit French plural header, envsubst, C-library lookup by LANGUAGE (C.UTF-8 honored, C ignored), named syntax and python-format errors')
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
  # TypeScript beyond the original build: m99's own source through its original argv, the compiler's exit statuses, the packaged standard library, project and build modes, ES modules with declarations and source maps, its private Node, and a project's own compiler ahead of the platform's.
  m99=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==99);tref=m99['validation_reference'];ts=d/'ts-app';ts.mkdir();version='Version '+tref['typescript']
  for n,s in json.loads(gzip.decompress(base64.b64decode(m99['commands']['initial']['build'].split("Buffer.from('",1)[1].split("'",1)[0]))).items():(ts/n).write_text(s,encoding='utf-8')
  for n in tref['files']:shutil.copyfile(ROOT/'source-carrier'/n,ts/n)
  def tsc(*a,cwd=ts,env=None):r=subprocess.run(['tsc',*a],cwd=cwd,capture_output=True,env=env,timeout=90);return r.returncode,r.stdout.decode()
  assert sha((ts/'qa-ts-app.cts').read_bytes())==tref['source_ts_sha256'] and tsc('--strict','--noEmitOnError','--module','NodeNext','--target','ES2022','--outDir','dist','qa-ts-app.cts')==(0,'')
  # 7.0.2 emits these bytes on linux/amd64 (production, w8/m99) and linux/arm64: the enum lowered, every type erased.
  emitted=(ts/'dist'/'qa-ts-app.cjs').read_bytes();assert sha(emitted)=='af1acfde1f1d6e4c12bea3d17f01516494d0f06053ee372377670b7bf3621242' and emitted.startswith(b'"use strict";') and b'Tone["Cool"] = "cool";' in emitted and b'interface' not in emitted
  rows=tref['rows'];expected={'labels':[r['label'] for r in rows],'total':sum(r['quantity'] for r in rows),'tones':['cool' if i%2 else 'warm' for i in range(len(rows))]}
  out=json.loads(run(['node','dist/qa-ts-app.cjs','build'],cwd=ts).decode().split('QA_TS_AFTER ',1)[1])
  assert expected==tref['expected'] and out['result']==expected and out['input']==rows and out['compiler']=={'status':0,'error':None,'version':version} and out['compiled_js_sha256']==sha(emitted)
  # Exit 1 when noEmitOnError withholds output, 2 when output is emitted despite errors, 1 for an unknown option.
  (ts/'bad.cts').write_text("const n:number='bad';\nexport {};\n");code,diag=tsc('--strict','--noEmitOnError','--outDir','bad-out','bad.cts');assert code==1 and 'bad.cts(1,7): error TS2322:' in diag and not (ts/'bad-out').exists()
  code,diag=tsc('--strict','--outDir','emitted-anyway','bad.cts');assert code==2 and 'error TS2322' in diag and (ts/'emitted-anyway'/'bad.cjs').is_file()
  code,diag=tsc('--bex-unknown-option','bad.cts');assert code==1 and 'error TS5023' in diag
  # The packaged standard library: ES2022's Array.prototype.at checks under its target and is refused under lib es2020.
  (ts/'lib.mts').write_text('export const last:number|undefined=[1,2,3].at(-1);\n');assert tsc('--strict','--noEmit','--module','NodeNext','--target','ES2022','lib.mts')==(0,'')
  code,diag=tsc('--strict','--noEmit','--module','NodeNext','--target','ES2020','--lib','es2020','lib.mts');assert code==1 and 'error TS2550' in diag and "'es2022'" in diag
  # A bare tsc reads tsconfig.json; --build records its state and then finds the project up to date.
  proj=ts/'proj';(proj/'src').mkdir(parents=True);(proj/'src'/'index.ts').write_text('export const greet=(s:string):string=>`hi ${s}`;\nconsole.log(greet("雪 two words"));\n',encoding='utf-8')
  (proj/'tsconfig.json').write_text('{"compilerOptions":{"strict":true,"target":"ES2022","module":"commonjs","outDir":"build","rootDir":"src"},"include":["src"]}\n')
  assert tsc(cwd=proj)==(0,'') and run(['node','build/index.js'],cwd=proj).decode()=='hi 雪 two words\n' and tsc('--build',cwd=proj)==(0,'') and (proj/'tsconfig.tsbuildinfo').is_file()
  code,log=tsc('--build','--verbose',cwd=proj);assert code==0 and "Project 'tsconfig.json' is up to date" in log
  # ES modules: .mts compiles to .mjs with declarations and a source map naming its source, and Node runs the result.
  (ts/'m.mts').write_text('export function add(a:number,b:number):number{return a+b}\n');(ts/'main.mts').write_text('import {add} from "./m.mjs";\nconsole.log(JSON.stringify({v:add(2,3)}));\n')
  assert tsc('--strict','--module','NodeNext','--target','ES2022','--declaration','--sourceMap','--outDir','esm','main.mts','m.mts')==(0,'') and json.loads(run(['node','esm/main.mjs'],cwd=ts))=={'v':5}
  assert (ts/'esm'/'m.d.mts').read_text()=='export declare function add(a: number, b: number): number;\n' and json.loads((ts/'esm'/'m.mjs.map').read_text())['sources']==['../m.mts'] and (ts/'esm'/'m.mjs').read_text().endswith('//# sourceMappingURL=m.mjs.map')
  # The wrapper runs its private Node, so it works with no node on PATH; npm scripts still run a project's own node_modules/.bin/tsc first.
  bare={'PATH':'/usr/bin:/bin'};assert shutil.which('node',path=bare['PATH']) is None and tsc('--version',env=bare)==(0,version+'\n')
  local=ts/'local';(local/'node_modules'/'.bin').mkdir(parents=True);stub=local/'node_modules'/'.bin'/'tsc';stub.write_text('#!/bin/sh\necho "Version 5.9.3-project-local"\n');stub.chmod(0o755);(local/'package.json').write_text('{"name":"local","private":true,"scripts":{"v":"tsc --version"}}\n')
  assert run(['npm','run','--silent','v'],cwd=local,env=dict(os.environ,npm_config_cache=str(d/'npm-cache'),npm_config_update_notifier='false')).decode()=='Version 5.9.3-project-local\n' and tsc('--version',cwd=local)==(0,version+'\n')
  results.append('TypeScript: m99 original source via its own argv, frozen 7.0.2 emit and independent result; exit 1 with noEmitOnError and no output, 2 with output, 1 for an unknown option; packaged ES2022 lib refused under lib es2020; tsconfig project and --build modes; ES modules with declarations and source maps; private Node with no node on PATH; project-local tsc first in npm scripts')
  # Webpack beyond the original build: m101's own sources through its original argv, the CLI's exit statuses, a config file with --env, code splitting with a JSON module, source maps, the persistent cache inside the project, its private Node, a project's own webpack ahead of the platform's, and no install question answered without a terminal.
  m101=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==101);wref=m101['validation_reference'];wp=d/'webpack-app';wp.mkdir()
  for n,s in json.loads(gzip.decompress(base64.b64decode(m101['commands']['initial']['build'].split("Buffer.from('",1)[1].split("'",1)[0]))).items():(wp/n).write_text(s,encoding='utf-8')
  for n in wref['files']:shutil.copyfile(ROOT/'source-carrier'/n,wp/n)
  def webpack(*a,cwd=wp,env=None,stdin=None):r=subprocess.run(['webpack',*a],cwd=cwd,capture_output=True,env=env,stdin=stdin,timeout=90);return r.returncode,r.stdout.decode(),r.stderr.decode()
  node=('--mode','production','--target','node')
  assert {n:sha((wp/n).read_bytes()) for n in wref['sources']}==wref['sources'];code,out,_=webpack('./qa-webpack-entry.mjs',*node,'--output-path','./dist','--output-filename','bundle.cjs','--json');stats=json.loads(out)
  # 5.112.2 with its locked terser emits these bytes on linux/amd64 (production, w8/m101) and linux/arm64.
  bundle=(wp/'dist'/'bundle.cjs').read_bytes();assert code==0 and (stats['version'],stats['errors'],stats['warnings'],[a['name'] for a in stats['assets']])==(wref['control_compiler'],[],[],['bundle.cjs']) and sha(bundle)=='8fd8eb19651af333eae6068b96c15a73bb417a6ae59637ecb7928df878199bfc'
  assert bundle.startswith(b'(()=>{"use strict";') and b'require("node:http")' in bundle and b'import ' not in bundle and b'qa-webpack-domain' not in bundle
  (wp/'qa-webpack-stats.json').write_text(out);rows=wref['rows'];expected={'labels':[r['label'] for r in rows],'total':sum(r['quantity'] for r in rows),'sorted':sorted(r['quantity'] for r in rows)}
  got=json.loads(run(['node','dist/bundle.cjs','build'],cwd=wp).decode().split('QA_WEBPACK_AFTER ',1)[1])
  assert expected==wref['expected'] and got['result']==expected and got['input']==rows and got['compiler']=={'version':stats['version'],'hash':stats['hash'],'errors':0,'warnings':0} and got['bundle']=={'bytes':len(bundle),'sha256':sha(bundle)} and got['webpack_probe']['status']==0
  # Exit 1 for an unresolved import, with no output written; 2 for an unknown option.
  (wp/'broken.mjs').write_text("import './does-not-exist.mjs';\n");code,out,_=webpack('./broken.mjs',*node,'--output-path','bad-out','--json')
  assert code==1 and [e['message'] for e in json.loads(out)['errors']]==["Module not found: Error: Can't resolve './does-not-exist.mjs' in '"+str(wp)+"'"] and not (wp/'bad-out').exists()
  code,_,err=webpack('./broken.mjs','--bex-unknown-option');assert code==2 and "Unknown option '--bex-unknown-option'" in err
  # A bare webpack reads webpack.config.mjs, whose exported function receives --env.
  cfg=wp/'cfg';cfg.mkdir();(cfg/'unit.mjs').write_text('console.log(JSON.stringify({unit:"雪 two words"}));\n',encoding='utf-8')
  (cfg/'webpack.config.mjs').write_text("export default env=>({mode:'production',target:'node',entry:'./unit.mjs',output:{path:new URL('./out',import.meta.url).pathname,filename:env.name+'.cjs'}});\n")
  assert webpack('--env','name=from-env',cwd=cfg)[0]==0 and json.loads(run(['node','out/from-env.cjs'],cwd=cfg))=={'unit':'雪 two words'}
  # Code splitting: a dynamic import becomes its own chunk beside a bundled JSON module; the persistent cache lives in the project's node_modules/.cache and serves the second build.
  sp=wp/'split';sp.mkdir();(sp/'package.json').write_text('{"name":"split","private":true}\n');(sp/'data.json').write_text(json.dumps({'rows':rows},ensure_ascii=False),encoding='utf-8')
  (sp/'lazy.mjs').write_text('export const total=rows=>rows.reduce((s,r)=>s+r.quantity,0);\n');(sp/'main.mjs').write_text("import data from './data.json' with {type:'json'};\nconst {total}=await import('./lazy.mjs');\nconsole.log(JSON.stringify({labels:data.rows.map(r=>r.label),total:total(data.rows)}));\n")
  split=lambda:json.loads(webpack('./main.mjs',*node,'--output-path','out','--output-filename','main.cjs','--output-chunk-filename','[id].chunk.cjs','--cache-type','filesystem','--json',cwd=sp)[1])
  first=split();files=sorted(f for c in first['chunks'] for f in c['files']);assert len(files)==2 and 'main.cjs' in files and files==sorted(p.name for p in (sp/'out').iterdir()) and json.loads(run(['node','out/main.cjs'],cwd=sp))=={'labels':expected['labels'],'total':expected['total']}
  assert (sp/'node_modules'/'.cache'/'webpack').is_dir() and not any(m.get('cached') for m in first['modules']) and all(m.get('cached') for m in split()['modules'])
  # Source maps name both authored modules of a bundle that calls m101's domain module.
  (wp/'mapped.mjs').write_text("import {summarize} from './qa-webpack-domain.mjs';\nconsole.log(JSON.stringify(summarize(JSON.parse(process.argv[2]))));\n")
  assert webpack('./mapped.mjs',*node,'--output-path','maps','--output-filename','mapped.cjs','--devtool','source-map')[0]==0 and sorted(json.loads((wp/'maps'/'mapped.cjs.map').read_text())['sources'])==['webpack:///./mapped.mjs','webpack:///./qa-webpack-domain.mjs']
  assert (wp/'maps'/'mapped.cjs').read_text().endswith('//# sourceMappingURL=mapped.cjs.map') and json.loads(run(['node','maps/mapped.cjs',json.dumps(rows)],cwd=wp))==expected
  # The wrapper runs its private Node, so it bundles with no node on PATH; npm scripts still run a project's own node_modules/.bin/webpack first.
  bare={'PATH':'/usr/bin:/bin'};code,out,_=webpack('./mapped.mjs',*node,'--output-path','bare','--json',env=bare);assert shutil.which('node',path=bare['PATH']) is None and code==0 and json.loads(out)['version']==wref['control_compiler']
  local=wp/'local';(local/'node_modules'/'.bin').mkdir(parents=True);stub=local/'node_modules'/'.bin'/'webpack';stub.write_text('#!/bin/sh\necho project-local-webpack "$@"\n');stub.chmod(0o755);pkg='{"name":"local","private":true,"scripts":{"b":"webpack --mode production"}}\n';(local/'package.json').write_text(pkg)
  assert run(['npm','run','--silent','b'],cwd=local,env=dict(os.environ,npm_config_cache=str(d/'npm-cache'),npm_config_update_notifier='false')).decode()=='project-local-webpack --mode production\n' and shutil.which('webpack')=='/usr/bin/webpack'
  # No dev server is supplied: without a terminal, serve asks to install webpack-dev-server and returns, changing nothing.
  _,out,err=webpack('serve',cwd=local,stdin=subprocess.DEVNULL);assert "you need to install: 'webpack-dev-server'" in out+err and (local/'package.json').read_text()==pkg and sorted(p.name for p in (local/'node_modules').iterdir())==['.bin']
  results.append('Webpack: m101 original sources via their own argv, frozen 5.112.2 bundle and independent result; exit 1 for an unresolved import with no output, 2 for an unknown option; webpack.config.mjs with --env; a dynamic-import chunk beside a JSON module; the filesystem cache in the project serving the second build; source maps; private Node with no node on PATH; project-local webpack first in npm scripts; serve installs nothing without a terminal')
  # SWIG beyond the original build: m100's own interface through its original argv, the frozen generated sources, the packaged library (typemaps, exceptions, arrays, C++ std::string), authored include paths and SWIG_LIB, and its exit statuses.
  m100=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==100);sref=m100['validation_reference'];sw=d/'swig-app';(sw/'inc').mkdir(parents=True);(sw/'gen').mkdir()
  (sw/'qa_swig.i').write_text(json.loads(gzip.decompress(base64.b64decode(m100['commands']['initial']['build'].split("b64decode('",1)[1].split("'",1)[0])))['qa_swig.i'],encoding='utf-8')
  def swig(*a,env=None):r=subprocess.run(['swig',*a],cwd=sw,capture_output=True,env=env,timeout=90);return r.returncode,r.stderr.decode()
  assert sha((sw/'qa_swig.i').read_bytes())==sref['interface_sha256'] and 'SWIG_LIB' not in os.environ and run(['swig','-swiglib'])==b'/usr/share/swig4.0\n' and swig('-python','qa_swig.i')==(0,'')
  # 4.1.0 generates these bytes on linux/amd64 (production, w8/m100) and linux/arm64.
  assert sha((sw/'qa_swig_wrap.c').read_bytes())=='a5594a40f2a89b3ae243a88d48bbe67605dffc1662ef84770cafe7cad0a15ce1' and sha((sw/'qa_swig.py').read_bytes())=='25875c839be1bab0b992b2be6ae69cd96df39791bdcfee0a868021ccb6f306b9'
  includes=shlex.split(run(['python3-config','--includes']).decode());suffix=run(['python3-config','--extension-suffix']).decode().strip()
  def extension(module,wrapper,compiler='gcc'):run([compiler,'-shared','-fPIC',*includes,wrapper,'-o','_'+module+suffix],cwd=sw)
  extension('qa_swig','qa_swig_wrap.c');expected=[{'a':a,'b':b,'value':2*a+3*b} for a,b in sref['pairs']]
  got=json.loads(run(['python3','-c','import json,sys,qa_swig;print(json.dumps({"result":[{"a":a,"b":b,"value":qa_swig.weighted_sum(a,b)} for a,b in json.loads(sys.argv[1])],"unicode":qa_swig.dessert()}))',json.dumps(sref['pairs'])],cwd=sw))
  assert expected==sref['expected'] and got=={'result':expected,'unicode':'Crème brûlée & 李'} and sref['unicode']==got['unicode']
  # The packaged library: OUTPUT typemaps, an exception.i ValueError and carrays; C truncates toward zero, unlike Python's divmod(-17,5).
  (sw/'lib.i').write_text('%module lib_bind\n%include "typemaps.i"\n%include "exception.i"\n%include "carrays.i"\n%array_functions(int, intArray);\n%apply int *OUTPUT { int *quotient, int *remainder };\n%exception divide {\n  $action\n  if (result != 0) SWIG_exception(SWIG_ValueError, "bex: divisor must be positive");\n}\n%inline %{\nint divide(int a, int b, int *quotient, int *remainder) { if (b <= 0) return -1; *quotient = a / b; *remainder = a % b; return 0; }\nint sum_array(int *values, int n) { int s = 0; for (int i = 0; i < n; i++) s += values[i]; return s; }\n%}\n')
  assert swig('-python','lib.i')==(0,'');extension('lib_bind','lib_wrap.c')
  probe='import json,lib_bind\ntry:lib_bind.divide(1,0);refused=None\nexcept ValueError as e:refused=str(e)\na=lib_bind.new_intArray(4)\nfor i,v in enumerate([5,-2,40,0]):lib_bind.intArray_setitem(a,i,v)\nprint(json.dumps([lib_bind.divide(17,5),lib_bind.divide(-17,5),refused,lib_bind.sum_array(a,4),lib_bind.intArray_getitem(a,2)]))\nlib_bind.delete_intArray(a)\n'
  assert json.loads(run(['python3','-c',probe],cwd=sw))==[[0,3,2],[0,-3,-2],'bex: divisor must be positive',43,40]
  # Those files come from the data tree: an authored SWIG_LIB elsewhere is honored, and the same interface then finds none of them.
  code,diag=swig('-python','lib.i',env=dict(os.environ,SWIG_LIB=str(d/'no-swig-lib')));assert code==1 and "Unable to find 'swig.swg'" in diag and "lib.i:2: Error: Unable to find 'typemaps.i'" in diag
  # C++ through std_string.i: UTF-8 crosses std::string intact and only ASCII letters change.
  (sw/'cpp.i').write_text('%module cpp_bind\n%include "std_string.i"\n%{\n#include <string>\n%}\n%inline %{\nstd::string shout(const std::string &s) { std::string r(s); for (auto &c : r) if (c >= \'a\' && c <= \'z\') c -= 32; return r + "!"; }\n%}\n')
  assert swig('-c++','-python','cpp.i')==(0,'');extension('cpp_bind','cpp_wrap.cxx','g++');text='crème brûlée & 李 two words'
  assert run(['python3','-c','import sys,cpp_bind;sys.stdout.buffer.write(cpp_bind.shout(sys.argv[1]).encode())',text],cwd=sw).decode()==''.join(c.upper() if 'a'<=c<='z' else c for c in text)+'!'
  # An authored include directory is searched only when named; -o and -outdir place the wrapper and the proxy.
  (sw/'inc'/'custom.i').write_text('%inline %{\nint triple(int x) { return 3*x; }\n%}\n');(sw/'inc_bind.i').write_text('%module inc_bind\n%include "custom.i"\n')
  code,diag=swig('-python','inc_bind.i');assert code==1 and "inc_bind.i:2: Error: Unable to find 'custom.i'" in diag and not (sw/'inc_bind_wrap.c').exists()
  assert swig('-python','-Iinc','-o','gen/inc_wrap.c','-outdir','inc','inc_bind.i')==(0,'') and (sw/'inc'/'inc_bind.py').is_file() and not (sw/'gen'/'inc_bind.py').exists();extension('inc_bind','gen/inc_wrap.c')
  assert run(['python3','-c','import sys;sys.path[1:1]=["inc"];import inc_bind;print(inc_bind.triple(-14))'],cwd=sw)==b'-42\n'
  # Exit 1 for a syntax error, writing nothing, and for an unknown option.
  (sw/'bad.i').write_text('%module bad\n%inline %{\nint broken( ;\n%}\n');code,diag=swig('-python','bad.i');assert code==1 and 'bad.i:3: Error: Syntax error in input(1).' in diag and not (sw/'bad_wrap.c').exists() and not (sw/'bad.py').exists()
  code,diag=swig('-python','-bex-unknown-option','qa_swig.i');assert code==1 and 'Unrecognized option -bex-unknown-option' in diag
  results.append('SWIG: m100 original interface via its own argv, frozen 4.1.0 wrapper and proxy, independent C result and Unicode; OUTPUT typemaps, exception.i ValueError and carrays from the packaged library, which an authored SWIG_LIB replaces; C++ std::string via std_string.i; authored -I include path with -o/-outdir; exit 1 for a syntax error with no output and for an unknown option')
  # ZIP creation, which Render lists at build and runtime: m107's own program and zip 3.0's options in every phase, each archive read back with Python's zipfile alone.
  m107=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==107);zref=m107['validation_reference'];zq=ROOT/'qa-zip';zipdir=d/'zip';zipdir.mkdir()
  ZIP3='This is Zip 3.0 (July 5th 2008), by Info-ZIP.'
  # zip 3.0 (3.0-13+deb12u1) writes these bytes for m107's frozen marker on linux/amd64 and linux/arm64; production wrote the arm64 replica's bytes for its own markers (w8/m107).
  frozen={'build':'317e333e35d37cbee7b083083a949a64935dd8297e700857c44d656a525ee7d6','runtime':'67642885d48c45e8502a66c9b763a8333becff1bd0e3ce1b351f1204db8b35a9'}
  def zipcmd(*a,data=None):r=subprocess.run(['zip',*a],cwd=zipdir,input=data,capture_output=True,timeout=90);return r.returncode,r.stdout.decode()
  def entries(archive,expected):
   # Exact ordered names, bytes, sizes and CRC32; returns each member's method, flags and extra field ids.
   with zipfile.ZipFile(archive) as z:
    assert [v.filename for v in z.infolist()]==list(expected) and z.testzip() is None,z.namelist()
    out=[]
    for v in z.infolist():
     b=expected[v.filename];assert (z.read(v),v.file_size,v.CRC)==(b,len(b),zlib.crc32(b)),v.filename;ids,i=[],0
     while i<len(v.extra):h,n=struct.unpack_from('<HH',v.extra,i);ids.append(h);i+=4+n
     out.append((v.compress_type,v.flag_bits,ids))
    return out
  # zip stores a non-ASCII name as its UTF-8 bytes without the UTF-8 flag, so zipfile reads it as code page 437.
  cp437=lambda n:n.encode().decode('cp437')
  def program_members(label):return {'CHANGELOG':(zq/'CHANGELOG').read_bytes(),'LICENSE':(zq/'LICENSE').read_bytes(),'empty.txt':b'','greeting.txt':('Crème brûlée & 李\n'*7).encode(),'request.txt':label.encode(),'two words.bin':bytes((i*31+17)&255 for i in range(8192))}
  if PHASE=='build':
   # m107's own program through its original argv, outside the original build.
   zq.mkdir()
   for n,s in json.loads(gzip.decompress(base64.b64decode(m107['commands']['initial']['build'].split("Buffer.from('\"'\"'",1)[1].split("'",1)[0]))).items():(zq/n).write_text(s,encoding='utf-8')
   for n in zref['files']:shutil.copyfile(ROOT/'source-carrier'/n,zq/n)
   out=run(['node','qa434-app.cjs','build'],cwd=zq).decode();before=json.loads(out.split('QA434_BEFORE ',1)[1].split('\n',1)[0])
   assert before['marker']==m107['marker'] and before['source']==zref['files'] and before['unzip']=='UnZip 6.00 of 20 April 2009, by Debian. Original by Info-ZIP.'
  # Every phase: the retained build archive is the frozen bytes, with the program's members and methods.
  built=json.loads((zq/'qa434-built.json').read_text());blob=base64.b64decode(built['archive']['base64'])
  assert (built['marker'],built['source'],built['zip'],sha(blob),len(blob))==(m107['marker'],zref['files'],ZIP3,frozen['build'],built['archive']['bytes']) and 'No errors detected in compressed data of ' in built['unzip_test']
  assert [m for m,_,_ in entries(io.BytesIO(blob),program_members(m107['marker']+'|build'))]==[0,8,0,8,0,8]
  if PHASE=='runtime':
   # m107's own server creates a fresh archive per request with the runtime's zip, downloaded and inspected.
   with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
   server=subprocess.Popen(['node','qa434-app.cjs','serve'],cwd=zq,env=dict(os.environ,PORT=str(port)),stdout=subprocess.DEVNULL)
   def get(path):c=http.client.HTTPConnection('127.0.0.1',port,timeout=10);c.request('GET',path);r=c.getresponse();return r.status,r.getheader('Content-Type'),r.read()
   try:
    for _ in range(150):
     try:download=get('/download?qa=qa-zip-runtime');break
     except OSError:time.sleep(.03)
    else:raise RuntimeError('qa434-app.cjs serve did not listen')
    inspect=get('/inspect?qa=qa-zip-runtime')
   finally:server.terminate();server.wait(timeout=5)
   proof=json.loads(inspect[2]);fresh=base64.b64decode(proof['runtime']['archive']['base64'])
   assert download[:2]==(200,'application/zip') and inspect[:2]==(200,'application/json') and sha(download[2])==sha(fresh)==frozen['runtime'] and proof['built']==built and proof['runtime']['zip']==ZIP3
   entries(io.BytesIO(download[2]),program_members(m107['marker']+'|qa-zip-runtime'))
  # -0 stores every member; -9 deflates with the maximum-compression flag, but an empty member stays stored. unzip restores the Unicode, space-containing name and its bytes.
  members={'empty.txt':b'','greeting.txt':('Crème brûlée & 李\n'*100).encode(),'two words.bin':bytes(range(256))*64,'雪 two.txt':'Crème 雪\n'.encode()*50}
  for n,b in members.items():(zipdir/n).write_bytes(b)
  for label,level,methods in [('stored','-0',[(0,0)]*4),('deflated','-9',[(0,0)]+[(8,2)]*3)]:
   assert zipcmd('-q',level,label+'.zip',*members)==(0,'')
   assert [(m,f,ids) for m,f,ids in entries(zipdir/(label+'.zip'),{cp437(n):b for n,b in members.items()})]==[(m,f,[0x5455,0x7875]) for m,f in methods]
   if PHASE=='build':shutil.copyfile(zipdir/(label+'.zip'),ROOT/'public'/(label+'.zip'))
  run(['unzip','-q','-d','out','deflated.zip'],cwd=zipdir);assert {n:(zipdir/'out'/n).read_bytes() for n in os.listdir(zipdir/'out')}==members
  # -X drops the extended timestamp (UT) and Unix owner (ux) fields; -Z bzip2 compresses through libbz2; -y keeps a symlink as a link.
  assert zipcmd('-q','-X','plain.zip','empty.txt')==(0,'') and entries(zipdir/'plain.zip',{'empty.txt':b''})==[(0,0,[])]
  assert zipcmd('-q','-Z','bzip2','bz.zip','greeting.txt')==(0,'') and entries(zipdir/'bz.zip',{'greeting.txt':members['greeting.txt']})[0][0]==12
  os.symlink('two words.bin',zipdir/'link');assert zipcmd('-q','-y','link.zip','link')==zipcmd('-q','follow.zip','link')==(0,'')
  with zipfile.ZipFile(zipdir/'link.zip') as z:v=z.infolist()[0];assert stat.S_ISLNK(v.external_attr>>16) and z.read(v)==b'two words.bin'
  entries(zipdir/'follow.zip',{'link':members['two words.bin']})
  # Recursion with and without directory entries, junked paths, an exclusion and names read from stdin.
  tree={'tree/b.bin':bytes(range(256)),'tree/sub/a.txt':b'a\n'}
  for n,b in tree.items():(zipdir/n).parent.mkdir(parents=True,exist_ok=True);(zipdir/n).write_bytes(b)
  dirs={'tree/':b'','tree/sub/':b''}
  for argv,expected in [(['-r','r.zip','tree'],{**dirs,**tree}),(['-r','-D','D.zip','tree'],tree),(['-r','-j','j.zip','tree'],{'b.bin':tree['tree/b.bin'],'a.txt':tree['tree/sub/a.txt']}),(['-r','x.zip','tree','-x','*.bin'],{**dirs,'tree/sub/a.txt':tree['tree/sub/a.txt']})]:
   assert zipcmd('-q',*argv)==(0,'')
   with zipfile.ZipFile(zipdir/next(a for a in argv if a.endswith('.zip'))) as z:assert {n:z.read(n) for n in z.namelist()}==expected and len(z.namelist())==len(expected)
  assert zipcmd('-q','-@','at.zip',data=b'tree/sub/a.txt\ntree/b.bin\n')==(0,'') and entries(zipdir/'at.zip',{n:tree[n] for n in ('tree/sub/a.txt','tree/b.bin')})
  # Streams: stdin becomes the member '-', and an archive written to a pipe uses data descriptors.
  assert zipcmd('-q','stdin.zip','-',data=members['greeting.txt'])==(0,'') and entries(zipdir/'stdin.zip',{'-':members['greeting.txt']})[0][0]==8
  r=subprocess.run(['zip','-q','-','empty.txt','two words.bin'],cwd=zipdir,capture_output=True,timeout=90);assert r.returncode==0 and [f&8 for _,f,_ in entries(io.BytesIO(r.stdout),{n:members[n] for n in ('empty.txt','two words.bin')})]==[8,8]
  # Update replaces only the newer member, delete removes one, -z writes the archive comment from stdin and -T tests the archive.
  (zipdir/'old.txt').write_bytes(b'old\n');os.utime(zipdir/'old.txt',(1000000000,1000000000));assert zipcmd('-q','u.zip','old.txt','empty.txt')==(0,'');(zipdir/'old.txt').write_bytes(b'newer\n')
  assert zipcmd('-u','u.zip','old.txt','empty.txt')==(0,'updating: old.txt (stored 0%)\n') and entries(zipdir/'u.zip',{'old.txt':b'newer\n','empty.txt':b''})
  assert zipcmd('-d','u.zip','old.txt')==(0,'deleting: old.txt\n') and zipcmd('-z','u.zip',data='note 雪\n'.encode())[0]==0 and zipcmd('-T','u.zip')==(0,'test of u.zip OK\n')
  with zipfile.ZipFile(zipdir/'u.zip') as z:assert z.namelist()==['empty.txt'] and z.comment=='note 雪'.encode()
  # Exit statuses with zip's own messages: 12 when nothing matches, an archive kept unchanged; 18 under -MM for an unreadable input; 16 for an unknown option; 3 for a damaged archive, left as it was.
  kept=sha((zipdir/'u.zip').read_bytes());(zipdir/'damaged.zip').write_bytes(b'not a zip archive')
  assert zipcmd('-q','none.zip','absent.txt')==(12,'\nzip error: Nothing to do! (none.zip)\n') and not (zipdir/'none.zip').exists()
  assert zipcmd('-d','u.zip','absent.txt')==(12,'\tzip warning: name not matched: absent.txt\n\nzip error: Nothing to do! (u.zip)\n') and sha((zipdir/'u.zip').read_bytes())==kept
  assert zipcmd('-MM','-q','mm.zip','empty.txt','absent.txt')==(18,'zip I/O error: No such file or directory\nzip error: File not found or no read permission (absent.txt)\n') and not (zipdir/'mm.zip').exists()
  assert zipcmd('--bex-unknown-option','x2.zip','empty.txt')==(16,"\nzip error: Invalid command arguments (long option 'bex-unknown-option' not supported)\n")
  code,out=zipcmd('damaged.zip','empty.txt');assert code==3 and out.endswith('\nzip error: Zip file structure invalid (damaged.zip)\n') and (zipdir/'damaged.zip').read_bytes()==b'not a zip archive'
  results.append('ZIP (build and runtime): m107 original program via its own argv, frozen 3.0 build archive at every phase and fresh runtime archives from its server; -0/-9 methods with empty/binary/Unicode/space members, unzip restoring names and bytes; -X extra fields, bzip2 through libbz2, symlinks with and without -y; recursion, -D, -j, -x, -@; stdin member and piped archive with data descriptors; -u, -d, -z comment, -T; exit 12/18/16/3 with zip\'s messages and archives kept')
  # pigz is build-only, as Render lists it: it runs at build, and every phase reads the retained artifacts back with Python's zlib alone.
  compressed_dir=ROOT/'public';payloads={'empty':b'','binary':bytes(range(256))*64,'unicode':('Crème 雪\n'*100).encode()}
  m102=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==102);pref=m102['validation_reference'];pz=ROOT/'qa-pigz'
  original=payloads['m102-original']=(pref['input']['unit']*pref['input']['repetitions']).encode()
  # pigz 2.6 on zlib 1.2.13 emits these bytes for m102's input on linux/amd64 (production, w8/m102) and linux/arm64.
  frozen='8aecad5d82730581c6039f179160de01f9f1a3c9f03745548e701394d3d540ee'
  frozen_proof={'bytes':130,'sha256':frozen,'header_hex':'1f8b0800000000000003','CRC32':zlib.crc32(original),'ISIZE':len(original),'inflated_bytes':len(original),'inflated_sha256':pref['input']['sha256']}
  def pigz(*a,cwd=d,data=None,env=None,name='pigz'):r=subprocess.run([name,*a],cwd=cwd,input=data,capture_output=True,env=env,timeout=90);return r.returncode,r.stdout,r.stderr.decode()
  if PHASE=='build':
   # m102's own program through its original argv, outside the original build: the frozen production bytes and the program's own build proof.
   pz.mkdir()
   for n,s in json.loads(gzip.decompress(base64.b64decode(m102['commands']['initial']['build'].split("b64decode('",1)[1].split("'",1)[0]))).items():(pz/n).write_text(s,encoding='utf-8')
   for n in pref['files']:shutil.copyfile(ROOT/'source-carrier'/n,pz/n)
   assert json.loads(run([sys.executable,'qa-pigz-app.py','before'],cwd=pz).decode().split('QA_PIGZ_BEFORE ',1)[1])['pigz_path']=='/usr/bin/pigz'
   code,out,_=pigz('-n','-p','2','-c','qa-pigz-input.txt',cwd=pz);(pz/'qa-pigz-output.gz').write_bytes(out);assert code==0 and sha(out)==frozen
   got=json.loads(run([sys.executable,'qa-pigz-app.py','build'],cwd=pz).decode().split('QA_PIGZ_AFTER ',1)[1])
   assert got['build_compressor_version']=='pigz 2.6' and got['gzip']==frozen_proof
   for name,data in payloads.items():(compressed_dir/(name+'.gz')).write_bytes(pigz('-n','-p','2','-c',data=data)[1])
   # In place, a Unicode, space-containing name keeps its mode and time, and the header stores both (FLG.FNAME, MTIME); -d -N restores name, time and mode.
   f=d/'two words 雪.txt';f.write_bytes(original);f.chmod(0o640);os.utime(f,(1700000000,1700000000));assert pigz(f.name)[0]==0 and not f.exists()
   z=d/(f.name+'.gz');raw=z.read_bytes();assert (raw[3],struct.unpack('<I',raw[4:8])[0],raw[10:].split(b'\0',1)[0],z.stat().st_mode&0o777,int(z.stat().st_mtime))==(8,1700000000,f.name.encode(),0o640,1700000000)
   z.rename(d/'renamed.gz');assert pigz('-d','-N','renamed.gz')[0]==0 and not (d/'renamed.gz').exists() and (f.read_bytes(),f.stat().st_mode&0o777,int(f.stat().st_mtime))==(original,0o640,1700000000)
   # The output does not depend on the thread count: a 14-block input gives the same bytes on 1, 2, 4 and 8 threads (-n omits wall-clock MTIME on stdin); -i (independent blocks) differs and still inflates.
   big=original+b''.join(hashlib.sha256(i.to_bytes(4,'big')).digest() for i in range(12800))+original;outs={pigz('-n','-b','32','-p',p,'-c',data=big)[1] for p in '1248'}
   assert len(outs)==1 and zlib.decompress(next(iter(outs)),wbits=31)==big;indep=pigz('-n','-i','-b','32','-p','4','-c',data=big)[1];assert indep not in outs and zlib.decompress(indep,wbits=31)==big
   # zlib (-z, Adler-32 trailer), single-entry ZIP (-K) and zopfli (-11) formats, concatenated members, unpigz, and options read from PIGZ.
   zl=pigz('-z','-c',data=original)[1];assert zl[:2]==b'\x78\x5e' and zlib.decompress(zl)==original and struct.unpack('>I',zl[-4:])[0]==zlib.adler32(original)
   (d/'member.txt').write_bytes(original);assert pigz('-K','-k','member.txt')[0]==0
   with zipfile.ZipFile(d/'member.txt.zip') as archive:assert [(v.filename,v.compress_type,v.CRC,v.file_size) for v in archive.infolist()]==[('member.txt',8,zlib.crc32(original),len(original))] and archive.read('member.txt')==original
   z11=pigz('-11','-n','-c',data=original)[1];assert len(z11)<len(out) and zlib.decompress(z11,wbits=31)==original
   two=pigz('-c',data='雪 '.encode())[1]+pigz('-c',data=b'two words\n')[1];assert pigz('-d','-c',data=two)[1]==gzip.decompress(two)=='雪 two words\n'.encode() and pigz('-c',data=out,name='unpigz')[1]==original
   assert pigz('-c','qa-pigz-input.txt',cwd=pz,env=dict(os.environ,PIGZ='-n -p 2'))[1]==out!=pigz('-c','qa-pigz-input.txt',cwd=pz)[1]
   # Exit 1 for corrupt, truncated or missing input and for an existing output, which stays until -f; 22 (EINVAL) for an unknown option.
   assert pigz('-d','-c',data=b'invalid gzip')[0::2]==(1,'pigz: skipping: <stdin> unrecognized format\n') and pigz('-t',data=out[:60])[0::2]==(1,'pigz: skipping: <stdin>: corrupted -- incomplete deflate data\n') and pigz('-t',data=out)[0]==0
   assert pigz('absent.txt')[0::2]==(1,'pigz: skipping: absent.txt does not exist\n') and pigz('--bex-unknown-option')[0::2]==(22,'pigz: abort: invalid option: --bex-unknown-option\n')
   (d/'e.txt').write_bytes(b'first\n');assert pigz('e.txt')[0]==0;(d/'e.txt').write_bytes(b'second\n')
   assert pigz('e.txt')[0::2]==(1,'pigz: skipping: e.txt.gz exists\n') and gzip.decompress((d/'e.txt.gz').read_bytes())==b'first\n' and pigz('-f','e.txt')[0]==0 and gzip.decompress((d/'e.txt.gz').read_bytes())==b'second\n'
  for name,data in payloads.items():
   raw=(compressed_dir/(name+'.gz')).read_bytes();assert raw[:10]==bytes.fromhex('1f8b0800000000000003') and struct.unpack('<II',raw[-8:])==(zlib.crc32(data),len(data)) and zlib.decompress(raw,wbits=31)==data
  assert sha((compressed_dir/'m102-original.gz').read_bytes())==frozen
  if PHASE=='runtime':
   # m102's own server serves the built bytes with no pigz on PATH.
   with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
   server=subprocess.Popen([sys.executable,'qa-pigz-app.py','serve'],cwd=pz,env={'PATH':'/nonexistent','PORT':str(port)},stdout=subprocess.DEVNULL)
   def get(path):c=http.client.HTTPConnection('127.0.0.1',port,timeout=5);c.request('GET',path);r=c.getresponse();return r.status,r.getheader('Content-Type'),r.read()
   try:
    for _ in range(150):
     try:asset=get('/asset.gz');break
     except OSError:time.sleep(.03)
    else:raise RuntimeError('qa-pigz-app.py serve did not listen')
    inspect=get('/inspect')
   finally:server.terminate();server.wait(timeout=5)
   proof=json.loads(inspect[2]);assert asset[:2]==(200,'application/gzip') and sha(asset[2])==frozen and inspect[:2]==(200,'application/json') and proof['runtime']['gzip']==proof['build']['gzip']==frozen_proof and proof['build']['build_compressor_version']=='pigz 2.6'
  results.append('pigz (build only): m102 original program via its own argv, frozen 2.6 bytes and build proof; empty/binary/Unicode and original artifacts with exact header/CRC32/ISIZE/inflation at every phase; in-place name/mode/time and -N restore; output independent of thread count, -i; zlib/ZIP/zopfli formats, concatenated members, unpigz, PIGZ options; exit 1 for corrupt/truncated/missing input and an existing output kept until -f, 22 for an unknown option; the runtime serves the built bytes with no pigz on PATH')
  # Python packaging on Debian's system Python, which Render lists for builds and runtime (python3-setuptools, python3-pip, python3-dev). m103's original build runs in original-cases.py; this packages m103's own C source and wheel into a second project.
  m103=next(x for x in json.loads((ROOT/'original-cases.json').read_text()) if x['mid']==103);pref=m103['validation_reference'];pk=ROOT/'qa-pypkg';py='/usr/bin/python3';pip=[py,'-m','pip'];links=pk/'links'
  src=json.loads(lzma.decompress(base64.b64decode(m103['commands']['initial']['build'].split("b64decode('",1)[1].split("'",1)[0])));tag='cp311-cp311-linux_'+platform.machine()
  def pyrun(*a,cwd=pk,env=None):r=subprocess.run(list(a),cwd=cwd,capture_output=True,env=env,timeout=180);return r.returncode,r.stdout.decode(),r.stderr.decode()
  expected={'label':pref['unicode'],'dessert':pref['unicode'],'sums':[e['value'] for e in pref['expected']],'data':'Crème 雪 two words\n'}
  assert sys.executable==py and platform.python_version()=='3.11.2' and sysconfig.get_path('include')=='/usr/include/python3.11' and sysconfig.get_config_var('SOABI')=='cpython-311-'+sysconfig.get_config_var('MULTIARCH')
  wheel=links/'qa_local_data-1.0-py3-none-any.whl';built=pk/'wh'/('qa_meta-2.5-'+tag+'.whl');ext='qa_native_ext'+sysconfig.get_config_var('EXT_SUFFIX')
  def proof(*a,cwd=d,env=None):code,out,err=pyrun(*a,cwd=cwd,env=env);assert code==0,(a,err[-800:]);assert json.loads(out)==expected,(a,out)
  if PHASE=='build':
   # One project from m103's own C source and wheel: setuptools metadata (requirement, entry point, package data) and an sdist.
   (pk/'qa_meta').mkdir(parents=True);links.mkdir()
   with zipfile.ZipFile(wheel,'w') as z:
    for f,b in src[wheel.name]['wheel'].items():z.writestr(zipfile.ZipInfo(f,(2000,1,1,0,0,0)),b)
   assert sha(wheel.read_bytes())==m103['source_file_identities'][wheel.name]['sha256'];(pk/'qa_native_ext.c').write_text(src['qa_native_ext.c']['text'])
   (pk/'qa_meta'/'data.txt').write_text(expected['data'],encoding='utf-8');(pk/'qa_meta'/'__init__.py').write_text('import json,qa_native_ext\nfrom importlib.resources import files\nfrom qa_local_data import label\ndef main():print(json.dumps({"label":label(),"dessert":qa_native_ext.dessert(),"sums":[qa_native_ext.weighted_sum(a,b) for a,b in '+json.dumps(pref['pairs'])+'],"data":files("qa_meta").joinpath("data.txt").read_text(encoding="utf-8")},ensure_ascii=False))\n')
   (pk/'setup.py').write_text("from setuptools import Extension,setup\nsetup(name='qa-meta',version='2.5',packages=['qa_meta'],package_data={'qa_meta':['data.txt']},install_requires=['qa-local-data>=1.0'],entry_points={'console_scripts':['qa-meta=qa_meta:main']},ext_modules=[Extension('qa_native_ext',['qa_native_ext.c'])])\n")
   assert json.loads(run([py,'-c','import json,setuptools,pkg_resources,distutils,wheel;print(json.dumps([setuptools.__version__,wheel.__version__,setuptools.__file__,pkg_resources.__file__,distutils.__file__]))']))==['66.1.1','0.38.4','/usr/lib/python3/dist-packages/setuptools/__init__.py','/usr/lib/python3/dist-packages/pkg_resources/__init__.py','/usr/lib/python3/dist-packages/setuptools/_distutils/__init__.py']
   run([py,'setup.py','egg_info'],cwd=pk);info=pk/'qa_meta.egg-info';pkg=email.parser.Parser().parsestr((info/'PKG-INFO').read_text())
   assert (pkg['Name'],pkg['Version'],(info/'requires.txt').read_text(),(info/'entry_points.txt').read_text())==('qa-meta','2.5','qa-local-data>=1.0\n','[console_scripts]\nqa-meta = qa_meta:main\n') and {'qa_native_ext.c','qa_meta/data.txt','setup.py'}<=set((info/'SOURCES.txt').read_text().split())
   run([py,'setup.py','sdist','--formats=gztar'],cwd=pk)
   with tarfile.open(pk/'dist'/'qa-meta-2.5.tar.gz') as t:assert {'qa-meta-2.5/PKG-INFO','qa-meta-2.5/setup.py','qa-meta-2.5/qa_native_ext.c','qa-meta-2.5/qa_meta/data.txt','qa-meta-2.5/qa_meta.egg-info/requires.txt'}<=set(t.getnames()) and t.extractfile('qa-meta-2.5/qa_native_ext.c').read()==(pk/'qa_native_ext.c').read_bytes()
   # pip builds a platform wheel against the matching 3.11 headers through setuptools and wheel 0.38.4; every RECORD hash is checked independently.
   assert run(pip+['--version']).decode()=='pip 23.0.1 from /usr/lib/python3/dist-packages/pip (python 3.11)\n';run(pip+['wheel','--no-index','--no-deps','--no-build-isolation','--wheel-dir','wh','.'],cwd=pk)
   with zipfile.ZipFile(built) as z:
    record=[l.split(',') for l in z.read('qa_meta-2.5.dist-info/RECORD').decode().splitlines()];meta=email.parser.Parser().parsestr(z.read('qa_meta-2.5.dist-info/METADATA').decode())
    assert ('Tag: '+tag) in z.read('qa_meta-2.5.dist-info/WHEEL').decode().splitlines() and meta.get_all('Requires-Dist')==['qa-local-data (>=1.0)'] and {ext,'qa_meta/data.txt','qa_meta/__init__.py'}<={n for n,_,_ in record}
    assert all(h=='sha256='+base64.urlsafe_b64encode(hashlib.sha256(z.read(n)).digest()).rstrip(b'=').decode() and int(s)==len(z.read(n)) for n,h,s in record if h);so=z.read(ext)
   # Offline dependency resolution into an explicit target: the own wheel comes from --find-links, its module bytes exact; the console script and PYTHONPATH work as installed.
   run(pip+['install','--no-index','--find-links','links','--target','site',str(built)],cwd=pk);site=pk/'site'
   assert sha((site/'qa_local_data'/'__init__.py').read_bytes())==m103['expected_module_sha'] and (site/ext).read_bytes()==so and (site/'bin'/'qa-meta').read_text().startswith('#!/usr/bin/python3')
   env=dict(os.environ,PYTHONPATH=str(site));proof(py,'-c','import qa_meta;qa_meta.main()',env=env);proof(str(site/'bin'/'qa-meta'),env=env)
   dists={x.metadata['Name']:x for x in importlib.metadata.distributions(path=[str(site)])};assert {n:x.version for n,x in dists.items()}=={'qa-meta':'2.5','qa-local-data':'1.0'} and dists['qa-meta'].requires==['qa-local-data (>=1.0)'] and [e.value for e in dists['qa-meta'].entry_points]==['qa_meta:main']
   import pkg_resources;assert sorted(x.project_name for x in pkg_resources.WorkingSet([str(site)]).require('qa-meta'))==['qa-local-data','qa-meta']
   # Debian marks its site-packages externally managed (PEP 668): a system or --user install is refused, a venv bootstraps pip and setuptools offline from Debian's wheels and installs with dependencies.
   for extra in ([],['--user']):code,_,err=pyrun(*pip,'install','--no-index','--no-deps',*extra,str(wheel));assert code==1 and err.startswith('error: externally-managed-environment')
   run([py,'-m','venv','venv'],cwd=pk);vpy=str(pk/'venv'/'bin'/'python');run([vpy,'-m','pip','install','--no-index','--find-links','links',str(built)],cwd=pk);proof(str(pk/'venv'/'bin'/'qa-meta'))
   assert {p['name']:p['version'] for p in json.loads(run([vpy,'-m','pip','list','--format','json']))}=={'pip':'23.0.1','setuptools':'66.1.1','qa-meta':'2.5','qa-local-data':'1.0'} and pyrun(py,'-c','import qa_meta',cwd=d)[0]==1
   # pip's refusals: an unsatisfiable requirement, an invalid wheel (nothing written) and an unknown option.
   code,_,err=pyrun(*pip,'install','--no-index','--find-links','links','--target','t-missing','qa-local-data>=2');assert code==1 and 'ERROR: No matching distribution found for qa-local-data>=2' in err and not (pk/'t-missing').exists()
   (d/'bad-1.0-py3-none-any.whl').write_bytes(b'not a wheel');code,_,err=pyrun(*pip,'install','--no-index','--no-deps','--target','t-bad',str(d/'bad-1.0-py3-none-any.whl'));assert code==1 and "ERROR: Wheel 'bad' located at" in err and err.endswith(' is invalid.\n') and not (pk/'t-bad').exists()
   code,_,err=pyrun(*pip,'install','--bex-unknown-option');assert code==2 and err.endswith('no such option: --bex-unknown-option\n')
   # Development headers match the interpreter: python3-config, Python.h's version, an embedded interpreter linked to libpython3.11, and a named compiler error with no extension written.
   assert run(['python3-config','--includes'])==b'-I/usr/include/python3.11 -I/usr/include/python3.11\n' and run(['python3-config','--extension-suffix']).decode().strip()==sysconfig.get_config_var('EXT_SUFFIX') and '#define PY_VERSION              "3.11.2"\n' in Path('/usr/include/python3.11/patchlevel.h').read_text()
   (pk/'embed.c').write_text('#define PY_SSIZE_T_CLEAN\n#include <Python.h>\nint main(int argc,char **argv){if(argc!=3)return 2;Py_Initialize();PyObject *m=PyImport_ImportModule("json"),*r=m?PyObject_CallMethod(m,"dumps","(s)",argv[1]):NULL;if(!r)return 3;printf("%s %d %s %ld\\n",PY_VERSION,PY_VERSION_HEX==Py_Version,PyUnicode_AsUTF8(r),2*atol(argv[2])+3*7);Py_DECREF(r);Py_DECREF(m);return Py_FinalizeEx()<0?4:0;}\n')
   flags=lambda *a:shlex.split(run(['python3-config','--cflags',*a,'--ldflags']).decode());assert pyrun('gcc','embed.c','-o','embed',*flags())[0]==1;run(['gcc','embed.c','-o','embed',*flags('--embed')],cwd=pk)
   (pk/'broken.c').write_text('#include <Python.h>\nstatic PyObject *f(PyObject *s,PyObject *a){return PyLong_FromLong(undeclared_bex_name);}\n');(pk/'broken_setup.py').write_text("from setuptools import Extension,setup\nsetup(name='broken',ext_modules=[Extension('broken',['broken.c'])])\n")
   code,_,err=pyrun(py,'broken_setup.py','build_ext','--build-lib','broken-lib','--build-temp','broken-temp');assert code==1 and 'undeclared_bex_name' in err and 'undeclared (first use in this function)' in err and 'gcc\' failed with exit code 1' in err and not list(pk.glob('broken-lib/broken*'))
  # Every phase: the retained wheel installs again offline into a fresh target, both installations give the expected results with the same extension bytes, and the embedded interpreter runs.
  run(pip+['install','--no-index','--find-links',str(links),'--target',str(d/'again'),str(built)],cwd=d);assert (d/'again'/ext).read_bytes()==(pk/'site'/ext).read_bytes()
  for target in (d/'again',pk/'site'):proof(py,'-c','import qa_meta;qa_meta.main()',env=dict(os.environ,PYTHONPATH=str(target)))
  assert run([str(pk/'embed'),pref['unicode'],'-4'],cwd=d).decode()=='3.11.2 1 '+json.dumps(pref['unicode'])+' 13\n' and 'libpython3.11.so.1.0' in run(['ldd',str(pk/'embed')]).decode()
  results.append('Python packaging on system Python 3.11 (setuptools 66.1.1, pip 23.0.1, wheel 0.38.4, python3.11-dev): m103 own C source and wheel in a second project; egg_info requirement/entry point/package data and sdist; a cp311 platform wheel with independently checked RECORD; offline --find-links dependency install into a target with the exact own module bytes, console script, PYTHONPATH, importlib.metadata and pkg_resources; PEP 668 refuses system and --user installs while a venv installs offline; python3-config, Python.h version, an embedded libpython3.11 interpreter; named errors for an unsatisfiable requirement, an invalid wheel, an unknown option and a C compile error; at every phase the retained wheel reinstalls offline')
 return results
(ROOT/'public').mkdir(exist_ok=True)
checks=extra_cases()
print(json.dumps({'phase':PHASE,'checks':checks,'pass':True},ensure_ascii=False))
