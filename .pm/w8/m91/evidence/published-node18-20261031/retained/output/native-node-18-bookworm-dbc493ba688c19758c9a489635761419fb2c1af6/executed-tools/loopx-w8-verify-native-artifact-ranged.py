#!/usr/bin/env python3
"""Read-only review of real native candidates. Writes scratch evidence, never pins."""
import argparse
import base64
from collections import Counter
from datetime import datetime, timezone
import fcntl
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path('/Users/tianpan/projects/bex7')
REVIEWS = Path('/tmp/loopx-w8-native-artifact-reviews')
REPOSITORY = 'bex-co/bex-cnb-run'
DIGEST = re.compile(r'sha256:[a-f0-9]{64}')
MAX_DOCUMENT = 8 << 20
MAX_CONFIG = 4 << 20
MAX_BLOB = 16 << 30
MAX_ATTESTATIONS = 256 << 20
CACHE_SCHEMA = 'bex.native-anonymous-blob-pull/v2'
SPDX_TYPE = 'https://spdx.dev/Document'
TRIVY_TYPE = 'https://aquasecurity.github.io/trivy/attestation/vuln/v1'
REQUIRED = {'postgresql-client-18', 'pandoc', 'sqlite3', 'gettext', 'ghostscript', 'jq', 'rsync', 'swig', 'pigz', 'zip', 'python3-dev', 'python3-pip', 'python3-setuptools'}
COSIGN = Path('/private/tmp/bex7-native-acceptance-novb586u/cosign')
COSIGN_SHA256 = '9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4'


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def now():
    return datetime.now(timezone.utc).isoformat()


def dump(path, value):
    temporary = path.with_name(path.name + '.tmp-' + str(os.getpid()))
    temporary.write_text(json.dumps(value, indent=2) + '\n')
    temporary.replace(path)


def small_get(url, headers, limit, method='GET'):
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers, method=method), timeout=60) as response:
        raw = response.read(limit + 1)
        observed = response.headers.get('Docker-Content-Digest')
    require(len(raw) <= limit, 'registry document exceeded bounded read')
    return raw, observed


def public_registry(freshness, registry, repository):
    token_url, service = freshness.REGISTRY_AUTH[registry]
    query = urllib.parse.urlencode({'service': service, 'scope': 'repository:' + repository + ':pull'})
    raw, _ = small_get(token_url + '?' + query, {}, 128 << 10)
    payload = json.loads(raw)
    token = payload.get('token') or payload.get('access_token')
    require(isinstance(token, str) and token, 'anonymous pull token was not returned')
    host = freshness.REGISTRY_API.get(registry, registry)
    return 'https://' + host + '/v2/' + repository, {'Accept': freshness.MANIFEST_ACCEPT, 'Authorization': 'Bearer ' + token}


def manifest_document(url, headers, reference):
    raw, observed = small_get(url + '/manifests/' + reference, headers, MAX_DOCUMENT)
    actual = 'sha256:' + hashlib.sha256(raw).hexdigest()
    require(actual == observed, 'manifest bytes differ from registry digest')
    if reference.startswith('sha256:'):
        require(actual == reference, 'immutable manifest bytes differ from requested digest')
    return json.loads(raw), raw, actual


def descriptor(value):
    require(isinstance(value, dict) and DIGEST.fullmatch(value.get('digest', '')), 'invalid blob descriptor digest')
    require(type(value.get('size')) is int and 0 <= value['size'] <= MAX_BLOB, 'invalid blob descriptor size')
    return value['digest'], value['size']


def config_document(url, headers, desc):
    digest, size = descriptor(desc)
    require(size <= MAX_CONFIG, 'image config is too large')
    raw, _ = small_get(url + '/blobs/' + digest, headers, MAX_CONFIG)
    require(len(raw) == size and 'sha256:' + hashlib.sha256(raw).hexdigest() == digest, 'config bytes do not match descriptor')
    return json.loads(raw), raw


def official_ancestry(candidate, candidate_config, original, original_config):
    """Compare already SHA-verified configs; actual CI pulls/runs validate usable layer/config identity."""
    def coherent_layers(manifest, config, role):
        layers = manifest.get('layers')
        rootfs = config.get('rootfs', {})
        diffs = rootfs.get('diff_ids')
        require(isinstance(layers, list) and layers, role + ' manifest has no layer array')
        require(rootfs.get('type') == 'layers' and isinstance(diffs, list), role + ' config has no layer diffID array')
        require(len(layers) == len(diffs), role + ' layer/diffID cardinality differs')
        require(all(isinstance(diff, str) and DIGEST.fullmatch(diff) for diff in diffs), role + ' config has an invalid diffID')
        for layer in layers:
            descriptor(layer)
        return layers, diffs
    official_layers, official_diffs = coherent_layers(original, original_config, 'official source')
    candidate_layers, candidate_diffs = coherent_layers(candidate, candidate_config, 'candidate')
    count = len(official_diffs)
    require(candidate_diffs[:count] == official_diffs, 'candidate verified config does not inherit exact official base diffIDs in order')
    variance = []
    for index, (before, after) in enumerate(zip(official_layers, candidate_layers[:count])):
        if any(before.get(key) != after.get(key) for key in ('digest', 'size', 'mediaType')):
            variance.append({'layer_index': index, 'uncompressed_diff_id': official_diffs[index], 'official_descriptor': {key: before.get(key) for key in ('digest', 'size', 'mediaType')}, 'candidate_descriptor': {key: after.get(key) for key in ('digest', 'size', 'mediaType')}})
    return {'method': 'verified-config-diff-id-prefix/v1', 'official_layer_count': count, 'candidate_layer_count': len(candidate_layers), 'verified_config_diff_id_prefix_matches': True, 'official_base_diff_ids': official_diffs, 'compressed_descriptor_prefix_matches': not variance, 'compressed_descriptor_variance': variance, 'compressed_base_prefix_byte_delta': sum(layer['size'] for layer in candidate_layers[:count]) - sum(layer['size'] for layer in official_layers), 'recompression_compatible_variance_observed': bool(variance), 'manual_decompressed_layer_hashing_performed': False, 'usability_evidence': 'Actual per-architecture candidate CI runtime gates pull and execute this immutable image; raw compressed blob SHA/size checks remain independently required.'}


def image_platforms(index):
    result = {}
    for entry in index.get('manifests', []):
        platform = entry.get('platform', {})
        if platform.get('os') == 'linux' and platform.get('architecture') in ('amd64', 'arm64'):
            arch = platform['architecture']
            require(arch not in result, 'duplicate candidate architecture: ' + arch)
            digest, _ = descriptor(entry)
            result[arch] = digest
    require(set(result) == {'amd64', 'arm64'}, 'both Linux architectures are required')
    return result


def official_environment(config):
    result = {}
    for item in config.get('config', {}).get('Env', []):
        key, sep, value = item.partition('=')
        require(sep and key not in result, 'invalid or duplicate image environment entry')
        result[key] = value
    return result


def gh_json(arguments):
    last = None
    for attempt in range(3):
        try:
            result = subprocess.run(['gh', *arguments], env=dict(os.environ, GODEBUG='http2client=0'), capture_output=True, text=True, timeout=80)
            if result.returncode == 0:
                return json.loads(result.stdout)
            last = re.sub(r'(https?://[^?\s\"]+)\?[^\s\"]+', r'\1?[redacted]', result.stderr)[-1200:]
        except (subprocess.TimeoutExpired, json.JSONDecodeError) as error:
            last = type(error).__name__
        if attempt < 2:
            time.sleep(2 * (attempt + 1))
    raise RuntimeError('bounded read-only GitHub query failed: ' + str(last))


def source_bytes(sha, path, optional=False):
    require(re.fullmatch(r'[a-f0-9]{40}', sha), 'invalid immutable source SHA')
    known = subprocess.run(['git', 'cat-file', '-e', sha + '^{commit}'], cwd=ROOT, capture_output=True, timeout=15).returncode == 0
    if known:
        result = subprocess.run(['git', 'show', sha + ':' + path], cwd=ROOT, capture_output=True, timeout=20)
        if result.returncode == 0:
            require(len(result.stdout) <= MAX_DOCUMENT, 'source file exceeds read bound')
            return result.stdout
        if optional:
            return None
        raise RuntimeError('required reviewed source file missing: ' + path)
    try:
        raw, _ = small_get('https://raw.githubusercontent.com/bex-co/bex/' + sha + '/' + path, {}, MAX_DOCUMENT)
        return raw
    except urllib.error.HTTPError as error:
        if optional and error.code == 404:
            return None
        raise


def recipe_tree(sha):
    result = subprocess.run(['git', 'ls-tree', '-r', '-z', sha, '--', 'deploy/native-base'], cwd=ROOT, capture_output=True, timeout=20)
    if result.returncode == 0:
        rows = []
        for entry in result.stdout.decode().split('\0'):
            if not entry:
                continue
            metadata, path = entry.split('\t', 1)
            mode, kind, blob = metadata.split()
            if kind == 'blob':
                rows.append({'path': path, 'mode': mode, 'sha': blob})
    else:
        data = gh_json(['api', 'repos/bex-co/bex/git/trees/' + sha + '?recursive=1'])
        require(not data.get('truncated'), 'reviewed source tree response is truncated')
        rows = [entry for entry in data['tree'] if entry['path'].startswith('deploy/native-base/') and entry['type'] == 'blob']
    require(rows, 'reviewed factory recipe tree is empty')
    return {entry['path']: {'mode': entry['mode'], 'git_blob_id': entry['sha']} for entry in rows if not entry['path'].endswith('.md')}


def matrix_base(sha, name):
    matrix_source = source_bytes(sha, 'deploy/native-base/matrix.py')
    native_source = source_bytes(sha, 'lego/operator/internal/build/native.go')
    namespace = {'__name__': '__reviewed_native_matrix__'}
    exec(compile(matrix_source, 'reviewed-' + sha + '-matrix.py', 'exec'), namespace)
    rows = [row for row in namespace['matrix'](native_source.decode())['include'] if row['name'] == name]
    require(len(rows) == 1, 'reviewed matrix row missing or ambiguous')
    return rows[0]['base']


def recovery_provenance(verification_sha, name, base, digest, ci, out):
    if ci['primary_build_conclusion'] == 'success':
        return {'mode': 'fresh-build', 'producer_sha': verification_sha, 'verification_sha': verification_sha}
    require(ci['primary_build_conclusion'] == 'skipped', 'candidate lacks a successful build or explicit recovery skip')
    require(ci.get('recovery_selection_conclusion') == 'success', 'recovered candidate selection did not pass')
    raw = source_bytes(verification_sha, '.github/native-base-recovery.json')
    authorization = json.loads(raw)
    require(authorization.get('schema') == 'bex.native-publisher-recovery/v1', 'unrecognized reviewed recovery authorization')
    entry = authorization['images'].get(name)
    require(isinstance(entry, dict) and entry.get('base') == base, 'recovery is not authorized for this exact reviewed base')
    require(entry.get('ref') == 'ghcr.io/' + REPOSITORY + '@' + digest, 'recovery index is not the exact authorized immutable digest')
    producer = entry.get('producer', '')
    require(re.fullmatch(r'[a-f0-9]{40}', producer), 'recovery producer SHA is invalid')
    require(type(entry.get('run_id')) is int and type(entry.get('job_id')) is int, 'recovery producer run/job identity is missing')
    expected_url = 'https://github.com/bex-co/bex/actions/runs/' + str(entry['run_id']) + '/job/' + str(entry['job_id'])
    require(entry.get('evidence') == expected_url, 'reviewed producer evidence URL differs from run/job identity')
    before, after = recipe_tree(producer), recipe_tree(verification_sha)
    require(before == after, 'producer and verifier non-documentation recipe blobs/modes differ')
    require(matrix_base(producer, name) == base and matrix_base(verification_sha, name) == base, 'producer/verifier reviewed base inputs differ')
    job = gh_json(['api', 'repos/bex-co/bex/actions/jobs/' + str(entry['job_id'])])
    require(job.get('id') == entry['job_id'] and job.get('run_id') == entry['run_id'] and job.get('head_sha') == producer, 'original producer job identity/source does not match authorization')
    require(job.get('head_branch') == 'main' and job.get('status') == 'completed', 'original producer job is not a completed main job')
    require(job.get('name', '').startswith('build (' + name + ','), 'original producer job belongs to a different matrix row')
    steps = {step['name']: step for step in job['steps']}
    required = {'Run docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a', 'verify real native tool transformations without network on both architectures'}
    require(required <= steps.keys() and all(steps[key]['status'] == 'completed' and steps[key]['conclusion'] == 'success' for key in required), 'original producer build and both-arch runtime gates did not pass')
    (out / 'reviewed-recovery-authorization.json').write_bytes(raw)
    dump(out / 'producer-job.json', job)
    dump(out / 'recipe-inputs-producer.json', before)
    dump(out / 'recipe-inputs-verification.json', after)
    return {'mode': 'reviewed-immutable-recovery', 'producer_sha': producer, 'verification_sha': verification_sha, 'authorized_ref': entry['ref'], 'official_base': base, 'producer_run_id': entry['run_id'], 'producer_job_id': entry['job_id'], 'producer_job_conclusion': job['conclusion'], 'producer_evidence_url': expected_url, 'producer_successful_steps': sorted(required), 'recovery_authorization_sha256': hashlib.sha256(raw).hexdigest(), 'identical_non_documentation_recipe_file_count': len(before), 'recipe_git_blob_mode_manifest_sha256': hashlib.sha256(json.dumps(before, sort_keys=True).encode()).hexdigest(), 'producer_verifier_matrix_base_equal': True, 'approval_rule': 'Old producer overall failure does not approve this artifact. Current verification job, both-arch probes/scans/SBOM, strict current-source keyless signatures and anonymous blob hashes are still independently required.'}


def github_candidate_job(sha, name, run_id):
    if not run_id:
        runs = gh_json(['run', 'list', '--workflow', 'native-base-image.yml', '--commit', sha, '--limit', '20', '--json', 'databaseId,headSha'])
        require(runs, 'no actual native factory run was found for source SHA')
        run_id = str(runs[0]['databaseId'])
    run = gh_json(['run', 'view', str(run_id), '--json', 'headSha,jobs,url,status,conclusion'])
    require(run['headSha'] == sha, 'candidate workflow SHA does not match requested source')
    jobs = [job for job in run['jobs'] if job['name'].startswith('build (' + name + ',')]
    require(len(jobs) == 1, 'candidate matrix job is missing or ambiguous')
    job = jobs[0]
    require(job['status'] == 'completed' and job['conclusion'] == 'success', 'candidate matrix job gates are not green yet: ' + job['name'])
    expected = {'verify real native tool transformations without network on both architectures', 'scan amd64 candidate', 'scan arm64 candidate', 'sign and attest the immutable candidate'}
    steps = {step['name']: step for step in job['steps']}
    require(expected <= steps.keys(), 'candidate job omitted a required runtime/scan/signing step')
    require(all(steps[name]['status'] == 'completed' and steps[name]['conclusion'] == 'success' for name in expected), 'candidate runtime/scan/signing gate did not pass')
    primary = steps.get('Run docker/build-push-action@53b7df96c91f9c12dcc8a07bcb9ccacbed38856a')
    require(primary and primary['status'] == 'completed' and primary['conclusion'] in ('success', 'skipped'), 'primary build/recovery state is missing')
    recovery = steps.get('select a reviewed recovery artifact when its recipe is unchanged', {})
    return {'run_id': str(run_id), 'url': run['url'], 'run_status': run['status'], 'run_conclusion': run['conclusion'], 'job_id': job['databaseId'], 'job_url': job['url'], 'job_name': job['name'], 'job_conclusion': job['conclusion'], 'verified_steps': sorted(expected), 'primary_build_conclusion': primary['conclusion'], 'recovery_selection_conclusion': recovery.get('conclusion')}


def docker_source_root_aliases(directory, index_digest, platforms, manifests):
    """Reproduce Stereoscope0.3.1 Docker-export roots from independently verified layer-byte receipts."""
    directory = Path(directory)
    document = json.loads((directory / 'receipt.json').read_text())
    require(document.get('index_digest') == index_digest, 'Docker layer proof belongs to a different candidate index')
    rows = document.get('architectures', [])
    require(len(rows) == len(platforms) and {row.get('architecture') for row in rows} == set(platforms), 'Docker layer proof platform coverage differs')
    aliases, bindings = {}, []
    for row in rows:
        arch = row['architecture']
        platform, manifest = platforms[arch], manifests[arch]
        require(row.get('actual_registry_child_digest') == platform['manifest_digest'] and row.get('actual_registry_config_digest') == platform['config_digest'], 'Docker proof does not bind the actual candidate child/config')
        require(row.get('manual_decompressed_layer_hashing_performed') is True, 'Docker proof lacks actual decompressed layer hashing')
        diffs = platform['config']['rootfs']['diff_ids']
        proofs = row.get('layer_proofs', [])
        require(len(proofs) == len(manifest['layers']) == len(diffs), 'Docker proof layer cardinality differs from actual candidate')
        generated_layers = []
        for desc, diff, proof in zip(manifest['layers'], diffs, proofs):
            compressed_digest, compressed_size = descriptor(desc)
            require(proof.get('schema') == 'bex.native-compressed-and-diffid-proof/v1' and proof.get('registry_repository') == 'ghcr.io/' + REPOSITORY, 'unexpected trusted layer-proof schema/source')
            require(proof.get('compressed_digest') == compressed_digest and proof.get('compressed_bytes') == compressed_size and proof.get('uncompressed_digest') == diff, 'Docker proof compressed descriptor or actual config diffID differs')
            size = proof.get('uncompressed_bytes')
            require(type(size) is int and 0 <= size <= 64 << 30 and proof.get('manual_decompressed_layer_hashing_performed') is True, 'Docker proof uncompressed size/hash scope is invalid')
            cached = json.loads((directory / 'verified-layers' / (compressed_digest.split(':')[1] + '.json')).read_text())
            require(cached == proof, 'Docker aggregate proof differs from its independent per-blob read receipt')
            generated_layers.append({'mediaType': 'application/vnd.docker.image.rootfs.diff.tar.gzip', 'size': size, 'digest': diff})
        derived = {'schemaVersion': 2, 'mediaType': 'application/vnd.docker.distribution.manifest.v2+json', 'config': {'mediaType': 'application/vnd.docker.container.image.v1+json', 'size': manifest['config']['size'], 'digest': platform['config_digest']}, 'layers': generated_layers}
        raw = json.dumps(derived, separators=(',', ':'), ensure_ascii=False).encode()
        alias = 'sha256:' + hashlib.sha256(raw).hexdigest()
        require(alias == row.get('derived_docker_manifest_digest'), 'Docker proof synthetic digest differs from exact source algorithm')
        require((directory / (arch + '-derived-docker-manifest.json')).read_bytes() == raw, 'Docker proof serialized manifest differs')
        require(alias not in aliases, 'Docker proof alias is ambiguous across architectures')
        aliases[alias] = arch
        bindings.append({'architecture': arch, 'spdx_root_digest': alias, 'actual_registry_child_digest': platform['manifest_digest'], 'actual_registry_config_digest': platform['config_digest'], 'method': 'stereoscope-v0.3.1-docker-export-manifest-from-verified-config-and-layer-diffids-sizes/v1', 'verified_layer_count': len(proofs), 'manual_decompressed_layer_hashing_performed': True, 'proof_receipt_sha256': hashlib.sha256((directory / 'receipt.json').read_bytes()).hexdigest(), 'proof_directory': str(directory)})
    return aliases, bindings


def envelopes(path):
    require(path.stat().st_size <= MAX_ATTESTATIONS, 'verified attestation output exceeds review bound')
    text = path.read_text().strip()
    try:
        value = json.loads(text)
        return value if isinstance(value, list) else [value]
    except json.JSONDecodeError:
        return [json.loads(line) for line in text.splitlines() if line.strip()]


def statements(path, digest, predicate_type):
    result = []
    for envelope in envelopes(path):
        raw = base64.b64decode(envelope['payload'], validate=True)
        require(len(raw) <= MAX_ATTESTATIONS, 'decoded attestation exceeds review bound')
        statement = json.loads(raw)
        require(statement.get('predicateType') == predicate_type, 'unexpected signed predicate type')
        require(any(subject.get('digest', {}).get('sha256') == digest.split(':', 1)[1] for subject in statement.get('subject', [])), 'signed subject does not match candidate index')
        result.append((statement, raw))
    require(result, 'no verified attestations of requested type')
    return result


def package_arches(packages):
    arches = set()
    for package in packages:
        for external in package.get('externalRefs', []):
            locator = external.get('referenceLocator', '')
            if locator.startswith('pkg:deb/'):
                query = urllib.parse.parse_qs(urllib.parse.urlsplit(locator).query)
                arches.update(arch for arch in query.get('arch', []) if arch in ('amd64', 'arm64'))
    return arches


def spdx_binding(predicate, identities):
    require(predicate.get('spdxVersion', '').startswith('SPDX-'), 'predicate is not an SPDX document')
    packages = predicate.get('packages', [])
    roots = set(predicate.get('documentDescribes', []))
    roots.update(relation['relatedSpdxElement'] for relation in predicate.get('relationships', []) if relation.get('spdxElementId') == predicate.get('SPDXID') and relation.get('relationshipType') == 'DESCRIBES')
    root_packages = [package for package in packages if package.get('SPDXID') in roots]
    require(len(root_packages) == 1, 'SPDX must describe exactly one container image root')
    root = root_packages[0]
    digests = set()
    root_arches = set()
    for checksum in root.get('checksums', []):
        if checksum.get('algorithm') == 'SHA256':
            value = 'sha256:' + checksum.get('checksumValue', '')
            require(DIGEST.fullmatch(value), 'invalid SPDX container root SHA256')
            digests.add(value)
    for external in root.get('externalRefs', []):
        locator = external.get('referenceLocator', '')
        if locator.startswith('pkg:oci/'):
            parsed = urllib.parse.urlsplit(locator)
            root_arches.update(urllib.parse.parse_qs(parsed.query).get('arch', []))
            if '@' in parsed.path:
                value = urllib.parse.unquote(parsed.path.split('@', 1)[1])
                if DIGEST.fullmatch(value):
                    digests.add(value)
    require(digests and digests <= identities.keys(), 'SPDX root names an unrelated image digest')
    bound = {identities[digest] for digest in digests}
    require(len(bound) == 1, 'SPDX container root is not bound to one actual candidate child/config digest')
    arch = bound.pop()
    require(not root_arches or root_arches == {arch}, 'SPDX root architecture conflicts with child/config identity')
    require(package_arches(packages) == {arch}, 'SPDX Debian package architecture conflicts with its actual image identity')
    names = {package['name'] for package in packages}
    require(REQUIRED <= names, 'required native tools missing from SPDX: ' + ', '.join(sorted(REQUIRED - names)))
    require(bool({'libvips42', 'libvips42t64'} & names), 'libvips is missing from SPDX')
    interesting = REQUIRED | {'libbrotli1', 'libbrotli-dev', 'libvips42', 'libvips42t64', 'node', 'python', 'typescript', 'webpack'}
    relevant = [{'name': package['name'], 'version': package.get('versionInfo')} for package in packages if package['name'] in interesting]
    return arch, {'architecture': arch, 'root_identities': sorted(digests), 'package_count': len(packages), 'packages': relevant}


def trivy_binding(predicate, identities, platforms, ref):
    require(predicate.get('ArtifactType') == 'container_image' and predicate.get('SchemaVersion') == 2, 'predicate is not a real Trivy container-image JSON report')
    require(predicate.get('ArtifactName') == ref, 'Trivy scanned artifact name differs from immutable candidate index')
    metadata = predicate.get('Metadata', {})
    image_id = metadata.get('ImageID')
    require(image_id in identities, 'Trivy ImageID does not match a candidate config digest')
    arch = identities[image_id]
    require(image_id == platforms[arch]['config_digest'], 'Trivy ImageID is not that architecture\'s config')
    image_config = metadata.get('ImageConfig', {})
    require(image_config.get('architecture') == arch and image_config.get('os') == 'linux', 'Trivy actual image config has a different platform')
    require(image_config.get('rootfs', {}).get('diff_ids') == platforms[arch]['config']['rootfs']['diff_ids'], 'Trivy scanned filesystem layers differ from the actual candidate config')
    results = predicate.get('Results', [])
    require(isinstance(results, list) and any(result.get('Class') == 'os-pkgs' and result.get('Type') == 'debian' for result in results), 'Trivy report has no Debian OS scan result')
    severities = Counter()
    brotli = []
    count = 0
    for result in results:
        for vulnerability in result.get('Vulnerabilities', []):
            count += 1
            severities[vulnerability.get('Severity', 'UNKNOWN')] += 1
            if vulnerability.get('VulnerabilityID') == 'CVE-2026-111497' or 'brotli' in vulnerability.get('PkgName', '').lower():
                brotli.append({key: vulnerability.get(key) for key in ['VulnerabilityID', 'PkgName', 'InstalledVersion', 'FixedVersion', 'Status', 'Severity']})
    return arch, {'architecture': arch, 'config_digest': image_id, 'artifact_name': predicate['ArtifactName'], 'schema_version': predicate['SchemaVersion'], 'trivy_version': predicate.get('Trivy', {}).get('Version'), 'created_at': predicate.get('CreatedAt'), 'os': metadata.get('OS'), 'result_count': len(results), 'vulnerability_count': count, 'severity_counts': dict(severities), 'brotli_findings': brotli, 'policy': 'Every reported finding is retained. A vulnerability report is not a runtime probe or a zero-CVE claim.'}


class BlobReceipts:
    """One trusted scratch receipt cache; raw multi-GB image layers are streamed, not retained."""
    def __init__(self, path, opener=None):
        self.opener = opener
        self.path = path
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.data = path / 'receipts.json'

    def locked_cache(self):
        lock = open(self.path / 'receipts.lock', 'a+')
        fcntl.flock(lock, fcntl.LOCK_EX)
        return lock

    def read(self):
        if not self.data.exists():
            return {}
        require(self.data.stat().st_size <= MAX_DOCUMENT, 'blob receipt cache is too large')
        value = json.loads(self.data.read_text())
        require(value.get('schema') == CACHE_SCHEMA, 'unexpected blob receipt cache schema')
        return value['blobs']

    def pull(self, url, headers, desc, progress):
        digest, size = descriptor(desc)
        with open(self.path / (digest.split(':')[1] + '.lock'), 'a+') as blob_lock:
            fcntl.flock(blob_lock, fcntl.LOCK_EX)
            with self.locked_cache():
                cached = self.read().get(digest)
            if cached:
                require(cached.get('digest') == digest and cached.get('bytes_hashed') == size and cached.get('registry_repository') == 'ghcr.io/' + REPOSITORY and cached.get('schema') == CACHE_SCHEMA, 'invalid prior blob verification receipt')
                # Still prove the referenced blob is publicly readable now.
                small_get(url + '/blobs/' + digest, headers, 0, method='HEAD')
                return dict(cached, reused_prior_full_read=True, current_anonymous_head_at=now())
            progress('anonymous_blob_pull', digest=digest, expected_bytes=size)
            hashed = hashlib.sha256()
            total = 0
            heartbeat = time.monotonic()
            with (self.opener or urllib.request.urlopen)(urllib.request.Request(url + '/blobs/' + digest, headers=headers), timeout=60) as response:
                while True:
                    chunk = response.read(1 << 20)
                    if not chunk:
                        break
                    total += len(chunk)
                    require(total <= size, 'blob returned more bytes than its immutable descriptor')
                    hashed.update(chunk)
                    if time.monotonic() - heartbeat >= 30:
                        progress('anonymous_blob_pull', digest=digest, bytes_hashed=total, expected_bytes=size)
                        heartbeat = time.monotonic()
            require(total == size and 'sha256:' + hashed.hexdigest() == digest, 'anonymous blob bytes differ from descriptor digest/size')
            receipt = {'schema': CACHE_SCHEMA, 'registry_repository': 'ghcr.io/' + REPOSITORY, 'digest': digest, 'bytes_hashed': total, 'anonymous_full_get_at': now()}
            with self.locked_cache():
                cache = self.read()
                cache[digest] = receipt
                dump(self.data, {'schema': CACHE_SCHEMA, 'blobs': cache})
            return dict(receipt, reused_prior_full_read=False)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag')
    parser.add_argument('sha')
    parser.add_argument('base')
    parser.add_argument('--workflow-run', help='actual native factory run; otherwise discover by source SHA')
    parser.add_argument('--docker-layer-proof', help='actual independently byte-verified Docker-source layer proof; never an arbitrary alias map')
    args = parser.parse_args()
    require(re.fullmatch(r'[a-f0-9]{40}', args.sha), 'invalid reviewed source SHA')
    require(re.fullmatch(r'(elixir|golang|node|python|ruby|rust):[a-z0-9.-]+@sha256:[a-f0-9]{64}', args.base), 'base is not a reviewed immutable official primary')
    name = args.base.split('@', 1)[0].replace(':', '-')
    require(args.tag == 'native-' + name + '-' + args.sha, 'tag, reviewed primary and source SHA do not correspond')
    require('FRESHNESS_DIGESTS' not in os.environ, 'fixture digest injection is forbidden for real artifact review')
    out = REVIEWS / args.tag
    out.mkdir(parents=True, exist_ok=True)
    if (out / 'receipt.json').exists():
        (out / 'receipt.json').rename(out / ('receipt.previous-' + str(time.time_ns()) + '.json'))

    def progress(stage, **fields):
        event = {'at': now(), 'verified': False, 'stage': stage, **fields}
        dump(out / 'progress.json', event)
        print(json.dumps(event), file=sys.stderr, flush=True)

    progress('checking_candidate_workflow')
    ci = github_candidate_job(args.sha, name, args.workflow_run)
    dump(out / 'ci-runtime-scan-signing-gates.json', ci)
    spec = importlib.util.spec_from_file_location('freshness', ROOT / 'scripts/lib/toolchain-freshness.py')
    freshness = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(freshness)
    candidate_url, candidate_headers = public_registry(freshness, 'ghcr.io', REPOSITORY)
    progress('reading_anonymous_index')
    index, raw, digest = manifest_document(candidate_url, candidate_headers, args.tag)
    (out / 'index.json').write_bytes(raw)
    children = image_platforms(index)
    ref = 'ghcr.io/' + REPOSITORY + '@' + digest
    provenance = recovery_provenance(args.sha, name, args.base, digest, ci, out)
    producer_sha = provenance['producer_sha']
    dump(out / 'producer-verification-provenance.json', provenance)
    source_registry, source_repo, _ = freshness.parse_upstream('docker.io/library/' + args.base.split('@', 1)[0])
    source_url, source_headers = public_registry(freshness, source_registry, source_repo)
    source_index, source_raw, _ = manifest_document(source_url, source_headers, args.base.split('@', 1)[1])
    (out / 'official-index.json').write_bytes(source_raw)
    sources = image_platforms(source_index)
    platforms = {}
    candidate_manifests = {}
    identities = {}
    blobs = {}
    for arch, child in children.items():
        candidate, child_raw, _ = manifest_document(candidate_url, candidate_headers, child)
        original, original_raw, _ = manifest_document(source_url, source_headers, sources[arch])
        config, config_raw = config_document(candidate_url, candidate_headers, candidate['config'])
        original_config, _ = config_document(source_url, source_headers, original['config'])
        require(config.get('architecture') == arch and config.get('os') == 'linux', 'candidate config differs from index platform')
        require(original_config.get('architecture') == arch and original_config.get('os') == 'linux', 'official source config differs from index platform')
        labels = config.get('config', {}).get('Labels', {})
        require(labels.get('org.opencontainers.image.revision') == producer_sha and labels.get('org.opencontainers.image.source') == 'https://github.com/bex-co/bex', 'candidate OCI source/revision labels differ from authorized producer source')
        candidate_env, original_env = official_environment(config), official_environment(original_config)
        require(all(candidate_env.get(key) == value for key, value in original_env.items()), 'factory changed a primary runtime environment setting')
        official_layers = [layer['digest'] for layer in original['layers']]
        ancestry = official_ancestry(candidate, config, original, original_config)
        candidate_manifests[arch] = candidate
        config_digest = candidate['config']['digest']
        require(config_digest not in identities and child not in identities, 'platform identities are ambiguous')
        identities[config_digest] = identities[child] = arch
        platforms[arch] = {'manifest_digest': child, 'config_digest': config_digest, 'config': config, 'official_base_ancestry': ancestry, 'compressed_layer_sizes': {'candidate_bytes': sum(layer['size'] for layer in candidate['layers']), 'official_bytes': sum(layer['size'] for layer in original['layers']), 'new_bytes': sum(layer['size'] for layer in candidate['layers'][len(official_layers):]), 'candidate_layer_count': len(candidate['layers'])}}
        (out / (arch + '-manifest.json')).write_bytes(child_raw)
        (out / (arch + '-config.json')).write_bytes(config_raw)
        (out / (arch + '-official-manifest.json')).write_bytes(original_raw)
        for desc in [candidate['config'], *candidate['layers']]:
            d, size = descriptor(desc)
            if d in blobs:
                require(blobs[d]['size'] == size, 'same blob digest has conflicting sizes')
            blobs[d] = desc

    docker_root_bindings = []
    if args.docker_layer_proof:
        aliases, docker_root_bindings = docker_source_root_aliases(args.docker_layer_proof, digest, platforms, candidate_manifests)
        for alias, arch in aliases.items():
            require(alias not in identities or identities[alias] == arch, 'derived SPDX root conflicts with an actual platform identity')
            identities[alias] = arch
            platforms[arch]['official_base_ancestry']['manual_decompressed_layer_hashing_performed'] = True
            platforms[arch]['official_base_ancestry']['manual_hash_scope'] = 'All actual candidate compressed layer bytes and decompressed diffIDs/sizes were independently checked by the linked Docker-source proof.'

    require(hashlib.sha256(COSIGN.read_bytes()).hexdigest() == COSIGN_SHA256, 'verified cosign binary hash changed')
    common = ['--certificate-identity', 'https://github.com/bex-co/bex/.github/workflows/native-base-image.yml@refs/heads/main', '--certificate-oidc-issuer', 'https://token.actions.githubusercontent.com', '--certificate-github-workflow-sha', args.sha, '--output', 'json']
    with tempfile.TemporaryDirectory(prefix='w8-anonymous-cosign-') as docker_config:
        Path(docker_config, 'config.json').write_text('{}')
        empty_auth = Path(docker_config, 'containers-auth.json')
        empty_auth.write_text('{}')
        env = dict(os.environ, DOCKER_CONFIG=docker_config, REGISTRY_AUTH_FILE=str(empty_auth))
        for kind, command in [('signature', ['verify']), ('sbom', ['verify-attestation', '--type', 'spdxjson']), ('vulnerability', ['verify-attestation', '--type', TRIVY_TYPE])]:
            progress('verifying_' + kind, immutable_ref=ref)
            with (out / (kind + '.json')).open('wb') as stdout, (out / (kind + '.stderr.txt')).open('wb') as stderr:
                subprocess.run([str(COSIGN), *command, *common, ref], stdout=stdout, stderr=stderr, env=env, check=True, timeout=240)
    signed_payloads = []
    sboms, vulnerabilities = {}, {}
    for kind, predicate_type, target in [('sbom', SPDX_TYPE, sboms), ('vulnerability', TRIVY_TYPE, vulnerabilities)]:
        for index_number, (statement, payload_raw) in enumerate(statements(out / (kind + '.json'), digest, predicate_type)):
            predicate = statement['predicate']
            if kind == 'sbom':
                arch, summary = spdx_binding(predicate, identities)
            else:
                arch, summary = trivy_binding(predicate, identities, platforms, ref)
            target.setdefault(arch, []).append(summary)
            path = out / (kind + '-' + arch + '-predicate-' + str(index_number) + '.json.gz')
            with gzip.open(path, 'wt') as output:
                json.dump(predicate, output)
            signed_payloads.append({'kind': kind, 'architecture': arch, 'signed_statement_sha256': hashlib.sha256(payload_raw).hexdigest(), 'predicate_file': path.name, 'predicate_gzip_sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
        require(set(target) == {'amd64', 'arm64'}, kind + ' does not independently cover both actual platforms')
    progress('signed_reports_bound_to_actual_platforms')
    cache = BlobReceipts(REVIEWS / '.public-blob-receipts-v2')
    pulls = [cache.pull(candidate_url, candidate_headers, desc, progress) for desc in blobs.values()]
    receipt = {'reviewed_at': now(), 'official_base': args.base, 'tag': 'ghcr.io/' + REPOSITORY + ':' + args.tag, 'ref': ref, 'workflow_sha': args.sha, 'verification_workflow_sha': args.sha, 'producer_workflow_sha': producer_sha, 'producer_verification_provenance': provenance, 'ci_runtime_scan_signing_evidence': ci, 'platforms': {arch: {key: value for key, value in platform.items() if key != 'config'} for arch, platform in platforms.items()}, 'anonymous_manifest_bytes_sha_verified': True, 'anonymous_pull': {'all_unique_config_and_layer_bytes_fully_hashed': True, 'byte_hash_scope': ('Descriptor bytes: compressed layers and raw JSON configs; independently checked decompressed diffIDs/sizes are linked in spdx_additional_verified_root_bindings.' if docker_root_bindings else 'Descriptor bytes: compressed layers and raw JSON configs; no manual decompressed-layer hashing.'), 'unique_blob_count': len(pulls), 'bytes_hashed_this_run': sum(pull['bytes_hashed'] for pull in pulls if not pull['reused_prior_full_read']), 'bytes_reused_from_prior_verified_full_read': sum(pull['bytes_hashed'] for pull in pulls if pull['reused_prior_full_read']), 'prior_receipts_also_rechecked_by_anonymous_head': True, 'blob_receipts': pulls}, 'strict_keyless_signature_and_sbom_and_vulnerability_attestations_verified': True, 'spdx_additional_verified_root_bindings': docker_root_bindings, 'signed_payloads': signed_payloads, 'package_summaries_by_architecture': sboms, 'trivy_summaries_by_architecture': vulnerabilities, 'raw_cosign_outputs': {kind: {'file': kind + '.json', 'sha256': hashlib.sha256((out / (kind + '.json')).read_bytes()).hexdigest()} for kind in ['signature', 'sbom', 'vulnerability']}, 'scope': 'Published supplier candidate only. Exact generated/runtime images (including Python3.14.8), global rollout, hosted/phase/neighbor acceptance and all27-line adoption remain separate. No production pin changed.'}
    dump(out / 'receipt.json', receipt)
    dump(out / 'progress.json', {'at': now(), 'verified': True, 'stage': 'complete', 'ref': ref})
    print(json.dumps({'ref': ref, 'platforms': children, 'verified': True, 'receipt': str(out / 'receipt.json')}))


if __name__ == '__main__':
    main()
