"""Exact reused parser/listener regressions with private new fake provider only."""
import copy,json,tempfile,threading,unittest,types
from pathlib import Path
from unittest.mock import patch
import core as c
import opaque as t
import test_transport as fixtures
t.Transport=t.OpaqueTransport

def evidence_bytes(root):return b''.join(p.read_bytes() for p in (root/'evidence').rglob('*') if p.is_file())
fixtures.FakeTransport=fixtures.FakeAdapter
fixtures.completed=fixtures.f.complete
class ParserRegressionControls(unittest.TestCase):
    def test_actual_partial_master_YAML_error_never_copies_control_token_into_RESULT(self):
        import yaml
        token='CONTROL_ONLY_YAML_TOKEN_42'
        raw='api:\n  key: '+token+': [\n'
        with self.assertRaises(yaml.YAMLError) as original:
            yaml.safe_load(raw)
        self.assertIn(token,str(original.exception))  # Actual PyYAML reproduction, not a synthetic error class.
        with tempfile.TemporaryDirectory(prefix='m83-v2-parser-control-') as directory:
            root=Path(directory).resolve();owned=fixtures.FakeTransport(root)
            config=root/'private/credentials/cli.yaml';config.parent.mkdir(parents=True)
            config.write_text(raw);config.chmod(0o600)
            owned.auth=lambda:t.Transport.credentials(owned)
            report=owned.execute()
            self.assertFalse(report['PASS'])
            self.assertEqual(report['failure'],'credential-master-read:ScannerError')
            self.assertNotIn(token,json.dumps(report))
            self.assertNotIn(token.encode(),evidence_bytes(root))
            self.assertEqual(config.read_text(),raw)  # No unqualified overwrite of the own recovery file.
            self.assertEqual(owned.tokens,set())

    def test_partial_rotated_config_keeps_only_output_hashes_and_preserves_valid_master(self):
        import yaml
        old='CONTROL_ONLY_OLD';rotated='CONTROL_ONLY_UNKNOWN_ROTATION'
        with tempfile.TemporaryDirectory(prefix='m83-v2-rotation-control-') as directory:
            root=Path(directory).resolve();owned=fixtures.FakeTransport(root)
            master=root/'private/credentials/cli.yaml';master.parent.mkdir(parents=True)
            master.write_text(yaml.safe_dump({'api':{'key':old,'host':t.API+'/v1/'}}));master.chmod(0o600)
            class Native:
                def capture(self,argv,env,**kwargs):
                    config=Path(env['RENDER_CLI_CONFIG_PATH'])
                    config.write_text('api:\n  key: '+rotated+': [\n');config.chmod(0o600)
                    result=fixtures.completed((old+' '+rotated).encode())
                    result['stderr']=(rotated+' parser fragment').encode()
                    return result
            owned.runner=Native()
            with self.assertRaises(t.PrivateConfigError) as caught:
                t.Transport.native(owned,'render',['services','create'],'http://127.0.0.1:1/v1/')
            self.assertEqual(str(caught.exception),'native-render-rotated-config-read:ScannerError')
            self.assertNotIn(rotated.encode(),evidence_bytes(root))
            self.assertNotIn(old.encode(),evidence_bytes(root))
            retained=list((root/'evidence/outputs').glob('native-opaque-failure-*.json'))
            self.assertEqual(len(retained),1)
            receipt=json.loads(retained[0].read_text())
            self.assertNotIn('stdout',receipt);self.assertNotIn('stderr',receipt)
            self.assertIn('stdout_sha256',receipt);self.assertIn('stderr_sha256',receipt)
            self.assertEqual(yaml.safe_load(master.read_text())['api']['key'],old)
            self.assertTrue((root/'private/config-render/cli.yaml').exists())

    def test_cleanup_parser_exception_is_static_phase_class_and_still_nonpassing(self):
        import yaml
        token='CONTROL_ONLY_CLEANUP_TOKEN'
        with tempfile.TemporaryDirectory(prefix='m83-v2-cleanup-parser-') as directory:
            root=Path(directory).resolve();owned=fixtures.FakeTransport(root)
            def bad_logout():yaml.safe_load('api:\n  key: '+token+': [\n')
            owned.logout=bad_logout
            with patch.object(t,'NativeRelay',fixtures.FakeRelay):report=owned.execute()
            self.assertFalse(report['PASS'])
            self.assertIn('own-auth-cleanup:ScannerError',report['cleanup_errors'])
            self.assertNotIn(token,json.dumps(report))
            self.assertNotIn(token.encode(),evidence_bytes(root))
            self.assertTrue(all(not live for live in owned.remote.values()))

    def test_complete_rotation_redacts_new_access_refresh_and_secret_values_before_logging(self):
        import yaml
        values={'key':'CONTROL_ONLY_ACCESS_NEW','refresh_token':'CONTROL_ONLY_REFRESH_NEW','client_secret':'CONTROL_ONLY_SECRET_NEW'}
        with tempfile.TemporaryDirectory(prefix='m83-v2-valid-rotation-') as directory:
            root=Path(directory).resolve();owned=fixtures.FakeTransport(root)
            master=root/'private/credentials/cli.yaml';master.parent.mkdir(parents=True)
            master.write_text(yaml.safe_dump({'api':{'key':'CONTROL_ONLY_ACCESS_OLD','host':t.API+'/v1/'}}));master.chmod(0o600)
            class Native:
                def capture(self,argv,env,**kwargs):
                    config=Path(env['BEX_CLI_CONFIG_PATH'])
                    config.write_text(yaml.safe_dump({'api':{**values,'host':env['BEX_HOST']}}));config.chmod(0o600)
                    return fixtures.completed((' '.join(values.values())+' CONTROL_ONLY_ACCESS_OLD').encode())
            owned.runner=Native()
            result=t.Transport.native(owned,'bex',['workspaces','-o','json'],t.API+'/v1/')
            self.assertEqual(result['exit_code'],0)
            for value in list(values.values())+['CONTROL_ONLY_ACCESS_OLD']:
                self.assertNotIn(value.encode(),evidence_bytes(root))
            restored=yaml.safe_load(master.read_text())['api']
            self.assertEqual({key:restored[key] for key in values},values)

class ListenerRegressionControls(unittest.TestCase):
    def owner(self,root):return fixtures.FakeTransport(root)

    def server_class(self,owned,events,fail_close=False):
        class Server:
            server_port=1
            def __init__(self,*args,**kwargs):
                if not owned.active_relays:raise AssertionError('listener not registered before allocation')
                events.append('registered-before-allocation');events.append('allocated-listener')
                self.stopped=threading.Event();self.close_calls=0
            def handle_request(self):self.stopped.wait(.01)
            def shutdown(self):raise AssertionError('shutdown must never run in partial startup')
            def server_close(self):
                self.close_calls+=1;events.append('server-close')
                if fail_close and self.close_calls==1:raise OSError('CONTROL_ONLY first close failure')
                self.stopped.set()
        return Server

    def test_thread_constructor_and_before_start_failures_close_without_shutdown_or_unstarted_join(self):
        for phase in ('constructor','start'):
            with self.subTest(phase=phase),tempfile.TemporaryDirectory(prefix='m83-v2-listener-') as directory:
                root=Path(directory).resolve();owned=self.owner(root);events=[]
                class Thread:
                    ident=None
                    def __init__(self,*args,**kwargs):
                        if phase=='constructor':raise RuntimeError('CONTROL_ONLY thread construction')
                    def start(self):raise RuntimeError('CONTROL_ONLY before start')
                    def is_alive(self):return False
                    def join(self,*args,**kwargs):raise AssertionError('never-started thread joined')
                relay=t.NativeRelay(owned,{}, {})
                with patch.object(t.http.server,'ThreadingHTTPServer',self.server_class(owned,events)),patch.object(t.threading,'Thread',Thread):
                    with self.assertRaises(RuntimeError):relay.__enter__()
                self.assertTrue(relay.listener_closed)
                self.assertNotIn(relay,owned.active_relays)
                self.assertEqual(events,['registered-before-allocation','allocated-listener','server-close'])

    def test_real_owned_thread_start_then_error_or_signal_is_stopped_and_joined(self):
        RealThread=threading.Thread
        for failure in (RuntimeError,KeyboardInterrupt):
            with self.subTest(failure=failure.__name__),tempfile.TemporaryDirectory(prefix='m83-v2-started-thread-') as directory:
                root=Path(directory).resolve();owned=self.owner(root);events=[]
                class StartedThread(RealThread):
                    def start(self):
                        super().start()
                        raise failure('CONTROL_ONLY after actual thread start')
                relay=t.NativeRelay(owned,{}, {})
                with patch.object(t.http.server,'ThreadingHTTPServer',self.server_class(owned,events)),patch.object(t.threading,'Thread',StartedThread):
                    with self.assertRaises(failure):relay.__enter__()
                self.assertTrue(relay.listener_closed)
                self.assertIsNotNone(relay.thread.ident)
                self.assertFalse(relay.thread.is_alive())
                self.assertNotIn(relay,owned.active_relays)
                self.assertEqual(events.count('server-close'),1)

    def test_failed_partial_close_retains_exact_owner_for_bounded_final_retry(self):
        with tempfile.TemporaryDirectory(prefix='m83-v2-retry-close-') as directory:
            root=Path(directory).resolve();owned=self.owner(root);events=[]
            class Thread:
                ident=None
                def __init__(self,*args,**kwargs):pass
                def start(self):raise RuntimeError('CONTROL_ONLY no start')
                def is_alive(self):return False
            relay=t.NativeRelay(owned,{}, {})
            with patch.object(t.http.server,'ThreadingHTTPServer',self.server_class(owned,events,fail_close=True)),patch.object(t.threading,'Thread',Thread):
                with self.assertRaises(RuntimeError):relay.__enter__()
            self.assertIn(relay,owned.active_relays)
            self.assertFalse(relay.listener_closed)
            relay.close()
            self.assertTrue(relay.listener_closed)
            self.assertNotIn(relay,owned.active_relays)
            self.assertEqual(events.count('server-close'),2)
