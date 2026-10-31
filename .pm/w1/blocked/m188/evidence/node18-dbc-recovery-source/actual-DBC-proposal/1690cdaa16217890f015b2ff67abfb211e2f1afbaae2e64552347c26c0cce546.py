#!/usr/bin/env python3
"""Read-only m188 proposal generator. Repository changes always require root review."""
import argparse, base64, datetime, gzip, hashlib, importlib.util, json, re, subprocess, sys
from urllib.parse import unquote, urlparse, parse_qs
from pathlib import Path

sys.dont_write_bytecode = True
ROOT = Path('/Users/tianpan/projects/bex')
NATIVE = 'lego/operator/internal/build/native.go'
INVENTORY = 'lego/operator/internal/build/toolchain-freshness.json'
REPO = 'ghcr.io/bex-co/bex-cnb-run'
IDENTITY = 'https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main'
ANCESTOR = 'a1f63153a9a5f84fbde0b9ecc29319a9f0da09d0'
FIXED_RECIPE = 'bfdeb62211ae5c4a875f5474a51c63c036e1d4dd'
RECOVERY_WORKFLOW = 'da0e56e80a1e4eb08cacabd67efde3c768a6ae81'
REVIEWED_WORKFLOW_SHA256 = 'a85673654cd0b69e35797791fe6e87906e5625637bcf6b43b4937e4730b29547'
APPROVED_PRODUCER_SOURCES = {FIXED_RECIPE, RECOVERY_WORKFLOW, 'dbc493ba688c19758c9a489635761419fb2c1af6', '846a56038319f1b5c212d1fcda8291d20e49c2b0', '3990b9acf35f62ba1d702387bc2061a83342b2b3', 'fd358d2010f8b5e2715250d7148a505ba12c1cb6', '5e89a6076905a91b4496fe36a476774c3f50585c', 'd95645e9f054238ec53a4932fac51560ffedd1f8', '49ee8d3c4f58bfbf6dce649fd9ab236da14c1d11', '4330878c280bc9748444c8e1e50ff7388df1228a', '21ed733d6d47eb48cb6dcf1b7ffa396c3bb46314', 'c8b90f83c289d5d11502c2f0c26a183630dcdec6', 'c8ca76473129d092a8d74ada5e09c5c4705987cd', 'd94787bb4fb93751e485fe6d47946d82cc6da052', 'e47a2c6157339a3a786118866f28b4ee8b619774', 'd52ebeac794ad48b38e5a42b3564943aaf50ad41', '5803d2030fadbd6cddb9e04e73acc4d9ddf82a0e', '99f95c17afd5709365869dfff1408265e5d514ec', '4462ee030984637bb765ae560b6ffe38c71f3af4', '6fd4f45536a2574c404b05539dacb6aaafb781f8'}
APPROVED_VERIFICATION_SOURCES = set(APPROVED_PRODUCER_SOURCES)
APPROVED_SOURCES = APPROVED_PRODUCER_SOURCES | APPROVED_VERIFICATION_SOURCES
PRODUCER_RUNTIME_RESTRICTIONS = {'49ee8d3c4f58bfbf6dce649fd9ab236da14c1d11': {'node-18-bookworm'}, 'dbc493ba688c19758c9a489635761419fb2c1af6': {'node-18-bookworm'}}
VERIFICATION_RUNTIME_RESTRICTIONS = {'dbc493ba688c19758c9a489635761419fb2c1af6': {'node-18-bookworm'}}
SOURCE_MODEL = 'separate_image_producer_and_signature_verification/v2'

# Exact additive tool repair permits historical signed recipes beside this current tree.
INSTALL_RECIPE_PATH = 'deploy/native-base/install.sh'
HISTORICAL_INSTALL_SHA256 = 'd3622e1745ada843c84cbcb113ba7c5e305ccc2d8e4058f75c38d1f05f4ac0b8'
REVIEWED_CURRENT_INSTALL_SHA256 = 'fe880434bb8f428500be52cd626534f69c9ae089d162e3549a50a769b09e5462'
HISTORICAL_INSTALL_PRODUCERS = frozenset(['21ed733d6d47eb48cb6dcf1b7ffa396c3bb46314', '3990b9acf35f62ba1d702387bc2061a83342b2b3', '4330878c280bc9748444c8e1e50ff7388df1228a', '4462ee030984637bb765ae560b6ffe38c71f3af4', '49ee8d3c4f58bfbf6dce649fd9ab236da14c1d11', '5803d2030fadbd6cddb9e04e73acc4d9ddf82a0e', '5e89a6076905a91b4496fe36a476774c3f50585c', '6fd4f45536a2574c404b05539dacb6aaafb781f8', '846a56038319f1b5c212d1fcda8291d20e49c2b0', '99f95c17afd5709365869dfff1408265e5d514ec', 'bfdeb62211ae5c4a875f5474a51c63c036e1d4dd', 'c8b90f83c289d5d11502c2f0c26a183630dcdec6', 'c8ca76473129d092a8d74ada5e09c5c4705987cd', 'd52ebeac794ad48b38e5a42b3564943aaf50ad41', 'd94787bb4fb93751e485fe6d47946d82cc6da052', 'd95645e9f054238ec53a4932fac51560ffedd1f8', 'e47a2c6157339a3a786118866f28b4ee8b619774', 'fd358d2010f8b5e2715250d7148a505ba12c1cb6'])
WORKFLOW = '.github/workflows/native-base-image.yml'

# Reviewed current checkout compatibility only; these are NOT artifact source roles.
CURRENT_TREE_RECIPE_SOURCE = '94165237dbe63f997b0119ce70ad243da74468fa'
CURRENT_TREE_WORKFLOW_SOURCE = '2cc8723df139bc4684d8d2a4d1eebc0c9dca004c'
CURRENT_TREE_WORKFLOW_SHA256 = 'b0e11ada0c7f07ecd8964029f164d1f7c284063255f80413214bc94fde268370'
CURRENT_TREE_RECIPE_PRODUCERS = HISTORICAL_INSTALL_PRODUCERS | frozenset({'da0e56e80a1e4eb08cacabd67efde3c768a6ae81'})
CURRENT_TREE_RECIPE_FILES = {'deploy/native-base/install.sh': '3a83ad0af37163a78a68f92d0e281796866d475353ab514f928c713cf6951eb3', 'deploy/native-base/fetch-python-tools.py': 'eccbb9cb2befa6b37870a06e833ac41b647423639e163282be5a4ebd23e86afe', 'deploy/native-base/fetch_pgdg_key_test.py': '562af6c74af5e071e47f02e035b8db0ff6f168d2389e69772f47f4c359e8686b', 'deploy/native-base/fetch_python_tools_test.py': 'f27a4bbd7491a85b54c7eaa61c9594c4ca6de5a5247688af103fff4c9c17e230'}
CURRENT_TREE_EXTRA_RECIPE_FILES = {'deploy/native-base/fetch_pgdg_key_test.py': '562af6c74af5e071e47f02e035b8db0ff6f168d2389e69772f47f4c359e8686b', 'deploy/native-base/fetch_python_tools_test.py': 'f27a4bbd7491a85b54c7eaa61c9594c4ca6de5a5247688af103fff4c9c17e230'}
CURRENT_TREE_SIGNING_INPUTS = {'.github/scripts/native-oci-stage.py': '891d6581c29c8d3d39be80b60882c0e05f974e167c5f1784713b69d3fa159d59', '.github/scripts/test_native_oci_stage.py': 'd1e7f01511863391f3c8a2bcc8c1f80e42c30fe3f04fd1dc84f598e2acd69803', '.github/scripts/test_native_oci_workflow.py': '6538a7ba0273c65593933f0d9d34acdf0f788380c4f09ed620abf1f1519e37ad'}
CURRENT_TREE_OLD_FETCH_SHA256 = '93a5325abd7b65bb34223b83bcce314869955ee3c9a12fb3e0a172ee43cb1ff6'
CURRENT_TREE_WORKFLOW_REVERSALS = ((b"      - name: test verified OCI staging and publication gates\n        run: |\n          python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'\n          python3 -B -m unittest discover -s deploy/native-base -p '*_test.py'\n", b"      - name: test verified OCI staging and publication gates\n        run: python3 -B -m unittest discover -s .github/scripts -p 'test_native_oci_*.py'\n"), (b'            # JavaDB is almost 1GiB; keep its fetch outside the complete scan budget.\n            database_timeout=21m\n            if [ "$database" = --download-java-db-only ]; then database_timeout=30m; fi\n            TMPDIR="$work_dir/db" env -u TRIVY_OUTPUT timeout --kill-after=10s "$database_timeout" trivy image "$database"\n', b'            TMPDIR="$work_dir/db" env -u TRIVY_OUTPUT timeout --kill-after=10s 21m trivy image "$database"\n'))
LOCAL_GREEN_PROOFS = {
    'portable-node20-compiler': '1df61ff850304884c6aade1130fc79a6cab8c77436be306d0c7f150f42b7eeaa',
    'companion-native-green': '259145659edd6435b9e298d69340572ef57c42d57b7f2bd8711dfcb2748bac76',
    'portable-node20-final-native': 'a8b2aa02a8b1077c8ff2fe1a39b7b57dac985b4258d76c34b708aabfd615aa37',
    'portable-python-ruby-suppliers': 'be27636357048f167a32ab1842e9ce11bd718274f071317c511b9139ebb9ea3d',
}
TARGETS = [('node', x) for x in ('14', '16', '18', '20')] + [('python', x) for x in ('3.7', '3.8', '3.9', '3.10')] + [('ruby', x) for x in ('3.0', '3.1', '3.2')]
GATES = {'verify real native tool transformations without network on both architectures', 'scan amd64 candidate', 'scan arm64 candidate', 'sign and attest the immutable candidate'}


def check(condition, message):
    if not condition:
        raise ValueError(message)


def read_proof_bytes(record):
    """Read manifest-selected original/derived bytes, preserving raw and gzip hashes."""
    path = Path(record['file'])
    data = path.read_bytes()
    compressed = record.get('compression') == 'gzip' or path.suffix == '.gz'
    if compressed:
        check(hashlib.sha256(data).hexdigest() == record.get('gzip_sha256'), 'compressed proof hash mismatch: ' + str(path))
        data = gzip.decompress(data)
    expected = record.get('raw_sha256', record.get('sha256'))
    check(hashlib.sha256(data).hexdigest() == expected, 'proof raw hash mismatch: ' + str(path))
    if 'raw_bytes' in record:
        check(len(data) == record['raw_bytes'], 'proof raw size mismatch: ' + str(path))
    return data


TARGETED_MODE = 'targeted_sigstore_bundles/v1'
COSIGN_SHA256 = '9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4'
PREDICATES = {'image_signature': 'https://sigstore.dev/cosign/sign/v1', 'spdx': 'https://spdx.dev/Document', 'trivy': 'https://aquasecurity.github.io/trivy/attestation/vuln/v1'}


def hashed_file(path, digest):
    raw = Path(path).read_bytes()
    check(hashlib.sha256(raw).hexdigest() == digest, 'persisted proof hash mismatch: ' + str(path))
    return raw


def proof_command(reference):
    command = json.loads(hashed_file(reference['command_file'], reference['command_sha256']))
    check(command.get('exit_code') == reference.get('exit_code'), 'command exit result mismatch')
    for stream in ('stdout', 'stderr'):
        hashed_file(command[stream + '_file'], command[stream + '_sha256'])
    return command


def check_bundle_command(command, item, proof):
    tool, root = proof['tool'], proof['root_index']
    expected = [tool['file'], 'verify-blob-attestation', '--bundle', item['raw_file_at_verification'], '--type', item['predicate_type'], '--check-claims=true', '--use-signed-timestamps=true', '--certificate-identity=' + IDENTITY, '--certificate-oidc-issuer=https://token.actions.githubusercontent.com', '--certificate-github-workflow-sha=' + proof['verification_source'], '--timeout=120s', root['file']]
    check(command.get('argv') == expected, 'bundle verification command differs from exact strict invocation')
    check(command.get('exit_code') == item.get('exit_code') == 0, 'raw bundle verification did not succeed')
    check(command.get('verification_source') == proof['verification_source'], 'bundle command verification SHA mismatch')
    check(command.get('root_index_file') == root['file'] and command.get('root_index_sha256') == root['sha256'], 'bundle command root index mismatch')
    check(command.get('bundle_file_at_verification') == item['raw_file_at_verification'] and command.get('bundle_raw_sha256') == item['raw_sha256'], 'bundle command raw input mismatch')
    check('Verified OK' in Path(command['stderr_file']).read_text(), 'missing actual cosign success output')


def envelope_hash(envelope):
    return hashlib.sha256(json.dumps(envelope, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def bundle_statement(item, proof):
    check(item['role'] in PREDICATES and item['predicate_type'] == PREDICATES[item['role']], 'unexpected bundle predicate')
    check(item['oci_blob_digest'] == 'sha256:' + item['raw_sha256'], 'OCI bundle digest mismatch')
    raw = read_proof_bytes({'file': item['gzip_file'], 'gzip_sha256': item['gzip_sha256'], 'raw_sha256': item['raw_sha256'], 'raw_bytes': item['raw_bytes']})
    manifest = json.loads(hashed_file(item['oci_manifest_file'], item['oci_manifest_digest'].removeprefix('sha256:')))
    check(manifest.get('subject', {}).get('digest') == 'sha256:' + proof['root_index']['sha256'], 'OCI bundle manifest subject mismatch')
    layers = manifest.get('layers', [])
    check(len(layers) == 1 and layers[0].get('digest') == item['oci_blob_digest'] and layers[0].get('size') == len(raw), 'OCI manifest does not describe exact verified bundle')
    bundle = json.loads(raw)
    check(bundle.get('mediaType', '').startswith('application/vnd.dev.sigstore.bundle.'), 'not a Sigstore bundle')
    material = bundle.get('verificationMaterial', {})
    check(material.get('certificate', {}).get('rawBytes') and material.get('tlogEntries') and material.get('timestampVerificationData', {}).get('rfc3161Timestamps'), 'missing certificate/transparency/TSA bundle material')
    check(all(entry.get('inclusionProof') and entry.get('canonicalizedBody') for entry in material['tlogEntries']), 'missing transparency inclusion material')
    command = proof_command(item)
    check_bundle_command(command, item, proof)
    envelope = bundle['dsseEnvelope']
    check(envelope.get('payloadType') == 'application/vnd.in-toto+json' and envelope.get('signatures'), 'invalid signed DSSE envelope')
    payload = base64.b64decode(envelope['payload'], validate=True)
    check(hashlib.sha256(payload).hexdigest() == item['statement_sha256'], 'decoded signed statement hash mismatch')
    statement = json.loads(payload)
    check(statement.get('_type') in ('https://in-toto.io/Statement/v0.1', 'https://in-toto.io/Statement/v1') and statement.get('predicateType') == item['predicate_type'], 'decoded statement type mismatch')
    subjects = statement.get('subject', [])
    check(len(subjects) == 1 and subjects[0] == item['subject'] and subjects[0].get('digest') == {'sha256': proof['root_index']['sha256']}, 'decoded signed root subject mismatch')
    check(subjects[0].get('name', '') == ('' if item['role'] == 'image_signature' else REPO), 'unexpected signed subject name; no reconstructed DockerReference allowed')
    return statement, envelope_hash(envelope)


def check_targeted_controls(proof):
    controls = json.loads(read_proof_bytes(proof['controls']))
    check(controls.get('schema') == 'targeted_sigstore_bundle_controls/v1' and controls.get('passed') is True and controls.get('verification_source') == proof['verification_source'] and controls.get('root_index_sha256') == proof['root_index']['sha256'], 'actual Sigstore controls lack source/index binding')
    signature = next(item for item in proof['bundles'] if item['role'] == 'image_signature')
    check(controls.get('bundle_sha256') == signature['raw_sha256'], 'control bundle differs from verified image signature')
    check(controls.get('positive') == {key: signature[key] for key in ('command_file', 'command_sha256', 'exit_code')}, 'control positive does not reference actual signature verification')
    expected = {'wrong_verification_sha': 'expected GithubWorkflowSHA', 'wrong_index': 'provided artifact digests do not match digests in statement', 'wrong_predicate': 'invalid predicate type'}
    check(set(controls.get('negatives', {})) == set(expected), 'missing real Sigstore negative controls')
    baseline = proof_command(signature)
    for label, error in expected.items():
        command = proof_command(controls['negatives'][label])
        check(isinstance(command.get('exit_code'), int) and command['exit_code'] != 0 and error in Path(command['stderr_file']).read_text(), 'negative control did not fail for intended semantic reason: ' + label)
        args = list(baseline['argv'])
        if label == 'wrong_verification_sha':
            args[10] = '--certificate-github-workflow-sha=' + '0' * 40
            check(command.get('verification_source') == '0' * 40, 'wrong SHA control did not change SHA')
        elif label == 'wrong_predicate':
            args[5] = PREDICATES['spdx']
        else:
            args[-1] = command['root_index_file']
            raw = hashed_file(command['root_index_file'], command['root_index_sha256'])
            check(hashlib.sha256(raw).hexdigest() != proof['root_index']['sha256'], 'wrong index control reused actual index')
        check(command.get('argv') == args and command.get('bundle_raw_sha256') == signature['raw_sha256'], 'negative control changed unrelated verification inputs')


def check_derived_envelopes(info, entries):
    check(info.get('derived') is True, 'DSSE JSONL must be explicitly labeled derived')
    raw = read_proof_bytes(info)
    lines = raw.splitlines()
    mapping = info.get('line_map', [])
    check(len(lines) == len(mapping) == len(entries) == 2 and {entry['line'] for entry in mapping} == {1, 2}, 'derived JSONL line map is incomplete')
    by_digest = {item['oci_blob_digest']: item for item in entries}
    check({entry['bundle_digest'] for entry in mapping} == set(by_digest), 'derived JSONL bundle mapping mismatch')
    for entry in mapping:
        item = by_digest[entry['bundle_digest']]
        check(entry['arch'] == item['arch'] and envelope_hash(json.loads(lines[entry['line'] - 1])) == item['_envelope_hash'], 'derived DSSE differs from the actually verified original bundle')


def verify_targeted_bundle_files(verified):
    """Validate persisted crypto evidence; this alone does not approve a candidate."""
    check(verified.get('proof_mode') == TARGETED_MODE, 'unsupported targeted proof mode')
    proof = json.loads(hashed_file(verified['targeted_proof_file'], verified['targeted_proof_sha256']))
    check(proof.get('schema') == proof.get('proof_mode') == TARGETED_MODE and proof.get('cryptographic_complete') is True, 'targeted cryptographic proofs are incomplete')
    check(proof.get('repo') == REPO and proof.get('producer_source') == verified.get('source') == verified.get('producer_source') and proof.get('verification_source') == verified.get('verification_source'), 'targeted producer/verification/repository binding mismatch')
    tool = proof['tool']
    check(tool.get('version') == '3.1.4' and tool.get('sha256') == COSIGN_SHA256, 'unreviewed cosign binary')
    hashed_file(tool['file'], COSIGN_SHA256)
    index = read_proof_bytes(proof['root_index'])
    check(len(index) == proof['root_index']['bytes'] and 'sha256:' + proof['root_index']['sha256'] == verified['digest'] and verified['ref'] == REPO + '@' + verified['digest'], 'targeted raw root index mismatch')
    expected = {('image_signature', None), ('spdx', 'amd64'), ('spdx', 'arm64'), ('trivy', 'amd64'), ('trivy', 'arm64')}
    check(len(proof.get('bundles', [])) == 5 and {(item['role'], item['arch']) for item in proof['bundles']} == expected, 'all five actual verified bundle roles/architectures are required')
    collection = json.loads(read_proof_bytes(proof['collection_index']))
    descriptors = {item['digest'] for item in collection['manifests']}
    statements = {}
    for item in proof['bundles']:
        check(item['oci_manifest_digest'] in descriptors, 'verified bundle absent from discovered repository collection')
        statement, item['_envelope_hash'] = bundle_statement(item, proof)
        statements[(item['role'], item['arch'])] = statement
    check_targeted_controls(proof)
    for role, key in (('spdx', 'sbom'), ('trivy', 'scans')):
        check_derived_envelopes(proof['derived'][key], [item for item in proof['bundles'] if item['role'] == role])
    sig = next(item for item in proof['bundles'] if item['role'] == 'image_signature')
    image = proof['derived']['image_statement']
    check(image.get('bundle_digest') == sig['oci_blob_digest'] and image.get('sha256') == sig['statement_sha256'], 'actual image signature statement mapping mismatch')
    check(json.loads(read_proof_bytes(image)) == statements[('image_signature', None)], 'image statement differs from actual signed payload')
    check(verified.get('signature', {}).get('signed_subject') == sig['subject'], 'receipt signature subject differs from actual signed subject')
    return proof, statements


def reconstruct_docker_archive_manifest(config_raw, registry_manifest, witnesses):
    """Pinned Stereoscope 0.3.1 algorithm, with every actual layer witness linked."""
    config_descriptor = registry_manifest['config']
    config_digest = 'sha256:' + hashlib.sha256(config_raw).hexdigest()
    check(config_descriptor.get('digest') == config_digest and config_descriptor.get('size') == len(config_raw), 'content equivalence raw config hash/size mismatch')
    config = json.loads(config_raw)
    diff_ids = config.get('rootfs', {}).get('diff_ids', [])
    layers = registry_manifest.get('layers', [])
    check(config.get('rootfs', {}).get('type') == 'layers' and len(witnesses) == len(layers) == len(diff_ids) > 0, 'content equivalence is missing an ordered layer witness')
    reconstructed_layers = []
    for layer, diff_id, witness in zip(layers, diff_ids, witnesses):
        check(witness.get('schema') == 'bex.native-compressed-and-diffid-proof/v1' and witness.get('registry_repository') == REPO and witness.get('manual_decompressed_layer_hashing_performed') is True, 'content equivalence lacks an actual full-stream layer witness')
        check(witness.get('compressed_digest') == layer.get('digest') and witness.get('compressed_bytes') == layer.get('size'), 'content equivalence compressed descriptor mismatch')
        check(witness.get('uncompressed_digest') == diff_id and re.fullmatch(r'sha256:[0-9a-f]{64}', diff_id), 'content equivalence ordered diffID mismatch')
        check(type(witness.get('uncompressed_bytes')) is int and witness['uncompressed_bytes'] > 0, 'content equivalence uncompressed size is missing')
        reconstructed_layers.append({'mediaType': 'application/vnd.docker.image.rootfs.diff.tar.gzip', 'size': witness['uncompressed_bytes'], 'digest': diff_id})
    reconstructed = {'schemaVersion': 2, 'mediaType': 'application/vnd.docker.distribution.manifest.v2+json', 'config': {'mediaType': 'application/vnd.docker.container.image.v1+json', 'size': len(config_raw), 'digest': config_digest}, 'layers': reconstructed_layers}
    return json.dumps(reconstructed, separators=(',', ':')).encode()



LAYER_CUSTODY_FILE = ROOT / '.pm/w1/m188/evidence/node14-layer-equivalence/MANIFEST.json'
LAYER_CUSTODY_SHA256 = '31c634c9ccc1ec493148162c187d19a3b512284ddca7b863527d2f7b7d9caf89'


GENERIC_EXECUTION_MODE = 'generic_native_supplier_driver/v1'
GENERIC_CONTENT_CUSTODIES = {
    'sha256:41a74a7d36f4b04cd87d8539c74736babb217e51f5dd25b778a957d9288274a0': {
        'name': 'python37', 'tag': 'python-3.7-bullseye', 'source': '1e3c903e037743db9ed8e3de606cc0b4306d7814ffb35ff021e171629574e21a', 'execution': '4108ec542d8a2d2fd87062998dffa840450f0e9092a5550988c0066f15e8a022',
        'review': 'ba6051728e6bcd13ada809b853eb17347acbe56febcf02745187820a93ac80aa', 'node_review': 'b01c2ff41214d0b2e4ad274859e4dd85a94a8b6b692429bcb46cdcdeddc71933',
    },
    'sha256:2802078e18496838c15159ec99c01c9b260128cef8e1d1ba29cd0a1836165113': {
        'name': 'node16', 'tag': 'node-16-bookworm', 'source': 'ceb614b662d13e146dced89c92eab6f4a1f6bff204222ecf88fc54361e2bdb99', 'execution': '4969ad98b1d10fe1bb46d01ca1b5c4138e0a8933fa6a9635ee9f841c917701ed',
        'review': '0a1ccb6f481d34ea91f93d1ccbe17a3db6920f5a758bcac500f28cfde302e245', 'node_review': '6e3fa7dca7d12b66253944207cfb23e1267dc92f8499896b197ba8bd507c9a00',
    },
}


def verify_generic_archive_equivalence(equivalence, proof, arch, descriptor, registry_manifest, signed_digest):
    root_digest = 'sha256:' + proof['root_index']['sha256']
    check(root_digest in GENERIC_CONTENT_CUSTODIES and arch == 'amd64', 'generic content evidence lacks exact root-approved custody/architecture')
    approved = GENERIC_CONTENT_CUSTODIES[root_digest]
    check(equivalence.get('execution_mode') == GENERIC_EXECUTION_MODE and equivalence.get('schema') == 'native_docker_archive_content_equivalence/v1' and equivalence.get('complete') is True and equivalence.get('repository') == REPO and equivalence.get('stereoscope_version') == '0.3.1', 'unsupported generic content evidence mode/schema')
    check(equivalence.get('root_index_digest') == root_digest and equivalence.get('architecture') == arch and equivalence.get('registry_child_digest') == descriptor['digest'] and equivalence.get('config_digest') == registry_manifest['config']['digest'] and equivalence.get('signed_spdx_digest') == signed_digest, 'generic content evidence target/config/signed root mismatch')
    evidence = ROOT / '.pm/w1/m188/evidence'
    source_dir = evidence / (approved['name'] + '-source-archive')
    execution_dir = evidence / (approved['name'] + '-execution-custody')
    allowed = {}
    for directory, expected_hash, schema in ((source_dir, approved['source'], 'bex.native-compact-evidence-manifest/v1'), (execution_dir, approved['execution'], 'native_retained_execution_custody/v1')):
        path = directory / 'MANIFEST.json'
        raw = hashed_file(path, expected_hash)
        manifest = json.loads(raw)
        check(manifest.get('schema') == schema, 'unreviewed generic custody manifest schema')
        allowed[str(path.resolve())] = {'sha256': expected_hash, 'bytes': len(raw)}
        entries = manifest['files'].items() if isinstance(manifest['files'], dict) else ((item['file'], item) for item in manifest['files'])
        for name, record in entries:
            target = (directory / name).resolve()
            check(target.is_relative_to(directory.resolve()), 'generic retained file escapes custody')
            raw = hashed_file(target, record['sha256'])
            check(len(raw) == record['bytes'], 'generic custody file size mismatch')
            allowed[str(target)] = record
    for suffix, expected_hash in (('layer-equivalence-review.json', approved['review']), ('independent-content-review.json', approved['node_review'])):
        path = evidence / (approved['name'] + '-' + suffix)
        raw = hashed_file(path, expected_hash)
        allowed[str(path.resolve())] = {'sha256': expected_hash, 'bytes': len(raw)}

    def retained(record):
        path = str(Path(record['file']).resolve())
        check(path in allowed and record.get('sha256') == allowed[path]['sha256'] and record.get('bytes') == allowed[path]['bytes'], 'generic content input lacks root-approved durable custody')
        return hashed_file(path, record['sha256'])

    roles = {item['role']: item for item in equivalence.get('execution_proofs', [])}
    required = {'driver_stdout', 'driver_stderr', 'driver_exit', 'batch_result', 'completion_progress', 'collector_code', 'registry_verifier', 'custody_manifest', 'execution_custody_manifest', 'independent_review', 'independent_node_review'}
    check(set(roles) == required and len(equivalence['execution_proofs']) == len(required), 'generic execution role set is incomplete or duplicated')
    records = {role: retained(record) for role, record in roles.items()}
    check(Path(roles['custody_manifest']['file']).resolve() == (source_dir / 'MANIFEST.json').resolve() and Path(roles['execution_custody_manifest']['file']).resolve() == (execution_dir / 'MANIFEST.json').resolve(), 'generic source/execution custody roles swapped')
    receipt = json.loads((source_dir / 'receipt.json').read_bytes())
    check(receipt.get('ref') == REPO + '@' + root_digest and receipt.get('verification_workflow_sha') == proof['verification_source'] and receipt.get('producer_workflow_sha') == proof['producer_source'], 'generic receipt producer/verification/index mismatch')
    code_hashes = receipt['evidence_tool_sha256']
    check(roles['collector_code']['sha256'] == code_hashes['/tmp/loopx-w8-native-review-v2.py'] == '278dcb063b4c1c571932e8f358056d5abe9154b48cb057365e67242865d27a36' and roles['registry_verifier']['sha256'] == code_hashes['/tmp/loopx-w8-verify-native-artifact.py'] == 'cf5932b2c5598bc9140a4bf9b99ab8c6ae0aa9613e73ec88140f492b086f1bb2', 'generic executed helper/policy selfhash mismatch')
    inputs = equivalence['inputs']
    index_raw, manifest_raw, config_raw = (retained(inputs[name]) for name in ('raw_index', 'raw_manifest', 'raw_config'))
    check('sha256:' + hashlib.sha256(index_raw).hexdigest() == root_digest and 'sha256:' + hashlib.sha256(manifest_raw).hexdigest() == descriptor['digest'] and len(manifest_raw) == descriptor['size'] and json.loads(manifest_raw) == registry_manifest, 'generic raw index/child chain mismatch')
    check([item for item in json.loads(index_raw)['manifests'] if item.get('platform', {}).get('os') == 'linux' and item.get('platform', {}).get('architecture') == arch] == [descriptor], 'generic architecture descriptor mismatch')
    config = json.loads(config_raw)
    check(config.get('architecture') == arch and config.get('os') == 'linux', 'generic raw config architecture mismatch')
    ordered = equivalence['ordered_layer_witnesses']
    check(len(ordered) == len(registry_manifest['layers']), 'generic content evidence missing layer witness')
    witnesses = []
    for ordinal, (item, layer) in enumerate(zip(ordered, registry_manifest['layers'])):
        check(item.get('ordinal') == ordinal and item.get('compressed_descriptor') == layer and 'layer_ordinal' not in item['receipt'], 'generic compressed descriptor/order/direct receipt mismatch')
        witness = json.loads(retained(item['receipt']))
        check(item.get('diff_id') == witness.get('uncompressed_digest') and item.get('uncompressed_bytes') == witness.get('uncompressed_bytes'), 'generic witness DiffID/size differs from retained object')
        witnesses.append(witness)
    reconstructed = reconstruct_docker_archive_manifest(config_raw, registry_manifest, witnesses)
    reconstruction = equivalence['reconstruction']
    check(retained(reconstruction) == reconstructed and 'sha256:' + hashlib.sha256(reconstructed).hexdigest() == signed_digest, 'generic reconstruction differs from exact signed SPDX root')
    events = [json.loads(line) for line in records['driver_stdout'].decode().splitlines() if line.strip()]
    check(events and records['driver_stderr'] == b'', 'generic execution transcript failed or is empty')
    exit_record, progress, batch = (json.loads(records[name]) for name in ('driver_exit', 'completion_progress', 'batch_result'))
    check(exit_record.get('exit_code') == 0 and exit_record.get('name') == approved['tag'] and exit_record.get('source') == proof['verification_source'] and exit_record.get('run') == str(receipt['ci_runtime_scan_signing_evidence']['run_id']), 'generic actual execution exit/source/run mismatch')
    check([item for item in batch if item.get('name') == approved['tag']] == [exit_record], 'generic batch/exit result mismatch')
    check(events[-1].get('candidate_approved') is True and events[-1].get('name') == approved['tag'] and events[-1].get('ref') == receipt['ref'] and Path(events[-1]['receipt']).resolve() == Path(exit_record['receipt']).resolve(), 'generic actual completed transcript identity mismatch')
    check(progress.get('stage') == 'complete' and progress.get('candidate_approved') is True and progress.get('ref') == receipt['ref'], 'generic completion record lacks exact artifact identity')
    stamp = datetime.datetime.fromisoformat
    check(stamp(receipt['reviewed_at']) <= stamp(progress['at']) <= stamp(exit_record['at']), 'generic receipt/completion/exit chronology mismatch')
    starts = [event for event in events if event.get('stage') == 'anonymous-compressed-and-decompressed-read']
    check(len(starts) == receipt['new_manual_layer_reads'] == 34, 'generic actual full-stream read count mismatch')
    aggregate = json.loads((source_dir / 'actual-layer-proof/receipt.json').read_bytes())
    check(aggregate['index_digest'] == root_digest, 'generic all-layer receipt index mismatch')
    provenance = equivalence.get('reused_layer_provenance', [])
    check(len(provenance) == 1, 'generic reused-layer provenance is missing or unexpected')
    reused = provenance[0]
    check(reused['accepted_root_index_digest'] == 'sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc' and Path(reused['accepted_custody_manifest']['file']).resolve() == LAYER_CUSTODY_FILE.resolve() and reused['accepted_custody_manifest']['sha256'] == LAYER_CUSTODY_SHA256, 'reused layer lacks accepted Node14 custody')
    old_custody = json.loads(read_proof_bytes(reused['accepted_custody_manifest']))
    old = reused['accepted_receipt']
    old_path = Path(old['file']).resolve()
    check(old_path.is_relative_to(LAYER_CUSTODY_FILE.parent.resolve()), 'reused receipt escapes accepted custody')
    old_meta = old_custody['files'].get(str(old_path.relative_to(LAYER_CUSTODY_FILE.parent.resolve())))
    check(old_meta and old['sha256'] == old_meta['sha256'] and old['bytes'] == old_meta['bytes'], 'reused receipt differs from accepted custody')
    old_witness = json.loads(read_proof_bytes(old))['layer_proofs'][old['layer_ordinal']]
    check(reused['compressed_digest'] == old_witness['compressed_digest'], 'reused witness compressed identity mismatch')
    seen, new_count, reused_count = set(), 0, 0
    for row in aggregate['architectures']:
        for ordinal, witness in enumerate(row['layer_proofs']):
            if witness['compressed_digest'] in seen:
                continue
            seen.add(witness['compressed_digest'])
            if stamp(witness['anonymous_full_get_at']) < stamp(events[0]['at']):
                check(witness == old_witness, 'generic prior witness is not the accepted exact layer')
                reused_count += 1
                continue
            check(any(event.get('name') == approved['tag'] and event.get('architecture') == row['architecture'] and event.get('layer') == ordinal and event.get('expected_compressed_bytes') == witness['compressed_bytes'] and stamp(event['at']) <= stamp(witness['anonymous_full_get_at']) <= stamp(receipt['reviewed_at']) for event in starts), 'generic actual stream start/witness chronology mismatch')
            new_count += 1
    check(new_count == 34 and reused_count == 1 and len(seen) == 35, 'generic complete controlflow/read coverage mismatch')
    return sum(witness['uncompressed_bytes'] for witness in witnesses)



BOUNDED_FILE_EXECUTION_MODE = 'bounded_verified_file_supplier_driver/v1'
BOUNDED_FILE_ROOT = 'sha256:3dd6306d5fc472b2dd7814635f933c6318d17754cb6fa94bd129801f8eefcd02'
BOUNDED_FILE_CUSTODY_SHA256 = '4a4e2d9fc663b5cf627f0365671c2f335cb007fceff1f5acec986be73c054c9a'


def verify_bounded_file_archive_equivalence(equivalence, proof, arch, descriptor, registry_manifest, signed_digest):
    root_digest = 'sha256:' + proof['root_index']['sha256']
    check(root_digest == BOUNDED_FILE_ROOT and arch == 'amd64', 'bounded-file mode exceeds root-approved candidate/architecture')
    check(equivalence.get('schema') == 'native_docker_archive_content_equivalence/v1' and equivalence.get('complete') is True and equivalence.get('repository') == REPO and equivalence.get('stereoscope_version') == '0.3.1', 'unsupported bounded-file content schema')
    check(equivalence.get('root_index_digest') == root_digest and equivalence.get('architecture') == arch and equivalence.get('registry_child_digest') == descriptor['digest'] and equivalence.get('config_digest') == registry_manifest['config']['digest'] and equivalence.get('signed_spdx_digest') == signed_digest, 'bounded-file target/config/signed root mismatch')
    directory = ROOT / '.pm/w1/m188/evidence/python38-source-archive'
    custody_file = directory / 'MANIFEST.json'
    raw = hashed_file(custody_file, BOUNDED_FILE_CUSTODY_SHA256)
    custody = json.loads(raw)
    check(custody.get('schema') == 'bex.native-compact-evidence-manifest/v1', 'bounded-file custody schema mismatch')
    allowed = {str(custody_file.resolve()): {'sha256': BOUNDED_FILE_CUSTODY_SHA256, 'bytes': len(raw)}}
    for name, record in custody['files'].items():
        path = (directory / name).resolve()
        check(path.is_relative_to(directory.resolve()), 'bounded-file input escapes durable custody')
        raw = hashed_file(path, record['sha256'])
        check(len(raw) == record['bytes'], 'bounded-file custody size mismatch')
        allowed[str(path)] = record
    review_file = ROOT / '.pm/w1/m188/evidence/python38-v8-cnb-content-review.json'
    review_hash = 'd53222d8a829f10c1a523b79e90b1c8bb4b96fe87675bd5e765c1405e5d0f723'
    review_raw = hashed_file(review_file, review_hash)
    allowed[str(review_file.resolve())] = {'sha256': review_hash, 'bytes': len(review_raw)}

    def retained(record):
        path = str(Path(record['file']).resolve())
        check(path in allowed and record.get('sha256') == allowed[path]['sha256'] and record.get('bytes') == allowed[path]['bytes'], 'bounded-file input lacks root-approved durable custody')
        return hashed_file(path, record['sha256'])

    expected_roles = {
        'driver_stdout': directory / 'execution/python38-review-v8.txt',
        'driver_exit': directory / 'process-exit.json',
        'completion_progress': directory / 'progress.json',
        'collector_code': directory / 'executed-tools/loopx-w8-native-review-v8.py',
        'registry_verifier': directory / 'executed-tools/loopx-w8-verify-native-artifact.py',
        'prior_collector_code': directory / 'executed-tools/loopx-w8-native-review-v7.py',
        'prior_driver_stdout': directory / 'execution/python38-review-v7.txt',
        'layer_file_policy': directory / 'layer-file-resource-policy.json',
        'layer_file_cleanup': directory / 'own-layer-file-cleanup.json',
        'custody_manifest': custody_file,
        'independent_review': review_file,
        'independent_node_review': directory / 'independent-review/bex-m188-python38-independent-content-review.json',
    }
    roles = {item['role']: item for item in equivalence.get('execution_proofs', [])}
    check(set(roles) == set(expected_roles) and len(equivalence['execution_proofs']) == len(roles), 'bounded-file execution role set is incomplete or duplicated')
    check(all(Path(roles[role]['file']).resolve() == path.resolve() for role, path in expected_roles.items()), 'bounded-file execution role substituted')
    records = {role: retained(record) for role, record in roles.items()}
    receipt = json.loads((directory / 'receipt.json').read_bytes())
    check(receipt.get('ref') == REPO + '@' + root_digest and receipt.get('verification_workflow_sha') == proof['verification_source'] and receipt.get('producer_workflow_sha') == proof['producer_source'], 'bounded-file receipt source/index mismatch')
    expected_tools = {'collector_code': 'b1331524b7f4ce87a8b984856bc9fa61037888b9026fb7800752423e60e7731a', 'registry_verifier': 'cf5932b2c5598bc9140a4bf9b99ab8c6ae0aa9613e73ec88140f492b086f1bb2', 'prior_collector_code': 'cc0fd80a6923f7717f5a63abee59c556cb531a2bea60b626161c51f62122c09f'}
    check(all(roles[role]['sha256'] == sha for role, sha in expected_tools.items()), 'bounded-file executed helper/policy mismatch')
    code_hashes = {Path(name).name: sha for name, sha in receipt['evidence_tool_sha256'].items()}
    check(code_hashes == {'loopx-w8-native-review-v8.py': expected_tools['collector_code'], 'loopx-w8-verify-native-artifact.py': expected_tools['registry_verifier']}, 'bounded-file helper receipt selfhash mismatch')
    index_raw, manifest_raw, config_raw = (retained(equivalence['inputs'][name]) for name in ('raw_index', 'raw_manifest', 'raw_config'))
    check('sha256:' + hashlib.sha256(index_raw).hexdigest() == root_digest and 'sha256:' + hashlib.sha256(manifest_raw).hexdigest() == descriptor['digest'] and len(manifest_raw) == descriptor['size'] and json.loads(manifest_raw) == registry_manifest, 'bounded-file raw index/child binding mismatch')
    check([item for item in json.loads(index_raw)['manifests'] if item.get('platform', {}).get('os') == 'linux' and item.get('platform', {}).get('architecture') == arch] == [descriptor], 'bounded-file architecture descriptor mismatch')
    config = json.loads(config_raw)
    check(config.get('os') == 'linux' and config.get('architecture') == arch, 'bounded-file raw config architecture mismatch')
    ordered = equivalence['ordered_layer_witnesses']
    check(len(ordered) == len(registry_manifest['layers']) == 17, 'bounded-file content witness count mismatch')
    witnesses = []
    for ordinal, (item, layer) in enumerate(zip(ordered, registry_manifest['layers'])):
        check(item.get('ordinal') == ordinal and item.get('compressed_descriptor') == layer and 'layer_ordinal' not in item['receipt'], 'bounded-file descriptor/order/direct witness mismatch')
        witness = json.loads(retained(item['receipt']))
        check(item.get('diff_id') == witness.get('uncompressed_digest') and item.get('uncompressed_bytes') == witness.get('uncompressed_bytes'), 'bounded-file witness DiffID/size mismatch')
        witnesses.append(witness)
    reconstructed = reconstruct_docker_archive_manifest(config_raw, registry_manifest, witnesses)
    check(retained(equivalence['reconstruction']) == reconstructed and 'sha256:' + hashlib.sha256(reconstructed).hexdigest() == signed_digest, 'bounded-file reconstruction differs from signed SPDX root')
    events = [json.loads(line) for line in records['driver_stdout'].decode().splitlines() if line.strip()]
    prior_events = []
    for line in records['prior_driver_stdout'].decode().splitlines():
        try:
            prior_events.append(json.loads(line))
        except ValueError:
            pass  # The original V7 failed attempt retains its actual traceback.
    exit_record, progress = (json.loads(records[name]) for name in ('driver_exit', 'completion_progress'))
    check(exit_record.get('actual_exit_code') == 0 and exit_record.get('actual_exec_session') == 25565 and exit_record.get('tool_sha256') == expected_tools['collector_code'] and exit_record.get('owned_layer_directory_empty') is True, 'bounded-file actual process completion mismatch')
    check(events and events[-1].get('candidate_approved') is True and events[-1].get('name') == 'python-3.8-bookworm' and events[-1].get('ref') == receipt['ref'] and Path(events[-1]['receipt']).resolve() == Path(exit_record['receipt']).resolve(), 'bounded-file completed transcript identity mismatch')
    check(progress.get('stage') == 'complete' and progress.get('candidate_approved') is True and progress.get('ref') == receipt['ref'], 'bounded-file completion identity mismatch')
    stamp = datetime.datetime.fromisoformat
    check(stamp(receipt['reviewed_at']) <= stamp(progress['at']) <= stamp(exit_record['at']), 'bounded-file receipt/completion/exit chronology mismatch')
    policy = json.loads(records['layer_file_policy'])
    check(policy.get('maximum_case_bytes') == policy.get('minimum_remaining_free_bytes') == 2 << 30 and policy.get('full_compressed_SHA_size_before_decompression') is True and policy.get('atomic_verified_rename') is True and policy.get('stable_bounded_partials') is True and policy.get('remove_completed_own_files_only_after_proof') is True, 'bounded-file resource/integrity policy mismatch')
    aggregate = json.loads((directory / 'actual-layer-proof/receipt.json').read_bytes())
    check(aggregate['index_digest'] == root_digest and {row['architecture'] for row in aggregate['architectures']} == {'amd64', 'arm64'} and len(aggregate['architectures']) == 2, 'bounded-file all-architecture receipt mismatch')
    all_witnesses, by_ordinal = {}, {}
    for row in aggregate['architectures']:
        check(len(row['layer_proofs']) == 17, 'bounded-file architecture layer count mismatch')
        for ordinal, witness in enumerate(row['layer_proofs']):
            digest = witness['compressed_digest']
            check(digest not in all_witnesses or all_witnesses[digest] == witness, 'bounded-file duplicate witness disagreement')
            all_witnesses[digest] = witness
            by_ordinal[row['architecture'], ordinal] = witness
    check([by_ordinal['amd64', i] for i in range(17)] == witnesses, 'bounded-file ordered witnesses differ from aggregate')
    starts = {(event['architecture'], event['layer']): event for event in events if event.get('stage') == 'bounded-own-layer-download'}
    decompressions = {(event['architecture'], event['layer']): event for event in events if event.get('stage') == 'actual-local-layer-decompression'}
    cleanup = json.loads(records['layer_file_cleanup'])
    removed = {item['digest']: item for item in cleanup['removed']}
    check(len(starts) == len(decompressions) == len(removed) == len(cleanup['removed']) == receipt['new_manual_layer_reads'] == 31 and len(all_witnesses) == 33, 'bounded-file actual full-read coverage mismatch')
    for key, start in starts.items():
        witness = by_ordinal[key]
        dec, removed_record = decompressions[key], removed[witness['compressed_digest']]
        check(start.get('name') == dec.get('name') == 'python-3.8-bookworm' and start['expected_compressed_bytes'] == dec['verified_compressed_bytes'] == removed_record['bytes'] == witness['compressed_bytes'], 'bounded-file actual download/decompression/cleanup descriptor mismatch')
        witness_file = directory / 'actual-layer-proof/verified-layers' / (witness['compressed_digest'].split(':')[1] + '.json')
        check(json.loads(witness_file.read_bytes()) == witness and hashlib.sha256(witness_file.read_bytes()).hexdigest() == removed_record['proof_sha256'], 'bounded-file cleanup witness hash mismatch')
        check(stamp(start['at']) <= stamp(dec['at']) <= stamp(witness['anonymous_full_get_at']) <= stamp(removed_record['removed_at']) <= stamp(receipt['reviewed_at']), 'bounded-file actual read/witness/cleanup chronology mismatch')
    check(sum(item['bytes'] for item in removed.values()) == cleanup['reclaimed_bytes'] == 1143671089, 'bounded-file deleted-own-byte accounting mismatch')
    prior_digest = 'sha256:01272fe8adbacc44afd2b92994b31c40a151f4324ca392050d9e8d580927dd32'
    old_layers = equivalence.get('prior_driver_layer_provenance', [])
    check(len(old_layers) == 1, 'bounded-file prior V7 witness provenance missing')
    old = old_layers[0]
    check(old.get('compressed_digest') == prior_digest and old.get('architecture') == 'amd64' and old.get('ordinal') == 3 and old.get('collector_code') == {k: v for k, v in roles['prior_collector_code'].items() if k != 'role'} and old.get('driver_stdout') == {k: v for k, v in roles['prior_driver_stdout'].items() if k != 'role'}, 'bounded-file prior V7 source/transcript mismatch')
    old_witness = json.loads(retained(old['receipt']))
    check(old_witness == by_ordinal['amd64', 3] and old_witness['compressed_digest'] == prior_digest, 'bounded-file prior V7 witness identity mismatch')
    prior_stream = [event for event in prior_events if event.get('architecture') == 'amd64' and event.get('layer') == 3]
    check(prior_stream and prior_stream[0].get('stage') == 'anonymous-compressed-and-decompressed-read' and prior_stream[0].get('expected_compressed_bytes') == old_witness['compressed_bytes'], 'bounded-file prior V7 measurement start missing')
    check(stamp(prior_stream[0]['at']) < stamp(prior_stream[-1]['at']) < stamp(old_witness['anonymous_full_get_at']) < stamp(events[0]['at']) and prior_stream[-1]['compressed_bytes'] < old_witness['compressed_bytes'] and prior_stream[-1]['decompressed_bytes'] < old_witness['uncompressed_bytes'], 'bounded-file prior V7 measurement chronology mismatch')
    reused = equivalence.get('reused_layer_provenance', [])
    check(len(reused) == 1, 'bounded-file accepted reused witness missing')
    reused = reused[0]
    check(reused.get('accepted_root_index_digest') == 'sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc' and Path(reused['accepted_custody_manifest']['file']).resolve() == LAYER_CUSTODY_FILE.resolve() and reused['accepted_custody_manifest']['sha256'] == LAYER_CUSTODY_SHA256, 'bounded-file accepted Node14 custody mismatch')
    accepted_custody_raw = read_proof_bytes(reused['accepted_custody_manifest'])
    check(len(accepted_custody_raw) == reused['accepted_custody_manifest'].get('bytes'), 'bounded-file accepted custody size mismatch')
    accepted_custody = json.loads(accepted_custody_raw)
    old = reused['accepted_receipt']
    old_path = Path(old['file']).resolve()
    check(old_path.is_relative_to(LAYER_CUSTODY_FILE.parent.resolve()), 'bounded-file accepted witness escapes custody')
    meta = accepted_custody['files'].get(str(old_path.relative_to(LAYER_CUSTODY_FILE.parent.resolve())))
    check(meta and old['sha256'] == meta['sha256'] and old['bytes'] == meta['bytes'] and old.get('layer_ordinal') == 18, 'bounded-file accepted Node14 witness custody mismatch')
    empty_witness = json.loads(read_proof_bytes(old))['layer_proofs'][18]
    check(reused['compressed_digest'] == empty_witness['compressed_digest'] and all_witnesses[empty_witness['compressed_digest']] == empty_witness and set(all_witnesses) - set(removed) == {prior_digest, empty_witness['compressed_digest']}, 'bounded-file prior witness coverage mismatch')
    check(sum(w['compressed_bytes'] for w in all_witnesses.values()) == policy['planned_unique_compressed_bytes'], 'bounded-file total descriptor budget mismatch')
    return sum(witness['uncompressed_bytes'] for witness in witnesses)


def verify_docker_archive_equivalence(reference, proof, arch, descriptor, registry_manifest, signed_digest):
    """Only root-approved retained full-stream evidence can resolve a synthetic root."""
    equivalence = json.loads(read_proof_bytes(reference))
    if equivalence.get('execution_mode') == BOUNDED_FILE_EXECUTION_MODE:
        return verify_bounded_file_archive_equivalence(equivalence, proof, arch, descriptor, registry_manifest, signed_digest)
    if equivalence.get('execution_mode') == GENERIC_EXECUTION_MODE:
        return verify_generic_archive_equivalence(equivalence, proof, arch, descriptor, registry_manifest, signed_digest)
    check('execution_mode' not in equivalence, 'unapproved content execution mode')
    check(equivalence.get('schema') == 'native_docker_archive_content_equivalence/v1' and equivalence.get('stereoscope_version') == '0.3.1' and equivalence.get('repository') == REPO and equivalence.get('complete') is True, 'unreviewed content equivalence algorithm/schema')
    root_digest = 'sha256:' + proof['root_index']['sha256']
    check(equivalence.get('root_index_digest') == root_digest and equivalence.get('architecture') == arch and equivalence.get('registry_child_digest') == descriptor['digest'] and equivalence.get('config_digest') == registry_manifest['config']['digest'] and equivalence.get('signed_spdx_digest') == signed_digest, 'content equivalence target/config/signed subject mismatch')
    custody = json.loads(hashed_file(LAYER_CUSTODY_FILE, LAYER_CUSTODY_SHA256))
    check(custody.get('schema') == 'bex.retained-layer-equivalence-evidence/v1' and custody.get('index') == root_digest, 'content equivalence exceeds root-approved custody')
    allowed = {}
    for relative, record in custody['files'].items():
        path = (LAYER_CUSTODY_FILE.parent / relative).resolve()
        check(path.is_relative_to(LAYER_CUSTODY_FILE.parent.resolve()), 'custody file escapes retained directory')
        raw = hashed_file(path, record['sha256'])
        check(len(raw) == record['bytes'], 'retained custody size mismatch')
        allowed[str(path)] = record

    def retained(record):
        path = str(Path(record['file']).resolve())
        check(path in allowed and record.get('sha256') == allowed[path]['sha256'], 'content proof input is outside root-approved durable fileset')
        raw = hashed_file(path, record['sha256'])
        check(record.get('bytes') == len(raw) == allowed[path]['bytes'], 'content proof retained input size mismatch')
        return raw

    inputs = equivalence['inputs']
    raw_index = retained(inputs['raw_index'])
    raw_manifest = retained(inputs['raw_manifest'])
    raw_config = retained(inputs['raw_config'])
    check('sha256:' + hashlib.sha256(raw_index).hexdigest() == root_digest and 'sha256:' + hashlib.sha256(raw_manifest).hexdigest() == descriptor['digest'] and json.loads(raw_manifest) == registry_manifest, 'content equivalence raw registry chain mismatch')
    original_index = json.loads(raw_index)
    descriptors = [item for item in original_index['manifests'] if item.get('platform', {}).get('architecture') == arch and item.get('platform', {}).get('os') == 'linux']
    check(descriptors == [descriptor] and descriptor['size'] == len(raw_manifest), 'content equivalence architecture descriptor mismatch')
    config = json.loads(raw_config)
    check(config.get('architecture') == arch and config.get('os') == 'linux', 'content equivalence raw config architecture mismatch')
    ordered = equivalence['ordered_layer_witnesses']
    check(len(ordered) == len(registry_manifest['layers']), 'content proof missing ordered retained witness')
    witnesses = []
    for ordinal, (item, layer) in enumerate(zip(ordered, registry_manifest['layers'])):
        check(item.get('ordinal') == ordinal and item.get('compressed_descriptor') == layer, 'content proof compressed descriptor/order mismatch')
        receipt = item['receipt']
        check(receipt.get('layer_ordinal') == ordinal, 'retained witness receipt ordinal mismatch')
        aggregate = json.loads(retained(receipt))
        check(aggregate.get('architecture') == arch and aggregate.get('index_digest') == root_digest and aggregate.get('actual_registry_child_digest') == descriptor['digest'] and aggregate.get('actual_registry_config_digest') == registry_manifest['config']['digest'], 'retained full-stream witness target mismatch')
        witness = aggregate['layer_proofs'][ordinal]
        check(item.get('diff_id') == witness.get('uncompressed_digest') and item.get('uncompressed_bytes') == witness.get('uncompressed_bytes'), 'content proof witness fields differ from retained receipt')
        witnesses.append(witness)
    reconstructed = reconstruct_docker_archive_manifest(raw_config, registry_manifest, witnesses)
    reconstruction = equivalence['reconstruction']
    check(retained(reconstruction) == reconstructed and reconstruction.get('bytes') == len(reconstructed) and reconstruction.get('sha256') == hashlib.sha256(reconstructed).hexdigest() and 'sha256:' + reconstruction['sha256'] == signed_digest, 'content reconstruction does not hash to exact signed SPDX root')
    execution = equivalence.get('execution_proofs', [])
    roles = {record['role']: record for record in execution}
    required_roles = {'progress_log', 'warmup_log', 'collector_code', 'warmup_helper', 'independent_review', 'independent_cnb_review', 'custody_manifest'}
    check(len(roles) == len(execution) and required_roles <= set(roles), 'content equivalence lacks retained execution provenance')
    custody_ref = roles['custody_manifest']
    check(Path(custody_ref['file']).resolve() == LAYER_CUSTODY_FILE.resolve() and custody_ref['sha256'] == LAYER_CUSTODY_SHA256, 'content proof custody reference was not root-approved')
    records = {}
    for role, record in roles.items():
        if role != 'custody_manifest':
            records[role] = retained(record)
    review = json.loads(records['independent_review'])
    reviewed_arch = review.get('architectures', {}).get(arch, {})
    check(review.get('schema') == 'bex.independent-layer-witness-review/v1' and review.get('candidate_index') == root_digest and reviewed_arch.get('child_digest') == descriptor['digest'] and reviewed_arch.get('config_digest') == registry_manifest['config']['digest'] and reviewed_arch.get('derived_manifest_digest') == signed_digest and reviewed_arch.get('all_ordered_compressed_descriptor_and_diffid_bindings_verified') is True and reviewed_arch.get('all_actual_full_stream_execution_pairs_verified') is True, 'independent actual-byte provenance review mismatch')
    events = [json.loads(line) for role in ('progress_log', 'warmup_log') for line in records[role].decode().splitlines() if line.strip()]
    for ordinal, witness in enumerate(witnesses):
        at = datetime.datetime.fromisoformat(witness['anonymous_full_get_at'])
        starts = [event for event in events if event.get('stage') == 'anonymous-decompressed-layer-read' and event.get('compressed_digest') == witness['compressed_digest'] and event.get('expected_compressed_bytes') == witness['compressed_bytes'] and datetime.datetime.fromisoformat(event['at']) <= at]
        # Identical content-addressed layers may share one verified read across architectures.
        pairs = [(start, event) for start in starts for event in events if event.get('stage') == 'layer-read-verified' and event.get('arch') == start.get('arch') and event.get('layer') == start.get('layer') and event.get('compressed_bytes') == witness['compressed_bytes'] and event.get('uncompressed_bytes') == witness['uncompressed_bytes'] and datetime.datetime.fromisoformat(event['at']) >= datetime.datetime.fromisoformat(start['at']) and abs((datetime.datetime.fromisoformat(event['at']) - at).total_seconds()) < 1]
        check(pairs, 'missing actual full-stream layer execution pair: ' + str(ordinal))
    return sum(witness['uncompressed_bytes'] for witness in witnesses)



def verify_targeted_proof(verified, row):
    proof, statements = verify_targeted_bundle_files(verified)
    binding_info = proof.get('architecture_binding', {})
    check(verified.get('architecture_binding') == binding_info, 'candidate architecture proof reference differs from authoritative manifest')
    binding = json.loads(read_proof_bytes(binding_info))
    check(binding.get('schema') == 'native_architecture_binding/v1' and binding.get('root_index_sha256') == proof['root_index']['sha256'] and set(binding.get('per_arch', {})) == {'amd64', 'arm64'}, 'missing exact architecture binding proof')
    root = json.loads(read_proof_bytes(proof['root_index']))
    children = {item['platform']['architecture']: item for item in root['manifests'] if item.get('platform', {}).get('os') == 'linux' and item.get('platform', {}).get('architecture') in ('amd64', 'arm64')}
    check(set(children) == {'amd64', 'arm64'}, 'root index lacks both runtime architectures')
    for arch in ('amd64', 'arm64'):
        recorded = binding['per_arch'][arch]
        child = recorded['expected_child']
        digest = children[arch]['digest']
        manifest = json.loads(hashed_file(child['manifest_file'], digest.removeprefix('sha256:')))
        check(child.get('manifest_digest') == digest and child.get('manifest_sha256') == digest.removeprefix('sha256:') and child.get('config_digest') == manifest['config']['digest'], 'architecture child/config digest mismatch')
        check(child.get('ordered_layers') == [{'digest': item['digest'], 'size': item['size'], 'media_type': item['mediaType']} for item in manifest['layers']], 'architecture ordered layer mismatch')
        doc = statements[('spdx', arch)]['predicate']
        ids = set(doc.get('documentDescribes', [])) | {item['relatedSpdxElement'] for item in doc.get('relationships', []) if item.get('relationshipType') == 'DESCRIBES' and item.get('spdxElementId') == doc.get('SPDXID')}
        roots = [item for item in doc.get('packages', []) if item.get('SPDXID') in ids and item.get('primaryPackagePurpose') == 'CONTAINER']
        check(len(roots) == 1 and roots[0].get('name') == REPO, 'signed SPDX image root is ambiguous')
        checksums = [item['checksumValue'] for item in roots[0].get('checksums', []) if item.get('algorithm') == 'SHA256']
        purls = [item['referenceLocator'] for item in roots[0].get('externalRefs', []) if item.get('referenceType') == 'purl' and item.get('referenceLocator', '').startswith('pkg:oci/')]
        check(len(checksums) == len(purls) == 1 and re.fullmatch(r'[0-9a-f]{64}', checksums[0]), 'signed SPDX root lacks a unique SHA256/purl')
        signed_digest = 'sha256:' + checksums[0]
        check(unquote(purls[0]).split('?', 1)[0] == 'pkg:oci/' + REPO + '@' + signed_digest and parse_qs(urlparse(unquote(purls[0])).query).get('arch') == [arch], 'signed SPDX checksum/purl/architecture mismatch')
        spdx = recorded['spdx']
        check(spdx.get('verified') is True and spdx.get('root_checksum') == spdx.get('root_purl_digest') == signed_digest and spdx.get('expected_registry_child') == digest, 'recorded SPDX binding disagrees with signed root')
        witnessed_uncompressed_bytes = None
        if signed_digest == digest:
            check(spdx.get('binding') == 'exact_registry_manifest', 'direct signed SPDX root must use exact registry binding')
        else:
            check(spdx.get('binding') == 'verified_docker_archive_content', 'signed SPDX root lacks exact registry or concrete full-stream content binding: ' + arch)
            witnessed_uncompressed_bytes = verify_docker_archive_equivalence(spdx['content_equivalence'], proof, arch, children[arch], manifest, signed_digest)
        report = statements[('trivy', arch)]['predicate']
        metadata = report.get('Metadata', {})
        if witnessed_uncompressed_bytes is not None:
            check(metadata.get('Size') == witnessed_uncompressed_bytes, 'signed scan size differs from witnessed uncompressed layers')
        config = metadata.get('ImageConfig', {})
        check('sha256:' + metadata.get('ImageID', '').removeprefix('sha256:') == manifest['config']['digest'] and config.get('architecture') == arch and config.get('os') == 'linux', 'signed Trivy report does not bind exact architecture config')
        check(recorded['trivy'].get('verified') is True and recorded['trivy'].get('binding') == 'exact_registry_config' and recorded['trivy'].get('image_id') == recorded['trivy'].get('expected_config_digest') == manifest['config']['digest'], 'recorded Trivy binding mismatch')
        runtime_config = config.get('config', {})
        check(runtime_config.get('Labels', {}).get('org.opencontainers.image.revision') == verified['source'] and row['runtime'].upper() + '_VERSION=' + row['version'] in runtime_config.get('Env', []), 'signed architecture source/runtime version mismatch')
        signed_packages = []
        for package in doc.get('packages', []):
            if package.get('name') in ('libbrotli1', 'libbrotli-dev'):
                for ext in package.get('externalRefs', []):
                    loc = ext.get('referenceLocator', '')
                    if re.search(r'(?:[?&])arch=' + arch + r'(?:&|$)', loc):
                        signed_packages.append({'name': package['name'], 'version': package.get('versionInfo'), 'purl': loc})
        check(sorted(signed_packages, key=lambda item: item['name']) == sorted(verified['platforms'][arch]['brotli_packages'], key=lambda item: item['name']), 'receipt packages differ from signed architecture SPDX')
        severities, findings = {}, []
        for result in report.get('Results', []):
            for vulnerability in result.get('Vulnerabilities', []) or []:
                severity = vulnerability.get('Severity', 'UNKNOWN')
                severities[severity] = severities.get(severity, 0) + 1
                if vulnerability.get('VulnerabilityID') != 'CVE-2026-111497':
                    continue
                package, path, kind = vulnerability.get('PkgName', ''), vulnerability.get('PkgPath', ''), result.get('Type', '')
                system = package in ('libbrotli1', 'libbrotli-dev', 'brotli') and (kind == 'debian' or result.get('Class') == 'os-pkgs')
                node = package in ('node', 'nodejs') and (path.lstrip('/') in ('usr/local/bin/node', 'opt/bex/native-node-tools/node') or (row['runtime'] == 'node' and result.get('Class') != 'os-pkgs'))
                check(system or node, 'target CVE finding requires manual disposition')
                findings.append((vulnerability['VulnerabilityID'], package, path, vulnerability.get('InstalledVersion'), vulnerability.get('FixedVersion'), severity, kind))
        scan = verified['scans'][arch]
        check(scan.get('proof_file') == proof['derived']['scans']['file'], 'scan receipt points outside verified derived envelope file')
        actual = [(item['vulnerability_id'], item['package'], item['path'], item.get('installed_version'), item.get('fixed_version'), item['severity'], item['result_type']) for item in scan['brotli_findings']]
        check(actual == findings and scan['severity_counts'] == severities and all(item.get('disposition') for item in scan['brotli_findings']), 'receipt scan findings/dispositions differ from full signed report')
    check(binding_info.get('complete') is True and binding.get('complete') is True and not binding.get('failures') and proof.get('complete') is True and not proof.get('semantic_hold') and not verified.get('semantic_hold'), 'targeted candidate remains incomplete/held')
    check(verified.get('architecture_binding_verified') is True, 'candidate architecture binding is not explicitly verified')
    semantic = proof.get('semantic_checks', {})
    check(semantic.get('official_base') == row['upstream'] and all(semantic.get(key) is True for key in ('producer_runtime_approved', 'both_arch_runtime_versions_verified', 'both_arch_fixed_brotli_packages_verified', 'both_arch_scans_disposition_verified', 'fresh_runtime_scan_sign_gates_passed', 'fresh_runtime_gate_log_bound_to_index')), 'targeted final semantic checks are incomplete')
    check(semantic.get('architecture_binding_file') == binding_info['file'] and semantic.get('architecture_binding_sha256') == binding_info['sha256'], 'final semantic architecture proof reference mismatch')
    log = Path(verified['proof_directory']) / 'verification-job.log.gz'
    check(semantic.get('verification_job_log') == str(log) and semantic.get('verification_job_log_sha256') == hashlib.sha256(log.read_bytes()).hexdigest(), 'targeted semantic runtime log hash mismatch')
    return proof


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def git(*args):
    return subprocess.check_output(['git', '-C', str(ROOT), *args], text=True)


def source(sha, path):
    return git('show', sha + ':' + path)


def normalized_workflow(text):
    text = '\n'.join(line.split('#', 1)[0].rstrip() for line in text.splitlines() if line.split('#', 1)[0].strip())
    return re.sub(r'cancel-in-progress:\s*(?:true|false)', 'cancel-in-progress:<reviewed-policy>', text)



def verify_local_green_proofs():
    proofs = {}
    records = {}
    for name, expected_digest in LOCAL_GREEN_PROOFS.items():
        path = ROOT / '.pm/w1/m188/evidence' / (name + '.json')
        raw = path.read_bytes()
        check(hashlib.sha256(raw).hexdigest() == expected_digest, 'local green proof changed: root must review ' + str(path))
        record = json.loads(raw)
        check(record.get('source') == FIXED_RECIPE, 'local green source mismatch: ' + name)
        records[name] = record
        proofs[name] = {'path': str(path), 'sha256': expected_digest, 'source': FIXED_RECIPE}
    compiler = records['portable-node20-compiler']
    check(compiler.get('platform') == 'linux/arm64' and compiler.get('ELF') == {'GLIBC_max': '2.29', 'GLIBCXX_max': '3.4.26', 'CXXABI_max': '1.3.9'}, 'local portable compiler ABI evidence mismatch')
    for primary in ('python37', 'ruby30'):
        check(compiler[primary].get('canonical_probe_returncode') == 0 and compiler[primary].get('node') == '20.20.2' and compiler[primary].get('brotli') == '1.1.1', 'local oldest-primary decoder proof missing: ' + primary)
    results = '\n'.join(records['companion-native-green'].get('results', []))
    for runtime in ('Python', 'Ruby'):
        check('--- PASS: TestNative' + runtime + 'BackportBuildAndStart' in results, 'local authored companion HTTP proof missing: ' + runtime)
    final = records['portable-node20-final-native']
    check(final['runtime']['versions']['node'] == '20.20.2' and final['runtime']['versions']['brotli'] == '1.1.1' and final['runtime']['nodeEnv'] == '20.20.2' and final['runtime']['npm'] == '10.8.2', 'local final primary/runtime ENV/npm proof mismatch')
    check(any('--- PASS: TestNativeBackportsBuildAndStart/node20' in line for line in final.get('native_test_output', [])), 'local final-primary authored HTTP proof missing')
    for path, digest in records['portable-python-ruby-suppliers']['recipe_sha256'].items():
        check(hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest, 'local green recipe no longer matches current tree: ' + path)
    return proofs


def verify_portable_companion(row, security, gate_log):
    if row['runtime'] != 'node':
        return
    proof = security.get('portable_companion_baseline', {})
    check(proof.get('applicable') is True and proof.get('verified') is True and proof.get('distribution') == 'Debian11 Bullseye' and set(proof.get('architectures', [])) == {'amd64', 'arm64'}, 'missing dual-architecture Bullseye companion acceptance')
    check(proof.get('legacy_header_major') == row['line'] and proof.get('source_record_path') == '/usr/share/bex-node-brotli/sources.txt' and proof.get('source_record_presence_verified') is True, 'legacy companion source record/major mismatch')
    ids, successes = {}, {}
    for line in gate_log.splitlines():
        header = re.search(r'#(\d+) \[linux/(amd64|arm64) node-portability(?: [^\]]*)?\]', line)
        if header:
            ids[header.group(1)] = header.group(2)
        passed = re.search(r'#(\d+) [0-9.]+ Patched Node passed the Bullseye companion baseline', line)
        if passed and passed.group(1) in ids:
            successes[ids[passed.group(1)]] = line
    check(set(successes) == {'amd64', 'arm64'} and proof.get('success_log_lines') == successes, 'formal portability receipt does not match actual per-architecture publisher log lines')



def reviewed_current_workflow():
    """Return the exact reviewed signing flow and honest current-checkout metadata."""
    actual = (ROOT / WORKFLOW).read_bytes()
    actual_hash = hashlib.sha256(actual).hexdigest()
    canonical = actual
    profile = {'canonical_artifact_source': RECOVERY_WORKFLOW, 'canonical_workflow_sha256': REVIEWED_WORKFLOW_SHA256, 'actual_workflow_sha256': actual_hash, 'artifact_source_roles_added': []}
    if actual_hash != REVIEWED_WORKFLOW_SHA256:
        check(actual_hash == CURRENT_TREE_WORKFLOW_SHA256 and actual == source(CURRENT_TREE_WORKFLOW_SOURCE, WORKFLOW).encode(), 'current workflow exceeds exact reviewed checkout compatibility')
        for ancestor, descendant in ((CURRENT_TREE_RECIPE_SOURCE, CURRENT_TREE_WORKFLOW_SOURCE), (CURRENT_TREE_WORKFLOW_SOURCE, 'HEAD')):
            check(subprocess.run(['git', '-C', str(ROOT), 'merge-base', '--is-ancestor', ancestor, descendant], capture_output=True).returncode == 0, 'current checkout lacks the exact reviewed source ancestry')
        for new, old in CURRENT_TREE_WORKFLOW_REVERSALS:
            check(canonical.count(new) == 1, 'reviewed current workflow block is missing or ambiguous')
            canonical = canonical.replace(new, old, 1)
        profile.update(current_tree_workflow_source=CURRENT_TREE_WORKFLOW_SOURCE, current_tree_recipe_source=CURRENT_TREE_RECIPE_SOURCE, exact_reviewed_block_reversals=2)
        for path, digest in CURRENT_TREE_SIGNING_INPUTS.items():
            check((ROOT / path).is_file() and hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest, 'current signing helper differs from reviewed checkout: ' + path)
    check(hashlib.sha256(canonical).hexdigest() == REVIEWED_WORKFLOW_SHA256 and canonical == source(RECOVERY_WORKFLOW, WORKFLOW).encode(), 'checkout compatibility did not reconstruct exact canonical workflow')
    profile.update(raw_matches_canonical=actual == canonical, compatible_with_canonical=True)
    return canonical, profile


def reviewed_recipe_paths(producer, publisher_paths, current_paths):
    if producer in CURRENT_TREE_RECIPE_PRODUCERS and hashlib.sha256((ROOT / WORKFLOW).read_bytes()).hexdigest() == CURRENT_TREE_WORKFLOW_SHA256:
        check(current_paths == publisher_paths | set(CURRENT_TREE_EXTRA_RECIPE_FILES), 'current recipe file set exceeds exact reviewed test additions')
        for path, digest in CURRENT_TREE_RECIPE_FILES.items():
            check((ROOT / path).is_file() and hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest, 'current recipe file differs from exact reviewed checkout: ' + path)
    else:
        check(publisher_paths == current_paths, 'publisher recipe file set differs from current tree')


def reviewed_current_recipe_file(producer, path, objectid, current_bytes):
    if producer not in CURRENT_TREE_RECIPE_PRODUCERS or path not in ('deploy/native-base/install.sh', 'deploy/native-base/fetch-python-tools.py') or hashlib.sha256((ROOT / WORKFLOW).read_bytes()).hexdigest() != CURRENT_TREE_WORKFLOW_SHA256:
        return False
    expected = CURRENT_TREE_OLD_FETCH_SHA256
    if path == INSTALL_RECIPE_PATH:
        expected = REVIEWED_CURRENT_INSTALL_SHA256 if producer == 'da0e56e80a1e4eb08cacabd67efde3c768a6ae81' else HISTORICAL_INSTALL_SHA256
    return hashlib.sha256(current_bytes).hexdigest() == CURRENT_TREE_RECIPE_FILES[path] and hashlib.sha256(subprocess.check_output(['git', '-C', str(ROOT), 'cat-file', 'blob', objectid])).hexdigest() == expected


def verify_v2_source_binding(verified, row, receipt, matrix, native):
    producer_sha = verified.get('source', '')
    verification_sha = verified.get('verification_source', '')
    check(producer_sha in APPROVED_PRODUCER_SOURCES and verification_sha in APPROVED_VERIFICATION_SOURCES, 'producer/verification source exceeds explicitly approved role')
    if producer_sha in PRODUCER_RUNTIME_RESTRICTIONS:
        check(row['receipt_key'] in PRODUCER_RUNTIME_RESTRICTIONS[producer_sha], 'producer source is not approved for the selected runtime')
    if verification_sha in VERIFICATION_RUNTIME_RESTRICTIONS:
        check(row['receipt_key'] in VERIFICATION_RUNTIME_RESTRICTIONS[verification_sha], 'verification source is not approved for the selected runtime')
    for sha in (producer_sha, verification_sha):
        check(sha in receipt.get('accepted_sources', []), 'producer/verification source lacks explicit receipt approval')
        for ancestor in (ANCESTOR, FIXED_RECIPE):
            subprocess.run(['git', '-C', str(ROOT), 'merge-base', '--is-ancestor', ancestor, sha], check=True, capture_output=True)
    check(verified.get('producer_source') == producer_sha, 'producer alias mismatch')
    current_workflow_raw, _ = reviewed_current_workflow()
    check(hashlib.sha256(current_workflow_raw).hexdigest() == REVIEWED_WORKFLOW_SHA256, 'current workflow differs from the exact reviewed modern signing workflow')
    current_workflow = normalized_workflow(current_workflow_raw.decode())
    check(current_workflow == normalized_workflow(source(RECOVERY_WORKFLOW, WORKFLOW)), 'current workflow drifted from reviewed registry-source recovery workflow')
    verification_workflow = normalized_workflow(source(verification_sha, WORKFLOW))
    # Earlier explicitly approved commits retain their complete original gate
    # flow; modern signing changed tools and trust configuration, not recipes.
    if verification_sha == RECOVERY_WORKFLOW:
        check(verification_workflow == current_workflow, 'modern verification workflow differs from reviewed current workflow')
    producer_inputs = matrix.matrix(source(producer_sha, NATIVE))
    verification_inputs = matrix.matrix(source(verification_sha, NATIVE))
    check(producer_inputs == verification_inputs == matrix.matrix(native), 'producer/verification/current official input matrices differ')
    expected_base = row['upstream']
    for inputs in (producer_inputs, verification_inputs):
        check(next((entry['base'] for entry in inputs['include'] if entry['name'] == row['receipt_key']), None) == expected_base, 'per-runtime official input does not match reviewed current base')
    changes = [path for path in git('diff', '--no-renames', '--name-only', '-z', producer_sha, verification_sha, '--', 'deploy/native-base').split('\0') if path]
    check(all(path.endswith('.md') for path in changes), 'producer recipe differs from verification recipe')
    proofdir = Path(verified.get('proof_directory', '/no-dir'))
    if verified.get('proof_mode') == TARGETED_MODE:
        verify_targeted_proof(verified, row)
    else:
        check(False, 'candidate requires targeted raw bundle/index/command evidence and exact architecture bindings; legacy summary files alone are insufficient')
    with gzip.open(proofdir / 'verification-job.log.gz', 'rt') as log:
        verification_log = log.read()
    immutable = verified['ref']
    check('NATIVE_BASE=' + immutable in verification_log, 'fresh runtime gate log does not bind same immutable index')
    verify_portable_companion(row, verified['security'], verification_log)
    producer = verified.get('producer', {})
    check(producer.get('source') == producer_sha and producer.get('digest') == verified['digest'] and producer.get('original_build_passed') is True and producer.get('original_runtime_gates_passed') is True, 'producer build/runtime/index proof mismatch')
    verification_run, verification_job = verified.get('verification_run_id'), verified.get('verification_job_id')
    check(isinstance(verification_run, int) and isinstance(verification_job, int) and verified.get('run_id') == verification_run and verified.get('job_id') == verification_job, 'fresh verification run/job aliases mismatch')
    check(verified.get('job_url') == f'https://github.com/bex-co/bex/actions/runs/{verification_run}/job/{verification_job}', 'fresh verification job URL mismatch')
    recovered = verified.get('recovered')
    check(isinstance(recovered, bool), 'v2 recovered marker must be explicit')
    if recovered:
        check(producer_sha in APPROVED_PRODUCER_SOURCES and verification_sha in APPROVED_VERIFICATION_SOURCES - {FIXED_RECIPE}, 'unreviewed recovery producer/verification pair')
        authorization = json.loads(source(verification_sha, '.github/native-base-recovery.json'))
        check(authorization.get('schema') == 'bex.native-publisher-recovery/v1', 'invalid source-pinned recovery schema')
        entry = authorization['images'].get(row['receipt_key'])
        check(entry and entry.get('ref') == immutable and entry.get('producer') == producer_sha and entry.get('base') == expected_base, 'source-pinned recovery ref/producer/base mismatch')
        recorded = verified.get('recovery_authorization', {})
        check(all(recorded.get(key) == value for key, value in entry.items()) and recorded.get('verification_source') == verification_sha and recorded.get('source_path') == '.github/native-base-recovery.json', 'receipt recovery authorization differs from verification commit')
        check(producer.get('run_id') == entry['run_id'] and producer.get('job_id') == entry['job_id'] and producer.get('job_url') == f"https://github.com/bex-co/bex/actions/runs/{entry['run_id']}/job/{entry['job_id']}", 'original producer run/job mismatch')
        producer_log_path = Path(producer.get('log_gzip', '/no-file'))
        check(producer_log_path.is_file() and hashlib.sha256(producer_log_path.read_bytes()).hexdigest() == producer.get('log_sha256'), 'original producer log hash mismatch')
        with gzip.open(producer_log_path, 'rt') as log:
            producer_log = log.read()
        check('"containerimage.digest": "' + verified['digest'] + '"' in producer_log and 'NATIVE_BASE=' + immutable in producer_log, 'original build/runtime logs do not bind recovered index')
        verify_portable_companion(row, {'portable_companion_baseline': producer.get('portable_companion_baseline', {})}, producer_log)
        check('Rechecking reviewed artifact ' + immutable + '; producer ' + producer_sha in verification_log, 'fresh job does not prove exact recovery selection')
    else:
        check(producer_sha == verification_sha and verified.get('recovery_authorization') is None, 'nonrecovered image must be built and verified at same approved source')
        check(producer.get('run_id') == verification_run and producer.get('job_id') == verification_job, 'nonrecovered producer/fresh job mismatch')
    return producer_sha, verification_sha


def validate_candidate(verified, row, receipt, matrix=None, native=None):
    """All adoption checks for one row from the current plan; never writes or approves."""
    if native is None:
        native = (ROOT / NATIVE).read_text()
    if matrix is None:
        matrix = load_module('m188_candidate_matrix', ROOT / 'deploy/native-base/matrix.py')
    digest = verified.get('digest', '')
    check(re.fullmatch(r'sha256:[0-9a-f]{64}', digest) and verified.get('ref') == REPO + '@' + digest, 'invalid immutable image digest: ' + row['receipt_key'])
    sha, verification_sha = verify_v2_source_binding(verified, row, receipt, matrix, native)
    signature = verified.get('signature', {})
    check(signature.get('verified') is True and signature.get('identity') == IDENTITY and signature.get('oidc_issuer') == 'https://token.actions.githubusercontent.com' and signature.get('workflow_sha') == verification_sha, 'signature identity/verification SHA mismatch')
    check(verified.get('anonymous_metadata_pull') is True and verified.get('runtime') == row['runtime'] and verified.get('version') == row['version'], 'candidate availability/version mismatch')
    check(verified.get('signature_identity') == IDENTITY, 'signature identity mismatch')
    security = verified.get('security', {})
    check(security.get('cve') == 'CVE-2026-111497' and security.get('behavioral_gates_verified') is True and security.get('healthy_round_trip_verified') is True and security.get('permanent_decoder_error') == -17 and set(security.get('architectures', [])) == {'amd64', 'arm64'}, 'missing targeted behavioral acceptance')
    check(GATES <= {gate['name'] for gate in verified.get('runtime_gate_steps', []) if gate.get('conclusion') == 'success'}, 'missing successful publisher gates')
    publisher_tree = git('ls-tree', '-r', sha, '--', 'deploy/native-base').splitlines()
    publisher_paths = {entry.split(None, 3)[3] for entry in publisher_tree if not entry.endswith('.md')}
    current_paths = {path for path in git('ls-files', '--cached', '--others', '--exclude-standard', '--', 'deploy/native-base').splitlines() if not path.endswith('.md')}
    reviewed_recipe_paths(sha, publisher_paths, current_paths)
    for entry in publisher_tree:
        mode, kind, objectid, path = entry.split(None, 3)
        if not path.endswith('.md'):
            local = ROOT / path
            check(local.is_file(), 'publisher recipe file is missing: ' + path)
            current_bytes = local.read_bytes()
            same_blob = hashlib.sha1(('blob ' + str(len(current_bytes)) + '\0').encode() + current_bytes).hexdigest() == objectid
            compatible_install = (
                path == INSTALL_RECIPE_PATH and sha in HISTORICAL_INSTALL_PRODUCERS
                and hashlib.sha256(current_bytes).hexdigest() == REVIEWED_CURRENT_INSTALL_SHA256
                and hashlib.sha256(subprocess.check_output(['git', '-C', str(ROOT), 'cat-file', 'blob', objectid])).hexdigest() == HISTORICAL_INSTALL_SHA256
            )
            check(same_blob or compatible_install or reviewed_current_recipe_file(sha, path, objectid, current_bytes), 'publisher recipe differs from reviewed current file: ' + path)
    check(set(verified.get('platforms', {})) == {'amd64', 'arm64'} and set(verified.get('scans', {})) == {'amd64', 'arm64'}, 'missing architecture evidence')
    for arch in ('amd64', 'arm64'):
        platform = verified['platforms'][arch]
        check(platform.get('revision') == sha and row['runtime'].upper() + '_VERSION=' + row['version'] in platform.get('runtime_env', []), 'architecture provenance/runtime mismatch')
        for package in ('libbrotli1', 'libbrotli-dev'):
            check(any(pkg.get('name') == package and pkg.get('version', '').startswith('1.0.9-2+deb12u1') and re.search(r'(?:[?&])arch=' + arch + r'(?:&|$)', pkg.get('purl', '')) for pkg in platform.get('brotli_packages', [])), 'missing fixed signed system package: ' + package)
        scan = verified['scans'][arch]
        check(scan.get('attestation_verified') is True and scan.get('architecture') == arch and Path(scan.get('proof_file', '/no-file')).is_file(), 'missing signed architecture scan evidence')
    joburl = verified.get('job_url', '')
    check(isinstance(verified.get('job_id'), int) and re.fullmatch(r'https://github.com/bex-co/bex/actions/runs/\d+/job/' + str(verified['job_id']), joburl), 'missing publisher job identity')
    return sha, verification_sha


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--receipt', default='/tmp/bex-m188-published-acceptance.json')
    parser.add_argument('--plan', default='/tmp/bex-m188-adoption-plan.json')
    parser.add_argument('--prepare-dir', help='Only after complete verified receipt: write review copies into a NEW /tmp directory')
    args = parser.parse_args()
    native = (ROOT / NATIVE).read_text()
    inventory_text = (ROOT / INVENTORY).read_text()
    inventory = json.loads(inventory_text)
    receipt = json.loads(Path(args.receipt).read_text())
    local_green_proofs = verify_local_green_proofs()
    _, current_workflow_profile = reviewed_current_workflow()
    matrix = load_module('m188_matrix_plan', ROOT / 'deploy/native-base/matrix.py')
    freshness = load_module('m188_freshness_plan', ROOT / 'scripts/lib/toolchain-freshness.py')
    supplied = (ROOT / 'lego/operator/internal/build/nativebases.go').read_text()
    check(re.search(r'var nativeSuppliedBases = map\[string\]string\{\s*\}', supplied), 'nativeSuppliedBases is no longer empty: review concurrent supplied-base adoption')
    primaries = native.split('var nativeToolchains =', 1)[1].split('// nativeBun', 1)[0]
    rows = re.findall(r'\{\s*line:\s*"[^"\n]+"[^{}]*\}', primaries)
    mappings = []
    preview = native
    ids = {entry['id'] for entry in inventory['images']}
    for runtime, line in TARGETS:
        ident = 'native-' + runtime + '-' + line.replace('.', '-')
        official = [entry for entry in inventory['images'] if entry['id'] == ident]
        check(len(official) == 1, 'official inventory entry missing/duplicated: ' + ident)
        official = official[0]
        official_ref = official['committed']
        check(official_ref.startswith(runtime + ':' + line + '-'), 'unexpected official source for ' + ident)
        matches = [row for row in rows if 'image: "' + official_ref + '"' in row]
        check(len(matches) == 1 and not re.search(r'\bupstream:', matches[0]), 'native row changed or already adopted: ' + ident)
        row = matches[0]
        version = re.search(r'\bversion:\s*"([^"]+)"', row).group(1)
        tag = official_ref.split('@')[0].replace(':', '-')
        platform_id = 'platform-' + ident
        check(platform_id not in ids, 'platform inventory id already exists: ' + platform_id)
        upstream = REPO + ':native-' + tag
        image_template = upstream + '@<verified-published-index-digest>'
        proposed = row.replace('image: "' + official_ref + '"', 'image: "' + image_template + '", upstream: "' + official_ref + '"')
        check(native.count(row) == 1, 'ambiguous row location: ' + ident)
        preview = preview.replace(row, proposed, 1)
        mappings.append({'runtime': runtime, 'line': line, 'version': version, 'receipt_key': tag, 'official_inventory_id': ident, 'platform_inventory_id': platform_id, 'image_template': image_template, 'upstream': official_ref, 'inventory_template': {'id': platform_id, 'kind': 'native-base', 'upstream': upstream, 'committed': image_template, 'resolved_at': '<receipt checked_at UTC without fractions>', 'source': '<verified workflow/job/source, amd64+arm64 signature, SBOM, scan and behavioral receipts>', 'files': [NATIVE]}})
    check(matrix.matrix(preview) == matrix.matrix(native), 'adoption changed official publishing matrix')
    expected_tags = {row['receipt_key'] for row in mappings}
    complete = receipt.get('source_model') == SOURCE_MODEL and receipt.get('state') == 'complete' and not receipt.get('remaining') and not receipt.get('failures') and set(receipt.get('verified', {})) == expected_tags
    plan = {'schema': 'bex.m188.readonly-adoption-plan/v1', 'created_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'repository': str(ROOT), 'head': git('rev-parse', 'HEAD').strip(), 'source_sha256': {NATIVE: hashlib.sha256(native.encode()).hexdigest(), INVENTORY: hashlib.sha256(inventory_text.encode()).hexdigest()}, 'receipt': str(Path(args.receipt)), 'receipt_checked_at': receipt.get('checked_at'), 'receipt_state': receipt.get('state'), 'receipt_verified_count': len(receipt.get('verified', {})), 'complete_receipt_available': complete, 'required_portable_recipe_ancestor': FIXED_RECIPE, 'required_source_model': SOURCE_MODEL, 'approved_producer_sources': sorted(APPROVED_PRODUCER_SOURCES), 'approved_verification_sources': sorted(APPROVED_VERIFICATION_SOURCES), 'producer_runtime_restrictions': {sha: sorted(tags) for sha, tags in PRODUCER_RUNTIME_RESTRICTIONS.items()}, 'verification_runtime_restrictions': {sha: sorted(tags) for sha, tags in VERIFICATION_RUNTIME_RESTRICTIONS.items()}, 'current_reviewed_workflow_source': RECOVERY_WORKFLOW, 'current_reviewed_workflow_sha256': REVIEWED_WORKFLOW_SHA256, 'current_workflow_matches_reviewed_recovery': current_workflow_profile['compatible_with_canonical'], 'current_workflow_raw_matches_reviewed_recovery': current_workflow_profile['raw_matches_canonical'], 'current_workflow_compatible_with_reviewed_recovery': True, 'current_tree_workflow_compatibility': current_workflow_profile, 'source_binding': 'Producer OCI labels/recipe/official inputs remain separate from verification workflow SHA/signature/gates. Recovered authorization is loaded from verification commit; Earlier approved normal candidates retain their source-pinned original verifier; recovered producer may be any explicitly approved producer only with exact source-pinned authorization.', 'local_green_proofs_verified': local_green_proofs, 'supported_targeted_proof_mode': TARGETED_MODE, 'root_approved_layer_custody': {'file': str(LAYER_CUSTODY_FILE), 'sha256': LAYER_CUSTODY_SHA256}, 'targeted_proof_requirements': 'Digest-authenticated original gzip bundles/index/commands; unchanged derived DSSE; exact SPDX child or root-approved full-stream content reconstruction and exact Trivy config binding; no arbitrary alias or reconstructed signed DockerReference', 'published_node_portability_required': {'receipt_field': 'verified[node_tag].security.portable_companion_baseline', 'distribution': 'Debian11 Bullseye', 'architectures': ['amd64', 'arm64'], 'header_major': 'must equal selected 14/16/18/20 line', 'source_record_path': '/usr/share/bex-node-brotli/sources.txt', 'actual_per_architecture_log_lines_required': True}, 'adoption_only_matrix_equal': True, 'native_supplied_bases_empty': True, 'official_inventory_entries_retained_unchanged': True, 'mappings': mappings, 'scope': 'Data-only 11 reviewed native lines; preserve versions/defaults/official provenance. Other exact patches keep official lookup and existing reviewed fallback. No production deployment or repository writes.'}
    planpath = Path(args.plan).resolve()
    check(planpath.is_relative_to(Path('/tmp').resolve()), 'plan output must stay in /tmp')
    planpath.write_text(json.dumps(plan, indent=2) + '\n')
    print(json.dumps({'plan': str(planpath), 'matrix_equal': True, 'complete_receipt_available': complete, 'rows': len(mappings)}))
    if not args.prepare_dir:
        return
    check(complete, 'REFUSING adoption copies: receipt is incomplete; all 11 independently verified published candidates are required')
    check(receipt.get('source_model') == SOURCE_MODEL, 'receipt must use source-separated verification model v2')
    check(receipt.get('registry_repository') == REPO and receipt.get('tag_prefix') == 'native-' and receipt.get('workflow') == IDENTITY and receipt.get('required_security_ancestor') == ANCESTOR, 'receipt publication identity mismatch')
    prepared = native
    additions = []
    for row in mappings:
        verified = receipt['verified'][row['receipt_key']]
        sha, verification_sha = validate_candidate(verified, row, receipt, matrix, native)
        digest = verified['digest']
        joburl = verified['job_url']
        image = row['inventory_template']['upstream'] + '@' + digest
        token = 'image: "' + row['upstream'] + '"'
        check(prepared.count(token) == 1, 'native row changed during preparation')
        prepared = prepared.replace(token, 'image: "' + image + '", upstream: "' + row['upstream'] + '"', 1)
        entry = dict(row['inventory_template'], committed=image, resolved_at=datetime.datetime.fromisoformat(receipt['checked_at'].replace('Z', '+00:00')).astimezone(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'), source=f'Verified native Brotli backport; {joburl}; producer {sha}; verification {verification_sha}; amd64+arm64 signature, SPDX, signed scans and offline runtime gates')
        additions.append(entry)
    check(matrix.matrix(prepared) == matrix.matrix(native), 'adoption changed publishing inputs')
    proposed_inventory = dict(inventory, images=inventory['images'] + additions)
    freshness.parse_inventory(json.dumps(proposed_inventory))
    for entry in proposed_inventory['images']:
        if NATIVE in entry['files']:
            check(freshness.digest_of(entry['committed']) in prepared, 'inventory pin absent from prepared source')
    check((ROOT / NATIVE).read_text() == native and (ROOT / INVENTORY).read_text() == inventory_text, 'repository changed during preparation: retry against latest state')
    output = Path(args.prepare_dir).resolve()
    check(output.is_relative_to(Path('/tmp').resolve()) and not output.exists(), 'prepare output must be a NEW /tmp directory')
    output.mkdir()
    (output / 'native.go').write_text(prepared)
    (output / 'toolchain-freshness.json').write_text(json.dumps(proposed_inventory, indent=2) + '\n')
    (output / 'plan.json').write_text(json.dumps(plan, indent=2) + '\n')
    print('Prepared review copies only: ' + str(output))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, subprocess.CalledProcessError) as error:
        print(str(error), file=sys.stderr)
        raise SystemExit(1)
