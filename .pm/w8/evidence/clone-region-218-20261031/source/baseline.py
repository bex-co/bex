"""Fixed own-workspace metadata baseline; no auth or action on import."""
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from urllib.parse import urlencode

WORKSPACE = 'tea-da2isimlm39c739m4ofg'
SERVICE_ID = re.compile(r'srv-[a-z0-9]+\Z')


def collect(api):
    ids, cursors, receipts, raw_reads = set(), set(), [], []
    cursor = None
    for _ in range(10):
        query = {'ownerId': WORKSPACE, 'limit': 100}
        if cursor is not None:
            query['cursor'] = cursor
        result = api('GET', '/v1/services?' + urlencode(query))
        assert result['exit'] == 0 and result['HTTP'] == '200', 'baseline list read failed'
        raw = result['response'].encode()
        assert len(raw) <= 2 * 1024 * 1024, 'baseline page exceeded bound'
        page = json.loads(raw)
        assert isinstance(page, list) and len(page) <= 100, 'unexpected services envelope'
        raw_reads.append(result)
        receipts.append({'at': result['at'], 'request': result['request'], 'HTTP': result['HTTP'],
                         'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()})
        if not page:
            break
        for row in page:
            service = row['service']
            sid = service['id']
            assert SERVICE_ID.fullmatch(sid) and service['ownerId'] == WORKSPACE, 'foreign or invalid baseline identity'
            assert sid not in ids, 'unstable or repeated baseline service'
            ids.add(sid)
        cursor = page[-1]['cursor']
        assert isinstance(cursor, str) and cursor and len(cursor) <= 4096 and cursor not in cursors, 'unstable cursor'
        cursors.add(cursor)
    else:
        raise RuntimeError('baseline pagination did not terminate within ten pages')
    quota = api('GET', '/v1/owners/' + WORKSPACE + '/limits')
    assert quota['exit'] == 0 and quota['HTTP'] == '200', 'baseline quota read failed'
    raw = quota['response'].encode()
    assert len(raw) <= 2 * 1024 * 1024
    limits = json.loads(raw)
    assert isinstance(limits, dict), 'unexpected quota envelope'
    raw_reads.append(quota)
    receipts.append({'at': quota['at'], 'request': quota['request'], 'HTTP': quota['HTTP'],
                     'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()})
    return {'workspace': WORKSPACE, 'ids': sorted(ids), 'limits': limits, 'receipts': receipts}, raw_reads


def compare(before, after, resources):
    assert before['workspace'] == after['workspace'] == WORKSPACE
    missing = set(before['ids']) - set(after['ids'])
    remaining = {r['id'] for r in resources if r.get('id')} & set(after['ids'])
    assert not missing, 'baseline services disappeared; do not delete or recreate foreign services'
    assert not remaining, 'owned fixture still present after exact cleanup'
    return {'baseline_preserved': True, 'owned_fixture_absent': True,
            'concurrent_additions': sorted(set(after['ids']) - set(before['ids'])),
            'before_limits': before['limits'], 'after_limits': after['limits']}


def main():
    assert not sys.flags.optimize
    os.umask(0o077)
    phase, rawroot = sys.argv[1:]
    assert phase in ('capture', 'compare')
    root = Path(rawroot).resolve()
    assert root.parent == Path('/private/tmp') and root.name.startswith('bex-w8-218-live-')
    sys.path.insert(0, str(root))
    import driver
    assert driver.P == root
    driver.check_inputs(root)
    driver.check_ready(root)
    journal = driver.setup_journal(root)
    if phase == 'capture':
        assert not journal.data['resources'] and 'baseline_capture' not in journal.data
    else:
        assert 'baseline_capture' in journal.data
        assert all(r['state'] in ('deleted', 'not-created') for r in journal.data['resources'])
    snapshot, raw_reads = collect(driver.client.api)
    (root / ('baseline-' + phase + '-reads.json')).write_text(json.dumps(raw_reads, indent=2) + '\n')
    if phase == 'capture':
        journal.data['baseline_ids'] = snapshot['ids']
        journal.data['baseline_capture'] = snapshot
        journal.save()
        result = {'baseline_recorded': True, 'service_count': len(snapshot['ids']), 'limits': snapshot['limits']}
    else:
        result = compare(journal.data['baseline_capture'], snapshot, journal.data['resources'])
        journal.data['baseline_after'] = snapshot
        journal.data['baseline_comparison'] = result
        journal.save()
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
