"""Read-only actual utility evidence audit; no image approval or target execution."""
from pathlib import Path
import datetime
import base64
import difflib
import gzip
from html.parser import HTMLParser
import hashlib
import json
import re
import stat
import subprocess
import types
import urllib.parse
import zipfile
import zlib

ROOT = Path('/Users/tianpan/projects/bex7')
INPUT = Path(__file__).resolve().parent / 'intake'
CI = Path(__file__).resolve().parent
OUT = Path(__file__).resolve().parent
SOURCE = 'dc08ae951f2be48f160b035890ced7335222a768'
PRODUCER = '1feba939b4697894e800ed0abb4f25a0bf415641'
RUN_ID = 41428447253
REF = 'ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0'
EXPECTED_MANIFEST = json.loads((OUT / 'ACTUAL-EXTRACTION-BOUND.json').read_bytes())['manifest_sha256']
JOB = {'amd64': 125221453132}

def sha(body):
    return hashlib.sha256(body).hexdigest()

def identity(path):
    body = path.read_bytes()
    return {'bytes': len(body), 'sha256': sha(body)}

def load(path):
    return json.loads(path.read_bytes())

def check(condition, reason):
    if not condition:
        raise AssertionError(reason)

def one(root, suffix):
    rows = list(root.rglob(suffix))
    check(len(rows) == 1, 'ambiguous/missing ' + suffix)
    return rows[0]

def stdout_json(log):
    rows = []
    for line in log.splitlines():
        match = re.fullmatch(r'#[0-9]+ [0-9.]+ (\{.*\})', line)
        candidate = match[1] if match else line
        if candidate.startswith('{'):
            try:
                rows.append(json.loads(candidate))
            except ValueError:
                pass
    return rows

def git_tree(rev):
    raw = subprocess.check_output(['git', 'ls-tree', '-r', '-z', rev], cwd=ROOT)
    result = {}
    for line in raw.split(b'\0'):
        if line:
            data, name = line.split(b'\t', 1)
            mode, kind, blob = data.decode().split()
            result[name.decode()] = {'mode': mode, 'kind': kind, 'blob': blob}
    return result

helper_path = ROOT / '.github/scripts/native-common-qualify.py'
def git_body(rev, name):
    return subprocess.check_output(['git', 'show', rev + ':' + name], cwd=ROOT)
helper_body = git_body(SOURCE, '.github/scripts/native-common-qualify.py')
check(helper_body == helper_path.read_bytes(), 'retained reviewed helper differs from producer source')
helper = types.ModuleType('read_only_source_closure')
helper.__file__ = str(helper_path)
exec(compile(helper_body, str(helper_path), 'exec'), helper.__dict__)
head = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
tree = git_tree(SOURCE)
names = set(helper.SOURCE_FILES) | {'deploy/' + name for name in helper.FACTORY_PATHS}
names.update(name for name in tree if name.startswith('lego/operator/internal/build/'))
for package in helper.SOURCE_PACKAGES:
    prefix = 'lego/' + package + '/'
    names.update(name for name in tree if name.startswith(prefix) and '/' not in name[len(prefix):] and name.endswith('.go'))
check(len(names) == 301, 'producer tracked qualification closure changed')
# Bounded Git object reads; never export a working tree or follow local context links.
objects = subprocess.check_output(['git', 'cat-file', '--batch'], input=''.join(tree[n]['blob']+'\n' for n in sorted(names)).encode(), cwd=ROOT)
check(len(objects) < 16*1024*1024, 'source object read bound')
current, cursor = {}, 0
for name in sorted(names):
    end = objects.index(b'\n', cursor); oid, kind, size = objects[cursor:end].decode().split(); size=int(size)
    body=objects[end+1:end+1+size];cursor=end+1+size
    check(objects[cursor:cursor+1] == b'\n' and len(body)==size and kind=='blob' and oid==tree[name]['blob'], 'Git source object framing')
    cursor+=1
    check(hashlib.sha1(b'blob '+str(len(body)).encode()+b'\0'+body).hexdigest()==oid, 'Git source object identity')
    current[name]={'sha256':sha(body),'bytes':len(body),'mode':tree[name]['mode'],'type':'regular','permissions':int(tree[name]['mode'][-3:],8)}
check(cursor == len(objects), 'unexpected Git source bytes')
origin_tree=git_tree(head)
origin_names=set(helper.SOURCE_FILES)|{'deploy/'+name for name in helper.FACTORY_PATHS}
origin_names.update(name for name in origin_tree if name.startswith('lego/operator/internal/build/'))
for package in helper.SOURCE_PACKAGES:
    prefix='lego/'+package+'/'
    origin_names.update(name for name in origin_tree if name.startswith(prefix) and '/' not in name[len(prefix):] and name.endswith('.go'))
source_delta=sorted(set(names)^origin_names | {n for n in names&origin_names if tree[n]!=origin_tree[n]})
(OUT/'SOURCE-CUSTODY.json').write_text(json.dumps({'producer':SOURCE,'comparison_origin_main':head,'names':current,'source_delta_against_comparison_HEAD':source_delta,'helper_sha256':sha(helper_body),'working_tree_used':False},indent=2)+'\n')
extras = None  # No physical-working-tree/source_inventory call: local HEAD is older.
specs = json.loads(git_body(SOURCE, 'lego/operator/internal/build/testdata/native-utilities/original-cases.json'))
check(len(specs) == 13 and len({x['mid'] for x in specs}) == 13, 'original13 spec set differs')
jobs = {x['id']: x for x in load(CI / 'jobs-final.json')['jobs']}
artifacts = load(CI / 'artifacts-final.json')['artifacts']
results = []

for arch in ('amd64',):
    artifact = INPUT / arch / 'extracted/artifact'
    extraction = load(INPUT / arch / 'extracted/extraction.json')
    manifest_path = artifact / 'MANIFEST.json'
    check(identity(manifest_path)['sha256'] == EXPECTED_MANIFEST[arch] == extraction['manifest_sha256'], 'manifest custody:' + arch)
    rows = load(manifest_path)['files']
    retained = {row['path']: row for row in rows}
    check(len(retained) == len(rows) == 355, 'manifest path duplicates/count')
    actual = set()
    actual_dirs = set()
    for path in artifact.rglob('*'):
        check(not path.is_symlink(), 'unexpected symlink')
        if path.is_dir():
            actual_dirs.add(str(path.relative_to(artifact)))
            continue
        check(stat.S_ISREG(path.lstat().st_mode), 'unexpected special member')
        name = str(path.relative_to(artifact))
        actual.add(name)
        if name != 'MANIFEST.json':
            check(name in retained and path.stat().st_size == retained[name]['bytes'], 'unexpected file/size:' + name)
    check(actual == set(retained) | {'MANIFEST.json'}, 'manifest/actual file bijection differs')
    parents = {str(p) for name in retained for p in Path(name).parents if str(p) != '.'}
    check(actual_dirs == parents, 'unexpected directory')
    check(extraction['complete'] is True and extraction['completed_members'] == 356, 'extractor incomplete')
    check(sum((artifact / n).stat().st_size for n in actual) == extraction['actual_file_bytes'] == extraction['declared_bytes'], 'extraction size differs')
    archive = next(x for x in artifacts if x['name'] == 'common-generated-' + SOURCE + '-' + arch + '-1')
    check(archive['size_in_bytes'] == extraction['zip_bytes'] and archive['digest'] == 'sha256:' + extraction['zip_sha256'], 'GitHub ZIP identity differs')
    check(archive['workflow_run']['id'] == RUN_ID and archive['workflow_run']['head_sha'] == SOURCE, 'artifact run/source differs')
    job = jobs[JOB[arch]]
    check(job['head_sha'] == SOURCE and job['run_attempt'] == 1 and job['conclusion'] == 'success', 'actual successful job differs')
    gate = next(x for x in job['steps'] if x['name'] == 'actual generated offline builds, original replays and exact-default SPDX')
    check(gate['status'] == 'completed' and gate['conclusion'] == 'success', 'actual full utility gate incomplete')
    inputs, captured, cleanup = (load(artifact / name) for name in ('inputs.json', 'qualification-source.json', 'cleanup.json'))
    check(captured == {'source': SOURCE, 'files': current}, 'captured/current exact301 source differs')
    check(inputs['common_image'] == REF and inputs['producer'] == inputs['verification'] == PRODUCER and inputs['generator_source'] == SOURCE, 'input image/source differs')
    check(inputs['platform'] == 'linux/' + arch and inputs['run_id'] == str(RUN_ID) and inputs['run_attempt'] == '1', 'input platform/run differs')
    check(inputs['selection'] == helper.profile_selection('utilities', 'all'), 'utility selector changed')
    approved = json.loads(git_body(SOURCE, '.github/native-common-test-candidate.json'))['candidate']
    check(inputs['image_source'] == 'reviewed-candidate' and inputs['candidate_receipt_sha256'] == sha(git_body(SOURCE, '.github/native-common-test-candidate.json')) and inputs['image_approval_inputs_sha256'] == approved['approval']['inputs']['sha256'], 'manual reviewed image receipt custody differs')
    check(inputs['baseline_status'] == 'image-only-requalification' and inputs['previous_baseline_state'] == 'unvalidated-existing' and inputs['utility_jobs'] is None and inputs['utility_baseline_source'] is None, 'old utility baseline was borrowed')
    for step_name in ('record capacity observations before owned cleanup', "remove only this job's labeled resources, exact builder and credentials", 'hash retained successes and failures'):
        selected = [x for x in job['steps'] if x['name'] == step_name]
        check(len(selected) == 1 and selected[0]['status'] == 'completed' and selected[0]['conclusion'] == 'success', 'actual required cleanup/capacity/manifest gate incomplete:' + step_name)
    uploads = [x for x in job['steps'] if x['name'].startswith('Run actions/upload-artifact@')]
    check(len(uploads) == 1 and uploads[0]['conclusion'] == 'success', 'actual complete artifact upload unproved')
    check(cleanup['owner'] == inputs['owner'] and all(type(x['exit_code']) is int and x['exit_code'] == 0 for x in cleanup['operations']), 'declared cleanup incomplete')
    auth = next(x for x in cleanup['operations'] if x['resource'] == 'docker-auth')
    check(auth['removed'] is True and type(auth['logout_exit_code']) is int and auth['logout_exit_code'] == 0, 'credential scope removal unproved')
    ci_log = (CI / ('job-' + arch + '.log')).read_bytes()
    unprefixed = b''.join(re.sub(rb'^\xef\xbb\xbf', b'', re.sub(rb'^\d{4}-\d\d-\d\dT\S+Z ', b'', line)) for line in ci_log.splitlines(keepends=True))
    cleanup_messages = []
    pull_messages = []
    for line in ci_log.decode().splitlines():
        for marker, target in [('BEX_NATIVE_CLEANUP_RESULT ', cleanup_messages), ('BEX_BUILDKIT_PULL_ATTEMPT ', pull_messages)]:
            if marker in line:
                try:
                    target.append(json.loads(line.split(marker, 1)[1]))
                except ValueError:
                    pass
    expected_cleanup = {'owner': cleanup['owner'], 'operations': [{key: value for key, value in row.items() if row['resource'] != 'docker-auth' or key in ('resource', 'exit_code', 'logout_exit_code', 'removed', 'removal_error')} for row in cleanup['operations']]}
    check(cleanup_messages == [expected_cleanup], 'declared cleanup detached from exact actual CI stdout')
    bootstrap = load(artifact / 'bootstrap.json')
    check(bootstrap['owner'] == inputs['owner'] and bootstrap['image_output_emitted'] is True and len(pull_messages) == len(bootstrap['attempts']), 'actual bootstrap identity differs')
    for packet, attempt in zip(pull_messages, bootstrap['attempts']):
        check(packet['attempt'] == attempt and packet['owner'] == inputs['owner'] and packet['image'] == bootstrap['image'] and packet['command'] == bootstrap['command'], 'actual pull stdout/receipt differs')
        for stream in ('stdout', 'stderr'):
            check(base64.b64decode(packet[stream + '_base64'], validate=True) == (artifact / attempt[stream]['path']).read_bytes(), 'actual full pull byte custody differs')
    qualification_log = (artifact / 'qualification.log').read_bytes()
    check(qualification_log in unprefixed, 'artifact full Go log detached from actual CI stdout')
    case_results = re.findall(rb'--- (PASS|FAIL): (\S+) \(([0-9.]+)s\)', qualification_log)
    check(len(case_results) == 12 and all(x[0] == b'PASS' for x in case_results), 'actual Go cohort incomplete')
    generated = artifact / 'generated'
    check(len(list(generated.iterdir())) == 1, 'unexpected generated root')
    generated = next(generated.iterdir())
    expected_cases = {'supplier-removed', 'node-compute', 'node-static', 'python313-original', 'python3148-default',
                      'missing-jq', 'missing-rsync', 'missing-python-metadata', 'missing-python-pip', 'missing-python-headers'}
    check({x.name for x in generated.iterdir()} == expected_cases, 'unexpected/missing generated case')
    choices_proofs = {}
    for case in sorted(expected_cases):
        choices = load(generated / case / 'resolved-choices.json')
        check(choices['selection_precondition_passed'] is True and choices['choices'], 'resolved selection precondition differs')
        check(choices['scope'] == 'resolved image metadata and selection only; not generated-runtime or qualification success', 'resolved choices scope differs')
        if case == 'python3148-default':
            primary=choices['choices'][0]
            check(choices['exact_default_precondition_applicable'] is True and choices['expected_python_version']=='3.14.8' and primary['line']=='3.14.8' and primary['requested_exact']=='3.14.8' and primary['runtime_environment']==['PYTHON_VERSION=3.14.8'], 'exact default selection identity differs')
        choices_proofs[case] = choices
    positives = []
    for name in ('node-compute', 'node-static', 'python313-original', 'python3148-default'):
        directory = generated / name
        receipt = load(directory / 'receipt.json')
        check(receipt['common_base'] == REF and receipt['image_platform'] == 'linux/' + arch and receipt['network'] == 'none', 'positive image/network identity differs')
        check(receipt['owner'] == receipt['builder'] == inputs['owner'] and re.fullmatch(r'sha256:[a-f0-9]{64}', receipt['image_config_id']), 'owned builder/config differs')
        check(receipt['runtime_command_executed'] is (name != 'node-static') and receipt['static_artifacts_extracted'] is (name == 'node-static'), 'phase coverage differs')
        canonical, executed = (directory / 'Dockerfile.canonical').read_text(), (directory / 'Dockerfile').read_text()
        marker = '\nFROM ' + REF + '\n'
        check(canonical.count(marker) == 1 and 'apt-get' not in executed, 'common final base/tenant package install differs')
        overlay = 'COPY .bex-test-wheels/ /opt/bex-test-wheels/\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n'
        if name.startswith('python'):
            check(receipt['test_prerequisite_overlay'] is True and executed == canonical.replace(marker, marker + overlay, 1), 'Python test overlay changed canonical recipe')
            wheel_manifest = load(directory / 'test-wheelhouse.json')['files']
            wheels = directory / 'prepared-wheels'
            check({x.name for x in wheels.iterdir()} == {x['filename'] for x in wheel_manifest}, 'wheel closure file bijection differs')
            for row in wheel_manifest:
                member = str((wheels / row['filename']).relative_to(artifact))
                check(retained[member]['bytes'] == row['bytes'] and retained[member]['sha256'] == row['sha256'], 'wheel/member custody differs')
            for prefix in ('pyvips-3.2.0-', 'django-5.2.18-', 'poetry-2.1.3-'):
                check(any(x['filename'].lower().startswith(prefix) for x in wheel_manifest), 'required wheel pin absent')
            prefix = canonical.split('\nCOPY . .\n', 1)[0].replace(marker, '\nFROM ' + REF + ' AS bex-test-prerequisites\n', 1)
            poetry = '\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && '
            prepare = '\nRUN --network=default /usr/local/bin/python3 -m pip wheel --disable-pip-version-check --wheel-dir /opt/bex-test-wheels pyvips==3.2.0 Django==5.2.18 poetry==2.1.3\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n'
            check(prefix.count(poetry) == 1, 'canonical Poetry boundary differs')
            expected = prefix.replace(poetry, prepare + poetry, 1) + '\nFROM scratch\nCOPY --from=bex-test-prerequisites /opt/bex-test-wheels/ /\n'
            check((directory / 'Dockerfile.prerequisites').read_text() == expected, 'prerequisite recipe differs beyond reviewed wheel export')
        else:
            check(receipt['test_prerequisite_overlay'] is False and canonical == executed, 'unexpected Node recipe overlay')
        check(any(x.get('phase') == 'build' and x.get('pass') is True for x in stdout_json((directory / 'build.log').read_text())), 'actual build phase proof missing')
        if name != 'node-static':
            check(any(x.get('phase') == 'runtime' and x.get('pass') is True for x in stdout_json((directory / 'runtime.log').read_text())), 'actual runtime phase proof missing')
        positives.append({'case': name, 'image_config_id': receipt['image_config_id'], 'overlay': receipt['test_prerequisite_overlay']})
    for spec in specs:
        case = 'node-compute' if spec['runtime'] == 'node' else 'python313-original'
        output = generated / case / 'qa-output'
        work = output / ('m' + str(spec['mid']))
        for name, expected in spec['primary_reference']['files'].items():
            check(identity(work / name) == expected, 'original data identity differs')
        for name, expected in spec['source_file_identities'].items():
            if name.endswith('-marker.json') or (spec['mid'] == 103 and name == 'qm.json'):
                check(load(work / name) == spec['marker'], 'prepared marker differs')
            else:
                check(identity(work / name) == expected, 'original authored program/data bytes differ')
        check((output / ('build-' + str(spec['mid']) + '.log')).stat().st_size > 0, 'authored build output missing')
        responses = load(output / ('runtime-' + str(spec['mid']) + '-runtime.json'))
        check(len(responses) == 3 and all(x.get('pass') is True for x in responses), 'original3 HTTP proofs incomplete')
        for row in responses:
            check(spec['nonce'] in row['path'], 'foreign/stale HTTP path identity')
            if 'HTTP' in row:
                check(row['HTTP'] == '200' and row['curl_exit'] == 0, 'actual HTTP status/transport differs')
            if 'raw' in row:
                raw = row['raw'].encode()
                check(len(raw) == row['bytes'] and sha(raw) == row['sha256'], 'HTTP raw byte/hash binding differs')
                if 'decoded' in row:
                    check(json.loads(raw) == row['decoded'], 'HTTP decoded JSON binding differs')
                else:
                    prefix = str(work.relative_to(artifact)) + '/'
                    check('text/html' in row['content_type'] and any(name.startswith(prefix) and value['bytes'] == len(raw) and value['sha256'] == sha(raw) for name, value in retained.items()), 'HTML raw response not bound to actual generated file')
                    if spec['mid'] == 93:
                        oracle = row['independent_document_HTML_parse']
                        class DocumentParser(HTMLParser):
                            def __init__(self):
                                super().__init__(convert_charrefs=True)
                                self.stack, self.elements, self.links = [], {}, []
                            def handle_starttag(self, tag, attrs):
                                self.stack.append(tag)
                                if tag == 'a':
                                    self.links.append(dict(attrs).get('href'))
                            def handle_endtag(self, tag):
                                if self.stack and self.stack[-1] == tag:
                                    self.stack.pop()
                            def handle_data(self, text):
                                if text.strip() and self.stack and self.stack[-1] in oracle['elements']:
                                    self.elements.setdefault(self.stack[-1], []).append(text)
                        parser = DocumentParser()
                        parser.feed(raw.decode())
                        parser.close()
                        check(oracle['pass'] is True and oracle['bytes'] == len(raw) and oracle['sha256'] == sha(raw) and parser.elements == oracle['elements'] and parser.links == oracle['links'], 'original HTML oracle incomplete')
    for removed in ('jq', 'rsync', 'python-metadata', 'python-pip', 'python-headers'):
        directory = generated / ('missing-' + removed)
        receipt = load(directory / 'omission-receipt.json')
        check(receipt['common_base'] == REF and receipt['removed_supplier'] == removed and receipt['healthy_independent_prerequisites_observed'] is True and receipt['original_command_expected_failure_observed'] is True, 'omission identity/proof differs')
        text = (directory / 'build.log').read_text()
        ready = re.search(r'(?m)^#[0-9]+ [0-9.]+ BEX_SUPPLIER_REMOVAL_READY ' + re.escape(receipt['fresh_stdout_probe']) + r'$', text)
        check(ready is not None and receipt['expected_failure'] in text[ready.end():], 'omission must reach exact original error after fresh actual stdout')
    negative = (generated / 'supplier-removed/build.log').read_text()
    check('FileNotFoundError' in negative and 'pg_restore' in negative, 'full supplier removal didnot reach original PG command')
    public = generated / 'node-static/public'
    payloads = {'empty': b'', 'binary': bytes(range(256)) * 64, 'unicode': ('Crème 雪\n' * 100).encode()}
    for name, body in payloads.items():
        compressed = (public / (name + '.gz')).read_bytes()
        check(compressed[:8] == b'\x1f\x8b\x08\x00\x00\x00\x00\x00' and int.from_bytes(compressed[-8:-4], 'little') == zlib.crc32(body) and int.from_bytes(compressed[-4:], 'little') == len(body), 'static gzip header/CRC/ISIZE differs')
        check(gzip.decompress(compressed) == body, 'static gzip exact payload/CRC differs')
    zip_members = {'empty.txt': b'', 'greeting.txt': ('Crème brûlée & 李\n' * 100).encode(), 'two words.bin': bytes(range(256)) * 64}
    for name, method in [('stored', zipfile.ZIP_STORED), ('deflated', zipfile.ZIP_DEFLATED)]:
        with zipfile.ZipFile(public / (name + '.zip')) as archive:
            check(set(archive.namelist()) == set(zip_members), 'static ZIP member set differs')
            for member, body in zip_members.items():
                info = archive.getinfo(member)
                check(archive.read(member) == body and info.file_size == len(body) and info.CRC == zlib.crc32(body), 'static ZIP data/size/CRC differs')
            methods = [info.compress_type for info in archive.infolist()]
            check(all(value == zipfile.ZIP_STORED for value in methods) if name == 'stored' else any(value == zipfile.ZIP_DEFLATED for value in methods), 'original static ZIP compression-method contract differs')
    default = generated / 'python3148-default'
    config = load(default / 'receipt.json')['image_config_id']
    for phase in ('build', 'runtime'):
        proof = load(default / 'qa-output' / ('common-default-' + phase + '.json'))
        check(proof['phase'] == phase and proof['python'] == proof['python_env'] == '3.14.8' and proof['executable'].startswith('/usr/local/') and proof['loaded_library'] == 'libpython3.14.so' and 'not found' not in proof['ldd'] and proof['pass'] is True, 'actual default identity/loader differs')
        check(proof['packaging_and_c_extension_executed'] is (phase == 'build'), 'default phase claim differs')
    binding = load(default / 'generated-spdx-binding.json')
    syft, spdx = load(default / 'generated.syft.json'), load(default / 'generated.spdx.json')
    metadata = syft['source']['metadata']
    check(syft['source']['type'] == 'image' and metadata['imageID'] == config and (metadata['os'], metadata['architecture']) == ('linux', arch), 'actual Syft config/platform differs')
    roots = set(spdx.get('documentDescribes', []))
    roots.update(x['relatedSpdxElement'] for x in spdx.get('relationships', []) if x.get('spdxElementId') == spdx['SPDXID'] and x.get('relationshipType') == 'DESCRIBES')
    described = [x for x in spdx['packages'] if x.get('SPDXID') in roots]
    check(len(described) == 1 and {'sha256:' + x['checksumValue'] for x in described[0]['checksums'] if x['algorithm'] == 'SHA256'} == {metadata['manifestDigest']}, 'SPDX root doesnot bind same scan manifest')
    arches = set()
    for package in spdx['packages']:
        for external in package.get('externalRefs', []):
            locator = external.get('referenceLocator', '')
            if locator.startswith('pkg:deb/'):
                arches.update(x for x in urllib.parse.parse_qs(urllib.parse.urlsplit(locator).query).get('arch', []) if x != 'all')
    python = [x for x in spdx['packages'] if x.get('name') == 'python' and x.get('versionInfo') == '3.14.8']
    brotli = sorted({x['versionInfo'] for x in spdx['packages'] if x.get('name') == 'libbrotli1'})
    check(arches == {arch} and python and brotli == ['1.0.9-2+deb12u1'], 'actual exactPython/fixedBrotli/architecture package evidence differs')
    observed_binding = {'image_config_id': config, 'syft_manifest_digest': metadata['manifestDigest'], 'platform': 'linux/' + arch,
                        'python': '3.14.8', 'libbrotli1_versions': brotli, 'packages': len(spdx['packages'])}
    check(all(binding[key] == value for key, value in observed_binding.items()), 'cached SPDX binding differs from full actual report data')
    for kind in ('syft', 'spdx'):
        custody = binding['custody'][kind]
        check(custody['roundtrip'] is True, 'report roundtrip undeclared')
        for form in ('raw', 'gzip'):
            path = default / custody[form + '_file']
            member = str(path.relative_to(artifact))
            check(retained[member]['bytes'] == custody[form + '_bytes'] and retained[member]['sha256'] == custody[form + '_sha256'], 'full raw/gzip manifest custody differs')
            check(identity(path) == {'bytes': custody[form + '_bytes'], 'sha256': custody[form + '_sha256']}, 'actual full raw/gzip bytes differ')
        digest = hashlib.sha256()
        with gzip.open(default / custody['gzip_file'], 'rb') as stream:
            for chunk in iter(lambda: stream.read(1024 * 1024), b''):
                digest.update(chunk)
        check(digest.hexdigest() == custody['raw_sha256'], 'actual full SBOM gzip roundtrip differs')
    del syft, spdx
    critical = ['inputs.json', 'qualification-source.json', 'qualification.log', 'cleanup.json', 'bootstrap.json']
    critical.extend(str(path.relative_to(artifact)) for path in generated.rglob('*.json') if path.name in ('receipt.json', 'omission-receipt.json', 'generated-spdx-binding.json', 'test-wheelhouse.json', 'common-default-build.json', 'common-default-runtime.json', 'resolved-choices.json'))
    for name in critical:
        check(identity(artifact / name) == {key: retained[name][key] for key in ('bytes', 'sha256')}, 'critical member hash differs:' + name)
    results.append({'architecture': arch, 'job_id': JOB[arch], 'manifest': identity(manifest_path), 'files': len(actual),
                    'original_extraction': extraction, 'critical_member_hashes_checked': len(critical),
                    'captured_source': identity(artifact / 'qualification-source.json'), 'captured_source_paths': 301,
                    'resolved_choices_metadata':choices_proofs, 'positive_cases': positives, 'original_authored_cases': 13, 'original_runtime_HTTP_results': 39,
                    'omissions': 5, 'supplier_removal': True, 'static_gzip_ZIP_payload_CRC_checks': True,
                    'actual_default_SBOM': observed_binding, 'full_Syft_and_SPDX_gzip_roundtrips_checked': True,
                    'actual_cleanup_declared_success_and_CI_gate_success': True, 'all_reviewed_content_checks': 'PASS'})

# D3 production baseline is AMD64-only; per-arch exact captured-source check above remains mandatory.

report = {'schema': 'bex.D3-dc08-amd64-actual-utility-content-review/v1', 'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'outcome': 'PASS_D3_DC08_AMD64_UTILITY_CONTENT_ONLY_PENDING_ROOT_APPROVAL', 'candidate_ref': REF, 'source': SOURCE,
          'comparison_HEAD_at_audit': head, 'generator_source_checked_from_Git': SOURCE, 'unchanged_approved_image_producer_verification': PRODUCER, 'source_delta_against_comparison_HEAD':source_delta, 'physical_working_tree_compared':False, 'current_tracked_qualification_source_paths': 301, 'current_physical_source_extra_paths': extras,
          'helper_source': identity(helper_path), 'architectures': results,
          'limits': ['No image/candidate/baseline/production write.0941approvalwasalreadyRoot-reviewedandisunchanged; thisisdetailedD3dc08AMD64utilitycontentreview; Rootapprovalisstillrequiredforbaselineadoption.',
                     'No target/program/original-validator/Go/network/Docker/Kubernetes/auth or repository execution/mutation.',
                     'Original ZIP extraction CRC/all-memberSHA evidence was reused; no artifactZIP decompression repeated. Static fixture ZIP/gzip and complete SBOM gzip roundtrips were independently checked.',
                     'Some authored HTTP records retain decoded/asserted outcomes rather than every rawbody (notably m94); no byte-level replay of unretained bodies claimed.',
                     'Packaging/Cextension is explicitly build-only in default proof; runtime verifies exact interpreter/ENV/loader/Brotli. No predeploy/Kubernetes/native-init/allfamily/hosted28 claim.',
                     'Producer Git301 capture is exact. Stale local working tree was not compared or used as current source; no ignored local files read, removed, or waived. No baseline validator invoked.']}
(OUT / 'REVIEW.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'outcome': report['outcome'], 'root': str(OUT), 'review': identity(OUT / 'REVIEW.json'),
                  'arches': [{'architecture': row['architecture'], 'files': row['files'], 'packages': row['actual_default_SBOM']['packages']} for row in results],
                  'local_source_extras': extras}, indent=2))
