"""Exact retained-proof calibration of the one corrected audit expression."""
from pathlib import Path
import ast, copy, json, unittest

R = Path(__file__).resolve().parent

def predicate(name):
    tree=ast.parse((R/name).read_bytes())
    node=next(n for n in ast.walk(tree) if isinstance(n,ast.Call) and isinstance(n.func,ast.Name) and n.func.id=='need' and len(n.args)==2 and isinstance(n.args[1],ast.Constant) and n.args[1].value=='actual SSL/FFI/SONAME/link closure')
    args=ast.arguments(posonlyargs=[],args=[ast.arg(arg='data'),ast.arg(arg='line')],kwonlyargs=[],kw_defaults=[],defaults=[])
    return eval(compile(ast.fix_missing_locations(ast.Expression(body=ast.Lambda(args=args,body=node.args[0]))),'exact-audit-expression','eval'))

old,new=predicate('audit.py'),predicate('audit-soname-v2.py')

def proof_rows():
    for item in json.loads((R/'INPUTS.json').read_bytes()):
        for path in sorted((R/'outputs'/item['key']/'extracted/artifact').glob('generated/*/'+item['version']+'/qa-output/legacy-*.json')):
            data=json.loads(path.read_bytes())
            for kind in ('app','project'):yield item,data[kind]

class ABIControls(unittest.TestCase):
    def test_actual_both37_arch_build_runtime_app_project_old_red_new_pass(self):
        rows=[(i,d) for i,d in proof_rows() if i['case']=='3.7'];self.assertEqual(len(rows),8)
        for item,data in rows:
            with self.subTest(arch=item['architecture'],executable=data['executable']):
                self.assertEqual(data['libpython_soname'],'libpython3.7m.so.1.0')
                self.assertFalse(old(data,item['case']));self.assertTrue(new(data,item['case']))

    def test_existing_modern_predicate_unchanged_on_all_retained_phase_kind_rows(self):
        rows=[(i,d) for i,d in proof_rows() if i['case']!='3.7'];self.assertEqual(len(rows),12)
        for item,data in rows:self.assertTrue(old(data,item['case']) and new(data,item['case']))

    def test_wrong_version_missing_or_injected37_abi_refused(self):
        _,actual=next((i,d) for i,d in proof_rows() if i['case']=='3.7')
        for name in ('libpython3.6m.so.1.0','libpython3.7.so.1.0','libpython3.7foo.so.1.0','libpython3.8.so.1.0'):
            with self.subTest(soname=name):self.assertFalse(new({**actual,'libpython_soname':name},'3.7'))

    def test_missing_unresolved_link_and_wrong_ssl_still_refused(self):
        _,actual=next((i,d) for i,d in proof_rows() if i['case']=='3.7')
        bad=copy.deepcopy(actual);bad['linked'][next(iter(bad['linked']))]+='\nlibssl.so => not found\n'
        self.assertFalse(new(bad,'3.7'))
        bad=copy.deepcopy(actual);bad['linked'].pop(next(iter(bad['linked'])))
        self.assertFalse(new(bad,'3.7'))
        self.assertFalse(new({**actual,'openssl':'unknown'},'3.7'))

if __name__=='__main__':unittest.main(verbosity=2)
