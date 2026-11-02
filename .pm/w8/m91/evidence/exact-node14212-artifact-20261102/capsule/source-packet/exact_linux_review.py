#!/usr/bin/env python3
"""Exact Node14.21.2 retained V11, with the existing owned Linux Cosign transport."""
import argparse,hashlib,importlib.util,json,os,re,shutil,signal,tempfile,time
from pathlib import Path
from types import SimpleNamespace
import linux_cosign as lc
HERE=Path(__file__).resolve().parent
REF='ghcr.io/bex-co/bex-cnb-run@sha256:332869e644cdd45bf1f9e7c42ed3df91fedb95c4d0cee6bf2abf771c9eb760c7'
SOURCE='6883953e58e750fb41763d1d5e4bd4103bdb74c5'
RUN=41360577241
JOB=125010454454
NAME='node-14.21.2-bullseye'
def digest(raw):return hashlib.sha256(raw).hexdigest()
spec=importlib.util.spec_from_file_location('fixed_exact_adapter',HERE/'exact-node-v11-adapter.py')
exact=importlib.util.module_from_spec(spec);spec.loader.exec_module(exact)
def read_ref(ref):
    lc.require(isinstance(ref,dict) and set(ref)=={'path','bytes','sha256'} and isinstance(ref['path'],str)
        and Path(ref['path']).is_absolute() and type(ref['bytes']) is int and 0<ref['bytes']<=2<<20
        and re.fullmatch('[a-f0-9]{64}',ref['sha256']),'bounded actual descriptor required')
    p=Path(ref['path']);lc.require(p.is_file() and not p.is_symlink(),'descriptor regular file missing')
    raw=p.read_bytes();lc.require(len(raw)==ref['bytes'] and digest(raw)==ref['sha256'],'descriptor custody differs');return raw
def rest_projection(plan):
    run=json.loads(read_ref(plan['ci_inputs']['run']));jobs=json.loads(read_ref(plan['ci_inputs']['jobs']))
    lc.require(type(run.get('id')) is int and run['id']==RUN and run.get('head_sha')==SOURCE and run.get('head_branch')=='main'
        and run.get('repository',{}).get('full_name')=='bex-co/bex' and run.get('event')=='workflow_dispatch'
        and run.get('path')==exact.WORKFLOW and run.get('run_attempt')==1 and run.get('status')=='completed'
        and run.get('conclusion')=='success','actual exact publication run differs')
    lc.require(type(jobs.get('total_count')) is int and jobs['total_count']==len(jobs.get('jobs',[]))==2,'complete actual job collection required')
    mapped=[];ids=[]
    for j in jobs['jobs']:
        lc.require(type(j.get('id')) is int and j['id'] not in ids and j.get('run_id')==RUN and j.get('head_sha')==SOURCE
            and j.get('head_branch')=='main' and j.get('run_attempt')==1 and j.get('status')=='completed' and j.get('conclusion')=='success','actual job source/run/status differs')
        ids.append(j['id']);mapped.append(dict(databaseId=j['id'],name=j['name'],status=j['status'],conclusion=j['conclusion'],url=j['html_url'],steps=j['steps']))
    chosen=[j for j in jobs['jobs'] if j['id']==JOB]
    member=json.loads(read_ref(plan['ci_inputs']['job']))
    lc.require(set(member)=={'method','job'} and member['method']=='exact member of retained complete actual REST jobs response; not separately fetched job endpoint' and len(chosen)==1 and member['job']==chosen[0],'actual publisher job member differs')
    return dict(headSha=run['head_sha'],jobs=mapped,url=run['html_url'],status=run['status'],conclusion=run['conclusion'])
def candidate_inputs(plan):
    projection=rest_projection(plan)
    lc.require(JOB in [j['databaseId'] for j in projection['jobs']],'exact completed publisher missing')
    return projection
def validate_plan(plan,resources=False):
    exact.validate_plan(plan,Path('/Users/tianpan/projects/bex7'),executing=True)
    lc.require(plan['ref']==REF and plan['source_sha']==SOURCE and plan['name']==NAME and plan['run_id']==RUN and plan['job_id']==JOB and len(plan['source_inputs'])==30,'fixed exact publication inputs differ')
    lc.require(plan['cosign_execution']=={'profile':'owned-linux-cosign-3.1.4-v11-bundle','binary':{'path':str(lc.BINARY),'bytes':136437761,'sha256':lc.BINARY_SHA},'cached_image_id':lc.IMAGE},'fixed Linux profile differs')
    candidate_inputs(plan)
    if resources:
        sizes={role:json.loads(read_ref(plan['local_resource_sizing'][role])) for role in ['layers','bundles']}
        lc.require(all(s['ref']==REF and s['candidate_approved'] is False for s in sizes.values()),'actual sizing belongs to another candidate')
        layers=sizes['layers']['unique_config_and_layer_descriptors']
        lc.require(all(type(n) is int and n>0 for n in layers.values()) and len(layers)==39 and sum(layers.values())==1240078073 and max(layers.values())==196895515,'actual layer descriptor sizing differs')
        rows=sizes['bundles']['bundle_descriptors'];lc.require(len(rows)==5 and all(type(r['bytes']) is int and 0<r['bytes']<=256<<20 for r in rows),'actual bundle descriptor sizing differs')
        lc.require(sum(r['bytes'] for r in rows)==sizes['bundles']['unique_bundle_payload_bytes']==252033295
            and max(r['bytes'] for r in rows)==sizes['bundles']['largest_bundle_payload_bytes']==91938835,'actual bundle byte sizing differs')
        policy=plan['local_resource_policy']
        lc.require(policy['ref']==REF and policy['remaining_free_bytes']==(1<<30)+(128<<20) and policy['layer_workers']==1
            and policy['layer_file_budget_bytes']==2<<30 and policy['maximum_owned_bytes']==640<<20 and policy['stop_owned_bytes']==608<<20
            and policy['outer_monitor_interval_seconds']==1 and policy['outer_guard_required'] is True
            and plan['retained_driver_layer_reserve_bytes']==2<<30,'original fixed execution bounds differ')
        lc.require(policy['sizing_provenance']=={'layer_sizing_sha256':plan['local_resource_sizing']['layers']['sha256'],
            'bundle_sizing_sha256':plan['local_resource_sizing']['bundles']['sha256'],'referrer_index_sha256':sizes['bundles']['referrer_index_sha256']},'actual sizing linkage differs')
    return exact


def inner_command(row,source,digest_hex):
    return ['/usr/local/bin/cosign','verify-blob-attestation','--bundle','/bundle.json','--type',row['predicate_type'],
        '--digest',digest_hex,'--digestAlg','sha256','--certificate-identity',
        'https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main',
        '--certificate-oidc-issuer','https://token.actions.githubusercontent.com','--certificate-github-workflow-sha',source,
        '--use-signed-timestamps','--timeout','5m']
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
    expected=[__import__('sys').executable,'-B',str(HERE/'exact_linux_review.py'),'review','--plan',str(args.plan.resolve()),
        '--repo-root',str(args.repo_root.resolve()),'--output-root',str(args.output_root.resolve()),'--cache-root',str(args.cache_root),
        '--lifecycle-root',str(root),'--docker-config',str(args.docker_config),'--owner',args.owner]
    lc.require(scope.get('command')==expected,'actual outer worker arguments differ')
    manifest=(HERE/'MANIFEST.json').read_bytes();lc.require(digest(manifest)==scope.get('source_manifest_sha256'),'outer reviewed source manifest differs')
    for name,row in json.loads(manifest).items():
        p=HERE/name;lc.require(not Path(name).is_absolute() and '..' not in Path(name).parts and p.is_file() and not p.is_symlink()
            and p.stat().st_size==row['bytes'] and lc.sha(p)==row['sha256'],'executing component source differs:'+name)
    return scope['parent_pid']

# inner_command / verify_command / runtime_scope are copied from the reviewed
# Linux0941 component below; only runtime worker filename is rebound.
def review_class(driver,plan,repo,plan_path,executor):
    original_load=driver.load;projection=rest_projection(plan);job=json.loads(read_ref(plan['ci_inputs']['job']))['job']
    def actual_load(name,path):
        module=original_load(name,path)
        if name=='native_policy':
            module.COSIGN=lc.BINARY;module.COSIGN_SHA256=lc.BINARY_SHA
            def actual_ci(args):
                if args==['run','view',str(RUN),'--json','headSha,jobs,url,status,conclusion']:return projection
                if args==['api','repos/bex-co/bex/actions/jobs/'+str(JOB)]:return job
                raise lc.Refusal('unsupported actual exact REST projection request')
            module.gh_json=actual_ci
        return module
    driver.load=actual_load
    try:Exact=exact.review_class(driver,plan,repo,plan_path)
    finally:driver.load=original_load
    class LinuxReview(Exact):
        def __init__(self,args):
            try:super().__init__(args)
            finally:driver.load=original_load
            self.executor=executor
            for p in [Path(__file__).resolve(),HERE/'linux_cosign.py',HERE/'run_review.py',HERE/'exact-node-v11-adapter.py']:
                self.tool_hashes[str(p)]=digest(p.read_bytes());shutil.copyfile(p,self.out/'executed-tools'/p.name)
        def metadata(self):
            super().metadata()
            ci=self.out/'actual-CI-inputs';ci.mkdir()
            for key in ['run','jobs','job']:(ci/(key+'.json')).write_bytes(read_ref(plan['ci_inputs'][key]))
            self.v.dump(ci/'projection.json',{'method':'Exact retained completed REST run/jobs/member; explicit CLI-field mapping, not raw CLI output','inputs':plan['ci_inputs'],'projection':projection})
            self.ci['actual_metadata_method']='retained completed REST run/fulljobs plus explicitly retained member; no separate job-endpoint fetch or raw CLI claim'
            self.ci['actual_metadata_inputs']=plan['ci_inputs'];self.v.dump(self.out/'ci-runtime-scan-signing-gates.json',self.ci)
            self.v.dump(self.out/'linux-crypto-profile.json',{'binary_sha256':lc.BINARY_SHA,'cached_image_id':lc.IMAGE,
                'maximum_live_containers':1,'per_container_memory_bytes':512<<20,'raw_process_commands':'actual Docker argv, no projected native invocation',
                'strict_bundle_flags_report_blob_methods':'unchanged retained V11'})
    LinuxReview.verify_command=verify_command
    return LinuxReview

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('mode',choices=['source','review']);p.add_argument('--plan',type=Path,required=True)
    p.add_argument('--repo-root',type=Path,required=True);p.add_argument('--output-root',type=Path,required=True)
    p.add_argument('--cache-root',type=Path);p.add_argument('--lifecycle-root',type=Path);p.add_argument('--docker-config',type=Path);p.add_argument('--owner');args=p.parse_args()
    lc.require(args.plan.stat().st_size<=256<<10,'fixed plan bound');plan=json.loads(args.plan.read_bytes());validate_plan(plan,resources=True)
    lc.require(digest((args.repo_root/exact.FRESHNESS).read_bytes())==exact.FRESHNESS_SHA,'actual freshness support differs')
    if args.mode=='source':
        print(json.dumps({'source_checks':'PASS','name':NAME,'source':SOURCE,'source_inputs':30,'candidate_approved':False}));return
    lc.require(args.cache_root is not None and args.lifecycle_root is not None and args.docker_config is not None,'exact owned roots required')
    parent_pid=runtime_scope(args,plan);cancelled={'signal':None};handlers={};review=None
    try:
        for sig in [signal.SIGTERM,signal.SIGINT]:handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:cancelled.update(signal=n))
        executor=lc.Executor(args.owner,args.lifecycle_root,args.docker_config,cancelled=lambda:cancelled['signal'] is not None or os.getppid()!=parent_pid)
        driver=exact.import_pinned('loopx-w8-native-review-v11.py');exact.import_pinned('loopx-w8-verify-native-artifact-ranged.py')
        cls=review_class(driver,plan,args.repo_root,args.plan.resolve(),executor)
        original=SimpleNamespace(name=NAME,sha=SOURCE,run=str(RUN),repo_root=str(args.repo_root),policy_module=str(HERE/'retained/loopx-w8-verify-native-artifact-ranged.py'),output_root=str(args.output_root),cache_root=str(args.cache_root),layer_workers=1)
        review=cls(original);review.run();lc.require(cancelled['signal'] is None,'cancelled completed review')
    except BaseException as error:
        if review is not None:review.v.dump(review.out/'failed-review.json',{'candidate_approved':False,'error_type':type(error).__name__,'error':driver.safe_error(error)})
        raise
    finally:
        for sig,h in handlers.items():signal.signal(sig,h)
    lc.require(cancelled['signal'] is None,'cancelled while restoring handlers')
if __name__=='__main__':main()

