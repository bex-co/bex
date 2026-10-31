from pathlib import Path
import datetime,http.cookies,json,os,re,shutil,subprocess,time,urllib.parse
P=Path(__file__).resolve().parent;ROOT=Path('/Users/tianpan/projects/bex7');os.umask(0o077)
for n in ['bin','credentials','config-bex','config-render']:(P/n).mkdir(mode=0o700,exist_ok=True)
for opener in ['open','xdg-open','rundll32']:
 f=P/'bin'/opener;f.write_text('#!/bin/sh\nexit 0\n');f.chmod(0o755)
def env(client='bex'):
 prefix='BEX' if client=='bex' else 'RENDER';d={k:v for k,v in os.environ.items() if not k.startswith(('BEX_','RENDER_','QA_'))};d.update({'PATH':str(P/'bin')+':'+os.environ['PATH'],'DO_NOT_TRACK':'1','BROWSER':'/usr/bin/true','BEX_NO_UPDATE_NOTIFIER':'1',prefix+'_HOST':'https://api.bex.co/v1/',prefix+'_CLI_CONFIG_PATH':str(P/'credentials/cli.yaml'),prefix+'_CLI_CONFIG_DIR':str(P/'credentials'),prefix+'_CLI_DISABLE_ANALYTICS':'1'});return d

def allowed(u):
 x=urllib.parse.urlsplit(u);assert x.scheme=='https' and x.hostname in ['api.bex.co','auth.bex.co','oauth.bex.co','dashboard.bex.co'] and not x.username and not x.password;return u
session_file=P/'qa-session-handle.json'
if session_file.exists():
 sourcejar=Path(json.loads(session_file.read_text())['logout_jar']);assert sourcejar.exists()
else:
 bootstrap=subprocess.run(['bash','scripts/qa-login.sh',str(P/'qa-state.json')],cwd=ROOT,capture_output=True,text=True,timeout=120)

 if bootstrap.returncode:
  (P/'auth-bootstrap-failure.json').write_text(json.dumps({'qa_login_exit':bootstrap.returncode,'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}))
  raise SystemExit(bootstrap.returncode)
 sourcejar=Path(next(line[7:] for line in bootstrap.stdout.splitlines() if line.startswith('logout ')))
 session_file.write_text(json.dumps({'logout_jar':str(sourcejar),'state':str(P/'qa-state.json')}));session_file.chmod(0o600)
jar=P/'device-cookies.jar';shutil.copyfile(sourcejar,jar)
hops=[]
def request(url,form=None):
 allowed(url);args=['curl','--disable','-sS','--max-time','30','-b',str(jar),'-c',str(jar),'-D',str(P/'device-headers.txt'),'-o',str(P/'device-body.txt'),'-w','%{http_code}']
 if form:
  f=P/'device-form.txt';f.write_text(urllib.parse.urlencode(form));args+=['-H','Origin: https://dashboard.bex.co','-H','Content-Type: application/x-www-form-urlencoded','--data-binary','@'+str(f)]
 r=subprocess.run([*args,url],capture_output=True,text=True,timeout=35);assert r.returncode==0;status=int(r.stdout);headers=(P/'device-headers.txt').read_text();location=next((line.partition(':')[2].strip() for line in headers.splitlines() if line.lower().startswith('location:')),None);nexturl=urllib.parse.urljoin(url,location) if location else None
 if nexturl:allowed(nexturl)
 hops.append({'host':urllib.parse.urlsplit(url).hostname,'path':urllib.parse.urlsplit(url).path,'query_keys':sorted(urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)),'method':'POST' if form else 'GET','HTTP':status});return status,nexturl
log=(P/'login-private.log').open('w');proc=subprocess.Popen([str(P/'release/bex'),'login'],env=env(),stdout=log,stderr=log)
try:
 url=None
 for _ in range(60):
  text=(P/'login-private.log').read_text();m=re.search(r'https://[^\s]+/oauth2/device/verify\?user_code=[^\s]+',text)
  if m:url=m[0];break
  assert proc.poll() is None,'login exited before verification URL';time.sleep(.5)
 assert url,'no actual verification URL';allowed(url);device_pair=None
 for _ in range(24):
  q=urllib.parse.parse_qs(urllib.parse.urlsplit(url).query);path=urllib.parse.urlsplit(url).path
  if path=='/auth/device' and q.get('device_challenge') and q.get('user_code'):
   status,target=request(url);assert status==200;device_pair={k:q[k][0] for k in ['user_code','device_challenge']};break
  status,target=request(url);assert target and status in (301,302,303,307,308);url=target
 assert device_pair,'own device confirmation unavailable';status,url=request('https://dashboard.bex.co/auth/device',device_pair);assert status in (302,303) and url
 for _ in range(24):
  q=urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)
  if q.get('login_challenge'):
   done=subprocess.run(['bash','scripts/qa-login.sh','--cookie-jar',str(jar),'--login-challenge',q['login_challenge'][0],str(P/'device-state.json')],cwd=ROOT,capture_output=True,text=True,timeout=120)
   (P/'device-helper-private.log').write_text(done.stdout+'\n'+done.stderr);assert done.returncode==0,'qa-login challenge completion failed; see private log'
   helperjar=next(line[7:] for line in done.stdout.splitlines() if line.startswith('logout '));shutil.copyfile(helperjar,jar);Path(helperjar).unlink();(P/'device-state.json').unlink(missing_ok=True);break
  if urllib.parse.urlsplit(url).path=='/auth/device/success':break
  status,target=request(url)
  if not target:
   assert status==200 and urllib.parse.urlsplit(url).path=='/auth/device/success';break
  assert status in (301,302,303,307,308);url=target
 code=proc.wait(timeout=35);assert code==0,'native login failed';cfg=P/'credentials/cli.yaml';assert cfg.exists() and cfg.stat().st_mode&0o777==0o600
 receipt={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'native_login_exit':code,'grant':'isolated QA human session via qa-login.sh and own actual device verification/confirmation','browser_UI':'not exercised; same-origin HTTP confirmation used for this own request','hops':hops,'configuration_mode':oct(cfg.stat().st_mode&0o777),'logout_jar':str(sourcejar),'credentials_printed':False}
 (P/'auth-receipt.json').write_text(json.dumps(receipt,indent=2));print(json.dumps(receipt))
finally:
 if proc.poll() is None:proc.terminate();proc.wait(timeout=10)
 log.close()
 for n in ['login-private.log','device-form.txt','device-headers.txt','device-body.txt','device-helper-private.log']:(P/n).unlink(missing_ok=True)
