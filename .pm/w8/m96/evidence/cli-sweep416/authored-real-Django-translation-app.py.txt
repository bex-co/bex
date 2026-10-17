import gettext,hashlib,io,json,os,platform,shutil,subprocess,sys
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import django
reference=json.loads(Path('qa-gettext-reference.json').read_text());marker=json.loads(Path('qa-gettext-marker.json').read_text());program_sha=hashlib.sha256(Path(__file__).read_bytes()).hexdigest();catalog=Path('locale/fr/LC_MESSAGES/django.mo')
def digest(path):
 b=Path(path).read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
def before():
 files={n:digest(n) for n in reference['files']};assert files==reference['files'];po=digest('locale/fr/LC_MESSAGES/django.po');assert po==reference['po']
 return {'marker':marker,'python':platform.python_version(),'django':django.get_version(),'files':files,'po':po,'program_sha256':program_sha,'tools':{n:shutil.which(n) for n in ['msgfmt','msgmerge','msguniq','xgettext']}}
if sys.argv[1]=='before':print('QA_GETTEXT_BEFORE '+json.dumps(before(),separators=(',',':')),flush=True);raise SystemExit(0)
os.environ.setdefault('DJANGO_SETTINGS_MODULE','qa_gettext_settings');django.setup()
from django.utils.translation import gettext as dg,ngettext as dn,override

def translate():
 with catalog.open('rb') as f:g=gettext.GNUTranslations(f)
 def values(t,n):return {'hello':t('Hello, %(name)s!')%{'name':'Zoë & 李'},'dessert':t('Dessert'),'plurals':{str(i):n('There is %(count)d apple.','There are %(count)d apples.',i)%{'count':i} for i in [0,1,2,5]}}
 std=values(g.gettext,g.ngettext)
 with override('fr'):actual=values(dg,dn)
 assert actual==std==reference['expected'];mo=catalog.read_bytes();assert mo[:4].hex() in ['de120495','950412de'];compiled=subprocess.run(['msgfmt','--check-format','-o','-','locale/fr/LC_MESSAGES/django.po'],capture_output=True,timeout=5);assert compiled.returncode==0 and compiled.stdout==mo;fresh=gettext.GNUTranslations(io.BytesIO(compiled.stdout));assert values(fresh.gettext,fresh.ngettext)==reference['expected'];r=subprocess.run(['msgfmt','--version'],capture_output=True,text=True,timeout=5);assert r.returncode==0
 return {**before(),'translations':actual,'independent_stdlib_translations':std,'catalog':{**digest(catalog),'GNU_MO_magic':True},'fresh_msgfmt_compilation':{'exit':compiled.returncode,'bytes':len(compiled.stdout),'sha256':hashlib.sha256(compiled.stdout).hexdigest()},'msgfmt_version':r.stdout.splitlines()[0]}
if sys.argv[1]=='build':
 v=translate();Path('qa-gettext-build.json').write_text(json.dumps(v));print('QA_GETTEXT_AFTER '+json.dumps(v,separators=(',',':')),flush=True)
else:
 build=json.loads(Path('qa-gettext-build.json').read_text())
 class Handler(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   v=translate();v['request_url']=self.path;path=self.path.split('?',1)[0]
   if path not in ['/','/translations','/catalog.mo']:self.send_error(404);return
   data=catalog.read_bytes() if path=='/catalog.mo' else json.dumps({'build':build,'runtime':v},ensure_ascii=False).encode();self.send_response(200);self.send_header('Content-Type','application/octet-stream' if path=='/catalog.mo' else 'application/json');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data);print('QA_GETTEXT_HTTP '+json.dumps({'marker':marker,'path':self.path,'catalog_sha256':v['catalog']['sha256']},separators=(',',':')),flush=True)
 ThreadingHTTPServer(('0.0.0.0',int(os.environ.get('PORT','10000'))),Handler).serve_forever()
