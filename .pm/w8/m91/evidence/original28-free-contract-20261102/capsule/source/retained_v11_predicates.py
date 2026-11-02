"""Exact pure canonical V11 report/root binders; never verify/download/execute."""
import hashlib,json,re,urllib.parse
from pathlib import Path
from collections import Counter
from core import require
DIGEST=re.compile(r'sha256:[a-f0-9]{64}')
MAX_DOCUMENT=8<<20
MAX_BLOB=16<<30
REPOSITORY='bex-co/bex-cnb-run'
REQUIRED={'postgresql-client-18','pandoc','sqlite3','gettext','ghostscript','jq','rsync','swig','pigz','zip','python3-dev','python3-pip','python3-setuptools'}
def descriptor(value):
    require(isinstance(value, dict) and DIGEST.fullmatch(value.get('digest', '')), 'invalid blob descriptor digest')
    require(type(value.get('size')) is int and 0 <= value['size'] <= MAX_BLOB, 'invalid blob descriptor size')
    return value['digest'], value['size']

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
