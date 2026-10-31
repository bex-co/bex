def expected_manifest():
    return {'release': reference['manifest']['release'], 'paths': [v['path'] for v in reference['manifest']['assets'] if v['publish']]}

def expected_files():
    return {n: {'bytes': len(reference['assets'][n].encode()), 'sha256': hashlib.sha256(reference['assets'][n].encode()).hexdigest()} for n in expected_manifest()['paths']}

def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and v.get('node', '').startswith('v24.') and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and all((v.get('tools', {}).get(n, {}).get('error') == 'ENOENT' and v['tools'][n]['status'] is None for n in ['jq', 'rsync']))

def valid_jq(v, x):
    return valid_base(v, x) and v.get('manifest') == expected_manifest() and (v.get('tools', {}).get('jq', {}).get('status') == 0) and (v['tools']['jq']['error'] is None)

def valid_after(v, x):
    return valid_jq(v, x) and v.get('copied_files') == expected_files() and (v['tools']['rsync']['status'] == 0) and (v['tools']['rsync']['error'] is None)
