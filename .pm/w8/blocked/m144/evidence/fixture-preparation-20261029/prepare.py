from pathlib import Path
import base64,datetime,hashlib,json,os,secrets,shlex,shutil,zlib
from naming import identity
P=Path(__file__).resolve().parent;os.umask(0o077)
assert not (P/'spec.json').exists(),'do not regenerate this unique identity'
run=secrets.token_hex(8);names=identity(run);name=names['name'];source=(P/'disk-fixture.py').read_bytes();encoded=base64.b64encode(zlib.compress(source,9)).decode()
cmd='python3 -u -c '+shlex.quote('import base64,zlib;exec(compile(zlib.decompress(base64.b64decode('+repr(encoded)+')),"m144-owned-disk-fixture","exec"))')
assert len(cmd.encode())<=4096
(P/'start-command.txt').write_text(cmd)
spec={'run':run,'name_date_utc':names['name_date_utc'],'name':name,'workspace':'tea-da2isimlm39c739m4ofg','disk_name':name+'-disk','mount':'/var/data','initial_gib':1,'grown_gib':5,'plan':'starter','max_runtime_seconds':7200,'max_estimated_usage_usd':0.03,'fixture_source_sha256':hashlib.sha256(source).hexdigest(),'source_repository':'https://github.com/git-fixtures/basic','branch':'master','expected_source_commit':'6ecf0ef2c2dffb796033e5a02219af86ec6584e5','protected_existing_services':['srv-dbhtfte9e4qs73fgrfc0','srv-da40m1qii7bs73drbqlg'],'historical_deleted_215_ids':['srv-dbhm79akl95s73b459i0','dsk-dbhm79akl95s73b459j0'],'created_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
(P/'spec.json').write_text(json.dumps(spec,indent=2));(P/'credentials/fixture-token').write_text(secrets.token_urlsafe(32))
(P/'ledger.json').write_text(json.dumps({'run':run,'workspace':spec['workspace'],'resources':[],'state':'prepared only; no hosted request made'},indent=2))
for f in ['auth.py','cleanup-auth.py']:
 shutil.copyfile(Path('/private/tmp/bex7-native-acceptance-novb586u')/f,P/f)
(P/'release/bex').symlink_to('/private/tmp/bex7-native-acceptance-novb586u/release/bex')
print(json.dumps({'directory':str(P),'name':name,'start_command_bytes':len(cmd.encode()),'resources_created':0}))
