def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('sources') == reference['sources']) and v.get('node', '').startswith('v24.')

def valid_before(v, x):
    return valid_base(v, x) and v.get('webpack_probe', {}).get('status') is None and (v['webpack_probe'].get('error') == 'ENOENT')

def valid_after(v, x):
    return valid_base(v, x) and v.get('webpack_probe', {}).get('status') == 0 and (v['webpack_probe'].get('error') is None) and (v.get('compiler', {}).get('version') == reference['control_compiler']) and (v['compiler'].get('errors') == 0) and (v.get('input') == reference['rows']) and (v.get('result') == reference['expected']) and (v.get('bundle', {}).get('bytes', 0) > 0) and (len(v['bundle'].get('sha256', '')) == 64)
