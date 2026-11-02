"""Explicit general V3MAIN derivation; no m83 specification/readiness/oracle import."""
import base64,datetime,gzip,hashlib,http.server,json,os,re,secrets,shutil,signal,sys,threading,time
from pathlib import Path
from urllib.parse import parse_qs,urlencode,urlsplit
import core as c
from host import Budget,Runner,Stop,START_FREE,encoded,free_bytes
HERE=Path(__file__).resolve().parent
REPO=Path('/Users/tianpan/projects/bex7')
API='https://api.bex.co'
HOP={'connection','transfer-encoding','content-length','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','upgrade'}
class PrivateConfigError(Stop):
    """Only static call-site phase and exception class may leave this boundary."""
    def __init__(self,phase,cause_class):
        self.phase,self.cause_class=phase,cause_class
        super().__init__(phase+':'+cause_class)

def safe_error(phase,error):
    if isinstance(error,PrivateConfigError):
        return error.phase+':'+error.cause_class
    return phase+':'+type(error).__name__

def now():
    return datetime.datetime.now(datetime.timezone.utc)

def digest(path):
    result = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for body in iter(lambda: stream.read(65536), b''):
            result.update(body)
    return result.hexdigest()

def read_json(path, cap=2*1024*1024):
    path = Path(path)
    if path.is_symlink() or not path.is_file() or path.stat().st_size > cap:
        raise Stop('bounded regular JSON input required')
    return json.loads(path.read_bytes())

def bounded_file(path,cap):
    path=Path(path)
    if not path.exists():return b''
    c.require(not path.is_symlink() and path.is_file() and path.stat().st_size<=cap,'bounded capture file exceeded')
    with path.open('rb') as stream:body=stream.read(cap+1)
    c.require(len(body)<=cap,'bounded capture file grew past cap')
    return body

def decode_json(body):
    text = body.decode()
    if not text.lstrip().startswith(('{', '[')):
        messages = [line[5:].strip() for line in text.splitlines() if line.startswith('data:')]
        c.require(len(messages) == 1, 'one complete MCP JSON message required')
        text = messages[0]
    return json.loads(text)

def parse_headers(raw):
    blocks = raw.decode('iso-8859-1').replace('\r\n', '\n').strip().split('\n\n')
    blocks = [block.splitlines() for block in blocks if block.startswith('HTTP/')]
    c.require(bool(blocks), 'actual HTTP status line absent')
    lines = blocks[-1]
    status = re.fullmatch(r'HTTP/(1\.1|2)(?:\.0)?\s+(\d{3})(?:\s+.*)?', lines[0])
    c.require(status is not None, 'unqualified actual HTTP status line')
    headers = {}
    for line in lines[1:]:
        if ':' in line:
            name, value = line.split(':', 1)
            name = name.strip().lower()
            headers[name] = headers[name] + ', ' + value.strip() if name in headers else value.strip()
    return status[1], int(status[2]), headers

def split_included_response(raw):
    """Curl --include prefix blocks; body bytes are never text-decoded here."""
    headers=[]
    while raw.startswith(b'HTTP/'):
        end=raw.find(b'\r\n\r\n');width=4
        if end<0:end=raw.find(b'\n\n');width=2
        c.require(0<=end<=65536,'bounded included HTTP header block missing')
        headers.append(raw[:end+width]);raw=raw[end+width:]
    c.require(headers and sum(map(len,headers))<=65536,'included HTTP headers missing/oversized')
    return b''.join(headers),raw

def command_receipt(result, argv, include_output=True, redact=()):
    out = {key: value for key, value in result.items() if key not in ('stdout', 'stderr')}
    out['argv'] = [str(value) for value in argv]
    for key in ('stdout', 'stderr'):
        raw = result[key]
        out[key + '_bytes'], out[key + '_sha256'] = len(raw), c.sha(raw)
        if include_output:
            value = raw.decode(errors='replace')
            for token in redact:
                if token:
                    value = value.replace(token, '[REDACTED]')
            out[key] = value
    return out

class OpaqueTransport:
    def record(self, name, value, cleanup=False):
            try:
                self.budget.json('evidence/' + name, value, cleanup)
            except BaseException as error:
                if not cleanup:
                    raise
                self.errors.append('cleanup evidence ' + name + ': ' + type(error).__name__)
    def persist(self, data):
            # Critical actual201 identities remain writable in the reserved stop
            # margin. A failed persistence never prevents exact in-memory cleanup.
            self.budget.json('evidence/ledger.json', data, cleanup=True)
    def next_name(self, prefix):
            with self.lock:
                self.sequence += 1
                return prefix + '-%05d' % self.sequence
    def set_handlers(self):
            def stop(signum, frame):
                self.budget.error = 'received signal' + str(signum)
                if not self.finishing:
                    raise Stop(self.budget.error)
            for signum in (signal.SIGTERM, signal.SIGINT):
                self.handlers[signum] = signal.getsignal(signum)
                signal.signal(signum, stop)
    def register_session_handle(self):
            handle = self.private / 'qa-session-handle.json'
            if handle.exists():
                value = read_json(handle, 65536)
                path = Path(value['logout_jar'])
                if path.exists():
                    self.budget.register_jar(path, REPO / '.playwright-mcp')
                c.require(value.get('state') == str(self.private / 'qa-state.json'), 'own QA state path differs')
    def auth(self):
            self.check_ready()
            self.require_legal_window()
            self.auth_started = True
            command = [sys.executable, str(self.private / 'auth.py')]
            self.budget.permit_external_write('private',2*1024*1024)
            result = self.runner.capture(command, timeout=1200, limit=262144)
            self.register_session_handle()
            self.record('auth-command.json', command_receipt(result, command, include_output=False), cleanup=True)
            c.completed(result)
            cfg = self.private / 'credentials/cli.yaml'
            c.require(cfg.is_file() and not cfg.is_symlink() and cfg.stat().st_mode & 0o777 == 0o600,
                      'own native credential mode differs')
            self.credentials()
            receipt = read_json(self.private / 'auth-receipt.json', 65536)
            c.require(receipt.get('native_login_exit') == 0, 'own grant login unproved')
            self.record('auth.json', {key: receipt.get(key) for key in
                        ('at', 'native_login_exit', 'grant', 'configuration_mode', 'credentials_printed')})
            self.auth_completed = True
    def config_document(self,path,phase):
            try:
                import yaml  # Existing qualified dependency; no install or raw parser diagnostics.
                c.require(path.is_file() and not path.is_symlink() and path.stat().st_size<=65536
                          and path.stat().st_mode & 0o077==0,'not own bounded private config')
                doc=yaml.safe_load(path.read_text())
                c.require(isinstance(doc,dict) and isinstance(doc.get('api'),dict),'private config shape differs')
                api=doc['api']
                for key,value in api.items():
                    c.require(isinstance(key,str),'private config field name invalid')
                    if isinstance(value,str) and any(word in key.lower() for word in ('key','token','secret','password')) and value:
                        self.tokens.add(value)
                token=api.get('key') or api.get('token') or api.get('access_token')
                c.require(isinstance(token,str) and token and '\n' not in token and '\r' not in token,
                          'own access token unavailable')
                return doc
            except BaseException as error:
                raise PrivateConfigError(phase,type(error).__name__) from None
    def config_bytes(self,doc,phase):
            try:
                import yaml
                return yaml.safe_dump(doc).encode()
            except BaseException as error:
                raise PrivateConfigError(phase,type(error).__name__) from None
    def credentials(self):
            doc=self.config_document(self.private/'credentials/cli.yaml','credential-master-read')
            api=doc['api']
            return doc,api.get('key') or api.get('token') or api.get('access_token')
    def environment(self, client, config, host):
            prefix = 'BEX' if client == 'bex' else 'RENDER'
            env = {key: value for key, value in os.environ.items() if not key.startswith(('BEX_', 'RENDER_', 'QA_'))}
            env.update({'PATH': str(self.private/'bin') + os.pathsep + os.environ['PATH'], 'DO_NOT_TRACK': '1',
                        'BROWSER': '/usr/bin/true', 'BEX_NO_UPDATE_NOTIFIER': '1', prefix+'_HOST': host,
                        prefix+'_CLI_CONFIG_PATH': str(config), prefix+'_CLI_CONFIG_DIR': str(config.parent),
                        prefix+'_WORKSPACE': c.WORKSPACE, prefix+'_CLI_DISABLE_ANALYTICS': '1'})
            return env
    def native(self, client, args, host, cleanup=False, timeout=45):
            c.require(client in ('bex','render'),'fixed native role required')
            doc, _ = self.credentials()
            directory = self.private / ('config-' + client)
            directory.mkdir(mode=0o700, exist_ok=True)
            config = directory / 'cli.yaml'
            c.require(not config.exists(), 'per-call native config already exists')
            doc['api']['host'] = host
            self.budget.write(config.relative_to(self.root), self.config_bytes(doc,'native-'+client+'-config-write'), cleanup)
            command = [str(self.private / ('release/bex' if client == 'bex' else 'render-pin')), *args]
            result,fresh=None,None
            try:
                result = self.runner.capture(command, timeout=timeout, env=self.environment(client, config, host), cleanup=cleanup)
                fresh=self.config_document(config,'native-'+client+'-rotated-config-read')
                self.record('outputs/' + self.next_name('native') + '.json',
                            command_receipt(result, [client, *args], redact=self.tokens), cleanup)
                return result
            except BaseException as error:
                if result is not None:
                    # A partial/invalid rotated config may contain an unknown new
                    # token. Keep only counts/hashes/exit/group proof, never raw
                    # stdout/stderr or parser text when redaction cannot be proved.
                    summary=command_receipt(result,[client,*args],include_output=False)
                    summary['failure']=safe_error('native-'+client,error)
                    self.record('outputs/'+self.next_name('native-opaque-failure')+'.json',summary,cleanup=True)
                raise
            finally:
                if fresh is not None:
                    fresh['api']['host'] = API + '/v1/'
                    self.budget.write('private/credentials/cli.yaml', self.config_bytes(fresh,'native-'+client+'-rotation-persist'), cleanup=True)
                    config.unlink()
    def refresh(self, cleanup=False):
            doc, token = self.credentials()
            expiry = int(doc['api'].get('expires_at', 0))
            if expiry and expiry <= time.time() + 120:
                # Native client's own rotation/persistence, read-only workspace list.
                result = self.native('bex', ['workspaces', '-o', 'json'], API+'/v1/', cleanup)
                c.completed(result)
                doc, token = self.credentials()
                c.require(int(doc['api']['expires_at']) > time.time()+120, 'own native refresh unproved')
            return token
    def https(self, method, url, headers, body=None, cleanup=False, maximum=c.MAX_METADATA, on_response=None):
            parsed = urlsplit(url)
            c.require(parsed.scheme == 'https' and parsed.netloc == 'api.bex.co' and not parsed.fragment,
                      'not fixed API origin')
            stem = self.next_name('api')
            header_path, body_path = self.root/'scratch'/(stem+'.headers'), self.root/'scratch'/(stem+'.body')
            request_path = self.root/'scratch'/(stem+'.request')
            private_config = []
            for key, value in headers.items():
                c.require('\r' not in value and '\n' not in value and '\n' not in key, 'header newline refused')
                escaped = (key + ': ' + value).replace('\\', '\\\\').replace('"', '\\"')
                private_config.append('header = "' + escaped + '"')
            command = ['curl', '--disable', '--silent', '--show-error', '--connect-timeout', '5', '--max-time', '30',
                       '--max-filesize', str(maximum), '--proto', '=https', '--config', '-', '--request', method,
                       '--dump-header', str(header_path), '--output', str(body_path), '--write-out', '%{http_code}\n%{http_version}']
            if body is not None:
                c.require(len(body) <= 1024*1024, 'API request body1MiB bound')
                self.budget.write(request_path.relative_to(self.root), body, cleanup)
                command += ['--data-binary', '@'+str(request_path)]
            result = None
            try:
                if cleanup:
                    # Remote DELETE/absence reads still run if the observer or
                    # disk artifact budget failed. Their finite entities stay in
                    # bounded memory; failed receipt writes remain a final error.
                    marker=('\nM83-CURL-END-'+secrets.token_hex(16)+'\n').encode()
                    memory_command=['curl','--disable','--silent','--show-error','--connect-timeout','5','--max-time','30',
                        '--max-filesize',str(maximum),'--proto','=https','--config','-','--request',method,
                        '--include','--output','-','--write-out',marker.decode()+'%{http_code}\n%{http_version}']
                    c.require(body is None,'cleanup metadata transport has no request body')
                    result=self.runner.capture([*memory_command,url],timeout=35,input_bytes=('\n'.join(private_config)+'\n').encode(),
                                               limit=maximum+65536+256,cleanup=True)
                    entity_and_headers,separator,metadata_bytes=result['stdout'].rpartition(marker)
                    c.require(separator,'cleanup response metadata incomplete')
                    raw_headers,raw=split_included_response(entity_and_headers)
                    metadata=metadata_bytes.decode().splitlines()
                else:
                    self.budget.permit_external_write('scratch',maximum+65536)
                    result = self.runner.capture([*command, url], timeout=35, input_bytes=('\n'.join(private_config)+'\n').encode(), limit=65536)
                    raw = bounded_file(body_path,maximum)
                    raw_headers = bounded_file(header_path,65536)
                    metadata=result['stdout'].decode().splitlines()
                c.require(len(raw) <= maximum and len(raw_headers) <= 65536, 'API entity/header bound')
                try:
                    protocol, status, response_headers = parse_headers(raw_headers)
                except BaseException:
                    self.record('outputs/'+stem+'-unparsed.json', {
                        'method':method,'path':url.removeprefix(API),'raw_bytes':len(raw),'raw_sha256':c.sha(raw),
                        'raw_headers_sha256':c.sha(raw_headers),
                        **({'raw_entity_gzip_base64':base64.b64encode(gzip.compress(raw,mtime=0)).decode()} if on_response is not None else {}),
                        'transport':command_receipt(result,['curl','<fixed API request; auth stdin>'],include_output=False)},cleanup=True)
                    raise
                c.require(len(metadata) == 2 and metadata[0] == str(status) and metadata[1] in (protocol, protocol+'.0'), 'curl metadata/status mismatch')
                entity = c.gunzip_bounded(raw, maximum) if response_headers.get('content-encoding') == 'gzip' else raw
                # A real complete201 must be bound before any optional evidence
                # write can hit the stop guard. The callback persists in the8MiB
                # cleanup margin; its in-memory exact SID remains owned on failure.
                if on_response is not None and result['complete'] and result['exit_code'] == 0:
                    on_response(status, entity)
                event = {'at': now().isoformat(), 'request': {'method': method, 'path': url.removeprefix(API), 'authorization': '[REDACTED]'},
                         'status': status, 'actual_protocol': protocol, 'raw_bytes': len(raw), 'raw_sha256': c.sha(raw),
                         'decoded_bytes': len(entity), 'decoded_sha256': c.sha(entity),
                         'response_headers': {key: value for key, value in response_headers.items() if key in ('date', 'content-type', 'content-encoding', 'x-request-id', 'cf-ray')},
                         'transport': command_receipt(result, ['curl', '<fixed API request; auth stdin>'], include_output=False)}
                if on_response is not None:
                    event['complete_raw_entity_gzip_base64'] = base64.b64encode(gzip.compress(raw,mtime=0)).decode()
                self.record('outputs/'+stem+'.json', event, cleanup or on_response is not None)
                return {'status': status, 'headers': response_headers, 'raw': raw, 'body': entity, 'transport': result, 'event': event}
            except BaseException:
                if result is not None and on_response is not None:
                    # Preserve bounded partial/complete native response bytes even
                    # when protocol parsing, binding or an artifact write failed.
                    try:
                        raw=bounded_file(body_path,maximum);raw_headers=bounded_file(header_path,65536)
                        self.record('outputs/'+stem+'-native-failure.json',{'raw_bytes':len(raw),'raw_sha256':c.sha(raw),
                            'raw_headers_sha256':c.sha(raw_headers),'raw_entity_gzip_base64':base64.b64encode(gzip.compress(raw,mtime=0)).decode(),
                            'transport':command_receipt(result,['curl','<fixed native POST; auth stdin>'],include_output=False)},cleanup=True)
                    except BaseException as error:self.errors.append('native failure retention:'+type(error).__name__)
                raise
            finally:
                for path in (request_path, header_path, body_path):
                    path.unlink(missing_ok=True)
    def api(self, method, path, body=None, cleanup=False):
            c.require(self.allowed_api(method, path, body, cleanup), 'API request outside fixed own scope')
            token = self.refresh(cleanup)
            headers = {'Authorization': 'Bearer '+token, 'User-Agent': 'render-cli/2.29.0 (macOS - 26.0.1)',
                       'X-Bex-Workspace': c.WORKSPACE, 'Accept': 'application/json, text/event-stream',
                       'MCP-Protocol-Version': '2025-03-26'}
            if body is not None:
                headers['Content-Type'] = 'application/json'
            result = self.https(method, API+path, headers, encoded(body) if body is not None else None, cleanup)
            c.completed(result['transport'])
            if method=='GET' and (path.startswith('/v1/services?') or path.startswith('/v1/env-groups?')
                or path.startswith('/v1/projects?') or path.startswith('/v1/blueprints?') or path.endswith('/limits')):
                c.require(not any(token.encode() in result['raw'] or token.encode() in result['body'] for token in self.tokens if token),
                          'credential unexpectedly present in metadata; do not retain raw')
                self.record('outputs/'+self.next_name('raw-metadata')+'.json',{'observation':result['event'],
                    'content_encoding':result['headers'].get('content-encoding','identity'),
                    'raw_entity_gzip_base64':base64.b64encode(gzip.compress(result['raw'],mtime=0)).decode(),
                    'decoded_entity_gzip_base64':base64.b64encode(gzip.compress(result['body'],mtime=0)).decode()},cleanup)
            return result
    def get_json(self, path, cleanup=False):
            result = self.api('GET', path, cleanup=cleanup)
            c.require(result['status'] == 200, 'metadata GET not200:' + path)
            self.last_metadata_event = result['event']
            return decode_json(result['body'])
    def collect(self, cleanup=False):
            ids, names, cursors = set(), set(), set()
            cursor = None
            for _ in range(10):
                query = {'ownerId': c.WORKSPACE, 'limit': 100}
                if cursor is not None:query['cursor'] = cursor
                rows = self.get_json('/v1/services?' + urlencode(query), cleanup)
                c.require(isinstance(rows, list) and len(rows) <= 100, 'complete bounded services page required')
                if not rows:break
                for item in rows:
                    view = item['service']; sid = view['id']
                    c.require(c.SERVICE_ID.fullmatch(sid) and view['ownerId'] == c.WORKSPACE and sid not in ids,
                              'baseline foreign/repeated service identity')
                    ids.add(sid); names.add(view['name'])
                cursor = rows[-1]['cursor']
                c.require(isinstance(cursor, str) and cursor and len(cursor) <= 4096 and cursor not in cursors, 'unstable service cursor')
                cursors.add(cursor)
            else:
                raise Stop('service pagination did not terminate within10pages')
            limits = self.get_json('/v1/owners/' + c.WORKSPACE + '/limits', cleanup)
            limits_event = self.last_metadata_event
            self.record(self.next_name('raw-limits')+'.json', {'observation':limits_event,'raw_decoded_JSON':limits},cleanup)
            metadata = {}
            for path, wrapper in [('/v1/env-groups', 'envGroup'), ('/v1/projects', 'project'), ('/v1/blueprints', 'blueprint')]:
                rows = self.get_json(path+'?'+urlencode({'ownerId': c.WORKSPACE, 'limit':100}), cleanup)
                c.require(isinstance(rows, list) and len(rows) < 100, 'metadata list completeness unproved')
                projected = []
                for item in rows:
                    view = item.get(wrapper, item)
                    c.require(isinstance(view.get('id'), str), 'metadata identity missing')
                    projected.append(view['id'])
                metadata[path] = sorted(projected)
            return {'workspace': c.WORKSPACE, 'at': now().isoformat(), 'complete': True,
                    'ids': sorted(ids), 'names': sorted(names), 'limits': limits,
                    'limits_observed_at':limits_event['at'],'limits_observation':limits_event,'metadata': metadata}
    def free_gate(self):
            owner = self.get_json('/v1/owners/'+c.WORKSPACE)
            owner_event = self.last_metadata_event
            usage = self.get_json('/v1/usage?'+urlencode({'ownerId':c.WORKSPACE}))
            usage_event = self.last_metadata_event
            limits = self.get_json('/v1/owners/'+c.WORKSPACE+'/limits')
            limits_event = self.last_metadata_event
            observations = {'owner':owner_event,'usage':usage_event,'limits':limits_event}
            inputs_name = self.next_name('free-gate-inputs')+'.json'
            self.record(inputs_name, {'at':now().isoformat(),
                        'owner':{key:owner.get(key) for key in ('id','name')} if isinstance(owner,dict) else None,
                        'usage':{key:usage.get(key) for key in ('workspaceId','period','freeInstanceHours','coverage')} if isinstance(usage,dict) else None,
                        'limits':{'services':limits.get('services')} if isinstance(limits,dict) else None,
                        'coverage_scope':'diagnostic only; no aggregate-health or Free allowance claim',
                        'observations':observations})
            try:
                result = c.gate_free(owner, usage, limits, now(), observations)
            except c.Unproved as error:
                # The frozen Free predicates emit static reasons only.
                self.record(self.next_name('free-gate-refusal')+'.json',
                            {'state':'REFUSED','inputs':inputs_name,'reason':str(error)})
                raise
            self.record(self.next_name('free-gate')+'.json', {**result, 'owner': {key:owner[key] for key in ('id','name')},
                        'usage': {**{key:usage[key] for key in ('workspaceId','period','freeInstanceHours')},
                                  'coverage':usage.get('coverage')}, 'limits':limits})
            return result
    def deployments(self, row, cleanup=False):
            items = self.get_json('/v1/services/'+row['id']+'/deploys?limit=20', cleanup)
            c.require(isinstance(items, list) and len(items) < 20, 'complete first-deploy list unproved')
            rows = [item.get('deploy', item) for item in items]
            self.journal.record_children(row['id'], rows)
            self.record(self.next_name('deployments')+'.json', {'id':row['id'],'rows':rows}, cleanup)
            return rows
    def logout(self):
            self.register_session_handle()
            c.require(all(row.get('state') in ('deleted','not-created') for row in self.data['resources']),
                      'retain own auth for unknown/remaining resource reconciliation')
            if not self.auth_started:return
            handle=read_json(self.private/'qa-session-handle.json',65536) if (self.private/'qa-session-handle.json').exists() else None
            had_native_config=(self.private/'credentials/cli.yaml').exists()
            fallback_done=False
            if self.auth_completed and not had_native_config:
                self.errors.append('own successful OAuth grant config missing before logout; revocation unproved')
            # If opaque bootstrap was interrupted before returning its logout
            # handle, the exact own state still permits session revocation. Do not
            # discover/delete unknown global jars or claim their absence.
            if (handle is None or not Path(handle['logout_jar']).exists()) and (self.private/'qa-state.json').exists():
                command=['bash',str(REPO/'scripts/qa-login.sh'),'--logout',str(self.private/'qa-state.json')]
                fallback=self.runner.capture(command,timeout=90,limit=65536,cleanup=True)
                self.record('logout-own-state-fallback.json',command_receipt(fallback,command,include_output=False),cleanup=True)
                c.completed(fallback)
                fallback_done=True
                if handle is None:
                    self.errors.append('opaque bootstrap returned no global jar handle; own session revoked from exact own state, jar-file absence unproved')
            created=[{'id':row['id'],'state':row['state']} for row in self.data['resources'] if row.get('id') and row.get('HTTP')==201]
            try:self.budget.json('private/ledger.json',{'workspace':c.WORKSPACE,'resources':created},cleanup=True)
            except BaseException as error:self.errors.append('opaque logout count projection:'+type(error).__name__)
            command=[sys.executable,str(self.private/'cleanup-auth.py')]
            result=self.runner.capture(command,timeout=180,limit=262144,cleanup=True)
            self.record('logout-command.json',command_receipt(result,command,include_output=False),cleanup=True)
            c.completed(result)
            receipt=read_json(self.private/'auth-cleanup-receipt.json',65536)
            c.require(receipt.get('own_credential_files_removed') is True,'own logout/file removal unproved')
            if type(receipt.get('resources_created_by_run')) is int:
                c.require(receipt['resources_created_by_run']==len(created),'opaque logout resource-count projection differs')
            if had_native_config:c.require(type(receipt.get('native_logout_exit')) is int and receipt['native_logout_exit']==0,
                                           'own OAuth logout outcome unproved')
            if handle or fallback_done:
                c.require(fallback_done or (type(receipt.get('Kratos_logout_exit')) is int and receipt['Kratos_logout_exit']==0),
                          'own Kratos logout outcome unproved')
            for name in ('credentials/cli.yaml','device-cookies.jar','qa-state.json','device-state.json',
                         'config-bex/cli.yaml','config-render/cli.yaml','qa-session-handle.json'):
                c.require(not (self.private/name).exists(),'own credential/state remains:'+name)
            if handle:c.require(not Path(handle['logout_jar']).exists(),'exact own logout jar remains')
            self.record('logout.json',receipt,cleanup=True)


class NativeRelay:
    def __init__(self,transport,spec,row):
        self.transport,self.spec,self.row=transport,spec,row
        self.active=threading.Event();self.active.set()
        self.requests_done=threading.Event();self.requests_done.set();self.request_count=0
        self.lock=threading.RLock();self.closed=False;self.listener_closed=False
        self.server=None;self.thread=None;self.start_attempted=False
        self.stop_event=threading.Event()

    def serve(self):
        # A stop flag is valid before, during and after Thread.start. Unlike
        # BaseServer.shutdown, it cannot wait on a never-entered serve_forever.
        try:
            while not self.stop_event.is_set():
                self.server.handle_request()
        except BaseException as error:
            if not self.stop_event.is_set():
                self.transport.errors.append(safe_error('native-relay-listener',error))

    def dispatch(self,method,path,raw,headers):
        t=self.transport
        with self.lock:
            if method!='POST' or path!='/v1/services' or self.closed:
                return 403,{'content-type':'application/json'},b'{"message":"outside exact native create scope"}'
            t.budget.check()
            body=decode_json(raw);t.journal.capture_wire(self.row,raw,headers)
            t.journal.forward_once(self.row,body,self.spec)
            self.active.clear()
        try:
            forwarded={key:value for key,value in headers.items() if key.lower() not in HOP|{'host'}}
            c.require(forwarded.get('Authorization',forwarded.get('authorization','')).startswith('Bearer '),'native bearer missing')
            def bind(status,entity):
                if status==201:
                    t.journal.bind201(self.row,201,decode_json(entity))
                elif 400<=status<500:
                    self.row['state']='not-created';self.row['HTTP']=status;t.persist(t.data)
            result=t.https('POST',API+path,forwarded,raw,on_response=bind)
            c.completed(result['transport'])
            t.record(t.next_name('native-POST')+'.json',{'request':body,'status':result['status'],
                     'raw_response_bytes':len(result['raw']),'raw_response_sha256':c.sha(result['raw']),
                     'complete_raw_response_gzip_base64':base64.b64encode(gzip.compress(result['raw'],mtime=0)).decode()},cleanup=True)
            return result['status'],result['headers'],result['raw']
        finally:self.active.set()

    def __enter__(self):
        relay=self
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self,*args):pass
            def handle_fixed(self):
                with relay.lock:
                    relay.request_count+=1;relay.requests_done.clear()
                try:
                    try:
                        length=int(self.headers.get('Content-Length','0'))
                        if not 0<=length<=1024*1024:raise Stop('native request1MiB bound')
                        self.connection.settimeout(10)
                        raw=self.rfile.read(length)
                        c.require(len(raw)==length,'native request incomplete')
                        status,headers,body=relay.dispatch(self.command,self.path,raw,dict(self.headers))
                    except BaseException as error:
                        relay.transport.errors.append('native relay:'+type(error).__name__)
                        status,headers,body=502,{'content-type':'application/json'},b'{"message":"native create uncertain or refused; do not retry"}'
                    self.send_response(status)
                    for key,value in headers.items():
                        if key.lower() not in HOP:self.send_header(key,value)
                    self.send_header('Content-Length',str(len(body)));self.end_headers()
                    try:self.wfile.write(body)
                    except (BrokenPipeError,ConnectionResetError):pass
                finally:
                    with relay.lock:
                        relay.request_count-=1
                        if relay.request_count==0:relay.requests_done.set()
            do_POST=handle_fixed;do_GET=handle_fixed;do_DELETE=handle_fixed;do_PATCH=handle_fixed;do_PUT=handle_fixed
        # Register the owner before constructor/startup can fail. A partial
        # __enter__ never relies on __exit__ being invoked by the with statement.
        self.transport.active_relays.append(self)
        try:
            self.server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
            self.server.daemon_threads=True;self.server.timeout=.2
            self.thread=threading.Thread(target=self.serve,name='m83-native-relay',daemon=True)
            self.start_attempted=True;self.thread.start()
            self.host='http://127.0.0.1:'+str(self.server.server_port)+'/v1/'
            return self
        except BaseException:
            try:self.close()
            except BaseException as cleanup_error:
                self.transport.errors.append(safe_error('native-relay-partial-enter-cleanup',cleanup_error))
            raise

    def close(self):
        self.closed=True
        self.stop_event.set()
        if self.server is not None and not self.listener_closed:
            self.server.server_close();self.listener_closed=True
        thread_started=self.thread is not None and getattr(self.thread,'ident',None) is not None
        if thread_started:self.thread.join(timeout=2)
        thread_alive=self.thread is not None and getattr(self.thread,'is_alive',lambda:False)()
        c.require(not thread_alive and self.active.wait(40) and self.requests_done.wait(12),
                  'native relay/upstream/handler still active; remote outcome unknown')
        if self in self.transport.active_relays:self.transport.active_relays.remove(self)

    def __exit__(self,*args):self.close()
