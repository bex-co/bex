"""Offline HTTP-oracle controls; these do not execute a new supplier image."""
import base64
import copy
import gzip
import importlib.util
import json
import lzma
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import threading
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('original_replay', ROOT / 'original-cases.py')
replay = importlib.util.module_from_spec(spec)
spec.loader.exec_module(replay)
CORPUS = json.loads(gzip.decompress((ROOT / 'recorded-node-http.json.gz').read_bytes()))


class OriginalNodeHTTPControls(unittest.TestCase):
    def exercise(self, case, corrupt=False, corrupt_before=False):
        x = copy.deepcopy(next(x for x in replay.selected_cases('node') if x['mid'] == case['mid']))
        frames = {r['path']: copy.deepcopy(r) for r in case['frames']}
        if corrupt or corrupt_before:
            first = next(iter(frames.values()))
            body = json.loads(base64.b64decode(first['body_base64']))
            if corrupt:
                body['runtime']['marker'] = 'foreign-case-with-HTTP200'
            else:
                # The historical missing-supplier oracle stays intact; it must
                # not be substituted for the healthy supplied-stage callback.
                body['build']['before']['tools']['pandoc'].update(status=None, error='ENOENT')
            first['body_base64'] = base64.b64encode(json.dumps(body).encode()).decode()

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                frame = frames[self.path]
                data = base64.b64decode(frame['body_base64'])
                self.send_response(200)
                self.send_header('Content-Type', frame['content_type'])
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.01}, daemon=True)
        thread.start()
        x['create_view'] = {'serviceDetails': {'url': f'http://127.0.0.1:{server.server_port}'}}
        try:
            return replay.exercise_http(x, replay.validation(x), ROOT / 'cases' / f"m{x['mid']}")
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=2)
            self.assertFalse(thread.is_alive())

    def test_all_six_original_node_oracles_accept_retained_valid_bodies(self):
        self.assertEqual({x['mid'] for x in CORPUS['cases']}, {91, 93, 98, 99, 101, 107})
        for case in CORPUS['cases']:
            with self.subTest(mid=case['mid']):
                results = self.exercise(case)
                self.assertEqual(len(results), 3)
                self.assertTrue(all(x['pass'] for x in results))

    def test_HTTP200_does_not_accept_foreign_runtime_identity(self):
        for case in CORPUS['cases']:
            with self.subTest(mid=case['mid']):
                try:
                    results = self.exercise(case, corrupt=True)
                except AssertionError:
                    continue
                self.assertFalse(all(x['pass'] for x in results))

    def test_pandoc_positive_callback_requires_a_healthy_before_probe(self):
        case = next(x for x in CORPUS['cases'] if x['mid'] == 93)
        results = self.exercise(case, corrupt_before=True)
        self.assertFalse(all(x['pass'] for x in results))

    def test_unknown_primary_never_runs_node_cases(self):
        with self.assertRaises(AssertionError):
            replay.selected_cases('ruby')

    def test_prepared_marker_is_accepted_and_authored_program_tampering_rejected(self):
        x = next(x for x in replay.selected_cases('node') if x['mid'] == 93)
        command = x['commands']['initial']['build']
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            for name in x['source_file_identities']:
                match = re.search(r"cat > " + re.escape(name) + r" <<'([^']+)'\n", command)
                self.assertIsNotNone(match, name)
                body = command[match.end():].split('\n' + match[1] + '\n', 1)[0] + '\n'
                (work / name).write_text(body)
            replay.verify_authored_files(x, work)
            program = work / 'qa407-probe.cjs'
            program.write_bytes(program.read_bytes() + b'\n// modified\n')
            with self.assertRaisesRegex(AssertionError, 'authored source changed'):
                replay.verify_authored_files(x, work)

    def test_HTTP200_marker_cannot_borrow_another_prepared_case(self):
        x = next(x for x in replay.selected_cases('node') if x['mid'] == 93)
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            marker = 'qa407-marker.json'
            (work / marker).write_text(json.dumps('foreign-marker'))
            with self.assertRaisesRegex(AssertionError, 'wrong prepared marker'):
                replay.verify_authored_files(dict(x, source_file_identities={marker: x['source_file_identities'][marker]}), work)

    def test_original_packaging_compressed_marker_is_preserved(self):
        x = next(x for x in replay.selected_cases('node') if x['mid'] == 103)
        encoded = re.search(r"base64.b64decode\('([^']+)'\)", x['commands']['initial']['build'])[1]
        authored = json.loads(lzma.decompress(base64.b64decode(encoded)))
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            marker = 'qm.json'
            (work / marker).write_text(authored[marker]['text'])
            replay.verify_authored_files(dict(x, source_file_identities={marker: x['source_file_identities'][marker]}), work)
            (work / marker).write_text(json.dumps('foreign-packaging-marker'))
            with self.assertRaisesRegex(AssertionError, 'wrong prepared marker'):
                replay.verify_authored_files(dict(x, source_file_identities={marker: x['source_file_identities'][marker]}), work)


if __name__ == '__main__':
    unittest.main()
