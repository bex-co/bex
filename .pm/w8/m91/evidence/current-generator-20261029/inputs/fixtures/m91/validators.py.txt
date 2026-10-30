def valid_before(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('backup') == reference['backup']) and v.get('node', '').startswith('v24.') and (set(v.get('tools', {})) == {'psql', 'pg_dump', 'pg_restore'}) and all((z['error'] == 'ENOENT' and z['status'] is None for z in v['tools'].values()))

def valid_after(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('backup') == reference['backup']) and (v.get('rows') == reference['rows']) and (v.get('rows_sha256') == reference['rows_sha256']) and (v.get('restored_from_archive') is True) and v.get('node', '').startswith('v24.') and (set(v.get('tools', {})) == {'psql', 'pg_dump', 'pg_restore'}) and all((z['status'] == 0 and z['error'] is None and ('(PostgreSQL) 18.' in z['stdout']) for z in v['tools'].values()))
