def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('fixture_sha256') == reference['fixture_sha256']) and v.get('python', '').startswith('3.11.')

def valid_before(v, x):
    return valid_base(v, x) and v.get('tools') == {'setuptools': False, 'pip': False, 'Python_h': False}

def valid_meta(v, x, stage):
    return valid_base(v, x) and v.get('distribution') == {'name': 'qa-native-helper', 'version': '1.0'} and (v.get('tools') == {'setuptools': True, 'pip': stage in ('packaging', 'full'), 'Python_h': stage == 'full'})

def valid_wheel(v, x, stage):
    return valid_meta(v, x, stage) and v.get('wheel') == {'label': reference['unicode'], 'installed_module_sha256': expected_module_sha}

def valid_after(v, x):
    return valid_wheel(v, x, 'full') and v.get('result') == reference['expected'] and (v.get('unicode') == reference['unicode']) and (v.get('extension', {}).get('bytes', 0) > 0) and (len(v['extension'].get('sha256', '')) == 64)
