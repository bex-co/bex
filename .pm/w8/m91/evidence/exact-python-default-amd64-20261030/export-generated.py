from pathlib import Path
import json,os,subprocess,tempfile
repo=Path('/Users/tianpan/projects/bex7')
root=Path('/private/tmp/bex-w8-222-amd64-acceptance')
source=repo/'lego/operator/internal/build/nativebases_test.go'
extra=(root/'exporter-test-overlay.go.txt').read_text()
with tempfile.TemporaryDirectory(prefix='bex222-amd64-export-overlay-') as temp:
    patched=Path(temp)/source.name
    patched.write_text(source.read_text()+extra)
    overlay=Path(temp)/'overlay.json'
    overlay.write_text(json.dumps({'Replace':{str(source):str(patched)}}))
    env=dict(os.environ,GOWORK='off',BEX_NATIVE_QA_OUTPUT=str(root),BEX_NATIVE_QA_IMAGE=(root/'supplier-reference.txt').read_text().strip())
    with (root/'generator-export.log').open('w') as log:
        subprocess.run(['go','test','-overlay',str(overlay),'./internal/build','-run','^TestScratchPythonDefaultRecipe$','-count=1','-v'],cwd=repo/'lego/operator',env=env,stdout=log,stderr=subprocess.STDOUT,check=True)
with (root/'generated-build.log').open('w') as log:
    p=subprocess.run(['docker','build','--platform','linux/amd64','--progress=plain','-t','bex-w8-222-python-default:amd64','--secret','id=render-env,src='+str(root/'render-env'),str(root/'generated-context')],env=dict(os.environ,DOCKER_BUILDKIT='1'),stdout=log,stderr=subprocess.STDOUT)
(root/'generated-build-exit.txt').write_text(str(p.returncode)+'\n')
raise SystemExit(p.returncode)
