def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and v.get('node', '').startswith('v24.') and (v.get('source_ts_sha256') == reference['source_ts_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('compiler', {}).get('status') is None and (v['compiler'].get('error') == 'ENOENT')

def valid_after(v, x):
    return valid_base(v, x) and v.get('compiler', {}).get('status') == 0 and (v['compiler'].get('version') == 'Version ' + reference['typescript']) and (v['compiler'].get('error') is None) and (v.get('input') == reference['rows']) and (v.get('result') == reference['expected']) and (len(v.get('compiled_js_sha256', '')) == 64)
