from pathlib import Path
import os,subprocess,json,time,signal,hashlib,importlib.util,shutil,sys
sys.dont_write_bytecode=True
ROOT=Path(__file__).resolve().parent;CWD=Path('/Users/tianpan/projects/bex7/lego/operator');GO='/opt/homebrew/Cellar/go/1.26.4/libexec/bin/go'
p=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py');assert hashlib.sha256(p.read_bytes()).hexdigest()=='e158acde2c44aac290d09f109ddf4f3bf2ca93dc660161dd326fb3ad7cc72c03';spec=importlib.util.spec_from_file_location('checked_group',p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
env=dict(os.environ,GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',GOTELEMETRY='off',GOMAXPROCS='2',GOMEMLIMIT='512MiB',PATH=str(Path(GO).parent)+':'+os.environ['PATH'])
for key in list(env):
 if key.startswith('BEX_NATIVE_TEST_') or key in ('BEX_BUILDKIT_INTEGRATION','GOOS','GOARCH','CGO_ENABLED'):env.pop(key)
rows=[]
checks=[('focused',[GO,'test','-run','^TestImageMetadata(Cache|RetriesOnlyTransientReads)','./internal/build'],120,0),('old-requestcount-red',[GO,'test','-overlay',str(ROOT/'overlay-old-resolver.json'),'-run','^TestImageMetadataCacheReusesVerifiedReadsAndIsolatesCallers$','./internal/build'],120,1),('package',[GO,'test','./internal/build'],180,0),('race',[GO,'test','-race','-run','^TestImageMetadataCache','./internal/build'],300,0)]
for label,args,limit,wanted in checks:
 start=time.monotonic();process=None;error=None;absent=False
 with (ROOT/(label+'.stdout')).open('wb') as out,(ROOT/(label+'.stderr')).open('wb') as err:
  try:
   process=subprocess.Popen(args,cwd=CWD,env=env,stdout=out,stderr=err,start_new_session=True)
   while process.poll() is None:
    if time.monotonic()-start>limit:raise RuntimeError('deadline')
    if shutil.disk_usage(ROOT).free<8*1024**3:raise RuntimeError('spacefloor')
    time.sleep(.1)
  except BaseException as failure:error=type(failure).__name__+':'+str(failure)
  finally:
   if process is not None:absent=m.stop_group(process)
 row={'label':label,'argv':args,'exit_code':process.returncode if process else None,'expected_exit':wanted,'error':error,'group_absent':absent,'elapsed_seconds':round(time.monotonic()-start,3)};rows.append(row);(ROOT/'CHECKS.json').write_text(json.dumps(rows,indent=2)+'\n');print(json.dumps(row),flush=True)
 if error or not absent or row['exit_code']!=wanted:raise SystemExit(1)
