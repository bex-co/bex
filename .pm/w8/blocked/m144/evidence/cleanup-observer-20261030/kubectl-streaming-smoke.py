import importlib.util
import json
import os
from pathlib import Path
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

spec = importlib.util.spec_from_file_location('fixture', 'scripts/disk_fixture.py')
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)
stop = threading.Event()
seen = []
class API(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass
    def do_GET(self):
        seen.append(self.path)
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        events = [
            {'type': 'BOOKMARK', 'object': {'metadata': {'resourceVersion': '101', 'annotations': {'k8s.io/initial-events-end': 'true'}}}},
            {'type': 'MODIFIED', 'object': {'metadata': {'name': 'qa-owned', 'uid': 'own-uid', 'resourceVersion': '102', 'labels': {'bex.co/app-id': 'srv-owned', 'app.bex.co/workspace': 'tea-owned'}}, 'spec': {'disk': {'sizeGB': 1}, 'env': {'PRIVATE_CANARY': 'never-retained'}}}},
        ]
        for event in events:
            self.wfile.write((json.dumps(event) + '\n').encode())
            self.wfile.flush()
        stop.wait(5)
server = ThreadingHTTPServer(('127.0.0.1', 0), API)
threading.Thread(target=server.serve_forever, daemon=True).start()
previous = os.environ.get('KUBECONFIG')
try:
    with tempfile.TemporaryDirectory() as directory:
        config = Path(directory) / 'config'
        config.write_text(json.dumps({'apiVersion': 'v1', 'kind': 'Config', 'clusters': [{'name': 'stub', 'cluster': {'server': 'http://127.0.0.1:' + str(server.server_port)}}], 'contexts': [{'name': 'stub', 'context': {'cluster': 'stub', 'user': 'stub'}}], 'users': [{'name': 'stub', 'user': {}}], 'current-context': 'stub'}))
        os.environ['KUBECONFIG'] = str(config)
        watch = fixture.CleanupWatch('/apis/app.bex.co/v1alpha1/namespaces/tea-owned/apps', 'qa-owned', 'cleanup_app', '100', [])
        try:
            rows = watch.drain()
            if not rows:
                threading.Event().wait(0.1)
                rows = watch.drain()
            assert watch.version == '101' and rows[0]['resourceVersion'] == '102'
            assert 'PRIVATE_CANARY' not in json.dumps(rows)
            assert 'sendInitialEvents=true' in seen[0]
        finally:
            watch.close()
        assert not watch.thread.is_alive() and watch.process.poll() is not None
        print('PASS: installed kubectl streams the localhost bookmark/event and cancellation reaps its child')
finally:
    if previous is None:
        os.environ.pop('KUBECONFIG', None)
    else:
        os.environ['KUBECONFIG'] = previous
    stop.set()
    server.shutdown()
    server.server_close()
