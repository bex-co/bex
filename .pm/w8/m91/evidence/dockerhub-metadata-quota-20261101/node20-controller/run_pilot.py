"""One fixed local Node20 donor pilot. Import/--plan does not touch Docker."""
from pathlib import Path
import hashlib,importlib.util,json,os,re,shutil,signal,subprocess,sys,time
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent


def digest(path):
    value=hashlib.sha256()
    with Path(path).open('rb') as source:
        while body:=source.read(1024*1024):value.update(body)
    return value.hexdigest()


def helper_for(plan):
    path=Path(plan['source_root'])/'.github/scripts/native-common-qualify.py'
    spec=importlib.util.spec_from_file_location('fixed_native_qualification',path)
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    return module


def verify(plan,helper):
    source=Path(plan['source_root']);manifest=json.loads(Path(plan['source_manifest']).read_bytes())
    assert digest(plan['source_manifest'])==plan['source_manifest_sha256'] and manifest['source']==plan['source_sha']
    expected={name:{'bytes':row['bytes'],'sha256':row['sha256'],'mode':row['git_mode'],'type':'regular','permissions':row['permissions']} for name,row in manifest['files'].items()}
    assert len(expected)==300 and helper.source_inventory(source)==expected,'current physical source300 differs'
    assert digest(plan['test_binary'])==plan['test_binary_sha256']
    for name,value in plan['tools_sha256'].items():assert digest(Path(plan['tools_root'])/name)==value
    assert digest(Path(plan['host_buildx_plugin_dir'])/'docker-buildx')==plan['host_buildx_plugin_sha256']
    policy=json.loads((source/'lego/operator/internal/build/toolchain-freshness.json').read_bytes())
    assert [row['committed'] for row in policy['images'] if row['id']=='helper-buildkit']==[plan['buildkit_image']]
    selection=helper.profile_selection('node-donors','node-20')
    assert selection['go_timeout']=='90m' and selection['job_minutes']=='120'
    assert selection['selector']==r'^TestCommonNodeGeneratedImages$/^node\-20$'
    return selection


def stop_group(process):
    """Reap the leader AND prove the whole owned group gone."""
    def alive():
        process.poll() # Reap an exited leader before probing its Darwin group.
        try:os.killpg(process.pid,0);return True
        except ProcessLookupError:return False
        except PermissionError:return True # Unproved, never absence.
    process.poll()
    try:os.killpg(process.pid,signal.SIGTERM)
    except (ProcessLookupError,PermissionError):pass
    until=time.monotonic()+3
    while alive() and time.monotonic()<until:
        process.poll();time.sleep(.05)
    if alive():
        try:os.killpg(process.pid,signal.SIGKILL)
        except (ProcessLookupError,PermissionError):pass
    try:process.wait(timeout=5)
    except subprocess.TimeoutExpired:return False
    until=time.monotonic()+5
    while alive() and time.monotonic()<until:time.sleep(.05)
    return process.returncode is not None and not alive()


class Pilot:
    """Only this fixed owner's process group, runner, builder and outputs."""
    def __init__(self,plan,helper):
        self.p,self.h=plan,helper;self.root=Path(plan['run_root']);self.e=self.root/'evidence'
        self.owner=plan['owner'];self.node=self.owner+'0';self.builder='buildx_buildkit_'+self.node
        self.volume=self.builder+'_state';self.runner=self.owner+'-runner';self.cid=None
        self.config=self.root/(self.owner+'-docker-config');self.cancel=None;self.unknown=False
        self.proved_empty=False;self.groups={};self.cleaning=False;self.records=[];self.errors=[];self.phase=None;self.phase_at=0;self.started=time.monotonic();self.observed=0
        self.env=dict(os.environ,DOCKER_CONFIG=str(self.config),DOCKER_HOST=plan['endpoint'],PYTHONDONTWRITEBYTECODE='1');self.env.pop('DOCKER_CONTEXT',None)

    def save(self,name,value):
        try:self.h.dump(self.e/name,value)
        except BaseException as error:self.errors.append('artifact '+name+':'+type(error).__name__)

    def owned_size(self):return sum(p.stat().st_size for p in self.root.rglob('*') if p.is_file() and not p.is_symlink())

    def resource_reason(self):
        if self.cancel:return 'cancelled'
        if time.monotonic()-self.started>self.p['bounds']['whole_seconds']:return 'whole deadline'
        if self.owned_size()>self.p['bounds']['host_files_bytes']:return 'host file operating bound'
        if shutil.disk_usage(self.root).free<self.p['bounds']['remaining_host_floor_bytes']:return 'host free operating floor'
        return None

    def command(self,label,args,seconds,cleanup=False,observe=False):
        argv=['docker',*args];began=time.monotonic();bound=None;process=None;absent=False;out=err=b''
        if self.cleaning and self.groups:raise RuntimeError('prior client process group absence unproved')
        if not cleanup:
            reason=self.resource_reason()
            if reason:raise RuntimeError(reason+' before command')
        files=[]
        try:
            if cleanup:
                process=subprocess.Popen(argv,env=self.env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
                self.groups[process.pid]=process
                try:out,err=process.communicate(timeout=seconds)
                except subprocess.TimeoutExpired:bound='timeout'
            else:
                files=[(self.e/(label+'.stdout')).open('wb'),(self.e/(label+'.stderr')).open('wb')]
                process=subprocess.Popen(argv,env=self.env,stdout=files[0],stderr=files[1],start_new_session=True)
                self.groups[process.pid]=process
                while process.poll() is None:
                    reason=self.resource_reason()
                    if reason:bound=reason;break
                    if time.monotonic()-began>seconds:bound='command timeout';break
                    if observe:self.observe()
                    time.sleep(.2)
        finally:
            ended=time.monotonic()
            try:
                if process is not None:
                    absent=stop_group(process)
                    if absent:self.groups.pop(process.pid,None)
                    if cleanup and bound:out,err=process.communicate(timeout=2)
            finally:
                for file in files:file.close()
        if not cleanup:
            bound=bound or self.resource_reason()
            if ended-began>seconds:bound=bound or 'command timeout'
        if not cleanup:
            if label=='case':
                with (self.e/(label+'.stdout')).open('rb') as stream:stream.seek(max(0,(self.e/(label+'.stdout')).stat().st_size-1024*1024));out=stream.read()
                err=b'' # Full raw case stderr is retained on disk, never duplicated in memory.
            else:out=(self.e/(label+'.stdout')).read_bytes();err=(self.e/(label+'.stderr')).read_bytes()
        else:
            # Cleanup API happens before fallible artifact writes.
            try:(self.e/(label+'.stdout')).write_bytes(out);(self.e/(label+'.stderr')).write_bytes(err)
            except OSError as error:self.errors.append('cleanup output:'+type(error).__name__)
        row={'label':label,'argv':argv,'exit_code':process.returncode if process else None,'bound':bound,'elapsed_seconds':round(time.monotonic()-began,3),'direct_child_reaped':process is not None and process.returncode is not None,'group_absent':absent,'pgid':process.pid if process else None,'pending_pgids':sorted(self.groups),'stdout':label+'.stdout','stderr':label+'.stderr'}
        self.records.append(row);self.save('commands.json',self.records)
        if bound or not absent:raise RuntimeError(label+':'+str(bound or 'remaining group'))
        return row['exit_code'],out,err

    def inspect(self,kind,name,label):
        code,out,err=self.command(label,[kind,'inspect',name],10,True)
        if code==0:
            rows=json.loads(out);assert len(rows)==1;return rows[0]
        messages={'container':('Error response from daemon: No such container: '+name,'Error: No such container: '+name,'Error: No such object: '+name),'volume':('Error response from daemon: get '+name+': no such volume','Error response from daemon: No such volume: '+name)}
        if code==1 and out.strip() in (b'',b'[]') and err.decode().strip() in messages[kind]:return None
        raise RuntimeError(label+': absence/identity unproved')

    def observe(self):
        phase_path=self.e/'phase';phase=phase_path.read_text().strip() if phase_path.exists() else 'starting'
        now=time.monotonic()
        if phase!=self.phase:self.phase,self.phase_at=phase,now
        limits={'starting':60,'alias-preflight':30,'pull':195,'bootstrap':300,'case':5430}
        if phase not in limits or now-self.phase_at>limits[phase]:raise RuntimeError('phase deadline:'+phase)
        if now-self.observed<self.p['bounds']['observe_interval_seconds']:return
        self.observed=now
        row=self.inspect('container',self.builder,'builder-observe-'+str(len(self.records)))
        if row is None:return
        assert row['Config']['Image']==self.p['buildkit_image'] and row['Name']=='/'+self.builder
        mounts=[x for x in row['Mounts'] if x['Destination']=='/var/lib/buildkit'];assert len(mounts)==1 and mounts[0]['Name']==self.volume
        if row['State']['Running']:
            code,out,_=self.command('builder-du-'+str(len(self.records)),['exec',self.builder,'du','-sk','/var/lib/buildkit'],15,True)
            if code or not re.fullmatch(rb'\d+\s+/var/lib/buildkit\n?',out):raise RuntimeError('owned builder measurement unavailable')
            if int(out.split()[0])*1024>self.p['bounds']['builder_state_polled_bytes']:raise RuntimeError('owned builder state operating bound')
            self.command('owned-stats-'+str(len(self.records)),['stats','--no-stream','--format','{{json .}}',self.builder,self.runner],10,True)

    def stop_runner(self):
        row=self.inspect('container',self.runner,'runner-cleanup-before')
        if row is not None:
            assert row['Name']=='/'+self.runner and row['Config']['Labels'].get(self.h.LABEL)==self.owner and row['Image']==self.p['runner_image']
            if self.cid is not None:assert row['Id']==self.cid
            if row['State']['Running']:
                try:self.command('runner-stop',['stop','--time','10',row['Id']],20,True)
                except BaseException as error:self.errors.append('runner stop:'+type(error).__name__)
            code,_,_=self.command('runner-remove',['rm','--force',row['Id']],15,True);assert code==0
        assert self.inspect('container',self.runner,'runner-cleanup-after') is None

    def cleanup(self):
        # No daemon cleanup while any earlier client/observer group is unproved.
        for identity,process in list(self.groups.items()):
            try:
                if stop_group(process):self.groups.pop(identity,None)
            except BaseException as error:self.errors.append('client group:'+type(error).__name__)
        if self.groups:
            self.errors.append('client process group cleanup pending');self.save('pending-client-groups.json',{'stage':'prior-clients','pgids':sorted(self.groups)});return
        self.cleaning=True
        # Stop/reap the runner and all in-container clients before builder removal.
        if not self.proved_empty:
            if self.config.exists():self.h.owned_docker_config(self.e,{'owner':self.owner,'docker_config':str(self.config)});shutil.rmtree(self.config)
            return # Before-absence failed: no authority to touch existing resources.
        try:self.stop_runner()
        except BaseException as error:
            self.errors.append('runner cleanup pending:'+type(error).__name__)
            return # Preserve exact builder/config for reconciliation; no removal before client absence.
        state=self.config/'buildx/instances'/self.owner
        if state.is_file():
            try:self.save('own-builder-state.json',json.loads(state.read_bytes()))
            except BaseException as error:self.errors.append('state receipt:'+type(error).__name__)
        try:
            argv=[sys.executable,str(Path(self.p['source_root'])/'.github/scripts/native-common-qualify.py'),'cleanup',str(self.e)]
            # The existing helper runs in its own bounded process group as well.
            out=subprocess.Popen(argv,env=self.env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
            self.groups[out.pid]=out
            try:stdout,stderr=out.communicate(timeout=180)
            except subprocess.TimeoutExpired:
                stdout=stderr=b'';self.errors.append('helper cleanup timeout')
            finally:
                if stop_group(out):self.groups.pop(out.pid,None)
                else:raise RuntimeError('helper cleanup process group remains')
            stdout,stderr=out.communicate(timeout=2)
            self.save('helper-cleanup-command.json',{'argv':argv,'exit_code':out.returncode,'stdout':stdout.decode(errors='replace'),'stderr':stderr.decode(errors='replace')})
            if out.returncode:self.errors.append('existing cleanup helper nonzero')
        except BaseException as error:self.errors.append('helper cleanup:'+type(error).__name__)
        if self.groups:
            self.errors.append('helper/client process group cleanup pending');self.save('pending-client-groups.json',{'stage':'helper-client','pgids':sorted(self.groups)});return
        # Reconcile only exact nonce-derived builder resources; preserve helper
        # errors. Before-absence and pinned builder image prevent foreign cleanup.
        try:
            row=self.inspect('container',self.builder,'builder-cleanup-before')
            if row:
                assert row['Config']['Image']==self.p['buildkit_image'] and row['Name']=='/'+self.builder
                mounts=[x for x in row['Mounts'] if x['Destination']=='/var/lib/buildkit'];assert len(mounts)==1 and mounts[0]['Name']==self.volume
                code,_,_=self.command('builder-remove',['rm','--force',row['Id']],20,True);assert code==0
            assert self.inspect('container',self.builder,'builder-cleanup-after') is None
            volume=self.inspect('volume',self.volume,'volume-cleanup-before')
            if volume:
                assert volume['Name']==self.volume
                code,_,_=self.command('volume-remove',['volume','rm',self.volume],20,True);assert code==0
            assert self.inspect('volume',self.volume,'volume-cleanup-after') is None
            for kind,listing,removal in [('container',['ps','-aq'],['rm','--force']),('image',['image','ls','-q'],['image','rm','--force'])]:
                code,out,_=self.command(kind+'-owned-final',listing+['--filter','label='+self.h.LABEL+'='+self.owner],20,True);assert code==0
                for identity in sorted(set(out.decode().split())):
                    assert re.fullmatch(r'(?:sha256:)?[a-f0-9]{12,64}',identity)
                    code,_,_=self.command(kind+'-late-remove-'+identity.replace(':','-'),removal+[identity],20,True);assert code==0
                code,out,_=self.command(kind+'-owned-absence',listing+['--filter','label='+self.h.LABEL+'='+self.owner],20,True);assert code==0 and not out.strip()
        except BaseException as error:self.errors.append('owned reconciliation:'+type(error).__name__)
        if self.groups:
            self.errors.append('owned cleanup client process group pending; config retained');self.save('pending-client-groups.json',{'stage':'owned-reconciliation','pgids':sorted(self.groups)});return
        if self.config.exists():
            try:self.h.owned_docker_config(self.e,{'owner':self.owner,'docker_config':str(self.config)});shutil.rmtree(self.config)
            except BaseException as error:self.errors.append('own config cleanup:'+type(error).__name__)
        if self.unknown:self.errors.append('unknown create outcome retained')

    def execute(self):
        selection=verify(self.p,self.h);assert not self.root.exists();self.root.mkdir(mode=0o700);self.e.mkdir(mode=0o700);self.config.mkdir(mode=0o700)
        self.h.dump(self.config/'.qualification-owner.json',{'owner':self.owner,'evidence_root':str(self.e)})
        self.h.dump(self.config/'config.json',{'cliPluginsExtraDirs':[self.p['host_buildx_plugin_dir']]})
        for name in ('private-tmp','gotmp'):(self.root/name).mkdir(mode=0o700)
        inputs={'owner':self.owner,'docker_config':str(self.config),'scope':'genuine local onecase pilot; no CI execution identity','common_image':self.p['common_image'],'generator_source':self.p['source_sha'],'platform':self.p['platform'],'selection':selection,'source_manifest_sha256':self.p['source_manifest_sha256'],'cache_scope':self.p['cache_scope']}
        self.h.dump(self.e/'inputs.json',inputs)
        handlers={};failure=None;case_pass=False
        try:
            for sig in (signal.SIGTERM,signal.SIGINT):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda signum,frame:setattr(self,'cancel',signum))
            os.chdir(Path(self.p['source_root'])/'lego/operator/internal/build');self.h.capacity(self.e,'before-builder')
            assert self.inspect('container',self.runner,'before-runner') is None and self.inspect('container',self.builder,'before-builder') is None and self.inspect('volume',self.volume,'before-volume') is None
            self.proved_empty=True
            self.command('helper-cache-before',['image','inspect',self.p['buildkit_image']],15,True) # Observation, not cold-cache admission.
            env={row[1]:'0' for row in self.h.PROFILES.values()};env.update(BEX_NATIVE_TEST_NODE_DONORS='1',BEX_BUILDKIT_INTEGRATION='1',BEX_NATIVE_TEST_COMMON_IMAGE=self.p['common_image'],BEX_NATIVE_TEST_OWNER=self.owner,BEX_NATIVE_TEST_BUILDER=self.owner,BEX_NATIVE_TEST_PLATFORM='linux/arm64',BEX_NATIVE_TEST_OUTPUT=str(self.e/'generated'),DOCKER_CONFIG=str(self.config),DOCKER_HOST=self.p['endpoint'],TMPDIR=str(self.root/'private-tmp'),GOTMPDIR=str(self.root/'gotmp'),GOMAXPROCS='2',GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',PILOT_EVIDENCE=str(self.e),PILOT_NODE=self.node,PILOT_BUILDER_CONTAINER=self.builder,PILOT_BUILDKIT=self.p['buildkit_image'])
            args=['create','--name',self.runner,'--label',self.h.LABEL+'='+self.owner,'--pull=never','--platform','linux/arm64','--read-only','--network','bridge','--memory','512m','--cpus','2','--pids-limit','128','--tmpfs','/tmp:rw,nosuid,nodev,size=64m','--workdir',str(Path(self.p['source_root'])/'lego/operator/internal/build')]
            mounts=[(self.root,self.root,False),(self.p['source_root'],self.p['source_root'],True),(self.p['test_binary'],'/bex-build.test',True),(Path(self.p['tools_root'])/'docker','/usr/local/bin/docker',True),(Path(self.p['tools_root'])/'docker-buildx','/usr/libexec/docker/cli-plugins/docker-buildx',True),('/var/run/docker.sock',self.p['inside_socket_destination'],True),(HERE/'pilot.sh','/pilot.sh',True)]
            for src,dst,ro in mounts:args+=['--mount','type=bind,src='+str(src)+',dst='+str(dst)+(',readonly' if ro else '')]
            for key,value in env.items():args+=['--env',key+'='+value]
            args+=['--entrypoint','/bin/sh',self.p['runner_image'],'/pilot.sh']
            self.unknown=True;code,out,_=self.command('runner-create',args,30);assert code==0 and re.fullmatch(rb'[a-f0-9]{64}\n?',out)
            self.cid=out.decode().strip();self.unknown=False # In-memory custody before optional artifacts.
            self.save('runner-cid.json',{'id':self.cid,'name':self.runner,'owner':self.owner})
            code,out,_=self.command('case',['start','--attach',self.cid],self.p['bounds']['whole_seconds'],observe=True);assert code==0 and b'--- PASS: TestCommonNodeGeneratedImages/node-20 ' in out
            receipts=list((self.e/'generated').glob('native-common-node-*/node-20/receipt.json'));assert len(receipts)==1
            receipt=json.loads(receipts[0].read_bytes());custody=json.loads((receipts[0].parent/'source-custody.json').read_bytes())
            assert receipt['common_base']==self.p['common_image'] and receipt['image_platform']=='linux/arm64' and receipt['network']=='none' and receipt['runtime_command_executed'] is True
            assert custody['runtime_artifact_mode']=='published-runtime-donors' and custody['node_runtime_artifact']==self.p['reviewed_node20_donor'] and custody['node_source']==self.p['official_node20'] and custody['test_only_network_overlay'] is False and custody['canonical_sha256']==custody['executed_sha256']
            late=self.resource_reason()
            if late:raise RuntimeError(late)
            case_pass=True
        except BaseException as error:failure=type(error).__name__+':'+str(error)
        finally:
            self.cleanup()
            for sig,handler in handlers.items():signal.signal(sig,handler)
            try:verify(self.p,self.h);self.h.capacity(self.e,'after-tests')
            except BaseException as error:self.errors.append('final source/capacity:'+type(error).__name__)
            late=self.resource_reason()
            if late:failure=failure or late
            payload={'PASS':case_pass and failure is None and not self.errors,'scope':'ONE current donorNode20 ARM64 local generated case only; no adoption/botharch/fullfamily/hosted/K8s claim','failure':failure,'cleanup_or_artifact_errors':self.errors,'owner':self.owner,'case_pass':case_pass,'cancel_signal':self.cancel,'pending_pgids':sorted(self.groups),'elapsed_seconds':round(time.monotonic()-self.started,3)}
            self.save('RESULT.json',payload)
            self.h.manifest(self.e)
            late=self.resource_reason()
            if late:
                failure=failure or late;payload.update(PASS=False,failure=failure)
                self.save('RESULT.json',payload);self.h.manifest(self.e)
        return case_pass and failure is None and not self.errors


def main():
    plan=json.loads((HERE/'PLAN.json').read_bytes())
    if sys.argv[1:]!=['--execute']:
        print(json.dumps(plan,indent=2));return 0
    return 0 if Pilot(plan,helper_for(plan)).execute() else 1


if __name__=='__main__':raise SystemExit(main())
