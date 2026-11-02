def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('sql') == {k: reference['sql'][k] for k in ['bytes', 'sha256']}) and v.get('python', '').startswith('3.13.') and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('builtin_rows') == reference['rows'] and (set(v.get('tools', {})) == {'sqlite3'}) and (v['tools']['sqlite3']['error'] == 'ENOENT') and (v['tools']['sqlite3']['status'] is None)

def valid_after(v, x):
    return valid_base(v, x) and v.get('rows') == v.get('builtin_rows') == reference['rows'] and (v.get('database', {}).get('SQLite3_header') is True) and (v.get('tools', {}).get('sqlite3', {}).get('status') == 0) and (v['tools']['sqlite3']['error'] is None)
