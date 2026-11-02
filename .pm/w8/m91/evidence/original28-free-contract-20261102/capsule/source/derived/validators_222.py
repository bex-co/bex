def valid_base(v,x):return v.get('marker')==x['marker'] and v.get('files')==reference['files'] and v.get('program_sha256')==x['commands']['initial']['program_sha256']

def valid_after(v,x):return valid_base(v,x) and v.get('python')=='3.14.8' and v.get('brotli',{}).get('patched') is True and v['brotli']['minimum_fixed']=='1.0.9-2+deb12u1'
