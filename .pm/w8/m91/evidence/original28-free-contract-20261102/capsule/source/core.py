"""Small exact general predicates reused from reviewed V3MAIN, no m83 fixture gate."""
import datetime,hashlib,zlib,json,re
WORKSPACE='tea-da2isimlm39c739m4ofg'
WORKSPACE_NAME='tian-personal'
MAX_METADATA=2*1024*1024
MAX_BODY=1024*1024
SERVICE_ID=re.compile(r'srv-[a-z0-9]+\Z')
DEPLOY_ID=re.compile(r'dep-[a-z0-9]+\Z')
class Unproved(ValueError):pass
def require(condition, reason):
    if not condition:
        raise Unproved(reason)

def sha(body):
    return hashlib.sha256(body).hexdigest()

def instant(value):
    parsed = datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))
    require(parsed.utcoffset() is not None, 'UTC offset required')
    return parsed.astimezone(datetime.timezone.utc)

def completed(capture):
    require(type(capture.get('exit_code')) is int and capture['exit_code'] == 0,
            'capture did not complete with canonical exit0')
    require(capture.get('complete') is True and capture.get('bound') is None
            and capture.get('signal') is None, 'capture was partial, signaled or bounded')

def gunzip_bounded(body, limit=MAX_BODY):
    decoder = zlib.decompressobj(16 + zlib.MAX_WBITS)
    output = decoder.decompress(body, limit + 1)
    require(len(output) <= limit and not decoder.unconsumed_tail and decoder.eof and not decoder.unused_data,
            'gzip is oversized, incomplete or has unqualified trailing data')
    return output

def compare_cleanup(before, after, resources):
    require(before['workspace'] == after['workspace'] == WORKSPACE, 'baseline workspace changed')
    require(before.get('complete') is True and after.get('complete') is True,
            'complete before/after baseline observations missing')
    require(all(row.get('state') in ('deleted', 'not-created') for row in resources), 'own or uncertain fixture remains')
    missing = set(before['ids']) - set(after['ids'])
    require(not missing, 'baseline resources disappeared; do not repair foreign state')
    require(not ({row['id'] for row in resources if row.get('id')} & set(after['ids'])), 'own resource remains listed')
    for phase, snapshot in [('before', before), ('after', after)]:
        limits = snapshot.get('limits')
        require(isinstance(limits, dict), phase + ' quota observation missing')
        for name in ('services', 'postgres', 'keyValues'):
            row = limits.get(name)
            require(isinstance(row, dict) and all(type(row.get(key)) is int and row[key] >= 0
                    for key in ('used', 'terminating', 'limit', 'freeUsed', 'freeLimit')), phase + ' typed quota row missing:' + name)
            require(row['terminating'] == 0, phase + ' quota termination has not converged to0')
        require(limits['services']['used'] == len(set(snapshot['ids'])) == len(snapshot['ids']),
                phase + ' service quota used count differs from complete service IDs')
    additions = sorted(set(after['ids']) - set(before['ids']))
    require(after['limits']['services']['used'] - before['limits']['services']['used'] == len(additions),
            'service used delta is not explained by observed foreign additions')
    for name in ('postgres', 'keyValues'):
        require(after['limits'][name]['used'] == before['limits'][name]['used'],
                'unexplained foreign datastore quota change:' + name)
    return {'state': 'PASS', 'baseline_preserved': True, 'own_resources_absent': True,
            'own_used_count_delta': 0, 'concurrent_additions': additions}

def gate_free(owner,usage,limits,now,observations):
    require(all(isinstance(value,dict) for value in (owner,usage,limits,observations)), 'actual Free metadata/captures missing')
    require(owner.get('id')==WORKSPACE and owner.get('name')==WORKSPACE_NAME,'actual selected workspace differs')
    require(usage.get('workspaceId')==WORKSPACE and usage.get('period')==now.strftime('%Y-%m'),'actual current Free period missing')
    for role in ('owner','usage','limits'):
        event=observations.get(role,{})
        require(isinstance(event,dict),'actual Free response observation missing')
        completed(event.get('transport',{}))
        require(event.get('status')==200 and 0<=(now-instant(event['at'])).total_seconds()<=300,
                'actual Free response observation stale, future or not200')
    # Coverage/Through aggregate all meters, including egress: diagnostic only.
    hours=usage.get('freeInstanceHours',{})
    require(isinstance(hours,dict),'actual Free instance-hours object missing')
    used,included=hours.get('used'),hours.get('included')
    import math
    require(type(used) in (int,float) and type(included) in (int,float) and math.isfinite(used) and math.isfinite(included)
            and used>=0 and included>0 and included-used>=10,'original28minimum10verifiedFreehours unavailable')
    row=limits.get('services',{})
    require(isinstance(row,dict),'actual service capacity object missing')
    require(all(type(row.get(k)) is int and row[k]>=0 for k in ('used','terminating','limit'))
            and row['terminating']==0 and (row['limit']==0 or row['used']<row['limit']),'actual typed Free service capacity unavailable')
    return {'state':'PASS','period':usage['period'],'workspace':WORKSPACE,'free_hours_remaining':included-used}
