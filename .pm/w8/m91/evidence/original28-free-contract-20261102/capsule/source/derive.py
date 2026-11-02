"""Build private reviewed harness derivations from allowlisted SOURCE, never import it."""
import ast
import hashlib
import json
from pathlib import Path

HERE=Path(__file__).resolve().parent
SOURCE=HERE/'original-inputs'
manifest=[]
def derive(path,names,transform=None,prefix=''):
    raw=path.read_text();tree=ast.parse(raw);parts=[]
    for node in tree.body:
        if isinstance(node,(ast.FunctionDef,ast.ClassDef)) and node.name in names:
            original=ast.get_source_segment(raw,node)
            before=ast.dump(node,include_attributes=False)
            if transform:node=transform.visit(node);ast.fix_missing_locations(node)
            emitted=ast.unparse(node)+'\n' if transform else original+'\n'
            manifest.append({'input':str(path.relative_to(HERE)),'input_sha256':hashlib.sha256(raw.encode()).hexdigest(),
                'name':node.name,'first_line':node.lineno,'last_line':node.end_lineno,'original_AST_sha256':hashlib.sha256(before.encode()).hexdigest(),
                'derived_AST_sha256':hashlib.sha256(ast.dump(ast.parse(emitted).body[0],include_attributes=False).encode()).hexdigest(),
                'transformation':prefix or 'Exact function source/AST copy; no top-level import/exec.'})
            parts.append(emitted)
    assert len(parts)==len(names),(path,names)
    return '\n'.join(parts)

class HTTPImports(ast.NodeTransformer):
    def visit_Import(self,node):
        node.names=[n for n in node.names if n.name not in ('subprocess','runpy','urllib.request')]
        return node if node.names else None

class HostedIO(HTTPImports):
    def visit_Call(self,node):
        node=self.generic_visit(node)
        if isinstance(node.func,ast.Name) and node.func.id in ('history','validators','save','now','cli','own','api'):
            node.func=ast.Attribute(ast.Name('ctx',ast.Load()),node.func.id,ast.Load())
        elif isinstance(node.func,ast.Name) and node.func.id=='exec':
            # This exact original one call executed http.py. Registry contains
            # only private dependency-bound derived function definitions.
            assert len(node.args)==2 and isinstance(node.args[1],ast.Name) and node.args[1].id=='scope'
            return ast.Call(ast.Attribute(ast.Name('scope',ast.Load()),'update',ast.Load()),
                [ast.Call(ast.Attribute(ast.Name('ctx',ast.Load()),'http_functions',ast.Load()),[ast.Name('x',ast.Load())],[])],[])
        elif isinstance(node.func,ast.Name) and node.func.id=='Path':
            assert len(node.args)==1
            return ast.Attribute(ast.Name('ctx',ast.Load()),'fixture',ast.Load())
        return node

out=HERE/'derived';out.mkdir(mode=0o700,exist_ok=True)
for mid in (91,93,94,95,96,97,98,99,100,101,102,103,107,222):
    base=SOURCE/'fixtures'/('note222' if mid==222 else 'm'+str(mid))
    validators=base/'validators.py';names=[n.name for n in ast.parse(validators.read_text()).body if isinstance(n,ast.FunctionDef)]
    (out/f'validators_{mid}.py').write_text(derive(validators,names))
    http=base/'http.py'
    if http.exists():
        names=[n.name for n in ast.parse(http.read_text()).body if isinstance(n,ast.FunctionDef)]
        (out/f'http_{mid}.py').write_text(derive(http,names,HTTPImports(),'Only remove local imports of subprocess/runpy/urllib.request; injected bounded providers replace them. All request literals and functional predicates unchanged.'))
(out/'hosted_proofs.py').write_text(derive(SOURCE/'hosted.py',['objects','decode_json'])+'\n'+derive(SOURCE/'hosted.py',['proof','shared'],HostedIO(),'Only explicit Context I/O/state calls and safe private function registry replace old globals/exec/fixture-path; all assertions and literals retained.'))
(out/'html.py').write_text('from html.parser import HTMLParser\nimport hashlib\n'+derive(SOURCE/'fixtures/m93/sweep407-html.py',['inspect_html']))
(out/'png.py').write_text('import struct,zlib,hashlib\n'+derive(SOURCE/'fixtures/m95/sweep406-png.py',['decode_png']))
(out/'naming.py').write_text('from datetime import datetime,timezone,timedelta\n'+derive(SOURCE/'native_naming.py',['utc_now','creation_day']))
(HERE/'DERIVATION-MANIFEST.json').write_text(json.dumps({'scope':'Copied/derived harness functions ONLY, never original top-level/app/shell. Host execution limited fake injected I/O until independent/live approval.','functions':manifest},indent=2)+'\n')
