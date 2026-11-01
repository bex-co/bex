#!/usr/bin/env python3
"""One private499 supplier review. No execution or network occurs on import."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import signal
import tempfile
import time
from types import SimpleNamespace

import linux_cosign as lc

BASE=Path('/private/tmp/bex-w8-common-v11-resource-source-20261101-ipmv7y8i/source')
BASE_SHA='4746a5022ab6df57ed95b6354d56b92eff15976d1737c2aa5a5ca15aa0080a4b'
REF='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
SOURCE='1feba939b4697894e800ed0abb4f25a0bf415641'
RUN=41341287851
JOB=124954099391
HERE=Path(__file__).resolve().parent

def digest(raw):return hashlib.sha256(raw).hexdigest()
def load(path,expected,name):
    lc.require(digest(path.read_bytes())==expected,'immutable imported source changed:'+path.name)
    spec=importlib.util.spec_from_file_location(name,path);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);return module

def base_module():return load(BASE/'common-v11-adapter.py',BASE_SHA,'fixed_common_resource_adapter')

def read_ref(ref):
    lc.require(isinstance(ref,dict) and set(ref)=={'path','bytes','sha256'} and isinstance(ref.get('path'),str)
        and Path(ref['path']).is_absolute() and type(ref.get('bytes')) is int and 0<ref['bytes']<=2<<20
        and isinstance(ref.get('sha256'),str) and re.fullmatch('[a-f0-9]{64}',ref['sha256']),'bounded actual REST descriptor required')
    path=Path(ref['path']);lc.require(path.is_file() and not path.is_symlink(),'actual REST regular file missing')
    raw=path.read_bytes();lc.require(len(raw)==ref['bytes'] and digest(raw)==ref['sha256'],'actual REST custody differs');return raw

def validate_plan(plan,resources=False):
    base=base_module();base.validate_plan(plan,executing=True)
    lc.require(plan['ref']==REF and plan['producer_sha']==plan['verification_sha']==SOURCE
        and type(plan.get('run_id')) is int and plan['run_id']==RUN and type(plan.get('job_id')) is int and plan['job_id']==JOB,
        'unknown/undeclared499 candidate source/ref/run/job')
    ci=plan.get('ci_inputs',{});lc.require(set(ci)=={'runs','jobs','run_attempt','projection_contract'} and type(ci['run_attempt']) is int
        and ci['run_attempt']==1,'actual run attempt/REST input contract differs')
    profile=plan.get('cosign_execution',{})
    lc.require(profile=={'profile':'owned-linux-cosign-3.1.4-v11-bundle','binary':{'path':str(lc.BINARY),'bytes':136437761,'sha256':lc.BINARY_SHA},'cached_image_id':lc.IMAGE},'fixed reviewed Linux transport profile differs')
    if resources:
        lc.require(isinstance(plan.get('local_resource_policy'),dict) and isinstance(plan.get('local_resource_sizing'),dict),'new499 descriptor sizing/resource policy required before review')
        for role in ('layers','bundles'):
            ref=plan['local_resource_sizing'].get(role);body=read_ref(ref);obj=json.loads(body)
            lc.require(obj.get('ref')==REF and obj.get('candidate_approved') is False,'sizing belongs to another/approved candidate')
        lc.require(plan['local_resource_policy'].get('ref')==REF,'sizing policy belongs to another candidate')
    return base

def rest_projection(plan):
    raw_runs=read_ref(plan['ci_inputs']['runs']);raw_jobs=read_ref(plan['ci_inputs']['jobs'])
    runs,jobs=json.loads(raw_runs),json.loads(raw_jobs)
    lc.require(isinstance(runs,dict) and isinstance(runs.get('workflow_runs'),list) and type(runs.get('total_count')) is int
        and runs['total_count']==len(runs['workflow_runs']),'actual run collection malformed/incomplete')
    lc.require(isinstance(jobs,dict) and isinstance(jobs.get('jobs'),list) and type(jobs.get('total_count')) is int
        and jobs['total_count']==len(jobs['jobs']),'actual job collection malformed/incomplete')
    selected=[r for r in runs['workflow_runs'] if isinstance(r,dict) and type(r.get('id')) is int and r['id']==RUN]
    lc.require(len(selected)==1,'actual completed run missing/duplicated');run=selected[0]
    lc.require(run.get('repository',{}).get('full_name')=='bex-co/bex' and run.get('path')=='.github/workflows/native-common-image.yml'
        and run.get('event')=='push' and run.get('head_branch')=='main' and run.get('head_sha')==SOURCE
        and type(run.get('run_attempt')) is int and run['run_attempt']==1 and run.get('status')=='completed'
        and run.get('conclusion')=='success' and run.get('html_url')=='https://github.com/bex-co/bex/actions/runs/'+str(RUN),
        'actual run repository/workflow/source/attempt/status differs')
    ids=[];mapped=[]
    for job in jobs['jobs']:
        lc.require(isinstance(job,dict) and type(job.get('id')) is int and type(job.get('run_id')) is int and job['run_id']==RUN
            and type(job.get('run_attempt')) is int and job['run_attempt']==1 and job.get('head_sha')==SOURCE and job.get('head_branch')=='main',
            'actual job source/attempt/run identity differs')
        lc.require(job.get('html_url')=='https://github.com/bex-co/bex/actions/runs/'+str(RUN)+'/job/'+str(job['id']),
            'actual job URL/run binding differs')
        lc.require(job['id'] not in ids,'duplicate actual job ID');ids.append(job['id'])
        mapped.append({'databaseId':job['id'],'name':job.get('name'),'status':job.get('status'),'conclusion':job.get('conclusion'),
            'url':job.get('html_url'),'steps':job.get('steps')})
    lc.require(JOB in ids,'actual publisher job missing')
    # Explicit field mapping; this object is never called raw CLI output.
    return {'databaseId':run['id'],'headSha':run['head_sha'],'headBranch':run['head_branch'],'jobs':mapped,
        'url':run['html_url'],'status':run['status'],'conclusion':run['conclusion']}

def inner_command(row,source,digest_hex):
    return ['/usr/local/bin/cosign','verify-blob-attestation','--bundle','/bundle.json','--type',row['predicate_type'],
        '--digest',digest_hex,'--digestAlg','sha256','--certificate-identity',
        'https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main',
        '--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',source,
        '--use-signed-timestamps','--timeout','5m']

def review_class(base,driver,plan,reader,equivalence,plan_path,executor):
    original_load=driver.load
    projection=rest_projection(plan)
    def actual_load(name,path):
        module=original_load(name,path)
        if name=='native_policy':
            module.COSIGN=lc.BINARY;module.COSIGN_SHA256=lc.BINARY_SHA
            def retained_actual_ci(arguments):
                lc.require(arguments==['run','view',str(RUN),'--json','databaseId,headSha,headBranch,jobs,url,status,conclusion'],
                    'unsupported actual REST→selector request')
                return projection
            module.gh_json=retained_actual_ci
        return module
    driver.load=actual_load
    try:Common=base.review_class(driver,plan,reader,equivalence,plan_path)
    finally:driver.load=original_load
    class LinuxReview(Common):
        def __init__(self,args):
            try:super().__init__(args)
            finally:driver.load=original_load
            self.executor=executor
            for path in (Path(__file__).resolve(),HERE/'linux_cosign.py',HERE/'run_review.py'):
                self.tool_hashes[str(path)]=digest(path.read_bytes());shutil.copyfile(path,self.out/'executed-tools'/path.name)
        def metadata(self):
            super().metadata()
            ci=self.out/'actual-CI-inputs';ci.mkdir()
            for key in ('runs','jobs'):(ci/(key+'.json')).write_bytes(read_ref(plan['ci_inputs'][key]))
            base.write_json(ci/'projection.json',{'method':'explicit retained actual REST field mapping; not raw CLI stdout',
                'inputs':plan['ci_inputs'],'projection':projection})
            self.ci['actual_metadata_method']='retained completed REST run/jobs with exact custody; explicit selector mapping'
            self.ci['actual_metadata_inputs']=plan['ci_inputs']
            self.v.dump(self.out/'ci-runtime-scan-signing-gates.json',self.ci)
            self.v.dump(self.out/'linux-crypto-profile.json',{'binary_sha256':lc.BINARY_SHA,'cached_image_id':lc.IMAGE,
                'maximum_live_containers':1,'per_container_memory_bytes':512<<20,'raw_process_commands':'actual Docker argv, never projected Darwin invocation',
                'all_strict_bundle_flags_and_inherited_report_blob_policy':'unchanged'})
        def verify_command(self,row,label,source_sha=None,digest=None):
            # Exact V11 labels, inner policy flags, retry predicate and backoff.
            lc.require(label in ('positive','wrong-source','wrong-subject'),'unsupported strict control label')
            expected=(None,None) if label=='positive' else (('0'*40,None) if label=='wrong-source' else (None,'sha256:'+'0'*64))
            lc.require((source_sha,digest)==expected,'wrong strict-control source/digest arguments')
            source=source_sha or self.a.sha;subject=(digest or self.digest).split(':')[1]
            inner=inner_command(row,source,subject);base_name=row['bundle_digest'].split(':')[1]+'-'+label;attempts=[]
            for attempt in range(1,4):
                stem=base_name+'-attempt'+str(attempt);out=self.bundle_dir/(stem+'.stdout.log');err=self.bundle_dir/(stem+'.stderr.log')
                tuf_parent=self.bundle_dir/'cosign-tuf-cache';lc.require(not tuf_parent.is_symlink(),'TUF parent is a symlink');tuf_parent.mkdir(mode=0o700,exist_ok=True)
                tuf=Path(tempfile.mkdtemp(prefix=stem+'-',dir=tuf_parent))
                transport=self.executor.execute(row['path'],tuf,inner,out,err)
                result=dict(transport,bundle_digest=row['bundle_digest'],stderr=str(err),tuf_cache=str(tuf))
                self.v.dump(self.bundle_dir/(stem+'.result.json'),result);attempts.append(result)
                text=err.read_text();transport_only=label=='positive' and 'getting trusted root from TUF' in text and any(message in text for message in ('TLS handshake timeout','i/o timeout','connection reset by peer','context deadline exceeded'))
                if result['exit_code']==0 or not transport_only or attempt==3:break
                self.progress('strict-Cosign-TUF-transport-retry',bundle_digest=row['bundle_digest'],attempt=attempt);time.sleep(2*attempt)
            result=dict(result,attempts=attempts);self.v.dump(self.bundle_dir/(base_name+'.result.json'),result);return result
    return LinuxReview

def runtime_scope(args,plan):
    root=args.lifecycle_root
    lc.require(root.is_absolute() and root.parent==Path('/private/tmp') and root.resolve()==root and not root.is_symlink(),
        'exact owned outer runtime root required')
    scope_path=root/'SCOPE.json';lc.require(scope_path.is_file() and not scope_path.is_symlink() and scope_path.stat().st_size<=65536,'actual outer scope missing')
    scope=json.loads(scope_path.read_bytes())
    lc.require(scope.get('owner')==args.owner and scope.get('container_name')==lc.owner_name(args.owner)
        and scope.get('candidate_ref')==REF and scope.get('producer')==scope.get('verification')==SOURCE
        and scope.get('plan_sha256')==digest(args.plan.read_bytes())
        and type(scope.get('parent_pid')) is int and scope['parent_pid']==os.getppid()
        and os.getpgrp()==os.getpid(),'worker is not bound to this actual outer process/scope')
    expected=[__import__('sys').executable,'-B',str(HERE/'common499_review.py'),'review','--plan',str(args.plan.resolve()),
        '--repo-root',str(args.repo_root.resolve()),'--output-root',str(args.output_root.resolve()),'--cache-root',str(args.cache_root),
        '--lifecycle-root',str(root),'--docker-config',str(args.docker_config),'--owner',args.owner]
    lc.require(scope.get('command')==expected,'actual outer worker arguments differ')
    manifest=(HERE/'MANIFEST.json').read_bytes();lc.require(digest(manifest)==scope.get('source_manifest_sha256'),'outer reviewed source manifest differs')
    for name,row in json.loads(manifest).items():
        p=HERE/name;lc.require(not Path(name).is_absolute() and '..' not in Path(name).parts and p.is_file() and not p.is_symlink()
            and p.stat().st_size==row['bytes'] and lc.sha(p)==row['sha256'],'executing component source differs:'+name)
    return scope['parent_pid']

def size_metadata(base,driver,plan,repo,out):
    """Explicit future Root preflight: immutable small OCI metadata only, no bodies/crypto."""
    lc.require(not any(out.iterdir()),'metadata sizing needs a fresh empty owned output directory')
    began=time.monotonic()
    def guard(extra=0):
        base.physical_guard(out)
        lc.require(time.monotonic()-began<300,'metadata-only sizing300s deadline')
        lc.require(sum(p.stat().st_size for p in out.iterdir() if p.is_file())+extra<=32<<20,'metadata-only sizing32MiB output cap')
    def save(name,raw):
        guard(len(raw))
        with (out/name).open('xb') as stream:stream.write(raw)
    def save_json(name,value):save(name,(json.dumps(value,indent=2,allow_nan=False)+'\n').encode())
    guard()
    policy=driver.load('native_policy',BASE/'retained/loopx-w8-verify-native-artifact-ranged.py');policy.ROOT=repo
    freshness=driver.load('native_freshness',repo/'scripts/lib/toolchain-freshness.py')
    url,headers=policy.public_registry(freshness,'ghcr.io',policy.REPOSITORY)
    index,raw,observed=policy.manifest_document(url,headers,REF.split('@')[1]);lc.require(observed==REF.split('@')[1],'actual sizing index differs')
    save('index.json',raw);children=policy.image_platforms(index);layers={};platforms={}
    for arch,digest_value in children.items():
        manifest,mraw,_=policy.manifest_document(url,headers,digest_value);save(arch+'-manifest.json',mraw)
        config,craw=policy.config_document(url,headers,manifest['config']);save(arch+'-config.json',craw)
        lc.require(config.get('architecture')==arch and config.get('os')=='linux','sizing platform/config differs')
        platforms[arch]={'manifest_digest':digest_value,'config_digest':manifest['config']['digest'],'layer_count':len(manifest['layers'])}
        for row in [manifest['config'],*manifest['layers']]:
            key,n=policy.descriptor(row)
            lc.require(key not in layers or layers[key]==n,'conflicting sizing descriptor');layers[key]=n
    try:
        refraw,_=policy.small_get(url+'/referrers/'+observed,headers,policy.MAX_DOCUMENT);referrers=json.loads(refraw)
    except __import__('urllib.error',fromlist=['HTTPError']).HTTPError as error:
        if error.code!=404:raise
        referrers,refraw,_=policy.manifest_document(url,headers,observed.replace(':','-'))
    save('referrer-index.json',refraw);entries=[r for r in referrers.get('manifests',[]) if r.get('artifactType')=='application/vnd.dev.sigstore.bundle.v0.3+json' and r.get('annotations',{}).get('dev.sigstore.bundle.predicateType') in driver.TYPES]
    lc.require(0<len(entries)<=64,'unbounded/missing sizing bundle descriptors');bundles={};records=[]
    for row in entries:
        bundle,braw,bd=policy.manifest_document(url,headers,row['digest']);lc.require(len(braw)==row['size'] and bundle.get('subject',{}).get('digest')==observed and len(bundle.get('layers',[]))==1,'sizing bundle manifest subject/shape differs')
        save(bd.split(':')[1]+'.manifest.json',braw);key,n=policy.descriptor(bundle['layers'][0]);lc.require(n<=policy.MAX_ATTESTATIONS,'sizing report bound differs')
        lc.require(key not in bundles or bundles[key]==n,'conflicting bundle sizing');bundles[key]=n;records.append({'manifest_digest':bd,'bundle_digest':key,'bytes':n,'predicate_type':row['annotations']['dev.sigstore.bundle.predicateType']})
    save_json('layers.json',{'ref':REF,'candidate_approved':False,'platforms':platforms,'unique_config_and_layer_descriptors':layers,'compressed_bytes':sum(layers.values()),'largest_descriptor_bytes':max(layers.values()),'method':'immutable SHA/size checked manifest/config metadata only; no layer body read'})
    save_json('bundles.json',{'ref':REF,'candidate_approved':False,'referrer_index_sha256':digest(refraw),'bundle_descriptors':records,'unique_bundle_payload_bytes':sum(bundles.values()),'largest_bundle_payload_bytes':max(bundles.values()),'method':'immutable SHA/size checked referrer/bundle manifests only; no bundle/report body read'})

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('mode',choices=('source','size','review'))
    parser.add_argument('--plan',type=Path,required=True);parser.add_argument('--repo-root',type=Path,required=True);parser.add_argument('--output-root',type=Path,required=True)
    parser.add_argument('--cache-root',type=Path);parser.add_argument('--lifecycle-root',type=Path);parser.add_argument('--docker-config',type=Path);parser.add_argument('--owner')
    args=parser.parse_args();lc.require(args.plan.stat().st_size<=256<<10,'plan source bound')
    plan=json.loads(args.plan.read_bytes());base=validate_plan(plan,resources=args.mode=='review')
    lc.require(not args.output_root.is_symlink(),'private output symlink refused')
    out=args.output_root.resolve();repo=args.repo_root.resolve();lc.require(out.is_dir() and out!=repo and repo not in out.parents,'private output root required')
    base.physical_guard(out);reader=base.GitSource(repo);equiv=base.source_equivalence(plan,reader)
    lc.require(digest((repo/'scripts/lib/toolchain-freshness.py').read_bytes())==plan['freshness_sha256'],'executing freshness source differs')
    projection=rest_projection(plan);base.candidate_job(plan,projection)
    if args.mode=='source':base.write_json(out/'source-and-CI-equivalence.json',{'source':equiv,'CI':base.candidate_job(plan,projection),'candidate_approved':False});return
    driver=base.import_pinned(BASE/'retained/loopx-w8-native-review-v11.py',base.PINNED['loopx-w8-native-review-v11.py'])
    for name,expected in base.PINNED.items():lc.require(digest((BASE/'retained'/name).read_bytes())==expected,'retained source differs')
    if args.mode=='size':size_metadata(base,driver,plan,repo,out);return
    lc.require(args.cache_root is not None and args.lifecycle_root is not None and args.docker_config is not None,'explicit owned review/cache/config roots required')
    parent_pid=runtime_scope(args,plan)
    cancelled={'signal':None};handlers={};review=None
    def cancel(signum,frame):cancelled['signal']=signum
    try:
        for signum in (signal.SIGTERM,signal.SIGINT):handlers[signum]=signal.getsignal(signum);signal.signal(signum,cancel)
        executor=lc.Executor(args.owner,args.lifecycle_root,args.docker_config,cancelled=lambda:cancelled['signal'] is not None or os.getppid()!=parent_pid)
        cls=review_class(base,driver,plan,reader,equiv,args.plan.resolve(),executor)
        original=SimpleNamespace(name='common-bookworm',sha=SOURCE,run=str(RUN),repo_root=str(repo),policy_module=str(BASE/'retained/loopx-w8-verify-native-artifact-ranged.py'),output_root=str(out),cache_root=str(args.cache_root),layer_workers=1)
        review=cls(original);review.run();lc.require(cancelled['signal'] is None,'cancelled completed review')
    except BaseException as error:
        if review is not None:base.write_json(review.out/'failed-review.json',{'candidate_approved':False,'error_type':type(error).__name__,'error':driver.safe_error(error)})
        raise
    finally:
        for signum,handler in handlers.items():signal.signal(signum,handler)
    lc.require(cancelled['signal'] is None,'cancelled while restoring review handlers')

if __name__=='__main__':main()
