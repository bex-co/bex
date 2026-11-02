def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and v.get('python', '').startswith('3.13.') and (v.get('installed_pyvips') == '3.2.0') and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('shared_library') is None

def valid_after(v, x):
    return valid_base(v, x) and bool(v.get('shared_library')) and (v.get('input') == {k: reference['input'][k] for k in ['bytes', 'sha256']}) and (v.get('image') == {'width': 2, 'height': 1, 'bands': 3, 'format': 'uchar', 'pixels_hex': reference['expected_rgb_hex']}) and (tuple((int(n) for n in v.get('libvips', '0.0').split('.')[:2])) >= (8, 2))
