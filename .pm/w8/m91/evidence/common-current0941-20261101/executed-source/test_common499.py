"""Pure source/REST/fake-Docker controls; no artifact, Docker or crypto actions."""
import ast
import copy
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import types
import unittest
from unittest.mock import patch

import common499_review as r
import linux_cosign as lc
import run_review as outer

HERE=Path(__file__).resolve().parent
OWNER='0123456789abcdef'

def body(value):return (json.dumps(value,indent=2)+'\n').encode()
def descriptor(path):return {'path':str(path),'bytes':path.stat().st_size,'sha256':r.digest(path.read_bytes())}
def plan():return json.loads((HERE/'PLAN.json').read_bytes())
def source_driver():
    base=r.base_module();return base,base.import_pinned(r.BASE/'retained/loopx-w8-native-review-v11.py',base.PINNED['loopx-w8-native-review-v11.py'])

class RESTControls(unittest.TestCase):
    def test_actual_retained_REST_fields_pass_the_unchanged_candidate_validator(self):
        p=plan();base=r.validate_plan(p);mapped=r.rest_projection(p);result=base.candidate_job(p,mapped)
        self.assertEqual(result['job_id'],r.JOB);self.assertEqual(result['primary_build_conclusion'],'success')
        self.assertEqual(result['recovery_selection_conclusion'],'skipped')
    def mutated(self,mutator):
        with tempfile.TemporaryDirectory(prefix='v11-rest-control-',dir='/private/tmp') as name:
            root=Path(name).resolve();p=plan();runs=json.loads(r.read_ref(p['ci_inputs']['runs']));jobs=json.loads(r.read_ref(p['ci_inputs']['jobs']))
            mutator(runs,jobs)
            for role,obj in [('runs',runs),('jobs',jobs)]:
                path=root/(role+'.json');path.write_bytes(body(obj));p['ci_inputs'][role]=descriptor(path)
            return r.rest_projection(p),p
    def test_typed_actual_run_job_attempt_source_workflow_and_duplicate_negatives(self):
        def run_obj(x):return next(z for z in x['workflow_runs'] if z['id']==r.RUN)
        def job_obj(x):return next(z for z in x['jobs'] if z['id']==r.JOB)
        cases=[lambda x,y:run_obj(x).update(head_sha='0'*40),lambda x,y:run_obj(x).update(run_attempt=True),
               lambda x,y:run_obj(x).update(path='.github/workflows/deploy.yml'),lambda x,y:run_obj(x).update(status='in_progress'),
               lambda x,y:run_obj(x).update(conclusion='failure'),lambda x,y:job_obj(y).update(id=True),
               lambda x,y:job_obj(y).update(run_attempt=2),lambda x,y:job_obj(y).update(head_sha='0'*40),
               lambda x,y:y['jobs'].append(copy.deepcopy(job_obj(y))),lambda x,y:x.update(total_count=True)]
        for index,case in enumerate(cases):
            with self.subTest(index=index),self.assertRaises(lc.Refusal):self.mutated(case)
    def test_missing_failed_or_duplicate_required_steps_still_refuse(self):
        for mode in ('missing','failed','duplicate'):
            def mutate(runs,jobs):
                steps=next(j for j in jobs['jobs'] if j['id']==r.JOB)['steps'];step=next(s for s in steps if s['name']=='scan arm64 candidate')
                if mode=='missing':steps.remove(step)
                elif mode=='failed':step['conclusion']='failure'
                else:steps.append(copy.deepcopy(step))
            with self.subTest(mode=mode):
                mapped,p=self.mutated(mutate)
                with self.assertRaises(RuntimeError):r.base_module().candidate_job(p,mapped)
    def test_actual_metadata_custody_tamper_and_unknown_ref_and_null_sizing_refuse(self):
        p=plan();p['ci_inputs']['jobs']['sha256']='0'*64
        with self.assertRaises(lc.Refusal):r.rest_projection(p)
        for ref in (None,'ghcr.io/bex-co/bex-cnb-run@sha256:'+'0'*64):
            p=plan();p['ref']=ref
            with self.subTest(ref=ref),self.assertRaises((lc.Refusal,RuntimeError)):r.validate_plan(p)
        with self.assertRaisesRegex(lc.Refusal,'sizing'):r.validate_plan(plan(),resources=True)
    def test_stale_other_image_sizing_refuses_even_with_matching_file_custody(self):
        with tempfile.TemporaryDirectory(prefix='v11-size-control-',dir='/private/tmp') as name:
            p=plan();p['local_resource_policy']={'ref':r.REF};p['local_resource_sizing']={}
            for role in ('layers','bundles'):
                path=Path(name)/(role+'.json');path.write_bytes(body({'ref':r.REF if role=='layers' else r.REF.replace('0941c2','000000'),'candidate_approved':False}));p['local_resource_sizing'][role]=descriptor(path)
            with self.assertRaises(lc.Refusal):r.validate_plan(p,resources=True)

class ArgsControls(unittest.TestCase):
    def test_exact_inner_policy_matches_original_command_expression_for_all_three_controls(self):
        tree=ast.parse((r.BASE/'retained/loopx-w8-native-review-v11.py').read_text())
        cls=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='Review')
        fn=next(n for n in cls.body if isinstance(n,ast.FunctionDef) and n.name=='verify_command')
        expr=next(n.value for n in fn.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='args' for t in n.targets))
        row={'path':'/CONTROL_ONLY/bundle.json','predicate_type':'https://sigstore.dev/cosign/sign/v1'}
        for source,digest in [(None,None),('0'*40,None),(None,'sha256:'+'0'*64)]:
            namespace={'v':types.SimpleNamespace(COSIGN=Path('/CONTROL_ONLY/cosign')),'row':row,'source_sha':source,'digest':digest,'self':types.SimpleNamespace(digest=r.REF.split('@')[1],a=types.SimpleNamespace(sha=r.SOURCE))}
            original=eval(compile(ast.Expression(expr),'ORIGINAL_ARGS_ONLY','eval'),namespace)
            original[0]='/usr/local/bin/cosign';original[original.index('--bundle')+1]='/bundle.json'
            self.assertEqual(original,r.inner_command(row,source or r.SOURCE,(digest or r.REF.split('@')[1]).split(':')[1]))
    def test_actual_outer_command_is_Linux_owned_bounded_and_not_a_Darwin_projection(self):
        inner=r.inner_command({'predicate_type':'CONTROL_ONLY'},r.SOURCE,r.REF.split('@')[1].split(':')[1])
        args=lc.docker_command(OWNER,Path('/CONTROL_ONLY/bundle.json'),Path('/CONTROL_ONLY/tuf'),Path('/CONTROL_ONLY/docker'),inner)
        self.assertEqual(args[:2],['docker','run']);self.assertIn('--pull=never',args);self.assertNotIn('--rm',args)
        self.assertEqual(args[args.index('--memory')+1],'512m');self.assertEqual(args[args.index('--name')+1],lc.owner_name(OWNER))
        self.assertEqual(args[args.index(lc.IMAGE)+1:],inner[1:]);self.assertNotIn(str(r.BASE/'cosign'),args)
        for bad in ([],['/usr/local/bin/cosign','verify'],['/CONTROL_ONLY/other','verify-blob-attestation']):
            with self.assertRaises(lc.Refusal):lc.docker_command(OWNER,Path('/a'),Path('/b'),Path('/c'),bad)

class PreservedCryptoControls(unittest.TestCase):
    def test_inherited_crypto_and_all_report_blob_methods_are_unmodified_and_retry_is_original(self):
        base,driver=source_driver();p=plan();reader=base.GitSource('/Users/tianpan/projects/bex7')
        with tempfile.TemporaryDirectory(prefix='v11-retry-control-',dir='/private/tmp') as name:
            root=Path(name).resolve();records=[]
            class Executor:
                def execute(self,bundle,tuf,inner,out,err):
                    records.append(inner);out.write_bytes(b'CONTROL_ONLY')
                    code=1 if len(records)<3 else 0
                    err.write_text('getting trusted root from TUF: i/o timeout' if code else 'CONTROL_ONLY success')
                    return {'exit_code':code,'command':['CONTROL_ONLY_NOT_DOCKER'],'raw_process_receipt':'CONTROL_ONLY'}
            cls=r.review_class(base,driver,p,reader,{},HERE/'PLAN.json',Executor())
            self.assertIs(cls.crypto_bundles,driver.Review.crypto_bundles)
            for method in ('fetch_bundles','signed_statements','derive_docker_roots_if_needed','pull_blobs','finish','run'):
                self.assertIs(getattr(cls,method),getattr(driver.Review,method))
            obj=object.__new__(cls);obj.executor=Executor();obj.bundle_dir=root;obj.a=types.SimpleNamespace(sha=r.SOURCE)
            obj.digest=r.REF.split('@')[1];obj.v=types.SimpleNamespace(dump=lambda path,value:path.write_text(json.dumps(value)))
            obj.progress=lambda *args,**kwargs:None
            row={'path':str(root/'CONTROL_ONLY.bundle'),'bundle_digest':'sha256:'+'a'*64,'predicate_type':'https://sigstore.dev/cosign/sign/v1'}
            with patch.object(r.time,'sleep') as sleep:result=obj.verify_command(row,'positive')
            self.assertEqual(result['exit_code'],0);self.assertEqual(len(result['attempts']),3);self.assertEqual(records[0],records[1]);self.assertEqual(records[1],records[2]);self.assertEqual([x.args for x in sleep.call_args_list],[(2,),(4,)])
            before=len(records)
            with self.assertRaises(lc.Refusal):obj.verify_command(row,'positive',source_sha='0'*40)
            self.assertEqual(len(records),before)
    def test_worker_scope_binds_real_parent_group_arguments_plan_and_reviewed_source(self):
        with tempfile.TemporaryDirectory(prefix='v11-scope-control-',dir='/private/tmp') as name:
            root=Path(name).resolve();component=root/'component';component.mkdir();(component/'CONTROL_ONLY.py').write_text('# not executable proof\n')
            manifest={'CONTROL_ONLY.py':{'bytes':(component/'CONTROL_ONLY.py').stat().st_size,'sha256':lc.sha(component/'CONTROL_ONLY.py')}}
            (component/'MANIFEST.json').write_bytes(body(manifest));path=root/'plan.json';path.write_bytes(body(plan()))
            args=types.SimpleNamespace(lifecycle_root=root,owner=OWNER,plan=path,repo_root=Path('/Users/tianpan/projects/bex7'),output_root=root/'evidence',cache_root=root/'cache',docker_config=root/'docker')
            expected=[sys.executable,'-B',str(component/'common499_review.py'),'review','--plan',str(path.resolve()),'--repo-root',str(args.repo_root.resolve()),'--output-root',str(args.output_root.resolve()),'--cache-root',str(args.cache_root),'--lifecycle-root',str(root),'--docker-config',str(args.docker_config),'--owner',OWNER]
            scope={'owner':OWNER,'container_name':lc.owner_name(OWNER),'candidate_ref':r.REF,'producer':r.SOURCE,'verification':r.SOURCE,'plan_sha256':lc.sha(path),'source_manifest_sha256':lc.sha(component/'MANIFEST.json'),'parent_pid':123,'command':expected}
            with patch.object(r,'HERE',component),patch.object(r.os,'getppid',return_value=123),patch.object(r.os,'getpid',return_value=456),patch.object(r.os,'getpgrp',return_value=456):
                (root/'SCOPE.json').write_bytes(body(scope));self.assertEqual(r.runtime_scope(args,plan()),123)
                for key,value in [('owner','f'*16),('parent_pid',True),('plan_sha256','0'*64),('command',expected+['--CONTROL_ONLY']),('source_manifest_sha256','0'*64)]:
                    wrong=copy.deepcopy(scope);wrong[key]=value;(root/'SCOPE.json').write_bytes(body(wrong))
                    with self.subTest(key=key),self.assertRaises(lc.Refusal):r.runtime_scope(args,plan())

class FakeDocker:
    def __init__(self):self.state=None;self.events=[];self.rm_fail=False
    def capture(self,argv,timeout=10):
        self.events.append(argv[1:]);result={'argv':argv,'exit':0,'stdout':'','stderr':'','complete':True}
        if argv[1:3]==['image','inspect']:result['stdout']=json.dumps([{'Id':lc.IMAGE,'Os':'linux','Architecture':'arm64'}])
        elif argv[1:3]==['container','inspect']:
            if self.state is None:result.update(exit=1,stderr='Error: No such container: '+argv[-1])
            else:result['stdout']=json.dumps([self.state])
        elif argv[1:3]==['rm','--force']:
            if self.rm_fail:result.update(exit=1,stderr='CONTROL_ONLY remove failed')
            else:self.state=None;result['stdout']=argv[-1]+'\n'
        else:raise AssertionError('no actual Docker:'+repr(argv))
        return result
    def client(self):return lc.Docker(self.capture)
    def create(self,args):
        mounts=[]
        for i,value in enumerate(args):
            if value=='--mount':
                fields=args[i+1].split(',');data=dict(x.split('=',1) for x in fields if '=' in x)
                mounts.append({'Type':'bind','Source':data['source'],'Destination':data['target'],'RW':'readonly' not in fields})
        self.state={'Id':'a'*64,'Name':'/'+lc.owner_name(OWNER),'Image':lc.IMAGE,'Config':{'Labels':{'bex.w8.owned':OWNER},'Entrypoint':['/usr/local/bin/cosign'],'Cmd':args[args.index(lc.IMAGE)+1:],'User':'501:20'},'Mounts':mounts,
            'HostConfig':{'Memory':512<<20,'NanoCpus':1_000_000_000,'PidsLimit':64,'ReadonlyRootfs':True},'State':{'Status':'exited','ExitCode':0,'OOMKilled':False,'Error':''}}

class ExecutionControls(unittest.TestCase):
    def roots(self):
        tmp=tempfile.TemporaryDirectory(prefix='v11-docker-control-',dir='/private/tmp');self.addCleanup(tmp.cleanup);root=Path(tmp.name).resolve()
        bundle=root/'bundle.json';bundle.write_bytes(b'CONTROL_ONLY_NOT_A_SIGNED_BUNDLE')
        tuf=root/'tuf';tuf.mkdir();config=root/'docker';config.mkdir();(config/'config.json').write_bytes(b'{"auths":{}}\n')
        binary=root/'CONTROL_ONLY_BINARY';binary.write_bytes(b'CONTROL_ONLY_NOT_EXECUTABLE')
        return root,bundle,tuf,config,binary
    def case(self,mode):
        root,bundle,tuf,config,binary=self.roots();docker=FakeDocker();cancel={'value':False};clock={'value':0};processes=[]
        def now():clock['value']+=100;return clock['value']
        class Process:
            pid=987654321
            def __init__(self,args,**kw):
                docker.create(args);self.returncode=None if mode=='timeout' else 0;self.terminated=False;self.reaped=False
                kw['stdout'].write(b'CONTROL_ONLY stdout');kw['stderr'].write(b'CONTROL_ONLY stderr');kw['stdout'].flush();kw['stderr'].flush();processes.append(self)
                if mode=='cancel':cancel['value']=True
                if mode=='wrong-state':docker.state['HostConfig']['Memory']=1
                if mode=='rm-fail':docker.rm_fail=True
            def poll(self):return self.returncode
            def wait(self,timeout):self.reaped=True;return self.returncode
            def terminate(self):self.terminated=True;self.returncode=-15;docker.state['State'].update(Status='exited',ExitCode=-15)
            def kill(self):self.returncode=-9
        executor=lc.Executor(OWNER,root,config,cancelled=lambda:cancel['value'],docker=docker.client(),popen=Process,clock=now,sleep=lambda seconds:None)
        if mode=='late-cancel':
            original_registry=executor.registry_write
            def registry(**fields):
                original_registry(**fields)
                if fields.get('pending') is False:cancel['value']=True
            executor.registry_write=registry
        inner=r.inner_command({'predicate_type':'CONTROL_ONLY'},r.SOURCE,r.REF.split('@')[1].split(':')[1])
        with patch.object(lc,'BINARY',binary),patch.object(lc,'BINARY_SHA',lc.sha(binary)):
            if mode=='happy':result=executor.execute(bundle,tuf,inner,root/'stdout',root/'stderr');self.assertEqual(result['exit_code'],0);self.assertEqual(result['command'][:2],['docker','run'])
            else:
                with self.assertRaises(lc.Refusal):executor.execute(bundle,tuf,inner,root/'stdout',root/'stderr')
        self.assertTrue(processes[0].reaped)
        if mode=='late-cancel':self.assertFalse(json.loads((root/'stderr.process.json').read_bytes())['complete'])
        if mode!='rm-fail':self.assertIsNone(docker.state)
        self.assertEqual(docker.events[-1],['container','inspect',lc.owner_name(OWNER)])
        return root,docker,processes
    def test_fake_actual_state_good_and_timeout_cancel_wrong_state_cleanup(self):
        for mode in ('happy','timeout','cancel','late-cancel','wrong-state'):
            with self.subTest(mode=mode):self.case(mode)
    def test_failed_remove_still_inspects_and_never_returns_success(self):self.case('rm-fail')
    def test_unknown_create404_and_foreign_ownership_cannot_be_declared_clean(self):
        docker=FakeDocker()
        with self.assertRaises(lc.CleanupFailure):docker.client().cleanup(OWNER,uncertain=True)
        args=lc.docker_command(OWNER,Path('/a'),Path('/b'),Path('/c'),['/usr/local/bin/cosign','verify-blob-attestation']);docker.create(args)
        docker.state['Config']['Labels']['bex.w8.owned']='CONTROL_ONLY_FOREIGN'
        with self.assertRaises(lc.CleanupFailure):docker.client().cleanup(OWNER)
        self.assertFalse(any(x[:2]==['rm','--force'] for x in docker.events))
    def test_wrong_binary_and_spawn_error_cannot_execute_or_skip_cleanup(self):
        root,bundle,tuf,config,binary=self.roots();docker=FakeDocker();calls=[]
        def fail(*args,**kwargs):calls.append(args);raise OSError('CONTROL_ONLY spawn failed')
        executor=lc.Executor(OWNER,root,config,docker=docker.client(),popen=fail)
        inner=r.inner_command({'predicate_type':'CONTROL_ONLY'},r.SOURCE,r.REF.split('@')[1].split(':')[1])
        with patch.object(lc,'BINARY',binary),patch.object(lc,'BINARY_SHA','0'*64):
            with self.assertRaises(lc.Refusal):executor.execute(bundle,tuf,inner,root/'out',root/'err')
        self.assertEqual(calls,[]);self.assertEqual(docker.events,[])
        with patch.object(lc,'BINARY',binary),patch.object(lc,'BINARY_SHA',lc.sha(binary)):
            with self.assertRaises(OSError):executor.execute(bundle,tuf,inner,root/'out',root/'err')
        self.assertEqual(len(calls),1);self.assertIsNone(docker.state)
        self.assertEqual(docker.events[-1],['container','inspect',lc.owner_name(OWNER)])
    def test_outer_unknown_lifecycle_is_not_accepted_on404(self):
        root,*_=self.roots();lc.write(root/'linux-cosign-lifecycle.json',{'owner':OWNER,'name':lc.owner_name(OWNER),'pending':True})
        with self.assertRaises(lc.CleanupFailure):outer.final_cleanup(root,OWNER,FakeDocker().client())

class SizingControls(unittest.TestCase):
    def test_metadata_only_size_positive_and_wrong_index_platform_refusals(self):
        base,driver=source_driver()
        for mode in ('positive','wrong-index','wrong-platform'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory(prefix='v11-metadata-control-',dir='/private/tmp') as tmp:
                out=Path(tmp).resolve();calls=[];index=r.REF.split('@')[1];children={'amd64':'sha256:'+'a'*64,'arm64':'sha256:'+'b'*64}
                bundle='sha256:'+'c'*64
                def manifest(url,headers,ref):
                    calls.append(('manifest',ref));raw=b'CONTROL_ONLY metadata'
                    if ref==index:return {},raw,'sha256:'+'0'*64 if mode=='wrong-index' else index
                    if ref in children.values():return {'config':{'digest':'sha256:'+'d'*64,'size':2},'layers':[{'digest':'sha256:'+'e'*64,'size':10}]},raw,ref
                    if ref==bundle:return {'subject':{'digest':index},'layers':[{'digest':'sha256:'+'f'*64,'size':20}]},raw,ref
                    raise AssertionError('not a metadata request:'+ref)
                def config(url,headers,row):
                    calls.append(('config',row['digest']));arch='amd64' if len([c for c in calls if c[0]=='config'])==1 else 'arm64'
                    return {'architecture':'other' if mode=='wrong-platform' else arch,'os':'linux'},b'{}'
                refs={'manifests':[{'artifactType':'application/vnd.dev.sigstore.bundle.v0.3+json','digest':bundle,'size':len(b'CONTROL_ONLY metadata'),'annotations':{'dev.sigstore.bundle.predicateType':driver.SIGN}}]}
                policy=types.SimpleNamespace(ROOT=None,REPOSITORY='bex-co/bex-cnb-run',MAX_DOCUMENT=16<<20,MAX_ATTESTATIONS=256<<20,
                    public_registry=lambda *args:('CONTROL_ONLY',{}),manifest_document=manifest,image_platforms=lambda value:children,
                    config_document=config,descriptor=lambda value:(value['digest'],value['size']),
                    small_get=lambda *args:(body(refs),{}))
                with patch.object(driver,'load',side_effect=lambda name,path:policy if name=='native_policy' else types.SimpleNamespace()):
                    if mode=='positive':r.size_metadata(base,driver,plan(),Path('/Users/tianpan/projects/bex7'),out)
                    else:
                        with self.assertRaises(lc.Refusal):r.size_metadata(base,driver,plan(),Path('/Users/tianpan/projects/bex7'),out)
                if mode=='positive':
                    self.assertFalse(json.loads((out/'layers.json').read_bytes())['candidate_approved'])
                    self.assertEqual(json.loads((out/'bundles.json').read_bytes())['unique_bundle_payload_bytes'],20)
                else:self.assertFalse((out/'bundles.json').exists())
                self.assertTrue(all(kind in ('manifest','config') for kind,value in calls),'never layer/report payload bodies')

class OuterControls(unittest.TestCase):
    def test_unknown_candidate_refuses_before_root_worker_or_Docker(self):
        with tempfile.TemporaryDirectory(prefix='v11-entry-control-',dir='/private/tmp') as tmp:
            p=plan();p['ref']=None;path=Path(tmp)/'plan.json';path.write_bytes(body(p))
            target=Path('/private/tmp')/('bex-w8-common499-review-'+secrets.token_hex(8))
            with patch.object(outer,'source_manifest',return_value={}):
                with self.assertRaises((lc.Refusal,RuntimeError)):outer.execute(target,path,lc.sha(path),'0'*64,process_factory=lambda *a,**k:self.fail('worker must not start'),docker=FakeDocker().client())
            self.assertFalse(target.exists())
    def test_real_owned_worker_reaped_and_container_cleanup_after_capture_failure_or_cancel(self):
        for mode in ('artifact-failure','cancel'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory(prefix='v11-outer-control-',dir='/private/tmp') as tmp:
                p=plan();path=Path(tmp)/'plan.json';path.write_bytes(body(p));target=Path('/private/tmp')/('bex-w8-common499-review-'+secrets.token_hex(8))
                docker=FakeDocker();children=[];original_write=lc.write;previous={s:signal.getsignal(s) for s in (signal.SIGTERM,signal.SIGINT)}
                def write(path,value):
                    if mode=='artifact-failure' and Path(path).name=='PROCESS.json':raise OSError('CONTROL_ONLY capture failure')
                    return original_write(path,value)
                def worker(*args,**kwargs):
                    scope=json.loads((target/'SCOPE.json').read_bytes());owner=scope['owner']
                    args=lc.docker_command(owner,Path('/a'),Path('/b'),Path('/c'),['/usr/local/bin/cosign','verify-blob-attestation'])
                    docker.create(args);docker.state['Name']='/'+lc.owner_name(owner);docker.state['Config']['Labels']['bex.w8.owned']=owner
                    original_write(target/'linux-cosign-lifecycle.json',{'owner':owner,'name':lc.owner_name(owner),'pending':True})
                    child=subprocess.Popen([sys.executable,'-B','-c','import time; time.sleep(30)'],stdout=kwargs['stdout'],stderr=kwargs['stderr'],start_new_session=True);children.append(child)
                    if mode=='cancel':signal.getsignal(signal.SIGTERM)(signal.SIGTERM,None)
                    return child
                try:
                    with patch.object(outer,'source_manifest',return_value={}),patch.object(r,'validate_plan',return_value=types.SimpleNamespace(candidate_job=lambda *args:{})),patch.object(r,'rest_projection',return_value={}),patch.object(lc,'write',write):
                        result=outer.execute(target,path,lc.sha(path),'0'*64,process_factory=worker,docker=docker.client())
                    self.assertFalse(result['candidate_approved']);self.assertTrue(result['process_cleanup']['group_absent']);self.assertIsNone(docker.state)
                    self.assertTrue(result['container_cleanup']['absence_proved']);self.assertEqual({s:signal.getsignal(s) for s in previous},previous)
                finally:
                    for child in children:
                        if child.poll() is None:os.killpg(child.pid,signal.SIGKILL)
                        child.wait(timeout=3)
                    if target.exists():shutil.rmtree(target)

class UncertaintyPropagationControls(unittest.TestCase):
    def test_failed125_unknown_create404_remains_pending_through_outer_cleanup_and_late_cancel(self):
        for late in (False,True):
            with self.subTest(late_cancel=late),tempfile.TemporaryDirectory(prefix='v11-unknown125-control-',dir='/private/tmp') as name:
                root=Path(name).resolve();bundle=root/'bundle.json';bundle.write_bytes(b'CONTROL_ONLY_NOT_SIGNATURE')
                tuf=root/'tuf';tuf.mkdir();config=root/'docker';config.mkdir();(config/'config.json').write_bytes(b'{"auths":{}}\n')
                binary=root/'CONTROL_ONLY_BINARY';binary.write_bytes(b'CONTROL_ONLY_NOT_EXECUTABLE');fake=FakeDocker();cancel={'value':False};writes=[]
                class RejectedCreate:
                    pid=987654321
                    def __init__(self,*args,**kwargs):self.returncode=125;self.reaped=False
                    def poll(self):return self.returncode
                    def wait(self,timeout):self.reaped=True;return self.returncode
                    def terminate(self):raise AssertionError('already exited')
                    def kill(self):raise AssertionError('already exited')
                executor=lc.Executor(OWNER,root,config,cancelled=lambda:cancel['value'],docker=fake.client(),popen=RejectedCreate)
                original_registry=executor.registry_write
                def registry(**fields):
                    writes.append(fields);original_registry(**fields)
                    if late and fields.get('cleanup') is not None:cancel['value']=True
                executor.registry_write=registry
                inner=r.inner_command({'predicate_type':'CONTROL_ONLY'},r.SOURCE,r.REF.split('@')[1].split(':')[1])
                with patch.object(lc,'BINARY',binary),patch.object(lc,'BINARY_SHA',lc.sha(binary)):
                    with self.assertRaises(lc.Refusal):executor.execute(bundle,tuf,inner,root/'stdout',root/'stderr')
                process=json.loads((root/'stderr.process.json').read_bytes());state=json.loads((root/'linux-cosign-lifecycle.json').read_bytes())
                self.assertFalse(process['complete']);self.assertTrue(process['cleanup']['absence_proved'])
                reason='unknown create/run outcome;404 alone cannot prove cleanup'
                self.assertIn(reason,process['cleanup']['errors']);self.assertIs(state['pending'],True)
                self.assertEqual(len(writes),3 if late else 2);self.assertTrue(all(row['pending'] is True for row in writes))
                with self.assertRaises(lc.CleanupFailure) as refusal:outer.final_cleanup(root,OWNER,fake.client())
                self.assertIn(reason,refusal.exception.receipt['errors'])
                self.assertFalse(any(row[:2]==['rm','--force'] for row in fake.events))

class ActualMissingInspectControls(unittest.TestCase):
    def actual(self):
        owner='b22dbd7ec72262d5';name=lc.owner_name(owner);path=Path(__file__).resolve().parent
        stdout=(path/'actual-inspect.stdout').read_bytes();stderr=(path/'actual-inspect.stderr').read_bytes()
        self.assertEqual(stdout,b'[]\n');self.assertEqual(stderr.decode().strip(),'Error response from daemon: No such container: '+name)
        return owner,{'exit':1,'stdout':stdout.decode(),'stderr':stderr.decode(),'complete':True}
    def test_actual_retained_empty_array_daemon_missing_name_is_proved_absent(self):
        owner,response=self.actual();calls=[]
        def capture(argv,timeout=10):calls.append(argv);return dict(response,argv=argv)
        obj,proof=lc.Docker(capture).inspect(owner)
        self.assertIsNone(obj);self.assertEqual(proof['stdout'],'[]\n');self.assertEqual(calls,[['docker','container','inspect',lc.owner_name(owner)]])
    def test_generic404_foreign_name_nonempty_json_incomplete_or_untyped_exit_are_not_absence(self):
        owner,response=self.actual()
        for key,value in [('stderr','Error response from daemon: 404 page not found'),
                          ('stderr','Error response from daemon: No such container: '+lc.owner_name('f'*16)),
                          ('stderr','Cannot connect to the Docker daemon'),('stdout','[{}]\n'),('stdout','false\n'),
                          ('stdout','null\n'),('stdout','warning\n[]\n'),('exit',True),('exit',1.0),('exit',2),('complete',False)]:
            changed=dict(response);changed[key]=value
            with self.subTest(key=key,value=value),self.assertRaises(lc.Refusal):
                lc.Docker(lambda argv,timeout=10:dict(changed,argv=argv)).inspect(owner)
    def test_real_empty_array_format_never_heals_unknown_create125_uncertainty(self):
        owner,response=self.actual();docker=lc.Docker(lambda argv,timeout=10:dict(response,argv=argv))
        clean=docker.cleanup(owner,uncertain=False);self.assertTrue(clean['absence_proved']);self.assertEqual(clean['errors'],[])
        with self.assertRaises(lc.CleanupFailure) as failure:docker.cleanup(owner,uncertain=True)
        self.assertTrue(failure.exception.receipt['absence_proved'])
        self.assertIn('unknown create/run outcome;404 alone cannot prove cleanup',failure.exception.receipt['errors'])

if __name__=='__main__':unittest.main()
