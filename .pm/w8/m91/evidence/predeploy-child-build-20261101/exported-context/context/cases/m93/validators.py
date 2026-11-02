def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('document') == {k: reference['document'][k] for k in ['bytes', 'sha256']}) and v.get('node', '').startswith('v24.') and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and set(v.get('tools', {})) == {'pandoc'} and (v['tools']['pandoc']['error'] == 'ENOENT') and (v['tools']['pandoc']['status'] is None)

def valid_after(v, x):
    return valid_base(v, x) and v.get('tools', {}).get('pandoc', {}).get('status') == 0 and (v['tools']['pandoc']['error'] is None) and v['tools']['pandoc']['stdout'].startswith('pandoc ') and all((t in v.get('html', {}).get('text', '') for t in reference['document']['required_html'])) and (v.get('html', {}).get('sha256') == hashlib.sha256(v['html']['text'].encode()).hexdigest())
