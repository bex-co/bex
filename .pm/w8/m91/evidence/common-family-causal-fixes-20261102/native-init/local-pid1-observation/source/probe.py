"""One fixed PID1/binfmt observation; import and --plan perform no Docker actions."""
from pathlib import Path
import hashlib,importlib.util,json,os,re,shutil,signal,struct,sys,time
HERE=Path(__file__).resolve().parent
IMAGE='sha256:ab3fe4defd29ba6231229a4d41440ac8bde8218e85870e53876277faa24b35c4'
FORMAT='{"Id":{{json .Id}},"Image":{{json .Image}},"Path":{{json .Path}},"Args":{{json .Args}},"Config":{"Entrypoint":{{json .Config.Entrypoint}},"Cmd":{{json .Config.Cmd}},"Labels":{{json .Config.Labels}}},"State":{{json .State}},"Mounts":{{json .Mounts}}}'
def digest(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def load(path):return json.loads(Path(path).read_bytes())
def require(ok,reason):
    if not ok:raise RuntimeError(reason)
def module(path,name):
    spec=importlib.util.spec_from_file_location(name,path);out=importlib.util.module_from_spec(spec);spec.loader.exec_module(out);return out

def execute():
    require(not sys.flags.optimize,'ordinary Python required')
    plan=load(HERE/'PLAN.json');dependency=plan['reused_controller'];require(digest(dependency['path'])==dependency['sha256'],'reviewed process controller changed')
    helper=Path(plan['source_root'])/'.github/scripts/native-common-qualify.py';require(digest(helper)==plan['helper_sha256'],'reviewed receipt helper changed')
    original=module(dependency['path'],'reviewed_pilot_lifecycle');h=module(helper,'reviewed_qualification_receipts')
    target=Path(plan['target']['path']);require(not target.is_symlink() and target.is_file(),'actual retained target missing')
    data=target.read_bytes();require(digest(target)==plan['target']['sha256'] and len(data)==1753250 and data[:6]==b'\x7fELF\x02\x01' and struct.unpack_from('<H',data,18)[0]==62,'actual AMD target differs')
    require(plan['cached_image']==IMAGE and re.fullmatch(r'bex-init-observe-[a-f0-9]{16}',plan['owner']) and plan['container_name']==plan['owner'],'fixed observation identity differs')
    require(plan['run_root']=='/private/tmp/'+plan['owner'],'fixed owned runtime differs')
    root=Path(plan['run_root']);root.mkdir(mode=0o700);(root/'evidence').mkdir(mode=0o700)
    class Observation(original.Pilot):
        def __init__(self):
            super().__init__(plan,h);self.runner=self.owner;self.created=False;self.absence=False;self.intent=False
        def resource_reason(self):
            reason=super().resource_reason()
            if reason:return reason
            if not self.cleaning and time.monotonic()-self.started>plan['bounds']['work_seconds']:return 'observation work deadline'
            size=sum(p.stat().st_size for p in self.e.iterdir() if p.is_file() and p.suffix in ('.stdout','.stderr'))
            if size>1<<20:return 'combined stdout/stderr1MiB bound'
            return None
        def inspect_own(self,label):
            code,out,err=self.command(label,['container','inspect','--format',FORMAT,self.runner],5,True)
            if code==1 and out.strip() in (b'',b'[]') and err.decode().strip() in ('Error response from daemon: No such container: '+self.runner,'Error: No such container: '+self.runner,'Error: No such object: '+self.runner):return None
            require(code==0,'exact container inspection unproved');obj=json.loads(out)
            require(obj['Image']==IMAGE and obj['Config']['Labels'].get(h.LABEL)==self.owner,'foreign/mismatched container; never remove')
            require(re.fullmatch(r'[a-f0-9]{64}',obj['Id']) and (self.cid is None or obj['Id']==self.cid),'exact returned CID differs')
            return obj
        def cleanup(self):
            self.cleaning=True
            for pid,p in list(self.groups.items()):
                if original.stop_group(p):self.groups.pop(pid,None)
            if self.groups:self.errors.append('pending client groups; no daemon cleanup');return
            if self.proved_empty and self.intent:
                try:
                    if self.cid is None:
                        obj=self.inspect_own('unknown-create-reconcile')
                        if obj is not None:self.cid=obj['Id'];self.unknown=False
                        elif self.unknown:self.errors.append('unknown create outcome;404 is not cleanup proof')
                    if self.cid is not None:
                        code,_,_=self.command('exact-remove',['rm','--force',self.cid],5,True)
                        if code:self.errors.append('exact returned CID removal nonzero')
                    self.absence=self.inspect_own('exact-absence') is None
                    if not self.absence:self.errors.append('exact container remains')
                except BaseException as error:self.errors.append('container cleanup:'+type(error).__name__)
            elif self.proved_empty:self.absence=True
            if self.groups:self.errors.append('late cleanup client group pending; preserve config');return
            if self.absence and not self.unknown:
                try:
                    require(load(self.config/'owner.json')=={'owner':self.owner,'runtime':str(self.root)},'owned config marker differs')
                    shutil.rmtree(self.config)
                except BaseException as error:self.errors.append('own config cleanup:'+type(error).__name__)
    probe=Observation();failure=None;previous={};result=None
    try:
        probe.config.mkdir(mode=0o700);h.dump(probe.config/'owner.json',{'owner':probe.owner,'runtime':str(root)});h.dump(probe.config/'config.json',{})
        stage=root/'input';stage.mkdir(mode=0o700);(stage/'native-init').write_bytes(data);(stage/'native-init').chmod(0o755);(stage/'observe.sh').write_bytes((HERE/'observe.sh').read_bytes())
        h.dump(probe.e/'INPUTS.json',{'plan':plan,'target_sha256':digest(stage/'native-init'),'script_sha256':digest(stage/'observe.sh'),'scope':'mixed-rootfs observation only; no app/native-init qualification'})
        def cancel(signum,frame):
            if probe.cancel is None:probe.cancel=signum
        for signum in (signal.SIGINT,signal.SIGTERM):previous[signum]=signal.getsignal(signum);signal.signal(signum,cancel)
        imageformat='{"Id":{{json .Id}},"Os":{{json .Os}},"Architecture":{{json .Architecture}}}'
        code,out,_=probe.command('cached-image',['image','inspect','--format',imageformat,IMAGE],6);obj=json.loads(out)
        require(code==0 and obj=={'Id':IMAGE,'Os':'linux','Architecture':'arm64'},'cached exact ARM image missing; no pull')
        code,out,_=probe.command('actual-daemon',['info','--format','{"Architecture":{{json .Architecture}},"OSType":{{json .OSType}}}'],6);obj=json.loads(out)
        require(code==0 and obj['OSType']=='linux' and obj['Architecture'] in ('aarch64','arm64'),'actual daemon is not ARM Linux')
        require(probe.inspect_own('before-absence') is None,'exact name already exists; no authority');probe.proved_empty=True
        probe.save('CREATE-INTENT.json',{'name':probe.runner,'image':IMAGE,'label':{h.LABEL:probe.owner},'entrypoint':'/native-init','args':['/bin/sh','/observe.sh']})
        require(not probe.errors,'intent artifact failed before create');probe.intent=True;probe.unknown=True
        args=['create','--pull=never','--platform','linux/arm64','--name',probe.runner,'--label',h.LABEL+'='+probe.owner,'--read-only','--network','none','--memory','128m','--cpus','1','--pids-limit','32','--tmpfs','/tmp:rw,nosuid,nodev,size=8388608','--mount','type=bind,source='+str(stage/'native-init')+',target=/native-init,readonly','--mount','type=bind,source='+str(stage/'observe.sh')+',target=/observe.sh,readonly','--entrypoint','/native-init',IMAGE,'/bin/sh','/observe.sh']
        code,out,_=probe.command('create',args,6)
        require(code==0 and re.fullmatch(rb'[a-f0-9]{64}\n?',out),'create outcome unproved')
        probe.cid=out.decode().strip();probe.created=True;probe.unknown=False # Before every optional artifact operation.
        probe.save('RETURNED-CID.json',{'id':probe.cid,'owner':probe.owner})
        obj=probe.inspect_own('before-start');require(obj['Path']=='/native-init' and obj['Args']==['/bin/sh','/observe.sh'] and obj['Config']['Entrypoint']==['/native-init'],'actual entrypoint/argv differs')
        code,_,_=probe.command('attach',['start','--attach',probe.cid],30)
        require(code==0,'mixed-rootfs observation exited nonzero; no fallback')
        obj=probe.inspect_own('after-exit');require(obj is not None and not obj['State']['Running'] and obj['State']['ExitCode']==0,'probe still active or nonzero')
    except BaseException as error:failure=type(error).__name__+':'+str(error)
    finally:
        try:probe.cleanup()
        except BaseException as error:probe.errors.append('final cleanup:'+type(error).__name__)
        for signum,handler in reversed(list(previous.items())):
            try:signal.signal(signum,handler)
            except BaseException as error:probe.errors.append('signal restore:'+type(error).__name__)
        elapsed=time.monotonic()-probe.started
        if elapsed>120:probe.errors.append('whole120s exceeded')
        if probe.cancel is not None:probe.errors.append('cancelled')
        if probe.groups:probe.errors.append('process group absence unproved')
        if sum(p.stat().st_size for p in probe.e.iterdir() if p.is_file() and p.suffix in ('.stdout','.stderr'))>1<<20:probe.errors.append('final combined output1MiB bound')
        result={'complete':failure is None and not probe.errors and probe.absence,'failure':failure,'errors':probe.errors,'owner':probe.owner,'cid':probe.cid,'unknown_create':probe.unknown,'pending_pgids':sorted(probe.groups),'exact_container_absence_proved':probe.absence,'own_config_absent':not probe.config.exists(),'cancel_signal':probe.cancel,'elapsed_seconds':elapsed,'scope':'Raw mixed-rootfs observation only; complete does not approve an emulator predicate or qualify native-init'}
        probe.save('RESULT.json',result)
        if probe.errors or probe.cancel is not None or time.monotonic()-probe.started>120:
            result.update(complete=False,cancel_signal=probe.cancel,elapsed_seconds=time.monotonic()-probe.started)
            probe.save('RESULT.json',result)
    print(json.dumps({'root':str(root),'result':result},indent=2));return 0 if result['complete'] else 2
if __name__=='__main__':
    if sys.argv[1:]!=['--execute']:print((HERE/'PLAN.json').read_text());raise SystemExit(0)
    raise SystemExit(execute())
