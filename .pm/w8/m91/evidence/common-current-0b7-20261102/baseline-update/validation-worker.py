import hashlib,json,os,sys,types,socket
from pathlib import Path
from unittest.mock import patch
sys.dont_write_bytecode=True
view=Path(sys.argv[1]);result=Path(sys.argv[2]);helperpath=view/'.github/scripts/native-common-qualify.py'
assert hashlib.sha256(helperpath.read_bytes()).hexdigest()=='f564dae9b771de166d18b16564165b8140a7bb857769cb94fed390cbdec406e5'
module=types.ModuleType('isolated_current0b7_validation');module.__file__=str(helperpath)
exec(compile(helperpath.read_bytes(),str(helperpath),'exec'),module.__dict__)
expected=json.loads((view/'.pm/w8/m91/evidence/common-current-0b7-20261102/utilities/amd64/artifact/qualification-source.json').read_text())
with patch.object(module.subprocess,'run',side_effect=AssertionError('helper subprocess forbidden')),patch.object(module.subprocess,'Popen',side_effect=AssertionError('helper Popen forbidden')),patch.object(socket,'socket',side_effect=AssertionError('network forbidden')):
 image=module.validate_image_candidate(view)
 full=module.validate_test_candidate(view)
 assert module.source_inventory(view)==expected['files']
 out={'PASS':True,'image_baseline_status':image['baseline_status'],'test_baseline_status':full['baseline_status'],'source':full['source']['sha'],'ref':full['ref'],'producer':full['producer'],'verification':full['verification'],'generator_source':full['generator_source'],'source_files':len(expected['files']),'image_input_count':len(module.image_source_inventory(view)),'architectures':list(full['utilities']),'unchanged_helper':True,'helper_subprocess_forbidden':True,'network_forbidden':True}
 result.write_text(json.dumps(out,indent=2)+'\n');print(json.dumps(out))
