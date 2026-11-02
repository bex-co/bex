"""Synthetic observations exercising copied original harness; never fixture apps."""
import base64,copy,gzip,hashlib,io,json,struct,types,zipfile,zlib
from pathlib import Path
import core as c
from harness import PATHS

HERE=Path(__file__).resolve().parent

def complete(stdout=b''):
    return {'stdout':stdout,'stderr':b'','exit_code':0,'complete':True,'bound':None,'signal':None,'elapsed_seconds':0}
def encoded(value):return json.dumps(value,ensure_ascii=False).encode()
def identity(raw):return {'bytes':len(raw),'sha256':c.sha(raw)}
def png(pixels):
    def chunk(kind,body):return struct.pack('>I',len(body))+kind+body+struct.pack('>I',zlib.crc32(kind+body)&0xffffffff)
    return b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',2,1,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(b'\0'+bytes.fromhex(pixels)))+chunk(b'IEND',b'')
def mo(reference):
    e=reference['expected'];hello=e['hello'].replace('Zoë & 李','%(name)s')
    # The original independent GNUTranslations parser is exercised on a real
    # tiny synthetic MO artifact, built here without msgfmt/original app.
    singular=e['plurals']['1'].replace('1','%(count)d');dual=e['plurals']['2'].replace('2','%(count)d');many=e['plurals']['5'].replace('5','%(count)d')
    messages={'':'Content-Type: text/plain; charset=UTF-8\nPlural-Forms: nplurals=2; plural=(n>1);\n',
              'Hello, %(name)s!':hello,'Dessert':e['dessert'],
              'There is %(count)d apple.\0There are %(count)d apples.':'\0'.join([singular,dual])}
    keys=sorted(messages);original=[x.encode() for x in keys];translated=[messages[x].encode() for x in keys]
    n=len(keys);offset=28+16*n;data=b'';a=[];b=[]
    for x in original:a.append((len(x),offset+len(data)));data+=x+b'\0'
    for x in translated:b.append((len(x),offset+len(data)));data+=x+b'\0'
    return struct.pack('<7I',0x950412de,0,n,28,28+8*n,0,0)+b''.join(struct.pack('<II',*x) for x in a+b)+data

class FakeProvider:
    def __init__(self,row,journal,source_bodies):
        self.row,self.journal,self.source_bodies=row,journal,source_bodies;self.events=[];self.mode=None
        self.build=self.after();self.artifact=None
        if row['mid']==93:
            self.artifact='<h1>Café 雪</h1><strong>bold</strong><em>emphasis</em><code>x &lt; 7</code><ul><li>alpha</li><li>beta</li></ul><a href="https://example.invalid/guide?q=qa407">Guide</a>'.encode()
            self.build['html']={'text':self.artifact.decode(),'sha256':c.sha(self.artifact)}
            self.build['before']=copy.deepcopy(self.build)
        elif row['mid'] in (95,97):
            ref=row['validation_reference'];self.artifact=png(ref['expected_rgb_hex'] if row['mid']==95 else ref['input']['expected_RGB_hex'])
            self.build['png']={**identity(self.artifact),'base64':base64.b64encode(self.artifact).decode()}
        elif row['mid']==96:
            self.artifact=mo(row['validation_reference']);self.build['catalog']={**identity(self.artifact),'GNU_MO_magic':True}
            self.build['fresh_msgfmt_compilation']={'exit':0,**identity(self.artifact)}
        elif row['mid']==98:self.artifact=row['validation_reference']['assets']['index.html'].encode()
        elif row['mid']==102:
            ref=row['validation_reference'];data=(ref['input']['unit']*ref['input']['repetitions']).encode();self.artifact=gzip.compress(data,mtime=0)
            self.build['gzip']={**identity(self.artifact),'header_hex':self.artifact[:10].hex(),'CRC32':zlib.crc32(data),'ISIZE':len(data),
                'inflated_bytes':len(data),'inflated_sha256':c.sha(data)}
        if row['mid']==107:self.build['archive']=self.archive(row['marker']+'|build')
    def after(self):
        x=self.row;r=x['validation_reference'];mid=x['mid']
        v={'marker':x['marker'],'files':copy.deepcopy(r['files']),'node':'v24.14.0','python':'3.13.12',
           'program_sha256':x['commands']['initial'].get('program_sha256'),'tools':{}}
        if mid==91:v.update(backup=r['backup'],rows=r['rows'],rows_sha256=r['rows_sha256'],restored_from_archive=True,
                           tools={n:{'status':0,'error':None,'stdout':n+' (PostgreSQL) 18.2'} for n in ('psql','pg_dump','pg_restore')})
        elif mid==93:v.update(document={k:r['document'][k] for k in ('bytes','sha256')},tools={'pandoc':{'status':0,'error':None,'stdout':'pandoc 3.9'}})
        elif mid==94:v.update(sql={k:r['sql'][k] for k in ('bytes','sha256')},rows=r['rows'],builtin_rows=r['rows'],database={'SQLite3_header':True,'sha256':'a'*64},tools={'sqlite3':{'status':0,'error':None}})
        elif mid==95:v.update(installed_pyvips='3.2.0',shared_library='CONTROL_ONLY_vips',input={k:r['input'][k] for k in ('bytes','sha256')},image={'width':2,'height':1,'bands':3,'format':'uchar','pixels_hex':r['expected_rgb_hex']},libvips='8.14.1')
        elif mid==96:v.update(po=r['po'],django=r['django'],tools={n:'CONTROL_ONLY_'+n for n in ('msgfmt','msgmerge','msguniq','xgettext')},translations=r['expected'],independent_stdlib_translations=r['expected'],msgfmt_version='msgfmt (GNU gettext-tools) 0.21')
        elif mid==97:v.update(pdf={k:r['input'][k] for k in ('bytes','sha256')},gs_path='CONTROL_ONLY_gs',gs_version='10.0')
        elif mid==98:
            manifest={'release':r['manifest']['release'],'paths':[a['path'] for a in r['manifest']['assets'] if a['publish']]}
            v.update(manifest=manifest,copied_files={n:identity(r['assets'][n].encode()) for n in manifest['paths']},tools={n:{'status':0,'error':None} for n in ('jq','rsync')})
        elif mid==99:v.update(source_ts_sha256=r['source_ts_sha256'],compiler={'status':0,'error':None,'version':'Version '+r['typescript']},input=r['rows'],result=r['expected'],compiled_js_sha256='a'*64)
        elif mid==100:v.update(interface_sha256=r['interface_sha256'],program_sha256=r['program_sha256'],swig_path='CONTROL_ONLY_swig',swig_version='SWIG Version 4.1',result=r['expected'],unicode=r['unicode'],extension={'bytes':123,'sha256':'a'*64},generated_wrapper_sha256='b'*64,generated_python_sha256='c'*64)
        elif mid==101:v.update(sources=r['sources'],webpack_probe={'status':0,'error':None},compiler={'version':r['control_compiler'],'errors':0},input=r['rows'],result=r['expected'],bundle={'bytes':123,'sha256':'a'*64})
        elif mid==102:v.update(program_sha256=r['program_sha256'],input={k:r['input'][k] for k in ('bytes','sha256')},build_compressor_version='pigz 2.6')
        elif mid==103:v.update(fixture_sha256=r['fixture_sha256'],python='3.11.12',distribution={'name':'qa-native-helper','version':'1.0'},tools={'setuptools':True,'pip':True,'Python_h':True},wheel={'label':r['unicode'],'installed_module_sha256':x['expected_module_sha']},result=r['expected'],unicode=r['unicode'],extension={'bytes':123,'sha256':'a'*64})
        elif mid==107:v.update(source=r['files'],zip='This is Zip 3.0',unzip='UnZip 6.0',unzip_test='No errors detected')
        elif mid==222:v.update(python='3.14.8',brotli={'patched':True,'minimum_fixed':'1.0.9-2+deb12u1'})
        return v
    def archive(self,label):
        expected={**self.source_bodies,'greeting.txt':('Crème brûlée & 李\n'*7).encode(),'two words.bin':bytes((i*31+17)&255 for i in range(8192)),'empty.txt':b'','request.txt':label.encode()}
        stream=io.BytesIO()
        with zipfile.ZipFile(stream,'w',compression=zipfile.ZIP_DEFLATED) as z:
            for name in sorted(expected):z.writestr(name,expected[name])
        raw=stream.getvalue();return {**identity(raw),'base64':base64.b64encode(raw).decode()}
    def persist(self):self.events.append(('persist',))
    def reference(self,url,reference):self.events.append(('reference',url));return self.source_bodies[url.rsplit('/',1)[1]]
    def native(self,client,args):
        self.events.append(('native',client,tuple(args)))
        if args[0]=='deploys':
            history=[{'id':'dep-first123','status':'live','commit':{'id':self.row['primary_reference']['commit']}}]
            if self.mode=='wrong-source':history[0]['commit']['id']='f'*40
            return complete(encoded(history))
        if args[args.index('--type')+1]=='build':
            value=copy.deepcopy(self.build)
            if self.mode=='build-proof':value['marker']='CONTROL_ONLY_WRONG'
            return complete(encoded([{'message':self.row['marker']+' '+json.dumps(value,ensure_ascii=False)}]))
        messages=[self.request_path(i) for i in range(3)]
        if self.mode=='app-correlation':messages.pop()
        return complete(encoded([{'message':s} for s in messages]))
    def request_path(self,i):
        x=self.row
        return PATHS[x['mid']][i]+('?qa=' if x['mid']==107 else '?qa'+str(x['sweep'])+'=')+x['nonce']+'-'+str(i+1)
    def http(self,argv,row,path,index):
        self.events.append(('HTTP',path));mid=row['mid'];build=copy.deepcopy(self.build);runtime=copy.deepcopy(build)
        if self.mode=='wrong-marker':runtime['marker']='CONTROL_ONLY_WRONG'
        runtime['request_url']=runtime['request_path']=path
        if self.mode=='database' and mid==94:runtime['database']['sha256']='b'*64
        if mid==107:
            nonce=row['nonce']+'-'+str(index+1)
            runtime['archive']=self.archive(row['marker']+'|'+nonce)
            if index==1:body=base64.b64decode(runtime['archive']['base64']);ctype='application/zip'
            else:body=encoded({'marker':row['marker'],'nonce':nonce,'built':build,'runtime':runtime,'boot':'CONTROL_ONLY_ONE_BOOT'});ctype='application/json'
        elif index==2 and mid in (93,95,96,97,98,102):
            body=self.artifact;ctype={93:'text/html; charset=utf-8',95:'image/png',96:'application/octet-stream',97:'image/png',98:'text/html; charset=utf-8',102:'application/gzip'}[mid]
        else:body=encoded({'build':build,'runtime':runtime,'node':'v24.14.0'});ctype='application/json'
        if self.mode=='artifact' and index==2:body=b'CONTROL_ONLY_WRONG_ARTIFACT'
        footer=b'\n200'+(b'' if mid in (91,94,222) else b' '+ctype.encode())
        return complete(body+footer)
    def api(self,method,path,body=None):
        self.events.append(('API',method,path));row=self.row
        if path.endswith('/env-vars'):
            data=[{'envVar':{'key':k,'value':v}} for k,v in row['env'].items()]
            if self.mode=='env':data.append({'envVar':{'key':'EXTRA','value':'CONTROL_ONLY'}})
        elif path=='/graphql':
            data=copy.deepcopy(row['create_view']);details=data.pop('serviceDetails');data.update(details);data.update(details['envSpecificDetails']);data={'data':{'service':data}}
        elif path=='/mcp':data={'result':{'structuredContent':copy.deepcopy(row['create_view'])}}
        else:data=copy.deepcopy(row['create_view'])
        raw=encoded(data);return {'status':200,'body':raw,'raw':raw,'transport':complete()}
