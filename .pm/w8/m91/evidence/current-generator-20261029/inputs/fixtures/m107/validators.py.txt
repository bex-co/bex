def valid_before(v, x):
    return v.get('marker') == x['marker'] and v.get('source') == reference['files'] and v.get('node', '').startswith('v24.') and v.get('unzip', '').startswith('UnZip ')

def valid_after(v, x):
    return v.get('marker') == x['marker'] and v.get('source') == reference['files'] and v.get('node', '').startswith('v24.') and v.get('zip', '').startswith('This is Zip ') and ('No errors detected' in v.get('unzip_test', ''))
