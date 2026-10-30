from pathlib import Path
import copy,datetime,importlib.util,json,shutil,tempfile
P=Path(__file__).resolve().parent
m=importlib.util.spec_from_file_location('qa_naming',P/'naming.py');n=importlib.util.module_from_spec(m);m.loader.exec_module(n)
spec=json.loads((P/'spec.json').read_text());day=datetime.datetime.strptime(spec['name_date_utc'],'%Y%m%d').replace(tzinfo=datetime.timezone.utc);midday=day+datetime.timedelta(hours=12);tomorrow=midday+datetime.timedelta(days=1)
assert n.creation_guard(spec,midday)==midday.isoformat()
for label,moment in [('stale_UTC_date',tomorrow),('UTC_boundary',day+datetime.timedelta(hours=23,minutes=59))]:
 try:n.creation_guard(spec,moment);raise RuntimeError('failed to refuse '+label)
 except ValueError:pass
# Refresh entirely isolated copies, never the production-prepared ledger/gate.
with tempfile.TemporaryDirectory(prefix='m144-utc-synthetic-') as t:
 t=Path(t);manifest=json.loads((P/'MANIFEST.json').read_text())
 for f in [*[v['path'] for v in manifest['files']],'MANIFEST.json']:shutil.copyfile(P/f,t/f)
 n.P=t;n.utc_now=lambda:tomorrow
 old=json.loads((t/'spec.json').read_text());gate_before=json.loads((t/'rollout-ready.example.json').read_text());r=n.refresh();fresh=json.loads((t/'spec.json').read_text());gate=json.loads((t/'rollout-ready.example.json').read_text());plan=json.loads((t/'acceptance-plan.json').read_text());ledger=json.loads((t/'ledger.json').read_text())
 assert fresh['run']==old['run'] and fresh['name']==n.identity(old['run'],tomorrow)['name'] and gate['fixture_name']==plan['fixture']['name']==ledger['prepared_service_name']==fresh['name']
 assert fresh['disk_name']==gate['disk_name']==ledger['prepared_disk_name'];assert all(fresh[k]==old[k] for k in ['workspace','plan','max_runtime_seconds','max_estimated_usage_usd','initial_gib','grown_gib','fixture_source_sha256'])
 assert all(gate[k]==gate_before[k] for k in ['coordinated_by_root','expires_at','max_runtime_seconds','max_estimated_usage_usd','max_services','max_disks','max_size_gib','authorized_actions'])
 (t/'rollout-ready.json').write_text('{}')
 try:n.refresh();raise RuntimeError('live gate refresh allowed')
 except ValueError:pass
 (t/'rollout-ready.json').unlink();ledger['create_intent']={'created_at':tomorrow.isoformat()};(t/'ledger.json').write_text(json.dumps(ledger))
 try:n.refresh();raise RuntimeError('post-intent rename allowed')
 except ValueError:pass
assert not (P/'rollout-ready.json').exists() and not (P/'credentials/cli.yaml').exists() and json.loads((P/'ledger.json').read_text())['resources']==[]
checks={'rule':'UTCqa-YYYYMMDD-w8-m144-nonce','current_name':spec['name'],'stale_date_refused':True,'final120seconds_UTC_refused':True,'isolated_next_day_refresh_consistent':True,'nonce_and_cost_expiry_preserved':True,'live_gate_refresh_refused':True,'post_create_intent_refresh_refused':True,'production_HTTP_requests':0,'resources_created':0};(P/'utc-naming-check-result.json').write_text(json.dumps(checks,indent=2));print(json.dumps(checks))
