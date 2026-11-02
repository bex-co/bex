"""Original proof functions in a private dependency-bound namespace.

Only generated reviewed harness definitions are compiled; never original
hosted/naming modules, fixture applications, shell commands or original modules.
"""
import copy,datetime,hashlib,io,json,types,re
from pathlib import Path
import core as c
from catalog import KEYS

HERE=Path(__file__).resolve().parent
PATHS={91:('/restore','/','/restore'),93:('/render','/','/document'),94:('/rows','/','/rows'),
       95:('/render','/','/png'),96:('/translations','/','/catalog.mo'),97:('/render','/','/png'),
       98:('/manifest','/','/copied-index'),99:('/typed','/','/typed/enum'),100:('/native','/','/native/weighted'),
       101:('/bundled','/','/bundled/modules'),102:('/inspect','/','/asset.gz'),103:('/native','/','/native/combined'),
       107:('/','/download','/inspect'),222:('/proof','/','/brotli')}

def functions(name,globals):
    path=HERE/'derived'/name
    expected=json.loads((HERE/'DERIVED-FILES.json').read_text())[name]
    raw=path.read_bytes();c.require(len(raw)==expected['bytes'] and c.sha(raw)==expected['sha256'],'derived reviewed harness bytes differ')
    namespace=dict(globals,__name__='private_original28_derived_'+name.replace('.','_'))
    exec(compile(raw,str(path),'exec'),namespace)
    return namespace

def byte_summaries(value):
    if type(value) is bytes:return {'bytes':len(value),'sha256':c.sha(value)}
    if isinstance(value,dict):return {key:byte_summaries(item) for key,item in value.items()}
    if isinstance(value,list):return [byte_summaries(item) for item in value]
    return value

class Context:
    def __init__(self,row,provider,fixture):
        self.row,self.provider,self.fixture=row,provider,Path(fixture)
        self.request_index=0
        self.methods=functions('hosted_proofs.py',{'ctx':self,'Path':Path,'json':json,'hashlib':hashlib,
            'subprocess':self,'WS':c.WORKSPACE,'re':re})
    def now(self):return datetime.datetime.now(datetime.timezone.utc).isoformat()
    def save(self):self.provider.persist()
    def own(self,x):
        c.require(x is self.row,'proof row outside fixed owned case');self.provider.journal.own(x['id'])
    def validators(self,x):
        c.require(x is self.row,'validator case differs')
        return functions('validators_'+str(x['mid'])+'.py',{'reference':x['validation_reference'],'hashlib':hashlib,
            'json':json,'expected_module_sha':x.get('expected_module_sha')})
    def helper(self,path):
        path=Path(path);c.require(path.parent==self.fixture,'helper outside private case fixture')
        if path.name=='sweep407-html.py':return functions('html.py',{})
        if path.name=='sweep406-png.py':return functions('png.py',{})
        raise c.Unproved('unknown private independent artifact helper')
    def http_functions(self,x):
        c.require(x is self.row,'HTTP proof case differs')
        return functions('http_'+str(x['mid'])+'.py',{'subprocess':self,'runpy':types.SimpleNamespace(run_path=self.helper),
            'urllib':types.SimpleNamespace(request=types.SimpleNamespace(urlopen=self.urlopen))})
    def urlopen(self,url,timeout):
        x=self.row;reference=x['validation_reference']
        allowed={'https://raw.githubusercontent.com/git-fixtures/basic/'+reference['commit']+'/'+name for name in reference['files']}
        c.require(x['mid']==107 and timeout==25 and url in allowed,'reference GET outside frozen public source')
        return io.BytesIO(self.provider.reference(url,reference))
    def request_path(self,index):
        x=self.row
        if x['mid']==107:return PATHS[107][index]+'?qa='+x['nonce']+'-'+str(index+1)
        return PATHS[x['mid']][index]+'?qa'+str(x['sweep'])+'='+x['nonce']+'-'+str(index+1)
    def run(self,argv,capture_output,timeout):
        c.require(capture_output is True and timeout==20 and self.request_index<3,'original finite HTTP capture contract differs')
        path=self.request_path(self.request_index);x=self.row
        footer='\n%{http_code}' if x['mid'] in (91,94,222) else '\n%{http_code} %{content_type}'
        exact=['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w',footer,
               x['create_view']['serviceDetails']['url']+path]
        c.require(argv==exact,'original independent request bytes/path/order differ')
        result=self.provider.http(argv,x,path,self.request_index);c.completed(result)
        self.request_index+=1
        return types.SimpleNamespace(returncode=result['exit_code'],stdout=result['stdout'],stderr=result['stderr'])
    def cli(self,args,client):
        x=self.row;c.require(client==x['client'],'native proof client differs');self.own(x)
        allowed=[['deploys','list',x['id'],'--confirm','-o','json']]
        for kind,text in [('build',x['marker']),('app',x['nonce'])]:
            allowed.append(['logs','--resources',x['id'],'--type',kind,'--text',text,'--limit','100','--confirm','-o','json'])
        c.require(args in allowed,'native proof command differs from original')
        result=self.provider.native(client,args);c.completed(result)
        return {'exit':result['exit_code'],'stdout':result['stdout'].decode(),'stderr':result['stderr'].decode(),
                'capture':{k:v for k,v in result.items() if k not in ('stdout','stderr')}}
    def history(self,x):
        self.own(x);r=self.cli(['deploys','list',x['id'],'--confirm','-o','json'],x['client']);history=json.loads(r['stdout'])
        c.require(isinstance(history,list) and 0<len(history)<=20,'complete bounded original first deployment history missing')
        self.provider.journal.record_children(x['id'],history)
        x.setdefault('history_receipts',[]).append({'at':self.now(),'native':r});x['deploy_children']=history
        self.save();return history
    def api(self,method,path,body=None):
        self.own(self.row);result=self.provider.api(method,path,body)
        c.completed(result['transport'])
        return {'HTTP':str(result['status']),'response':result['body'].decode(),
                'capture':byte_summaries({k:v for k,v in result.items() if k not in ('body','raw')})}
    def proof(self):
        result=self.methods['proof'](self.row)
        c.require(self.request_index==3,'not all3 original independent requests actually captured')
        return result
    def shared(self):return self.methods['shared'](self.row)
