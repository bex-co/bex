"""Fixed OTP input diagnostic. --plan is read-only; Root owns later --execute."""
from pathlib import Path
import hashlib,json,os,re,signal,subprocess,sys,time
HERE=Path(__file__).resolve().parent


def digest(path):
    h=hashlib.sha256()
    with Path(path).open('rb') as stream:
        while chunk:=stream.read(1<<20):h.update(chunk)
    return h.hexdigest()


# Exact reviewed group cleanup copied below from the bound source receipt.
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


def shell_quote(text):
    return "'"+text.replace("'", "'\\''")+"'"


class Probe:
    def __init__(self,plan):
        self.p=plan;self.root=Path(plan['run_root']);self.e=self.root/'evidence';self.config=self.root/'docker-config'
        self.groups={};self.records=[];self.sequence=0;self.cancel=None;self.errors=[];self.claimed={};self.finished={};self.started=time.monotonic();self.peak=0;self.minimum=None
        self.env={'PATH':'/usr/local/bin:/usr/bin:/bin','DOCKER_CONFIG':str(self.config),'DOCKER_HOST':plan['endpoint'],'DOCKER_CLI_HINTS':'false'}
        self.prefix=[plan['host_docker'],'--host',plan['endpoint'],'--config',str(self.config)]

    def save(self,name,value):
        (self.e/name).write_text(json.dumps(value,indent=2)+'\n')

    def reason(self):
        total=sum(f.stat().st_size for f in self.root.rglob('*') if f.is_file() and not f.is_symlink());self.peak=max(self.peak,total)
        fs=os.statvfs(self.root);free=fs.f_bavail*fs.f_frsize;self.minimum=free if self.minimum is None else min(self.minimum,free)
        if self.cancel:return 'cancelled'
        if free<self.p['bounds']['physical_free_floor_bytes']:return 'physical free floor'
        if total>=self.p['bounds']['host_stop_bytes']:return 'host output bound'
        if time.monotonic()-self.started>self.p['bounds']['whole_seconds']:return 'whole deadline'
        return None

    def command(self,label,args,seconds,cleanup=False):
        if self.groups:raise RuntimeError('unproved prior client groups')
        if not cleanup and (why:=self.reason()):raise RuntimeError(why)
        self.sequence+=1;label=str(self.sequence).zfill(3)+'-'+label
        process=None;bound=None;absent=False;began=time.monotonic();argv=self.prefix+args
        with (self.e/(label+'.stdout')).open('xb') as out,(self.e/(label+'.stderr')).open('xb') as err:
            try:
                process=subprocess.Popen(argv,stdin=subprocess.DEVNULL,stdout=out,stderr=err,env=self.env,start_new_session=True)
                self.groups[process.pid]=process
                while process.poll() is None:
                    if not cleanup and (why:=self.reason()):bound=why;break
                    if time.monotonic()-began>=seconds:bound='timeout';break
                    time.sleep(.1)
            finally:
                if process is not None:
                    absent=stop_group(process)
                    if absent:self.groups.pop(process.pid,None)
        if not cleanup:bound=bound or self.reason()
        row={'label':label,'argv':argv,'exit_code':process.returncode if process else None,'bound':bound,'elapsed_seconds':time.monotonic()-began,'pgid':process.pid if process else None,'group_absent':absent,'direct_child_reaped':process is not None and process.returncode is not None,'pending_pgids':sorted(self.groups),'stdout':label+'.stdout','stderr':label+'.stderr'}
        self.records.append(row);self.save('commands.json',self.records)
        if bound or not absent:raise RuntimeError(label+':'+str(bound or 'unproved group'))
        return row,(self.e/(label+'.stdout')).read_bytes(),(self.e/(label+'.stderr')).read_bytes()

    def inspect(self,name,label):
        row,out,err=self.command(label,['container','inspect',name],15,True)
        if row['exit_code']==0:
            value=json.loads(out);assert type(value) is list and len(value)==1;return value[0]
        expected=('Error response from daemon: No such container: '+name,'Error: No such container: '+name,'Error: No such object: '+name)
        if row['exit_code']==1 and out.strip() in (b'',b'[]') and err.decode().strip() in expected:return None
        raise RuntimeError(label+': container presence unknown')

    def remove(self,name):
        row=self.inspect(name,'cleanup-before-'+name)
        if row is not None:
            own=self.claimed[name]
            assert row['Name']=='/'+name and row['Config']['Labels'].get(self.p['label'])==self.p['owner']
            assert row['Config']['Image']==own['image']
            if own['cid'] is not None:assert row['Id']==own['cid']
            cid=row['Id'];own['cid']=cid;self.save('owned.json',self.claimed)
            command,_,_=self.command('remove-'+name,['rm','--force',cid],25,True);assert command['exit_code']==0
        assert self.inspect(name,'cleanup-after-'+name) is None
        self.claimed[name]['absence_proved']=True;self.save('owned.json',self.claimed)

    def create_start(self,name,image,create,payload,seconds):
        assert self.inspect(name,'before-'+name) is None
        self.claimed[name]={'image':image,'cid':None,'absence_proved':False};self.save('owned.json',self.claimed)
        row,out,_=self.command('create-'+name,['create','--pull=never','--name',name,'--label',self.p['label']+'='+self.p['owner'],*create,image,*payload],30)
        assert row['exit_code']==0 and re.fullmatch(rb'[a-f0-9]{64}\n?',out)
        cid=out.decode().strip();self.claimed[name]['cid']=cid;self.save('owned.json',self.claimed)
        record=self.inspect(name,'created-'+name)
        assert record is not None and record['Id']==cid and record['Config']['Image']==image and record['Config']['Labels'].get(self.p['label'])==self.p['owner']
        row,out,err=self.command('run-'+name,['start','--attach',cid],seconds)
        final=self.inspect(name,'finished-'+name);assert final is not None and final['Id']==cid and not final['State']['Running'] and final['State']['ExitCode']==row['exit_code']
        self.finished[name]={'exit_code':row['exit_code'],'stdout':out.decode(errors='replace'),'stderr':err.decode(errors='replace'),'container_state':final['State']}
        self.save('observations.json',self.finished);self.remove(name)
        return row,out,err

    def cleanup(self):
        for pgid,process in list(self.groups.items()):
            try:
                if stop_group(process):self.groups.pop(pgid,None)
            except BaseException as error:self.errors.append('group:'+type(error).__name__)
        if self.groups:self.errors.append('pending client groups; no daemon/config cleanup');return
        for name in self.claimed:
            if self.claimed[name]['absence_proved']:continue
            try:self.remove(name)
            except BaseException as error:self.errors.append('container '+name+':'+type(error).__name__)
            if self.groups:break
        if not self.groups and all(v['absence_proved'] for v in self.claimed.values()) and self.config.exists():
            assert not self.config.is_symlink() and self.config.parent==self.root
            assert json.loads((self.config/'.owner.json').read_text())=={'owner':self.p['owner'],'run_root':str(self.root)}
            assert {f.name for f in self.config.iterdir()}=={'.owner.json','config.json'}
            for f in self.config.iterdir():assert not f.is_symlink() and f.is_file();f.unlink()
            self.config.rmdir()

    def execute(self):
        assert not self.root.exists();self.root.mkdir(mode=0o700);self.e.mkdir(mode=0o700)
        original={};failure=None;before={str(HERE/'PLAN.json'):digest(HERE/'PLAN.json'),str(Path(__file__)):digest(__file__)}
        try:
            for path,sha in self.p['source_inputs'].items():assert digest(path)==sha
            self.config.mkdir(mode=0o700);(self.config/'config.json').write_text('{}\n');(self.config/'.owner.json').write_text(json.dumps({'owner':self.p['owner'],'run_root':str(self.root)}))
            for f in self.config.iterdir():f.chmod(0o600)
            for signum in (signal.SIGINT,signal.SIGTERM):
                original[signum]=signal.getsignal(signum);signal.signal(signum,lambda n,_:setattr(self,'cancel',n))
            assert self.reason() is None
            image=self.p['donor_image'];owner=self.p['owner']
            pull_create=['--network','bridge','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--cpus','1','--memory','128m','--pids-limit','64','--log-driver','none','--tmpfs','/tmp:rw,nosuid,nodev,size=8388608','--mount','type=bind,src='+self.p['linux_docker']+',dst=/docker,readonly','--mount','type=bind,src='+str(self.config)+',dst=/docker-config,readonly','--mount','type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock','--entrypoint','/docker']
            payload=['--host','unix:///var/run/docker.sock','--config','/docker-config','pull','--quiet','--platform','linux/amd64',image]
            pull,_,_=self.create_start(owner+'-pull',self.p['pull_helper_image'],pull_create,payload,600);assert pull['exit_code']==0
            row,out,_=self.command('cached-image',['image','inspect',image],15);assert row['exit_code']==0
            actual=json.loads(out);assert len(actual)==1 and actual[0]['Id']==self.p['expected_config'] and actual[0]['Architecture']=='amd64' and actual[0]['Os']=='linux' and actual[0]['Size']<=self.p['bounds']['pulled_image_postcheck_bytes']
            hold=owner+'-hold'
            assert self.inspect(hold,'hold-before') is None
            self.claimed[hold]={'image':image,'cid':None,'absence_proved':False};self.save('owned.json',self.claimed)
            hold_args=['create','--pull=never','--name',hold,'--label',self.p['label']+'='+owner,'--platform','linux/amd64','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--cpus','2','--memory','512m','--pids-limit','256','--log-driver','none','--tmpfs','/tmp:rw,nosuid,nodev,size=134217728','--entrypoint','/bin/true',image]
            row,out,_=self.command('hold-create',hold_args,30);assert row['exit_code']==0 and re.fullmatch(rb'[a-f0-9]{64}\n?',out)
            self.claimed[hold]['cid']=out.decode().strip();self.save('owned.json',self.claimed)
            holding=self.inspect(hold,'hold-created')
            assert holding is not None and holding['Id']==self.claimed[hold]['cid'] and holding['Name']=='/'+hold and holding['Image']==self.p['expected_config'] and holding['Config']['Image']==image and holding['Config']['Labels'].get(self.p['label'])==owner and holding['State']['Status']=='created' and holding['State']['Running'] is False and holding['State']['Pid']==0
            self.save('provider-hold.json',{'before':holding,'purpose':'Never-started owned image reference during all diagnostics; not immunity to forced external image deletion.'})
            for variant in self.p['variants']:
                name=variant['name'];output=self.e/('diagnostic-'+name);output.mkdir(mode=0o700)
                create=['--platform','linux/amd64','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges','--cpus','2','--memory','512m','--pids-limit','256','--log-driver','none','--tmpfs','/tmp:rw,nosuid,nodev,size=134217728','--workdir','/tmp','--mount','type=bind,src='+str(output)+',dst=/owned-diagnostic','--entrypoint','/usr/bin/env']
                environment=['-u','LANG','-u','LC_ALL','-u','LC_CTYPE','-u','ERL_AFLAGS','-u','ERL_FLAGS','-u','ERL_ZFLAGS','ERL_CRASH_DUMP=/owned-diagnostic/erl_crash.dump','ERL_CRASH_DUMP_BYTES=8388608','ERL_CRASH_DUMP_SECONDS=5']+variant['environment']
                wrapper='code=0; /bin/sh -ec '+shell_quote(variant['program'])+' || code=$?; if [ -f /owned-diagnostic/erl_crash.dump ]; then chmod 644 /owned-diagnostic/erl_crash.dump || printf "DUMP_CHMOD_FAILED\\n" >&2; fi; exit "$code"'
                self.create_start(owner+'-'+name,image,create,environment+['/bin/sh','-ec',wrapper],30)
            holding_after=self.inspect(hold,'hold-after-diagnostics')
            assert holding_after is not None and holding_after['Id']==self.claimed[hold]['cid'] and holding_after['Image']==self.p['expected_config'] and holding_after['State']['Status']=='created' and holding_after['State']['Running'] is False and holding_after['State']['Pid']==0
            self.save('provider-hold.json',{'before':holding,'after':holding_after,'purpose':'Never-started owned image reference held through all diagnostics; finally removes it.'})
        except BaseException as error:failure=type(error).__name__+':'+str(error)
        finally:
            try:self.cleanup()
            except BaseException as error:self.errors.append('cleanup:'+type(error).__name__)
            for signum,handler in original.items():signal.signal(signum,handler)
        final_reason=self.reason();unchanged=all(digest(path)==sha for path,sha in before.items()) and all(digest(path)==sha for path,sha in self.p['source_inputs'].items())
        complete=failure is None and not self.errors and not self.groups and not self.config.exists() and final_reason is None and unchanged and all(self.p['owner']+'-'+v['name'] in self.finished for v in self.p['variants'])
        dumps=[]
        for f in sorted(self.e.glob('diagnostic-*/erl_crash.dump')):
            raw=f.read_bytes();dumps.append({'path':str(f.relative_to(self.e)),'bytes':len(raw),'sha256':digest(f),'ends_with_dump_end_marker':raw.rstrip().endswith(b'=end'),'declared_maximum_bytes':8388608,'capture':'Actual dump bytes ASIS; a capped or incomplete dump is not full-state proof.'})
        result={'probe_complete':complete,'fix_established':False,'common_context_fix_proven':False,'failure':failure,'cleanup_errors':self.errors,'pending_pgids':sorted(self.groups),'source_unchanged':unchanged,'stop':final_reason,'cancel':self.cancel,'config_absent':not self.config.exists(),'containers':self.claimed,'observations':self.finished,'dumps':dumps,'peak_host_file_bytes':self.peak,'minimum_physical_free_bytes':self.minimum,'scope':'Cached selected provider AMD64 runtime under ARM-host emulation; diagnostic results only, not production/native-architecture acceptance.'}
        self.save('RESULT.json',result)
        late=self.reason()
        if late and not result['stop']:result['stop']=late;result['probe_complete']=False;self.save('RESULT.json',result)
        self.save('MANIFEST.json',{'files':[{'path':str(f.relative_to(self.e)),'bytes':f.stat().st_size,'sha256':digest(f)} for f in sorted(self.e.rglob('*')) if f.is_file() and f.name!='MANIFEST.json']})
        late=self.reason()
        if late and not result['stop']:result['stop']=late;result['probe_complete']=False;self.save('RESULT.json',result);self.save('MANIFEST.json',{'files':[{'path':str(f.relative_to(self.e)),'bytes':f.stat().st_size,'sha256':digest(f)} for f in sorted(self.e.rglob('*')) if f.is_file() and f.name!='MANIFEST.json']})
        print(json.dumps(result));return 0 if result['probe_complete'] else 2


def main():
    plan=json.loads((HERE/'PLAN.json').read_text())
    if sys.argv[1:]==['--plan']:print(json.dumps(plan,indent=2));return 0
    if sys.argv[1:]!=['--execute']:raise SystemExit('use --plan or Root-reviewed --execute')
    os.umask(0o077);return Probe(plan).execute()


if __name__=='__main__':raise SystemExit(main())
