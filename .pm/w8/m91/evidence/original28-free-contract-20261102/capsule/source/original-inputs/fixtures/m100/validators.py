def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and v.get('python', '').startswith('3.13.') and (v.get('interface_sha256') == reference['interface_sha256']) and (v.get('program_sha256') == reference['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('swig_path') is None

def valid_after(v, x):
    return valid_base(v, x) and bool(v.get('swig_path')) and ('SWIG Version ' in v.get('swig_version', '')) and (v.get('result') == reference['expected']) and (v.get('unicode') == reference['unicode']) and (v.get('extension', {}).get('bytes', 0) > 0) and (len(v['extension'].get('sha256', '')) == 64) and all((len(v.get(n, '')) == 64 for n in ['generated_wrapper_sha256', 'generated_python_sha256']))
