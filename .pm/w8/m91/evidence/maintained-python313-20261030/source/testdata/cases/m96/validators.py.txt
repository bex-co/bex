def valid_base(v, x):
    return v.get('marker') == x['marker'] and v.get('files') == reference['files'] and (v.get('po') == reference['po']) and v.get('python', '').startswith('3.13.') and (v.get('django') == reference['django']) and (v.get('program_sha256') == x['commands']['initial']['program_sha256'])

def valid_before(v, x):
    return valid_base(v, x) and v.get('tools') == {n: None for n in ['msgfmt', 'msgmerge', 'msguniq', 'xgettext']}

def valid_after(v, x):
    return valid_base(v, x) and set(v.get('tools', {})) == {'msgfmt', 'msgmerge', 'msguniq', 'xgettext'} and all(v['tools'].values()) and (v.get('translations') == v.get('independent_stdlib_translations') == reference['expected']) and (v.get('catalog', {}).get('GNU_MO_magic') is True) and (v.get('fresh_msgfmt_compilation') == {'exit': 0, 'bytes': v['catalog']['bytes'], 'sha256': v['catalog']['sha256']}) and v.get('msgfmt_version', '').startswith('msgfmt (GNU gettext-tools)')
