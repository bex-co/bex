def decode_json(raw):
 if not raw.lstrip().startswith(('{','[')):
  messages=[s[5:].strip() for s in raw.splitlines() if s.startswith('data:')];assert len(messages)==1;raw=messages[0]
 return json.loads(raw)

def objects(raw):
 decoder=json.JSONDecoder();out=[]
 while raw.strip():
  raw=raw.lstrip();v,n=decoder.raw_decode(raw);out.extend(v if isinstance(v,list) else [v]);raw=raw[n:]
 return out

def proof(x):
    h = ctx.history(x)
    assert x['create_result']['exit'] == 0
    assert len(h) == 1 and h[0]['status'] == 'live' and (h[0].get('commit', {}).get('id') == x['primary_reference']['commit']), 'first original deployment/source not Live'
    vs = ctx.validators(x)
    valid = vs['valid_after']
    dir = ctx.fixture
    attempt = {'at': ctx.now(), 'history': h, 'requests': []}
    x.setdefault('proof_attempts', []).append(attempt)
    ctx.save()

    def record(i, v):
        if i == len(attempt['requests']):
            attempt['requests'].append(v)
        else:
            attempt['requests'][i] = v
        ctx.save()
    if x['mid'] == 94:
        requests = []
        for i, root in enumerate(['/rows', '/', '/rows']):
            path = root + '?qa409=' + x['nonce'] + '-' + str(i + 1)
            r = subprocess.run(['curl', '--disable', '-sS', '--max-time', '15', '-H', 'Accept-Encoding: identity', '-w', '\n%{http_code}', x['create_view']['serviceDetails']['url'] + path], capture_output=True, timeout=20)
            body, _, status = r.stdout.rpartition(b'\n')
            v = json.loads(body)
            e = {'path': path, 'HTTP': status.decode(), 'exit': r.returncode, 'body': v, 'sha256': hashlib.sha256(body).hexdigest(), 'pass': r.returncode == 0 and status == b'200' and valid(v['build'], x) and valid(v['runtime'], x) and (v['build']['database'] == v['runtime']['database']) and (v['runtime']['request_url'] == path)}
            record(i, e)
            requests.append(e)
    else:
        scope = {}
        scope.update(ctx.http_functions(x))
        fn = scope['exercise']
        mid = x['mid']
        ref = x['validation_reference']
        if mid == 93:

            def healthy_before(v, x):
                return vs['valid_base'](v, x) and v['tools']['pandoc']['status'] == 0 and (v['tools']['pandoc']['error'] is None)
            requests = fn(x, dir, valid, healthy_before)
        elif mid in (95,):
            requests = fn(x, h, dir, valid, ref)
        elif mid == 96:
            requests = fn(x, h, dir, valid, ref)
        elif mid in (98, 103):
            requests = fn(x, dir, ref, valid, record)
        else:
            requests = fn(x, h, dir, valid, ref, record)
        attempt['requests'] = requests
        ctx.save()
    assert len(requests) == 3 and all((v.get('pass') for v in requests)), 'independent HTTP/artifact proof failed'
    logs = ctx.cli(['logs', '--resources', x['id'], '--type', 'build', '--text', x['marker'], '--limit', '100', '--confirm', '-o', 'json'], x['client'])
    attempt['build_logs'] = logs
    ctx.save()
    assert logs['exit'] == 0
    messages = [v.get('message', '') for v in objects(logs['stdout'])]
    proofs = []
    for m in messages:
        for line in m.splitlines():
            match = re.search('QA[A-Za-z0-9_]* (\\{.*\\})', line)
            if match:
                try:
                    v = json.loads(match[1])
                    proofs.append(v)
                except ValueError:
                    pass
    assert any((valid(v, x) for v in proofs)), 'no native build after proof'
    app = ctx.cli(['logs', '--resources', x['id'], '--type', 'app', '--text', x['nonce'], '--limit', '100', '--confirm', '-o', 'json'], x['client'])
    attempt['application_logs'] = app
    ctx.save()
    assert app['exit'] == 0
    appmessages = [v.get('message', '') for v in objects(app['stdout'])]
    assert all((any((req['path'] in message for message in appmessages)) for req in requests)), 'native request log correlation failed'
    attempt['pass'] = True
    ctx.save()
    return {'first_original_deploy': h[0]['id'], 'independent_HTTP_proofs': len(requests), 'native_build_and_app_logs': True}

def shared(x):
    ctx.own(x)
    sid = x['id']
    query = 'query Acceptance {service(id:' + json.dumps(sid) + '){id name repo branch runtime plan buildCommand startCommand preDeployCommand autoDeploy}}'
    checks = []
    for surface, method, path, body in [('REST', 'GET', '/v1/services/' + sid, None), ('GraphQL', 'POST', '/graphql', {'query': query}), ('MCP', 'POST', '/mcp', {'jsonrpc': '2.0', 'id': x['sweep'], 'method': 'tools/call', 'params': {'name': 'get_service', 'arguments': {'serviceId': sid, 'workspaceId': WS}}})]:
        receipt = ctx.api(method, path, body)
        checks.append({'surface': surface, 'receipt': receipt})
        x['shared_readbacks'] = checks
        ctx.save()
        assert receipt['HTTP'] == '200'
        v = decode_json(receipt['response'])
        if surface == 'GraphQL':
            assert not v.get('errors')
            v = v['data']['service']
        elif surface == 'MCP':
            assert not v.get('error') and (not v['result'].get('isError'))
            result = v['result']
            v = result.get('structuredContent')
            if v is None:
                v = json.loads(next((a['text'] for a in result['content'] if a['type'] == 'text')))
            v = v.get('service', v)
        details = v.get('serviceDetails', {})
        specific = details.get('envSpecificDetails', {})
        build = v.get('buildCommand') if surface == 'GraphQL' else specific.get('buildCommand')
        start = v.get('startCommand') if surface == 'GraphQL' else specific.get('startCommand')
        assert v['id'] == sid and v['name'] == x['name'] and (v['repo'] == x['primary_reference']['repo']) and (v['branch'] == x['primary_reference']['branch']) and ((v.get('runtime') or details.get('runtime')) == x['runtime']) and ((v.get('plan') or details.get('plan')) == 'free') and (build == x['commands']['initial']['build']) and (start == x['commands']['initial']['start']) and (v['autoDeploy'] in (False, 'no'))
        checks[-1]['pass'] = True
        ctx.save()
    receipt = ctx.api('GET', '/v1/services/' + sid + '/env-vars')
    x['env_readback'] = receipt
    ctx.save()
    assert receipt['HTTP'] == '200'
    env = {v['key']: v['value'] for row in decode_json(receipt['response']) for v in [row.get('envVar', row)]}
    assert env == x['env']
    return {'exact_readbacks': ['REST', 'GraphQL', 'MCP', 'own harmless version env']}
