from pathlib import Path
import datetime,json,os,subprocess,sys
P=Path(__file__).resolve().parent;ROOT=Path('/Users/tianpan/projects/bex7');os.umask(0o077)
handle=P/'qa-session-handle.json';session=json.loads(handle.read_text()) if handle.exists() else None;cfg=P/'credentials/cli.yaml';env={k:v for k,v in os.environ.items() if not k.startswith(('BEX_','RENDER_','QA_'))};env.update({'PATH':str(P/'bin')+':'+os.environ['PATH'],'DO_NOT_TRACK':'1','BROWSER':'/usr/bin/true','BEX_NO_UPDATE_NOTIFIER':'1','BEX_HOST':'https://api.bex.co/v1/','BEX_CLI_CONFIG_PATH':str(cfg),'BEX_CLI_CONFIG_DIR':str(P/'credentials'),'BEX_CLI_DISABLE_ANALYTICS':'1'});native=None
if cfg.exists():
 native=subprocess.run([str(P/'release/bex'),'logout','--confirm','-o','json'],env=env,capture_output=True,text=True,timeout=45)
helper=None
if session and Path(session['logout_jar']).exists():helper=subprocess.run(['bash','scripts/qa-login.sh','--logout',session['logout_jar']],cwd=ROOT,capture_output=True,text=True,timeout=90)
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'native_logout_exit':native.returncode if native else None,'Kratos_logout_exit':helper.returncode if helper else None,'resources_created_by_run':len(json.loads((P/'ledger.json').read_text())['resources']) if (P/'ledger.json').exists() else 0,'scope':'only this own native OAuth credential and own QA session'}
if (native is None or native.returncode==0) and (helper is None or helper.returncode==0):
 for f in [cfg,P/'device-cookies.jar',P/'qa-state.json',P/'device-state.json',P/'config-bex/cli.yaml',P/'config-render/cli.yaml',handle]:f.unlink(missing_ok=True)
 if session:Path(session['logout_jar']).unlink(missing_ok=True)
 result['own_credential_files_removed']=True
else:
 result['own_credential_files_removed']=False;result['remaining_gate']='Own session/credential revoke failed; retain named private handles for retry, never bulk revoke.'
(P/'auth-cleanup-receipt.json').write_text(json.dumps(result,indent=2));print(json.dumps(result));sys.exit(0 if result['own_credential_files_removed'] else 1)
