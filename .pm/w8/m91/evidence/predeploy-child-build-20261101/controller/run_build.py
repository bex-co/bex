"""Minimal fixed build adaptation of the retained reviewed V4 lifecycle."""
from pathlib import Path
import hashlib,importlib.util,json,os,re,shutil,signal,stat,sys,time
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('retained_v4',HERE/'retained-v4.py');v4=importlib.util.module_from_spec(spec);spec.loader.exec_module(v4)

def verify(p):
 assert v4.digest(HERE/'retained-v4.py')==p['retained_v4_sha256']
 assert v4.digest(p['source_manifest'])==p['source_manifest_sha256']
 source=Path(p['source_root']);m=json.loads(Path(p['source_manifest']).read_bytes());assert m['commit']==p['source_sha']
 for name,w in m['files'].items():
  q=source/name;s=q.lstat();assert stat.S_ISREG(s.st_mode) and not q.is_symlink() and s.st_size==w['bytes'] and v4.digest(q)==w['sha256'] and stat.S_IMODE(s.st_mode)==w['permissions'],name
 assert v4.digest(Path(p['export_root'])/'MANIFEST.json')==p['export_manifest_sha256']
 assert v4.digest(p['context_manifest'])==p['context_manifest_sha256']
 for name,w in json.loads(Path(p['context_manifest']).read_bytes()).items():
  q=Path(p['context_manifest']).parent/name;s=q.lstat();assert stat.S_ISREG(s.st_mode) and s.st_size==w['bytes'] and v4.digest(q)==w['sha256'] and format(stat.S_IMODE(s.st_mode),'04o')==w['mode'],name
 assert v4.digest(p['dockerfile'])==p['dockerfile_sha256']
 for name,w in p['tools_sha256'].items():assert v4.digest(Path(p['tools_root'])/name)==w
 assert v4.digest(Path(p['host_buildx_plugin_dir'])/'docker-buildx')==p['host_buildx_plugin_sha256']
 pol=json.loads((source/'lego/operator/internal/build/toolchain-freshness.json').read_bytes())
 assert [x['committed'] for x in pol['images'] if x['id']=='helper-buildkit']==[p['buildkit_image']]
 return True

class Build(v4.Pilot):
 def __init__(self,p,h):
  super().__init__(p,h);self.state_bytes=0;self.peak_combined=0;self.minimum_free=shutil.disk_usage(HERE).free;self.oci_seen=0;self.log_seen=0
  self.env={'PATH':os.environ['PATH'],'DOCKER_CONFIG':str(self.config),'DOCKER_HOST':p['endpoint'],'PYTHONDONTWRITEBYTECODE':'1'}
 def resource_reason(self):
  free=shutil.disk_usage(self.root if self.root.exists() else HERE).free;self.minimum_free=min(self.minimum_free,free)
  host=self.owned_size() if self.root.exists() else 0;combined=host+self.state_bytes;self.peak_combined=max(self.peak_combined,combined)
  oci=self.e/'child.oci';self.oci_seen=max(self.oci_seen,oci.stat().st_size if oci.exists() else 0)
  self.log_seen=sum(q.stat().st_size for q in self.e.rglob('*') if q.is_file() and q.suffix in ('.stdout','.stderr','.log','.txt')) if self.e.exists() else 0
  if self.cancel:return 'cancelled'
  if time.monotonic()-self.started>self.p['bounds']['whole_seconds']:return 'whole deadline'
  if combined>self.p['bounds']['combined_owned_bytes']:return 'combined own state/files12GiB bound'
  if self.oci_seen>self.p['bounds']['oci_bytes']:return 'OCI4GiB bound'
  if self.log_seen>self.p['bounds']['log_bytes']:return 'raw log16MiB bound'
  if free<self.p['bounds']['remaining_host_floor_bytes']:return 'physical8GiB floor'
  return None
 def observe(self):
  now=time.monotonic();path=self.e/'phase';phase=path.read_text().strip() if path.exists() else 'starting'
  if phase!=self.phase:self.phase,self.phase_at=phase,now;self.save('CURRENT.json',{'phase':phase,'at_monotonic':now})
  limit={'starting':60,'alias-preflight':30,'pull':195,'bootstrap':300,'case':1800}
  if phase not in limit or now-self.phase_at>limit[phase]:raise RuntimeError('phase deadline:'+phase)
  if now-self.observed<1:return
  self.observed=now
  row=self.inspect('container',self.builder,'builder-observe-'+str(len(self.records)))
  if row is None:return
  assert row['Config']['Image']==self.p['buildkit_image'] and row['Name']=='/'+self.builder
  mounts=[x for x in row['Mounts'] if x['Destination']=='/var/lib/buildkit'];assert len(mounts)==1 and mounts[0]['Name']==self.volume
  if row['State']['Running']:
   assert row['HostConfig']['Memory']==4*1024**3 and row['HostConfig']['CpuQuota']==200000 and row['HostConfig']['CpuPeriod']==100000
   code,out,_=self.command('builder-du-'+str(len(self.records)),['exec',self.builder,'du','-sk','/var/lib/buildkit'],10,True)
   assert code==0 and re.fullmatch(rb'\d+\s+/var/lib/buildkit\n?',out)
   self.state_bytes=int(out.split()[0])*1024
   self.save('USAGE.json',{'at_monotonic':now,'builder_state_allocated_bytes':self.state_bytes,'host_files_bytes':self.owned_size(),'combined_bytes':self.owned_size()+self.state_bytes,'physical_free_bytes':shutil.disk_usage(self.root).free,'phase':phase})
   reason=self.resource_reason()
   if reason:raise RuntimeError(reason)
 def execute(self):
  verify(self.p);assert not self.root.exists();assert self.minimum_free>=self.p['bounds']['remaining_host_floor_bytes']+self.p['bounds']['combined_owned_bytes']+(32<<20)
  self.root.mkdir(mode=0o700);self.e.mkdir(mode=0o700);self.config.mkdir(mode=0o700)
  self.h.dump(self.config/'.qualification-owner.json',{'owner':self.owner,'evidence_root':str(self.e)})
  self.h.dump(self.config/'config.json',{'cliPluginsExtraDirs':[self.p['host_buildx_plugin_dir']]})
  self.h.dump(self.e/'inputs.json',{'owner':self.owner,'docker_config':str(self.config),'scope':self.p['scope'],'common_image':self.p['common_image'],'generator_source':self.p['source_sha'],'platform':self.p['platform'],'source_manifest_sha256':self.p['source_manifest_sha256'],'cache_scope':self.p['cache_scope']})
  self.save('PLAN.json',self.p);handlers={};failure=None;passed=False
  try:
   for sig in (signal.SIGTERM,signal.SIGINT):handlers[sig]=signal.getsignal(sig);signal.signal(sig,lambda n,f:setattr(self,'cancel',n))
   assert self.inspect('container',self.runner,'before-runner') is None and self.inspect('container',self.builder,'before-builder') is None and self.inspect('volume',self.volume,'before-volume') is None
   self.proved_empty=True
   code,out,_=self.command('daemon-fit',['info','--format','{{json .MemTotal}} {{json .NCPU}}'],10);assert code==0
   memory,cpus=map(int,out.split());assert memory>=4*1024**3+512*1024**2 and cpus>=2
   code,out,_=self.command('cached-runner',['image','inspect','--format','{{.Id}} {{.Os}}/{{.Architecture}}',self.p['runner_image']],10);assert code==0 and out.decode().strip()==self.p['runner_image']+' linux/arm64'
   self.command('immutable-cache-before',['image','inspect','--format','{{.Id}}',self.p['buildkit_image']],10) # Observation only, never a cold-cache claim.
   env={'DOCKER_CONFIG':str(self.config),'DOCKER_HOST':self.p['endpoint'],'PILOT_EVIDENCE':str(self.e),'PILOT_OWNER':self.owner,'PILOT_NODE':self.node,'PILOT_BUILDER_CONTAINER':self.builder,'PILOT_BUILDKIT':self.p['buildkit_image'],'BUILD_RENDER_ENV':self.p['render_env'],'BUILD_DOCKERFILE':self.p['dockerfile'],'BUILD_CONTEXT':self.p['context']}
   args=['create','--name',self.runner,'--label',self.h.LABEL+'='+self.owner,'--pull=never','--platform','linux/arm64','--read-only','--network','bridge','--memory','512m','--cpus','2','--pids-limit','128','--tmpfs','/tmp:rw,nosuid,nodev,size=64m','--workdir',self.p['context']]
   mounts=[(self.root,self.root,False),(self.p['export_root'],self.p['export_root'],True),(Path(self.p['tools_root'])/'docker','/usr/local/bin/docker',True),(Path(self.p['tools_root'])/'docker-buildx','/usr/libexec/docker/cli-plugins/docker-buildx',True),('/var/run/docker.sock',self.p['inside_socket_destination'],True),(HERE/'build.sh','/build.sh',True)]
   for src,dst,ro in mounts:args+=['--mount','type=bind,src='+str(src)+',dst='+str(dst)+(',readonly' if ro else '')]
   for k,v in env.items():args+=['--env',k+'='+v]
   args+=['--entrypoint','/bin/sh',self.p['runner_image'],'/build.sh']
   self.save('RUNNER-CREATE-INTENT.json',{'owner':self.owner,'name':self.runner,'image':self.p['runner_image'],'argv':['docker',*args]})
   self.unknown=True;code,out,_=self.command('runner-create',args,30);assert code==0 and re.fullmatch(rb'[a-f0-9]{64}\n?',out)
   self.cid=out.decode().strip();self.unknown=False;self.save('runner-cid.json',{'id':self.cid,'name':self.runner,'owner':self.owner})
   code,out,_=self.command('case',['start','--attach',self.cid],self.p['bounds']['whole_seconds'],observe=True);assert code==0
   row=self.inspect('container',self.runner,'runner-finished');assert row['Id']==self.cid and not row['State']['Running'] and row['State']['ExitCode']==0
   assert (self.e/'child.oci').is_file() and (self.e/'build-metadata.json').is_file()
   reason=self.resource_reason()
   if reason:raise RuntimeError(reason)
   passed=True
  except BaseException as e:failure=type(e).__name__+': '+str(e)
  finally:
   self.cleanup()
   try:verify(self.p)
   except BaseException as e:self.errors.append('final immutable source:'+type(e).__name__)
   reason=self.resource_reason()
   if reason:failure=failure or reason
   for sig,h in reversed(list(handlers.items())):signal.signal(sig,h)
   if self.cancel:failure=failure or 'cancelled'
   result={'PASS':passed and not failure and not self.errors,'build_exit0':passed,'failure':failure,'cleanup_errors':self.errors,'cancel_signal':self.cancel,'pending_pgids':sorted(self.groups),'elapsed_seconds':round(time.monotonic()-self.started,3),'owner':self.owner,'runner_cid':self.cid,'peak_combined_observed_bytes':self.peak_combined,'minimum_physical_free_bytes':self.minimum_free,'maximum_oci_observed_bytes':self.oci_seen,'final_log_bytes':self.log_seen,'source_unchanged':not any('source' in x for x in self.errors),'handlers_restored':all(signal.getsignal(sig)==h for sig,h in handlers.items()),'config_absent':not self.config.exists(),'scope':self.p['scope']}
   self.save('RESULT.json',result);self.h.manifest(self.e);print(json.dumps(result),flush=True)
  return result['PASS']

def main():
 frozen=json.loads((HERE/'SOURCE-MANIFEST.json').read_bytes())
 for name,w in frozen.items():assert v4.digest(HERE/name)==w['sha256'] and (HERE/name).stat().st_size==w['bytes']
 p=json.loads((HERE/'PLAN.json').read_bytes());verify(p)
 if sys.argv[1:]!=['--execute']:print(json.dumps({'source_controls':'PASS','plan':p},indent=2));return 0
 return 0 if Build(p,v4.helper_for(p)).execute() else 1
if __name__=='__main__':raise SystemExit(main())
