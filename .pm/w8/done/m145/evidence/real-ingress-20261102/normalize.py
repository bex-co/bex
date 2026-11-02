import json,sys,re
for line in sys.stdin:
    line=line.strip()
    if not line.startswith('{') or 'RequestHost' not in line: continue
    d=json.loads(line)
    for k in ('Duration','time','StartUTC','StartLocal'):
        if k in d: d[k]='<v>'
    for k in [k for k in d if k.endswith('_Rndr-Id')]: d[k]='<uuid>' if re.fullmatch(r'[0-9a-f-]{36}',d[k]) else d[k]
    print(json.dumps(d,sort_keys=True))
