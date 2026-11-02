"""Read-only unchanged helper over real retained evidence; not a CI execution."""
from pathlib import Path
import importlib.util, json, os, sys

R=Path(__file__).resolve().parent
VIEW=R/'clean-git301-view'
HEAD='36fd9c8cbc6d6f04fddde1c7dd6eca5ba3e72c01'
assert json.loads((R/'STAGE-RESULT.json').read_bytes())['complete'] is True
os.environ['GITHUB_SHA']=HEAD
sys.dont_write_bytecode=True

def readonly(event,args):
    if event.startswith('socket.') or event in ('subprocess.Popen','os.system','os.exec','os.fork','os.posix_spawn'):
        raise PermissionError('offline validation: network/subprocess denied')
    if event in ('os.remove','os.rmdir','os.rename','os.chmod','os.chown','os.link','os.symlink','os.truncate','os.mkdir'):
        raise PermissionError('read-only validation: filesystem mutation denied')
    if event=='open':
        path,mode,flags=args
        if (isinstance(flags,int) and flags&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREAT|os.O_TRUNC)) or (isinstance(mode,str) and any(x in mode for x in 'wax+')):
            raise PermissionError('read-only validation: file write denied')

sys.addaudithook(readonly)
spec=importlib.util.spec_from_file_location('unchanged_d3_qualification',VIEW/'.github/scripts/native-common-qualify.py')
helper=importlib.util.module_from_spec(spec);spec.loader.exec_module(helper)
result=helper.validate_test_candidate(VIEW)
assert result['baseline_status']=='current-complete'
assert result['source']['sha']=='dc08ae951f2be48f160b035890ced7335222a768'
assert set(result['utilities'])=={'amd64'}
assert result['ref']=='ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
print(json.dumps({'scope':'LOCAL_READONLY_HELPER_VALIDATION_NOT_CI_OR_ADOPTION','status':result['baseline_status'],'actual_executing_utility_source':result['source']['sha'],'current_Git_view':HEAD,'source_paths':len(helper.source_inventory(VIEW)),'utility_architectures':list(result['utilities']),'candidate_receipt_sha256':result['receipt_sha256'],'image':result['ref'],'network_subprocess_and_filesystem_writes_denied':True},indent=2))
