"""UTC fixture names. Refresh is preparation-only; creation always checks again."""
from pathlib import Path
import argparse,datetime,fcntl,hashlib,json,re
P=Path(__file__).resolve().parent

def utc_now():return datetime.datetime.now(datetime.timezone.utc)
def identity(run,moment=None):
 if not re.fullmatch(r'[a-f0-9]{16}',run):raise ValueError('fixed prepared nonce required')
 moment=moment or utc_now();assert moment.utcoffset()==datetime.timedelta(0)
 day=moment.strftime('%Y%m%d');name='qa-'+day+'-w8-m144-'+run
 return {'name_date_utc':day,'name':name,'disk_name':name+'-disk'}
def creation_guard(spec,moment=None):
 moment=moment or utc_now();expected=identity(spec['run'],moment)
 if any(spec.get(k)!=v for k,v in expected.items()):raise ValueError('UTC name is stale; run q refresh-name before root writes the exact-name rollout gate')
 # Keep bounded control-plane request away from a UTC boundary; never adopt/retry a prior attempt.
 remaining=86400-(moment.hour*3600+moment.minute*60+moment.second+moment.microsecond/1e6)
 if remaining<120:raise ValueError('fresh create refused in final120seconds of UTC day; refresh name/gate after UTC rollover')
 return moment.isoformat()
def child(prefix,name):
 return prefix+name if len(prefix)+len(name)<=63 else prefix+name[:63-len(prefix)-9]+'-'+hashlib.sha256(name.encode()).hexdigest()[:8]
def refresh_manifest():
 f=P/'MANIFEST.json'
 if not f.exists():return
 manifest=json.loads(f.read_text());files=sorted({v['path'] for v in manifest['files']}|{'naming.py'})
 manifest['files']=[{'path':n,'bytes':(P/n).stat().st_size,'sha256':hashlib.sha256((P/n).read_bytes()).hexdigest()} for n in files];f.write_text(json.dumps(manifest,indent=2))
def refresh(already_locked=False):
 lock=None
 if not already_locked:
  lock=(P/'writer.lock').open('a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 try:
  sf=P/'spec.json';lf=P/'ledger.json';spec=json.loads(sf.read_text());ledger=json.loads(lf.read_text())
  if ledger.get('create_intent') or ledger.get('create_proof') or ledger.get('owned') or ledger.get('resources'):raise ValueError('name cannot change after any create attempt/proof/resource; reconcile original intent, never retry-adopt')
  if (P/'rollout-ready.json').exists():raise ValueError('live exact-name rollout gate exists; root must withdraw its unused gate before any preparation refresh')
  names=identity(spec['run']);old=spec['name'];spec.update(names);sf.write_text(json.dumps(spec,indent=2))
  f=P/'rollout-ready.example.json';gate=json.loads(f.read_text());gate.update({'fixture_name':names['name'],'disk_name':names['disk_name'],'name_date_utc':names['name_date_utc']});f.write_text(json.dumps(gate,indent=2))
  f=P/'acceptance-plan.json';plan=json.loads(f.read_text());plan['fixture']=spec;plan['naming']='UTC creation day: '+names['name']+'; disk '+names['disk_name']+'; preserved nonce '+spec['run'];plan['utc_naming_contract']='Run q refresh-name immediately before root writes its exact-name rollout-ready.json. Refresh refuses a live gate or ANY prior create_intent/proof/ownedresource. Create checks currentUTC before anyHTTP and again immediatelybefore recording created_at/POST; refuses final120seconds ofUTCday. Stale date requires a fresh unused rootgate; no retry/adoption/nonce replacement.'
  if not any(step.get('step')=='refresh UTC prepared identity' for step in plan['runbook']):plan['runbook'].insert(0,{'step':'refresh UTC prepared identity','command':str(P/'q')+' refresh-name','note':'Preparation only, before root writes rollout-ready.json. Date ruleUTC, notlocaltimezone. Does not authorize auth/create; a livegate/priorattempt refuses refresh.'})
  f.write_text(json.dumps(plan,indent=2,ensure_ascii=False))
  f=P/'owned-binding.example.json';binding=json.loads(f.read_text());app=spec['workspace']+'-'+names['name'];binding.update({'service_name':names['name'],'disk_name':names['disk_name'],'name_date_utc':names['name_date_utc'],'app_name':app,'pvc_name':child('disk-',app),'luks_secret_name':child('disk-',app)+'-luks','backup_cronjob':child('dskbak-',app)});f.write_text(json.dumps(binding,indent=2))
  f=P/'kubernetes-fixture-contract.json';contract=json.loads(f.read_text());contract['prepared_fixture_name']=names['name'];contract['UTC_naming_contract']={'rule':'qa-YYYYMMDD-w8-m144-<nonce>, YYYYMMDD from create_intent.created_at UTC; disk appends-disk','refresh':'Preparation-only q refresh-name before unused exact-name gate; date neverchangesafteranycreateattempt','creation':'Refuse staleUTCname beforeanyHTTP and justbeforePOST; refusefinal120secondsUTCday. Actual201API/K8stimestamps independently checked against intendedUTCdate.','canonical_intent_timestamp':'create_intent.created_at, awareRFC3339UTC; existing at alias retained onlyforcompatibility','expiry_cost_safeguards':'No changes: exactoneStarterservice/disk, samepreservednonce, sameworkspace, max5Gi/7200seconds/.03estimate, actualrootrolloutgate andunchangedexpiryrequired.'};f.write_text(json.dumps(contract,indent=2))
  ledger.update({'prepared_service_name':names['name'],'prepared_disk_name':names['disk_name'],'prepared_name_date_utc':names['name_date_utc']});lf.write_text(json.dumps(ledger,indent=2));refresh_manifest()
  return {'scope':'preparationonly/noHTTP','old_name':old,**names,'nonce_preserved':True,'resources':0,'root_live_gate_present':False,'next':'Rerun preparation-check.py and utc-naming-check.py, then python naming.py manifest; root writes its exact-name/date rollout gate only after that. Local mount test is source-based and has unchanged nonce/program.'}
 finally:
  if lock:lock.close()
if __name__=='__main__':
 a=argparse.ArgumentParser();a.add_argument('command',choices=['refresh','manifest']);args=a.parse_args();print(json.dumps(refresh() if args.command=='refresh' else (refresh_manifest() or {'manifest_refreshed':True,'scope':'localfilesonly/noHTTP'}),indent=2))
