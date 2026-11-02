def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and v.get('python', '').startswith('3.13.') and (v.get('program_sha256') == reference['program_sha256']) and (v.get('input') == {k: reference['input'][k] for k in ['bytes', 'sha256']})

def valid_before(v, x):
    return valid_base(v, x) and v.get('pigz_path') is None

def valid_after(v, x):
    import zlib
    g = v.get('gzip', {})
    data = (reference['input']['unit'] * reference['input']['repetitions']).encode()
    return valid_base(v, x) and g.get('bytes', 0) > 0 and (len(g.get('sha256', '')) == 64) and g.get('header_hex', '').startswith('1f8b08') and (g.get('CRC32') == zlib.crc32(data)) and (g.get('ISIZE') == len(data)) and (g.get('inflated_bytes') == len(data)) and (g.get('inflated_sha256') == reference['input']['sha256'])
