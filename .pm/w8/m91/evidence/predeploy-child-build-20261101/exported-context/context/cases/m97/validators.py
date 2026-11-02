def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('pdf') == {k: reference['input'][k] for k in ['bytes', 'sha256']}) and v.get('python', '').startswith('3.13.') and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('gs_path') is None

def valid_after(v, x):
    return valid_base(v, x) and bool(v.get('gs_path')) and bool(v.get('gs_version')) and (v.get('png', {}).get('bytes', 0) > 0)
