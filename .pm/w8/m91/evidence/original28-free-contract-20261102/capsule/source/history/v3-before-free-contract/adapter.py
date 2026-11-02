"""Original28 concrete adapter SOURCE. Actual gates must be supplied and reviewed.

Nothing runs on import. Source checkout leaves ADOPTION-CONTRACT.json null;
execute refuses that state before auth. No original module is imported/executed.
"""
import argparse,copy,datetime,hashlib,json,os,re,secrets,signal,sys,threading,time
from pathlib import Path
from urllib.parse import urlsplit,parse_qs,urlencode
import core as c
import readiness
import evidence
from catalog import Catalog,Journal,KEYS,validate_post,expected_secret_files
from harness import Context
from opaque import OpaqueTransport,NativeRelay,PrivateConfigError,safe_error,now,digest,read_json,command_receipt,API
from host import Budget,Runner,Stop,START_FREE,free_bytes

HERE=Path(__file__).resolve().parent
REPO=Path('/Users/tianpan/projects/bex7')

class Adapter(OpaqueTransport):
    def __init__(self,root,capsule_path,contract_path,source=HERE,budget=None,runner=None):
        self.root,self.source,self.private=Path(root),Path(source),Path(root)/'private'
        self.capsule_path,self.contract_path=Path(capsule_path),Path(contract_path)
        self.capsule_sha,self.contract_sha=digest(self.capsule_path),digest(self.contract_path)
        self.capsule,self.contract=read_json(self.capsule_path),read_json(self.contract_path)
        self.catalog=Catalog((self.source/'original-inputs/prepared-fixtures.json').read_bytes(),(self.source/'EMISSION-MAP.json').read_bytes())
        self.inputs=read_json(self.source/'CLIENTS-AND-REUSE.json')
        self.budget=budget or Budget(self.root);self.runner=runner or Runner(self.budget)
        self.sequence=0;self.lock=threading.RLock();self.tokens=set();self.errors=[];self.results={}
        self.data={'scope':'original28 fixed once catalog','workspace':c.WORKSPACE,'baseline_ids':None,'resources':[],'owner_nonce':secrets.token_hex(8)}
        self.journal=None;self.auth_started=False;self.auth_completed=False;self.active_relays=[];self.handlers={};self.finishing=False;self.phase='initialization'
        self.relay_type=NativeRelay
    def setup(self):
        for name in ('private','source','scratch','evidence'):(self.root/name).mkdir(mode=0o700,exist_ok=True)
        for name in ('auth.py','cleanup-auth.py','client_primitives.py'):
            body=(self.source/'reuse-source'/name).read_bytes()
            c.require(c.sha(body)==self.inputs['source_only_reuse'][name]['sha256'],'pinned auth helper differs')
            self.budget.write('private/'+name,body)
        for client,relative in [('bex','release/bex'),('render','render-pin')]:
            row=self.inputs['clients'][client];binary=Path(row['resolved_public_binary_path'])
            c.require(binary.is_file() and digest(binary)==row['sha256'],'genuine original client bytes differ')
            target=self.private/relative;target.parent.mkdir(mode=0o700,parents=True,exist_ok=True);target.symlink_to(binary)
        self.record('inputs.json',{'specs_sha256':c.sha((self.source/'original-inputs/prepared-fixtures.json').read_bytes()),
            'emission_map_sha256':c.sha((self.source/'EMISSION-MAP.json').read_bytes()),'capsule_sha256':self.capsule_sha,
            'contract_sha256':self.contract_sha,'clients':self.inputs['clients'],'original_ledger_auth_reused':False})
    def proof_reader(self,path):
        file=Path(path)
        c.require(file.is_absolute() and str(file).startswith('/private/tmp/') and not file.is_symlink()
                  and file.is_file() and file.stat().st_size<=2*1024*1024,'bounded private retained proof required')
        return file.read_bytes()
    def git(self,action,left,right):
        if action=='ancestor':
            result=self.runner.capture(['git','-C',str(REPO),'merge-base','--is-ancestor',left,right],timeout=10,limit=65536)
            c.completed(result);return True
        if action=='changes':
            result=self.runner.capture(['git','-C',str(REPO),'diff','--name-status','--no-renames','-z',left,right,'--'],timeout=10,limit=2*1024*1024)
            c.completed(result);pieces=result['stdout'].split(b'\0')
            c.require(pieces[-1]==b'' and len(pieces[:-1])%2==0,'complete actual Git source change-set missing')
            rows=[]
            for index in range(0,len(pieces)-1,2):
                status,path=pieces[index].decode(),pieces[index+1].decode()
                c.require(status in ('A','D','M','T') and path and not Path(path).is_absolute() and '..' not in Path(path).parts,
                          'actual Git change-set status/path unsupported')
                rows.append({'status':status,'path':path})
            return rows
        c.require(action=='blob','unknown source observation')
        result=self.runner.capture(['git','-C',str(REPO),'show',left+':'+right],timeout=10,limit=2*1024*1024);c.completed(result)
        tree=self.runner.capture(['git','-C',str(REPO),'ls-tree',left,'--',right],timeout=10,limit=65536);c.completed(tree)
        match=re.fullmatch(r'(100644|100755) blob ([a-f0-9]{40})\t'+re.escape(right)+r'\n',tree['stdout'].decode())
        c.require(match is not None,'actual Git mode/blob missing')
        return result['stdout'],match[1],match[2]
    def check_ready(self):
        c.require(digest(self.capsule_path)==self.capsule_sha and digest(self.contract_path)==self.contract_sha,'reviewed admission bytes changed')
        result=readiness.admit(self.capsule,self.contract,now(),evidence.Access(self.proof_reader),self.git)
        self.record(self.next_name('admission')+'.json',result);return result
    def require_legal_window(self):
        # Root-supplied retained pre-auth coverage; refresh after own auth and
        # before EVERY intent. Source prep cannot fabricate this observation.
        free=self.capsule.get('preauth_free',{})
        return c.gate_free(free.get('owner',{}),free.get('usage',{}),free.get('limits',{}),now())
    def allowed_api(self,method,path,body,cleanup):
        parsed=urlsplit(path);query=parse_qs(parsed.query)
        c.require(not parsed.scheme and not parsed.netloc and not parsed.fragment and '%' not in parsed.path,'qualified relative API path required')
        fixed='/v1/owners/'+c.WORKSPACE
        if method=='GET':
            if parsed.path in (fixed,fixed+'/limits'):return not query
            if parsed.path=='/v1/usage':return query=={'ownerId':[c.WORKSPACE]}
            if parsed.path in ('/v1/services','/v1/env-groups','/v1/projects','/v1/blueprints'):
                return query.get('ownerId')==[c.WORKSPACE] and query.get('limit')==['100'] and set(query)<={'ownerId','limit','cursor'} and all(len(v)==1 and len(v[0])<=4096 for v in query.values())
            for row in self.data['resources']:
                if not row.get('id'):continue
                sid=row['id'];base='/v1/services/'+sid
                if parsed.path in (base,base+'/env-vars'):
                    self.journal.own(sid);return not query
                if parsed.path==base+'/deploys':self.journal.own(sid);return query=={'limit':['20']}
                if parsed.path in {base+'/deploys/'+child for child in row['children']}:self.journal.own(sid);return not query
        if method=='DELETE' and cleanup and not query:
            sid=parsed.path.removeprefix('/v1/services/');self.journal.own(sid);return parsed.path=='/v1/services/'+sid
        if method=='POST' and path=='/graphql':
            return isinstance(body,dict) and set(body)=={'query'} and body['query'] in {
                'query Acceptance {service(id:'+json.dumps(r['id'])+'){id name repo branch runtime plan buildCommand startCommand preDeployCommand autoDeploy}}'
                for r in self.data['resources'] if r.get('id')}
        if method=='POST' and path=='/mcp':
            params=(body or {}).get('params',{});sid=params.get('arguments',{}).get('serviceId');row=self.journal.own(sid)
            return body=={'jsonrpc':'2.0','id':row['sweep'],'method':'tools/call','params':{'name':'get_service','arguments':{'serviceId':sid,'workspaceId':c.WORKSPACE}}}
        return False
    def provider(self,row):return Provider(self,row)
    def public(self,row,argv,cleanup=False):
        self.journal.own(row['id']);url=argv[-1];base=row['create_view']['serviceDetails']['url'];parsed=urlsplit(base)
        c.require(parsed.scheme=='https' and parsed.netloc==row['name']+'.onbex.co' and not parsed.path and not parsed.query
                  and url.startswith(base+'/'),'public request outside own actual201 URL')
        command=[*argv[:-1],'--max-filesize',str(c.MAX_BODY),'--connect-timeout','5','--proto','=https',url]
        result=self.runner.capture(command,timeout=20,limit=c.MAX_BODY+65536,cleanup=cleanup)
        self.record('outputs/'+self.next_name('original-public')+'.json',command_receipt(result,['curl','<exact owned URL>']),cleanup)
        return result
    def reference(self,url,reference):
        allowed={'https://raw.githubusercontent.com/git-fixtures/basic/'+reference['commit']+'/'+name for name in reference['files']}
        c.require(url in allowed,'frozen exact public reference URL differs')
        result=self.runner.capture(['curl','--disable','--fail','--silent','--show-error','--max-time','25','--max-filesize','65536','--proto','=https',url],timeout=30,limit=65536)
        c.completed(result);return result['stdout']
    def cleanup_row(self,row):
        if row['state'] in ('deleted','not-created'):return True
        if not row.get('id'):
            if not row['forwarded_once']:row['state']='not-created';self.persist(self.data);return True
            self.errors.append('unknown-forwarded-create:'+str((row['mid'],row['client'])));return False
        sid=row['id'];self.journal.own(sid)
        try:self.deployments(row,cleanup=True)
        except BaseException as error:self.errors.append(safe_error('cleanup-child-inventory',error))
        self.api('DELETE','/v1/services/'+sid,cleanup=True)
        deadline=time.monotonic()+180
        while time.monotonic()<deadline:
            detail=self.api('GET','/v1/services/'+sid,cleanup=True)
            children={child:self.api('GET','/v1/services/'+sid+'/deploys/'+child,cleanup=True)['status'] for child in row['children']}
            if detail['status']==404 and all(v==404 for v in children.values()):
                argv=['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code}',row['create_view']['serviceDetails']['url']+'/']
                result=self.public(row,argv,cleanup=True);c.completed(result)
                if result['stdout'].rpartition(b'\n')[2]==b'404':self.journal.deleted(sid,404,children,404);return True
            time.sleep(min(1,max(0,deadline-time.monotonic())))
        self.errors.append('exact-known-SID-child-URL-absence-unproved:'+sid);return False
    def finish(self,failure):
        self.finishing=True
        for relay in list(self.active_relays):
            try:relay.close()
            except BaseException as error:self.errors.append(safe_error('relay-cleanup',error))
        try:self.errors.extend(self.runner.stop_all())
        except BaseException as error:self.errors.append(safe_error('owned-group-cleanup',error))
        if self.journal:
            for row in self.data['resources']:
                try:self.cleanup_row(row)
                except BaseException as error:self.errors.append(safe_error('known-row-cleanup',error))
            try:
                after=self.collect(cleanup=True);checks=c.compare_cleanup(self.data['baseline'],after,self.data['resources'])
                self.record('baseline-after.json',{'snapshot':after,'checks':checks},cleanup=True)
            except BaseException as error:self.errors.append(safe_error('final-baseline-quota',error))
        try:self.logout()
        except BaseException as error:self.errors.append(safe_error('own-auth-cleanup',error))
        for signum,handler in self.handlers.items():signal.signal(signum,handler)
        monitor=self.budget.close()
        if monitor:self.errors.append('final-monitor:'+monitor)
        # Last writes also use cleanup reserve; any resulting error preventsPASS.
        passed=failure is None and not self.errors and len(self.results)==28 and all(v['state']=='PASS' for v in self.results.values())
        report={'PASS':passed,'failure':failure,'cleanup_errors':list(self.errors),'case_results':self.results,
                'resources':[{'mid':r['mid'],'client':r['client'],'id':r.get('id'),'state':r['state']} for r in self.data['resources']]}
        self.record('RESULT.json',report,cleanup=True)
        if self.errors:report['PASS']=False;report['cleanup_errors']=list(self.errors)
        return report
    def execute(self):
        failure=None
        try:
            # Deliberately before observer/setup/auth: absent adoption refuses.
            c.require(isinstance(self.contract,dict) and isinstance(self.contract.get('adoption'),dict),'actual reviewed adoption contract absent before auth')
            self.set_handlers();self.budget.start();self.setup();self.phase='auth';self.auth();self.phase='baseline-before'
            before=self.collect();c.compare_cleanup(before,before,[])
            self.data.update(baseline=before,baseline_ids=before['ids']);self.journal=Journal(self.data,self.persist);self.persist(self.data)
            for key in KEYS:
                self.phase='case-'+str(key);self.budget.check()
                current=self.collect();c.compare_cleanup(before,current,self.data['resources'])
                attempted={(r['mid'],r['client']) for r in self.data['resources']}
                spec=self.catalog.spec(key,now(),attempted);c.require(spec['name'] not in current['names'],'unused display name already present')
                admitted=self.check_ready();free=self.free_gate();row=self.journal.intent(spec,admitted,free,current['ids'])
                expected_secret_files(spec,verify_original=True)
                with self.relay_type(self,spec,row) as relay:result=self.native(spec['client'],spec['create_args'],relay.host,timeout=48)
                row['create_result']={'exit':result['exit_code'],'capture':{k:v for k,v in result.items() if k not in ('stdout','stderr')}}
                c.completed(result);c.require(row.get('HTTP')==201 and row['state']=='created' and row.get('wire_request'),'actual complete native201 journey missing')
                provider=self.provider(row);ctx=Context(row,provider,self.root/'scratch'/str(spec['mid']))
                deadline=time.monotonic()+300
                while True:
                    history=ctx.history(row)
                    c.require(len(history)==1 and history[0].get('commit',{}).get('id')==spec['primary_reference']['commit'],'first deploy/source changed')
                    if history[0]['status']=='live':break
                    c.require(history[0]['status'] not in ('build_failed','update_failed','canceled') and time.monotonic()<deadline,'original first deploy did not Live')
                    time.sleep(2)
                proof=ctx.proof();shared=ctx.shared()
                self.results[str(key)]={'state':'PASS','proof':proof,'shared':shared};self.persist(self.data)
                c.require(self.cleanup_row(row) and not self.errors,'exact case cleanup failed; no further intent')
                after=self.collect();c.compare_cleanup(before,after,self.data['resources'])
        except BaseException as error:failure=safe_error(self.phase,error)
        return self.finish(failure)

class Provider:
    def __init__(self,owner,row):self.owner,self.row,self.journal=owner,row,owner.journal
    def persist(self):self.owner.persist(self.owner.data)
    def native(self,client,args):return self.owner.native(client,args,API+'/v1/')
    def api(self,method,path,body=None):return self.owner.api(method,path,body)
    def http(self,argv,row,path,index):return self.owner.public(row,argv)
    def reference(self,url,reference):return self.owner.reference(url,reference)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=['execute']);parser.add_argument('--capsule',required=True,type=Path)
    parser.add_argument('--contract',required=True,type=Path);parser.add_argument('--capsule-sha',required=True);parser.add_argument('--contract-sha',required=True)
    parser.add_argument('--source-manifest-sha',required=True);args=parser.parse_args()
    c.require(not sys.flags.optimize,'original harness assert oracles require ordinary Python')
    c.require(digest(HERE/'MANIFEST.json')==args.source_manifest_sha,'approved source manifest differs')
    for name,expected in read_json(HERE/'MANIFEST.json').items():
        file=HERE/name;c.require(not Path(name).is_absolute() and '..' not in Path(name).parts and not file.is_symlink()
            and file.is_file() and file.stat().st_size==expected['bytes'] and digest(file)==expected['sha256'],'reviewed source payload differs')
    c.require(digest(args.capsule)==args.capsule_sha and digest(args.contract)==args.contract_sha,'Root-approved admission SHA differs')
    contract=read_json(args.contract);c.require(isinstance(contract,dict) and isinstance(contract.get('adoption'),dict),'actual adoption absent; refuse before auth')
    c.require(free_bytes('/private/tmp')>=START_FREE,'bounded host runtime does not fit')
    os.umask(0o077);root=Path('/private/tmp')/('bex-w8-original28-live-'+secrets.token_hex(8));root.mkdir(mode=0o700)
    report=Adapter(root,args.capsule,args.contract).execute()
    print(json.dumps({'root':str(root),'PASS':report['PASS'],'failure':report['failure'],'cleanup_errors':report['cleanup_errors']}));raise SystemExit(0 if report['PASS'] else 2)

if __name__=='__main__':main()
