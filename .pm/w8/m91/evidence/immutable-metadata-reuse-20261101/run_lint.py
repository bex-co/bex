from pathlib import Path
import subprocess,os,json,time,hashlib,importlib.util,sys
sys.dont_write_bytecode=True
root=Path(__file__).resolve().parent;cwd=Path('/Users/tianpan/projects/bex7/lego/operator');p=Path('/private/tmp/bex-w8-node14-local-pilot-v4-source-20261101-bop3sbht/run_pilot.py');spec=importlib.util.spec_from_file_location('lint_group',p);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
args=[str(cwd/'bin/golangci-lint'),'run','./internal/build/...','--timeout=90s'];env=dict(os.environ,GOWORK='off',GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOFLAGS='-mod=readonly',GOMAXPROCS='2',GOMEMLIMIT='512MiB',GOTELEMETRY='off',PATH='/opt/homebrew/Cellar/go/1.26.4/libexec/bin:'+os.environ['PATH']);start=time.monotonic();process=None;error=None
with (root/'lint.stdout').open('wb') as out,(root/'lint.stderr').open('wb') as err:
 try:
  process=subprocess.Popen(args,cwd=cwd,env=env,stdout=out,stderr=err,start_new_session=True);process.wait(timeout=100)
 except BaseException as failure:error=type(failure).__name__+':'+str(failure)
 finally:absent=m.stop_group(process) if process is not None else False
record={'argv':args,'exit_code':process.returncode if process else None,'error':error,'group_absent':absent,'elapsed_seconds':round(time.monotonic()-start,3)};(root/'LINT.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));raise SystemExit(0 if error is None and absent and record['exit_code']==0 else 1)
