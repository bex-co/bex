"""Owned Linux Cosign transport. Import has no process, Docker or network actions."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import threading
import time

BINARY=Path('/private/tmp/bex-w8-cosign-linux-verified-20261101-iqdmo3bn/cosign-linux-arm64')
BINARY_SHA='ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2'
IMAGE='sha256:ab3fe4defd29ba6231229a4d41440ac8bde8218e85870e53876277faa24b35c4'
PREFIX='bex-w8-common499-cosign-'
MAX_LOG=8<<20

class Refusal(RuntimeError):pass

class CleanupFailure(Refusal):
    def __init__(self,receipt):
        self.receipt=receipt
        super().__init__('exact owned container cleanup unproved')

def require(ok,reason):
    if not ok:raise Refusal(reason)

def sha(path):
    h=hashlib.sha256()
    with Path(path).open('rb') as stream:
        for raw in iter(lambda:stream.read(1<<20),b''):h.update(raw)
    return h.hexdigest()

def write(path,value):
    path=Path(path);require(not path.is_symlink(),'owned receipt is symlinked')
    raw=(json.dumps(value,indent=2,sort_keys=True)+'\n').encode();require(len(raw)<=65536,'owned receipt64KiB bound')
    temporary=path.with_name(path.name+'.tmp')
    with temporary.open('xb') as stream:stream.write(raw)
    temporary.chmod(0o600);temporary.replace(path)

def owner_name(owner):
    require(isinstance(owner,str) and re.fullmatch('[a-f0-9]{16}',owner),'invalid fixed execution owner')
    return PREFIX+owner

def docker_command(owner,bundle,tuf,docker_config,inner):
    require(inner[:2]==['/usr/local/bin/cosign','verify-blob-attestation'],'not exact bundle verification entrypoint')
    name=owner_name(owner)
    paths=[Path(bundle),Path(tuf),Path(docker_config)]
    require(all(p.is_absolute() and not p.is_symlink() for p in paths),'owned bind path must be absolute/non-symlink')
    return ['docker','run','--pull=never','--name',name,'--label','bex.w8.owned='+owner,
      '--read-only','--user','501:20','--cap-drop','ALL','--security-opt','no-new-privileges',
      '--cpus','1','--memory','512m','--pids-limit','64','--network','bridge',
      '--tmpfs','/tmp:rw,noexec,nosuid,size=16m',
      '--mount','type=bind,source='+str(BINARY)+',target=/usr/local/bin/cosign,readonly',
      '--mount','type=bind,source='+str(bundle)+',target=/bundle.json,readonly',
      '--mount','type=bind,source='+str(tuf)+',target=/tuf',
      '--mount','type=bind,source='+str(docker_config)+',target=/docker,readonly',
      '--env','TUF_ROOT=/tuf','--env','DOCKER_CONFIG=/docker','--env','XDG_CACHE_HOME=/tuf/cache','--env','GODEBUG=http2client=0',
      '--entrypoint','/usr/local/bin/cosign',IMAGE,*inner[1:]]

class Docker:
    def __init__(self,capture=None):self.capture=capture or self._capture
    @staticmethod
    def _capture(argv,timeout=10):
        p=subprocess.run(argv,stdin=subprocess.DEVNULL,capture_output=True,timeout=timeout)
        require(len(p.stdout)+len(p.stderr)<=65536,'Docker control output64KiB bound')
        return {'argv':argv,'exit':p.returncode,'stdout':p.stdout.decode(),'stderr':p.stderr.decode(),'complete':True}
    def inspect(self,owner):
        name=owner_name(owner);r=self.capture(['docker','container','inspect',name])
        require(r.get('complete') is True and type(r.get('exit')) is int,'Docker inspect incomplete')
        if r['exit']==1 and r['stdout'].strip() in ('','[]'):
            if r['stderr'].strip() in ('Error: No such container: '+name,'Error: No such object: '+name,
                                      'Error response from daemon: No such container: '+name):return None,r
        require(r['exit']==0,'Docker inspect absence not established')
        rows=json.loads(r['stdout']);require(isinstance(rows,list) and len(rows)==1 and isinstance(rows[0],dict),'unique own container inspection missing')
        obj=rows[0]
        require(obj.get('Name')=='/'+name and obj.get('Image')==IMAGE
                and obj.get('Config',{}).get('Labels',{}).get('bex.w8.owned')==owner,
                'container identity/ownership differs; never remove a foreign container')
        require(re.fullmatch('[a-f0-9]{64}',obj.get('Id','')),'actual container ID malformed')
        return obj,r
    def cleanup(self,owner,uncertain=False):
        # Always inspect after a failed removal too; neither an error nor an
        # unknown create404 is relabeled successful cleanup.
        obj=None;before=None;removed=None;after=None;absent=False;errors=[]
        try:obj,before=self.inspect(owner)
        except BaseException as error:errors.append('initial-inspect:'+type(error).__name__)
        if obj is not None:
            try:
                removed=self.capture(['docker','rm','--force',owner_name(owner)])
                require(removed.get('complete') is True and type(removed.get('exit')) is int and removed['exit']==0,'own container removal failed')
            except BaseException as error:errors.append('remove:'+type(error).__name__)
        try:
            remaining,after=self.inspect(owner);absent=remaining is None
            require(absent,'own container still exists after removal')
        except BaseException as error:errors.append('absence-inspect:'+type(error).__name__)
        if obj is None and uncertain:errors.append('unknown create/run outcome;404 alone cannot prove cleanup')
        receipt={'owner':owner,'name':owner_name(owner),'container_id':obj.get('Id') if obj else None,
                 'before':before,'remove':removed,'after':after,'absence_proved':absent,'errors':errors}
        if errors:raise CleanupFailure(receipt)
        return receipt
    def image(self):
        r=self.capture(['docker','image','inspect',IMAGE]);require(r.get('complete') is True and type(r.get('exit')) is int and r['exit']==0,'cached Linux image missing; never pull')
        rows=json.loads(r['stdout']);require(isinstance(rows,list) and len(rows)==1 and rows[0].get('Id')==IMAGE
               and rows[0].get('Os')=='linux' and rows[0].get('Architecture')=='arm64','cached Linux image identity/platform differs')
        return r

class Executor:
    def __init__(self,owner,lifecycle_root,docker_config,cancelled=lambda:False,docker=None,popen=None,clock=time.monotonic,sleep=time.sleep):
        self.owner,self.name=owner,owner_name(owner)
        self.root=Path(lifecycle_root).resolve();self.docker_config=Path(docker_config).resolve()
        self.cancelled,self.docker=cancelled,docker or Docker()
        self.popen,self.clock,self.sleep=popen or subprocess.Popen,clock,sleep
        self.serial=threading.Lock()
        self.registry=self.root/'linux-cosign-lifecycle.json'
        require(self.root.is_dir() and self.docker_config.is_dir(),'owned runtime/config roots missing')
    def registry_write(self,**fields):
        write(self.registry,{'owner':self.owner,'name':self.name,**fields})
    @staticmethod
    def _reap(p):
        if p.poll() is None:
            p.terminate()
            try:p.wait(timeout=2)
            except subprocess.TimeoutExpired:p.kill();p.wait(timeout=2)
        else:p.wait(timeout=2)
    def execute(self,bundle,tuf,inner,stdout_path,stderr_path):
        with self.serial:
            require(not self.cancelled(),'cancelled before Linux crypto transport')
            require(BINARY.is_file() and not BINARY.is_symlink() and sha(BINARY)==BINARY_SHA,'verified Linux Cosign bytes changed')
            self.docker.image()
            initial,_=self.docker.inspect(self.owner);require(initial is None,'owned name unexpectedly occupied before command')
            require(not Path(bundle).is_symlink() and not Path(tuf).is_symlink(),'owned bind input is symlinked')
            bundle,tuf=Path(bundle).resolve(),Path(tuf).resolve()
            require(bundle.is_file() and not bundle.is_symlink() and tuf.is_dir() and not tuf.is_symlink(),'owned bundle/TUF input missing')
            require(self.root in bundle.parents and self.root in tuf.parents,'bind inputs must stay inside exact owned review root')
            require((self.docker_config/'config.json').read_bytes()==b'{"auths":{}}\n','Cosign Docker config must remain empty/anonymous')
            command=docker_command(self.owner,bundle,tuf,self.docker_config,inner)
            self.registry_write(pending=True,command=command,source='registered before Docker spawn')
            p=None;error=None;cleanup=None;actual=None;started=self.clock();rc=None
            process_started=False
            try:
                with Path(stdout_path).open('xb') as stdout,Path(stderr_path).open('xb') as stderr:
                    require(not self.cancelled(),'cancelled before Docker spawn')
                    # Same PGID as the review worker: the outer group stop also
                    # catches this own CLI. Daemon-owned container is separate.
                    p=self.popen(command,stdin=subprocess.DEVNULL,stdout=stdout,stderr=stderr)
                    process_started=True
                    while p.poll() is None:
                        require(not self.cancelled(),'cancelled Linux crypto transport')
                        require(self.clock()-started<330,'Linux crypto330s transport timeout')
                        require(Path(stdout_path).stat().st_size+Path(stderr_path).stat().st_size<=MAX_LOG,'Linux crypto raw logs8MiB bound')
                        self.sleep(.05)
                    rc=p.wait(timeout=2)
                    require(type(rc) is int,'Linux process exit is not canonical integer')
                    require(Path(stdout_path).stat().st_size+Path(stderr_path).stat().st_size<=MAX_LOG,'Linux crypto final raw logs8MiB bound')
                require(not self.cancelled(),'cancelled after Linux crypto process')
                actual,_=self.docker.inspect(self.owner)
                require(actual is not None,'Linux command container disappeared before actual state binding')
                cfg=actual.get('Config',{});state=actual.get('State',{});host=actual.get('HostConfig',{})
                require(cfg.get('Entrypoint')==['/usr/local/bin/cosign'] and cfg.get('Cmd')==inner[1:]
                        and cfg.get('User')=='501:20','actual Linux entrypoint/strict arguments/user differ')
                binds={m.get('Destination'):(m.get('Source'),m.get('RW')) for m in actual.get('Mounts',[]) if m.get('Type')=='bind'}
                require(binds=={'/usr/local/bin/cosign':(str(BINARY),False),'/bundle.json':(str(bundle),False),'/tuf':(str(tuf),True),'/docker':(str(self.docker_config),False)},'actual Linux bundle/tool/TUF/config mounts differ')
                require(host.get('Memory')==512<<20 and host.get('NanoCpus')==1_000_000_000
                        and host.get('PidsLimit')==64 and host.get('ReadonlyRootfs') is True,
                        'actual Linux execution bounds differ')
                require(state.get('Status')=='exited' and type(state.get('ExitCode')) is int
                        and state['ExitCode']==rc and state.get('OOMKilled') is False and not state.get('Error'),
                        'actual Linux process state/exit not canonical')
            except BaseException as caught:error=caught
            finally:
                if p is not None:
                    try:self._reap(p)
                    except BaseException as caught:error=error or caught
                try:cleanup=self.docker.cleanup(self.owner,uncertain=process_started and actual is None)
                except BaseException as caught:
                    cleanup=getattr(caught,'receipt',None);error=error or caught
                receipt={'owner':self.owner,'command':command,'cosign_argv':inner,'tool_sha256':BINARY_SHA,
                    'container_image_id':IMAGE,'pid':p.pid if p is not None else None,'process_exit':p.returncode if p is not None else None,
                    'seconds':self.clock()-started,'container':actual,'cleanup':cleanup,
                    'complete':error is None and not self.cancelled(),'error_class':type(error).__name__ if error else None}
                try:
                    write(Path(stderr_path).with_suffix('.process.json'),receipt)
                    self.registry_write(pending=not bool(cleanup and cleanup.get('absence_proved') is True and cleanup.get('errors')==[]),last_process_complete=receipt['complete'],cleanup=cleanup)
                except BaseException as caught:error=error or caught
            if self.cancelled():
                error=error or Refusal('cancelled after Linux cleanup')
                receipt.update(complete=False,error_class=type(error).__name__)
                try:
                    write(Path(stderr_path).with_suffix('.process.json'),receipt)
                    self.registry_write(pending=not bool(cleanup and cleanup.get('absence_proved') is True and cleanup.get('errors')==[]),last_process_complete=False,cleanup=cleanup)
                except BaseException as caught:error=error or caught
            if error is not None:raise error
            return {'exit_code':rc,'command':command,'cosign_argv':inner,'container_image_id':IMAGE,'tool_sha256':BINARY_SHA,
                    'container_cleanup_proved':True,'raw_process_receipt':str(Path(stderr_path).with_suffix('.process.json'))}
