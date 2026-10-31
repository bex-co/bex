"""Preview fixed CI fixture operations; no commands execute and no production call occurs."""
from pathlib import Path
import argparse,hashlib,json,re
if not __debug__:raise RuntimeError('assertions required')
P=Path(__file__).resolve().parent;S=json.loads((P/'spec.json').read_text());L=json.loads((P/'ledger.json').read_text())
a=argparse.ArgumentParser();a.add_argument('owned_binding_json');args=a.parse_args();path=Path(args.owned_binding_json);b=json.loads(path.read_text())
assert L.get('create_proof',{}).get('HTTP')==201 and L.get('owned')
sid=L['owned']['id'];assert b['service_id']==sid and b['workspace']==S['workspace'] and sid not in L['baseline_ids'] and b['app_name']==S['workspace']+'-'+S['name'] and b['app_uid']
assert b['service_name']==S['name'] and b['disk_name']==S['disk_name']
def child(prefix,name):
 return prefix+name if len(prefix)+len(name)<=63 else prefix+name[:63-len(prefix)-9]+'-'+hashlib.sha256(name.encode()).hexdigest()[:8]
assert b['pvc_name']==child('disk-',b['app_name']) and b['luks_secret_name']==b['pvc_name']+'-luks' and b['backup_cronjob']==child('dskbak-',b['app_name'])
assert b['namespace'] and re.fullmatch(r'[a-z0-9-]+',b['namespace']) and b['pvc_uid'] and b['pv_uid'] and b['volume_handle'] and b['backup_cronjob_uid'] and b['secret_uid']
contract=json.loads((P/'kubernetes-fixture-contract.json').read_text())
print(json.dumps({'scope':'plan preview only; no production operation executed or authorized','binding_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'exact_immutable_binding':{k:b[k] for k in ['workspace','service_id','service_name','disk_name','namespace','app_name','app_uid','pvc_name','pvc_uid','pv_name','pv_uid','volume_handle']},'actions':contract['proposed_dispatch']['fixture_action_enum'],'protocol':'kubernetes-fixture-contract.json','gate':'Root must commit a sanitized clean ownership ledger after guarded actual REST201 create and validated K8s binding. Workflow admits only fixed action/phase and reviewed ledger reference; no shell/argv/manifest/device/PromQL inputs. This preview alone does not satisfy that gate.','cleanup':'Only own created backupJobUID may be deleted with UIDprecondition. REST/operator handle service/volume lifecycle; no direct provider or PVC deletion.'},indent=2))
