"""One-attempt registry layer budget measurement; no extraction or persisted bodies."""
import datetime
import hashlib
import io
import json
import math
import os
from pathlib import Path
import resource
import signal
import sys
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zlib

ROOT = Path(__file__).resolve().parent
INPUT = Path('/private/tmp/bex-w8-pnpm-standalone-smoke-plan-20261031-zha92l7x/input-identity.json')
INPUT_SHA = 'eabe0437697e7ddad7749432ade4ceffc4aeb0e59d57792bf7c78b9173504213'
CHUNK = 65536
MIB = 1024**2
FLOOR = 2*1024**3
LIMITS = {'rss_bytes': 128*MIB, 'metadata_bytes': 16*MIB, 'layer_inflated_bytes': 256*MIB,
          'image_inflated_bytes': 512*MIB, 'members_per_image': 50000, 'member_bytes': 128*MIB,
          'extension_bytes': 65536, 'path_bytes': 4096, 'pax_keys': 256,
          'gzip_members_per_layer': 8, 'layer_seconds': 240, 'total_seconds': 1200}
START = time.monotonic()
MIN_FREE = None
MAX_RSS = 0
LAST_PROGRESS = 0
IMAGE_BYTES = 0
MIN_FREE_BEFORE = None


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


class Stop(Exception):
    pass


def budget():
    global MIN_FREE, MAX_RSS
    st = os.statvfs('/Users/tianpan/projects/bex7')
    free = st.f_bavail*st.f_frsize
    MIN_FREE = free if MIN_FREE is None else min(MIN_FREE, free)
    rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    if sys.platform != 'darwin':
        rss *= 1024
    MAX_RSS = max(MAX_RSS, rss)
    if free < FLOOR:
        raise Stop('physical free floor reached')
    if rss > LIMITS['rss_bytes']:
        raise Stop('RSS limit reached')
    if sum(p.stat().st_size for p in ROOT.rglob('*') if p.is_file()) > LIMITS['metadata_bytes']:
        raise Stop('metadata limit reached')
    if time.monotonic()-START > LIMITS['total_seconds']:
        raise Stop('total time limit reached')


def save(name, value):
    data = (json.dumps(value, indent=2)+'\n').encode()
    if len(data) > MIB:
        raise Stop('single receipt limit reached')
    q = ROOT/(name+'.tmp')
    q.write_bytes(data)
    q.replace(ROOT/name)


def progress(value, force=False):
    global LAST_PROGRESS
    if force or time.monotonic()-LAST_PROGRESS >= 15:
        value = {'at': now(), **value, 'min_free': MIN_FREE, 'max_rss': MAX_RSS}
        print(json.dumps(value), flush=True)
        with (ROOT/'progress.jsonl').open('a') as f:
            f.write(json.dumps(value)+'\n')
        LAST_PROGRESS = time.monotonic()


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        out = super().redirect_request(req, fp, code, msg, headers, newurl)
        if out is not None and urllib.parse.urlsplit(req.full_url).netloc != urllib.parse.urlsplit(newurl).netloc:
            out.remove_header('Authorization')
        return out


class GzipStream:
    def __init__(self, response, expected, diffid, digest):
        self.response, self.expected, self.diffid, self.digest = response, expected, diffid, digest
        self.compressed = self.inflated = 0
        self.csha, self.dsha = hashlib.sha256(), hashlib.sha256()
        self.z = zlib.decompressobj(16+zlib.MAX_WBITS)
        self.pending = b''
        self.buf = bytearray()
        self.finished = False
        self.members = 1
        self.started = time.monotonic()

    def read(self, n):
        global IMAGE_BYTES
        if n < 0 or n > CHUNK:
            raise Stop('unbounded reader request refused')
        while len(self.buf) < n and not self.finished:
            budget()
            if time.monotonic()-self.started > LIMITS['layer_seconds']:
                raise Stop('layer time limit reached')
            if not self.pending:
                b = self.response.read(min(CHUNK, max(1, self.expected-self.compressed+1)))
                if not b:
                    if not self.z.eof:
                        raise Stop('premature gzip EOF')
                    self.finished = True
                    break
                self.compressed += len(b)
                self.csha.update(b)
                if self.compressed > self.expected:
                    raise Stop('overlong compressed body')
                self.pending = b
            if self.z.eof:
                self.members += 1
                if self.members > LIMITS['gzip_members_per_layer']:
                    raise Stop('gzip member limit reached')
                self.z = zlib.decompressobj(16+zlib.MAX_WBITS)
            allowance = min(CHUNK, LIMITS['layer_inflated_bytes']-self.inflated,
                            LIMITS['image_inflated_bytes']-IMAGE_BYTES)
            if allowance <= 0:
                raise Stop('decompressed byte limit reached before next output')
            b = self.z.decompress(self.pending, allowance)
            self.pending = self.z.unused_data if self.z.eof else self.z.unconsumed_tail
            self.inflated += len(b)
            IMAGE_BYTES += len(b)
            self.dsha.update(b)
            self.buf.extend(b)
            progress({'stage': 'body', 'digest': self.digest, 'compressed_bytes': self.compressed,
                      'inflated_bytes': self.inflated})
        b = bytes(self.buf[:n])
        del self.buf[:n]
        return b

    def finish(self):
        while self.read(CHUNK):
            pass
        if self.compressed != self.expected or 'sha256:'+self.csha.hexdigest() != self.digest:
            raise Stop('complete compressed size/SHA mismatch')
        if 'sha256:'+self.dsha.hexdigest() != self.diffid:
            raise Stop('complete decompressed DiffID mismatch')

    def counters(self):
        return {'compressed_bytes': self.compressed, 'compressed_sha256': self.csha.hexdigest(),
                'inflated_bytes': self.inflated, 'inflated_sha256': self.dsha.hexdigest(),
                'gzip_members': self.members}


class BoundedInfo(tarfile.TarInfo):
    def _proc_member(self, archive):
        if self.size < 0 or self.size > LIMITS['member_bytes']:
            raise Stop('tar member size limit reached')
        if self.type == tarfile.GNUTYPE_SPARSE:
            raise Stop('sparse tar unsupported for allocation estimate')
        if self.type in (tarfile.XHDTYPE, tarfile.XGLTYPE, tarfile.GNUTYPE_LONGNAME, tarfile.GNUTYPE_LONGLINK):
            if self.size > LIMITS['extension_bytes']:
                raise Stop('tar extension size limit reached')
        return super()._proc_member(archive)


def inspect_tar(stream):
    count = regular = data = allocated = 0
    types = {}
    with tarfile.open(fileobj=stream, mode='r|', bufsize=CHUNK, tarinfo=BoundedInfo) as archive:
        for member in archive:
            count += 1
            if count > LIMITS['members_per_image']:
                raise Stop('tar member count limit reached')
            if max(len(member.name.encode(errors='surrogateescape')), len(member.linkname.encode(errors='surrogateescape'))) > LIMITS['path_bytes']:
                raise Stop('tar name limit reached')
            if len(member.pax_headers) > LIMITS['pax_keys'] or len(json.dumps(member.pax_headers)) > LIMITS['extension_bytes']:
                raise Stop('pax inventory limit reached')
            if member.sparse is not None or any('sparse' in k.lower() for k in member.pax_headers):
                raise Stop('sparse allocation not supported')
            if member.size < 0 or member.size > LIMITS['member_bytes']:
                raise Stop('effective tar member limit reached')
            kind = member.type.decode('ascii', errors='backslashreplace')
            types[kind] = types.get(kind, 0)+1
            if member.isreg():
                regular += 1
                data += member.size
                allocated += math.ceil(member.size/4096)*4096
            archive.members.clear()
            budget()
    stream.finish()
    return {'visible_members': count, 'regular_files': regular, 'regular_file_bytes': data,
            'regular_4k_allocation': allocated, 'per_member_metadata_4k': count*4096,
            'file_allocation_plus_metadata_estimate': allocated+count*4096,
            'types': types}


def local_controls():
    payload = io.BytesIO()
    with tarfile.open(fileobj=payload, mode='w') as a:
        t = tarfile.TarInfo('probe');t.size=3;a.addfile(t, io.BytesIO(b'abc'))
    raw = payload.getvalue()
    import gzip
    body = gzip.compress(raw, mtime=0)
    global IMAGE_BYTES
    results = []
    for name, size, digest, diff, cap in [
        ('complete', len(body), hashlib.sha256(body).hexdigest(), hashlib.sha256(raw).hexdigest(), None),
        ('wrong-sha', len(body), '0'*64, hashlib.sha256(raw).hexdigest(), None),
        ('short-body', len(body)+1, hashlib.sha256(body).hexdigest(), hashlib.sha256(raw).hexdigest(), None),
        ('wrong-diffid', len(body), hashlib.sha256(body).hexdigest(), '0'*64, None),
        ('inflate-limit', len(body), hashlib.sha256(body).hexdigest(), hashlib.sha256(raw).hexdigest(), 512)]:
        IMAGE_BYTES = 0
        old = LIMITS['layer_inflated_bytes']
        if cap is not None:
            LIMITS['layer_inflated_bytes'] = cap
        try:
            s = GzipStream(io.BytesIO(body), size, 'sha256:'+diff, 'sha256:'+digest)
            r = inspect_tar(s)
            assert name == 'complete' and r['regular_file_bytes'] == 3
            results.append({'name': name, 'outcome': 'PASS'})
        except Stop as e:
            assert name != 'complete'
            results.append({'name': name, 'outcome': 'EXPECTED_REFUSAL', 'reason': str(e)})
        finally:
            LIMITS['layer_inflated_bytes'] = old
    save('local-controls.json', results)


def main():
    global IMAGE_BYTES, MIN_FREE_BEFORE
    os.umask(0o077)
    budget();MIN_FREE_BEFORE = MIN_FREE
    raw = INPUT.read_bytes()
    assert hashlib.sha256(raw).hexdigest() == INPUT_SHA
    identity = json.loads(raw)
    save('source-input-custody.json', {'at': now(), 'input_path': str(INPUT), 'input_sha256': INPUT_SHA,
         'driver_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(), 'limits': LIMITS,
         'no_persisted_layer_bodies': True, 'no_extraction': True, 'one_GET_per_descriptor': True})
    local_controls()
    if len(sys.argv) > 1:
        return
    tokenreq = urllib.request.Request('https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/node:pull')
    with urllib.request.urlopen(tokenreq, timeout=20) as r:
        t = json.loads(r.read(65536))['token']
    opener = urllib.request.build_opener(SafeRedirect())
    result = {'at': now(), 'type': 'BUDGET_PREFLIGHT_ONLY', 'images': [], 'complete': False}
    current = None
    try:
        for image in identity['images']:
            IMAGE_BYTES = 0
            assert len(image['layers']) == len(image['rootfs_diff_ids'])
            case = {'image': image['requested_tag'], 'child_digest': image['child_digest'], 'layers': []}
            result['images'].append(case)
            for descriptor, diffid in zip(image['layers'], image['rootfs_diff_ids']):
                budget();current = None
                assert descriptor['mediaType'] == 'application/vnd.docker.image.rootfs.diff.tar.gzip'
                assert 0 < descriptor['size'] <= 40*MIB
                attempt = {'digest': descriptor['digest'], 'expected_size': descriptor['size'],
                           'expected_diffid': diffid, 'started_at': now(), 'body_GET_attempt': 1}
                case['layers'].append(attempt);save('result.json', result)
                request = urllib.request.Request('https://registry-1.docker.io/v2/library/node/blobs/'+descriptor['digest'],
                    headers={'Authorization': 'Bearer '+t, 'Accept-Encoding': 'identity'})
                with opener.open(request, timeout=20) as response:
                    if response.status != 200:
                        raise Stop('non-200 body response')
                    cl = response.headers.get('Content-Length')
                    if cl is not None and int(cl) != descriptor['size']:
                        raise Stop('Content-Length differs from descriptor')
                    current = GzipStream(response, descriptor['size'], diffid, descriptor['digest'])
                    allocation = inspect_tar(current)
                attempt.update(current.counters());attempt.update(allocation)
                attempt.update({'complete': True, 'finished_at': now()})
                if sum(x.get('visible_members', 0) for x in case['layers']) > LIMITS['members_per_image']:
                    raise Stop('image tar member limit reached')
                save('result.json', result)
                progress({'stage': 'layer-complete', 'image': image['requested_tag'], **current.counters()}, True)
            compressed = sum(x['compressed_bytes'] for x in case['layers'])
            inflated = sum(x['inflated_bytes'] for x in case['layers'])
            allocation = sum(x['file_allocation_plus_metadata_estimate'] for x in case['layers'])
            snapshot = max(inflated, allocation)
            margin = max(32*MIB, math.ceil(snapshot*.10))
            case.update({'complete': True, 'compressed_bytes': compressed, 'decompressed_tar_bytes': inflated,
                         'conservative_file_allocation_estimate': allocation, 'snapshot_estimate': snapshot,
                         'daemon_metadata_margin_estimate': margin,
                         'conservative_compressed_plus_snapshot_plus_margin_estimate': compressed+snapshot+margin,
                         'fits_planned_384MiB_image_reservation_estimate': compressed+snapshot+margin <= 384*MIB,
                         'estimate_limits': '4KiB file blocks plus4KiB/member metadata, all layer entries summed without dedup; max with tar bytes, compressed retention plus max32MiB/10% daemon margin. A budget estimate, not actual Docker allocation or a hard kernel bound.'})
            save('result.json', result)
        result.update({'complete': True, 'outcome': 'VERIFIED_SIZE_PREFLIGHT', 'actual_exit': 0})
    except Exception as e:
        if current is not None:
            attempt.update(current.counters())
        result.update({'outcome': 'STOPPED_UNAPPROVED', 'actual_exit': 1,
            'error': {'class': type(e).__name__, 'detail': str(e) if isinstance(e, Stop) else None,
                      'HTTP_status': e.code if isinstance(e, urllib.error.HTTPError) else None}})
    finally:
        result.update({'finished_at': now(), 'min_free': MIN_FREE, 'max_rss': MAX_RSS,
                       'scratch_bytes': sum(p.stat().st_size for p in ROOT.rglob('*') if p.is_file()),
                       'runtime_or_supplier_approval': False})
        save('result.json', result)
    print(json.dumps({'outcome': result['outcome'], 'actual_exit': result['actual_exit'],
                      'images_complete': sum(bool(i.get('complete')) for i in result['images']),
                      'min_free': MIN_FREE, 'max_rss': MAX_RSS}), flush=True)
    raise SystemExit(result['actual_exit'])


if __name__ == '__main__':
    main()
