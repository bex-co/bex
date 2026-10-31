import datetime,json,os,pathlib,shutil,signal,subprocess,time,sys
root=pathlib.Path(__file__).resolve().parent
floor=2<<30;reserve=128<<20;start=shutil.disk_usage(root).free
assert start>=floor+(256<<20),(start,"scoped Go warm256MiB+2GiB reserve does not fit")
mode=sys.argv[1];tmp=root/('go-tmp-'+mode);tmp.mkdir()
env=dict(os.environ,GOWORK='off',GOMAXPROCS='2',GOMEMLIMIT='384MiB',GOTMPDIR=str(tmp));env['BEX_BUILDKIT_INTEGRATION']='1';env['BEX_NATIVE_LEGACY_PYTHON_INTEGRATION']='';env.pop('BEX_NATIVE_TEST_COMMON_IMAGE',None);env.pop('BEX_NATIVE_TEST_NODE_IMAGE',None)
cmd=([str(pathlib.Path('/Users/tianpan/projects/bex7/lego/operator/bin/golangci-lint')),'run','./internal/build'] if mode.startswith('lint') else ['go','test','-p','1','./internal/build','-run','^TestNativeLegacyPythonGeneratedImages$','-count=1','-v'])
started=datetime.datetime.now(datetime.timezone.utc).isoformat();samples=[];stop=None
with (root/('scoped-'+mode+'.log')).open('wb') as log:
 process=subprocess.Popen(cmd,cwd='/Users/tianpan/projects/bex7/lego/operator',env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
 while process.poll() is None:
  free=shutil.disk_usage(root).free;own=sum(p.stat().st_blocks*512 for p in tmp.rglob('*') if p.is_file());samples.append({'free':free,'own_tmp_bytes':own})
  if free<floor+reserve or own>480<<20:
   stop='proactive own capacity stop';os.killpg(process.pid,signal.SIGTERM);break
  time.sleep(1)
 try:code=process.wait(timeout=15)
 except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);code=process.wait()
result={'started_at':started,'completed_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'command':cmd,'exit_code':code,'stop':stop,'start_free_bytes':start,'minimum_observed_free_bytes':min(x['free'] for x in samples) if samples else start,'maximum_owned_tmp_bytes':max(x['own_tmp_bytes'] for x in samples) if samples else 0,'scope':'scoped local Go source/unit controls only; opt-in Docker test disabled'}
(root/('scoped-'+mode+'-result.json')).write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result));print((root/('scoped-'+mode+'.log')).read_text())
