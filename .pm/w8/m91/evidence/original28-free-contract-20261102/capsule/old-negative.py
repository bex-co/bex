import ast,copy,sys,unittest
from pathlib import Path
import opaque
import test_free_contract
old=Path('/private/tmp/bex-w8-original28-free-contract-source-20261101-_wkb6ut8/opaque.py')
tree=ast.parse(old.read_bytes())
cls=next(n for n in tree.body if isinstance(n,ast.ClassDef) and n.name=='OpaqueTransport')
method=next(n for n in cls.body if isinstance(n,ast.FunctionDef) and n.name=='free_gate')
namespace=opaque.__dict__
exec(compile(ast.fix_missing_locations(ast.Module(body=[copy.deepcopy(method)],type_ignores=[])),str(old),'exec'),namespace)
opaque.OpaqueTransport.free_gate=namespace['free_gate']
suite=unittest.TestSuite([test_free_contract.FreeContractControls('test_actual_opaque_method_succeeds_without_optional_coverage')])
result=unittest.TextTestRunner(verbosity=2).run(suite)
assert len(result.errors)==1 and not result.failures and "KeyError: 'coverage'" in result.errors[0][1]
print('EXPECTED_OLD_SOURCE_ERROR_ONLY: missing optional coverage; new regression reaches real old method')
