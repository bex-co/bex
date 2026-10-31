"""Real owned-loopback transport controls; no image/runtime/Trivy approval."""
import contextlib
import http.client
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

from test_native_oci_stage import Registry as SourceRegistry, descriptor, digest, encoded, fixture

SPEC = importlib.util.spec_from_file_location("native_oci_mirror", Path(__file__).with_name("native-oci-mirror.py"))
mirror = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(mirror)
stage = mirror.stage
REPORT = b'{"CONTROL_ONLY":true,"allFindings":["unfixed-critical","medium"],"unknown":{"keep":[1,2,3]}}\n'

TRIVY = r'''#!/usr/bin/env python3
import hashlib,http.client,json,os,pathlib,sys,time
root=pathlib.Path(os.environ['CONTROL_ROOT']);args=sys.argv[1:]
if '--download-db-only' in args or '--download-java-db-only' in args:
 with (root/'db-calls').open('a') as log:log.write(json.dumps(args)+'\n')
 sys.exit(0)
assert args[0]=='--config'
config=pathlib.Path(args[1]);settings=json.loads(config.read_text());host=settings['registry']['mirrors']['ghcr.io'][0]
assert settings=={'registry':{'mirrors':{'ghcr.io':[host]}}} and host.startswith('127.0.0.1:')
assert config.parent.stat().st_mode&0o077==0
ref=args[3];assert args[2]=='image' and ref==os.environ['CONTROL_REF']
assert args[4:]==['--image-src','remote','--parallel','1','--max-image-size','8589934592B']
(root/'invoked.json').write_text(json.dumps({'argv':args,'config':str(config),'pid':os.getpid(),'host':host,'platform':os.environ['TRIVY_PLATFORM']}))
with (root/'scan-calls.jsonl').open('a') as log:log.write(json.dumps(args)+'\n')
def get(kind,key):
 c=http.client.HTTPConnection(host,timeout=2);c.request('GET','/v2/bex-co/bex-cnb-run/'+kind+'/'+key);r=c.getresponse();data=r.read();c.close()
 assert r.status==200 and r.getheader('Docker-Content-Digest')==key
 assert 'sha256:'+hashlib.sha256(data).hexdigest()==key
 return data
mode=os.environ.get('CONTROL_MODE','good')
if mode=='fallback':
 c=http.client.HTTPConnection(host,timeout=2);c.request('GET','/v2/bex-co/bex-cnb-run/manifests/sha256:'+'f'*64);r=c.getresponse();r.read();c.close();assert r.status==404
elif mode!='bypass':
 root_doc=json.loads(get('manifests',ref.split('@')[1]))
 selected=next(d for d in root_doc['manifests'] if d['platform']['architecture']==os.environ['TRIVY_PLATFORM'].split('/')[1])
 manifest=json.loads(get('manifests',selected['digest']));get('blobs',manifest['config']['digest'])
 for layer in manifest['layers']:get('blobs',layer['digest'])
if mode=='hang':
 (root/'ready').touch()
 while True:time.sleep(1)
output=pathlib.Path(os.environ['TRIVY_OUTPUT']);output.write_bytes(bytes.fromhex(os.environ['CONTROL_REPORT']))
if mode=='fail':output.write_text('{"partial":');sys.exit(17)
'''


class MirrorFixture(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="native-oci-mirror-controls-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.data = fixture()
        # Preserve a genuine two-platform root, with only the selected graph
        # staged. The projected index.json has another SHA and is never served.
        other = fixture("arm64", index_architecture="arm64")
        root = json.loads(self.data["root"])
        root["manifests"].append(json.loads(other["root"])["manifests"][0])
        self.data["root"] = encoded(root)
        self.data.update(other_manifest=other["manifest"], other_config=other["config"])
        self.ref = stage.REPOSITORY + "@" + digest(self.data["root"])
        self.layout, self.stage_proof = self.root / "layout", self.root / "stage.json"
        stage.Stager(self.ref, "linux/amd64", self.layout, self.stage_proof, SourceRegistry(self.data), sleep=lambda _: None).run()
        self.proof = self.root / "mirror.json"
        self.output = self.root / "report.json"
        self.work = self.root / "work"
        self.work.mkdir()
        self.tool = self.root / "trivy"
        self.tool.write_text(TRIVY)
        self.tool.chmod(0o700)
        self.env = dict(os.environ, CONTROL_ROOT=str(self.root), CONTROL_REF=self.ref,
                        CONTROL_REPORT=REPORT.hex(), TRIVY_OUTPUT=str(self.output),
                        TRIVY_PLATFORM="linux/amd64", TMPDIR=str(self.work))
        self.command = [str(self.tool), "image", self.ref, "--image-src", "remote", "--parallel", "1", "--max-image-size", "8589934592B"]

    def verified(self):
        return mirror.VerifiedLayout(self.ref, "linux/amd64", self.layout, self.stage_proof)

    @contextlib.contextmanager
    def server(self):
        server = mirror.Registry(self.verified())
        thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
        thread.start()
        try:
            yield server
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=1)

    def request(self, server, method, path):
        connection = http.client.HTTPConnection("127.0.0.1", server.server_port, timeout=2)
        connection.request(method, path)
        response = connection.getresponse()
        result = response.status, dict(response.getheaders()), response.read()
        connection.close()
        return result

    def run_scan(self, mode="good", **options):
        with mock.patch.dict(os.environ, dict(self.env, CONTROL_MODE=mode), clear=True):
            return mirror.scan(self.ref, "linux/amd64", self.layout, self.stage_proof, self.proof,
                               self.command, os.getppid(), stop_seconds=0.3, **options)

    def assert_stopped(self):
        self.assertEqual(list(self.work.iterdir()), [])
        receipt = json.loads(self.proof.read_text())
        if "loopback_port" in receipt:
            connection = http.client.HTTPConnection("127.0.0.1", receipt["loopback_port"], timeout=1)
            with self.assertRaises(OSError):
                connection.request("GET", "/v2/")
            connection.close()


class MirrorControls(MirrorFixture):
    def test_real_http_preserves_original_index_child_config_and_layer_bytes(self):
        self.assertNotEqual((self.layout / "index.json").read_bytes(), self.data["root"])
        with self.server() as server:
            self.assertEqual(server.server_address[0], "127.0.0.1")
            for kind, name in (("manifests", "root"), ("manifests", "manifest"), ("blobs", "config"), ("blobs", "layer")):
                key = digest(self.data[name])
                path = "/v2/bex-co/bex-cnb-run/" + kind + "/" + key
                status, headers, body = self.request(server, "GET", path)
                self.assertEqual((status, body), (200, self.data[name]))
                self.assertEqual(headers["Docker-Content-Digest"], key)
                self.assertEqual(int(headers["Content-Length"]), len(body))
                status, headers, body = self.request(server, "HEAD", path)
                self.assertEqual((status, body), (200, b""))
                self.assertEqual(int(headers["Content-Length"]), len(self.data[name]))
            self.assertTrue(server.root_served)

    def test_unknown_digest_other_platform_and_writes_are_not_served(self):
        with self.server() as server:
            for path in ("/v2/bex-co/bex-cnb-run/blobs/sha256:" + "f" * 64,
                         "/v2/bex-co/bex-cnb-run/manifests/" + digest(self.data["other_manifest"]),
                         "/v2/another/repository/manifests/" + self.ref.split("@")[1]):
                self.assertEqual(self.request(server, "GET", path)[0], 404)
            self.assertEqual(self.request(server, "PUT", "/v2/bex-co/bex-cnb-run/blobs/uploads/")[0], 501)
            self.assertTrue(server.native_failure)

    def test_missing_or_corrupt_graph_fails_before_scanner_invocation(self):
        for name in ("root", "manifest", "config", "layer"):
            path = self.layout / "blobs/sha256" / digest(self.data[name])[7:]
            for mutation in ("missing", "corrupt"):
                with self.subTest(name=name, mutation=mutation):
                    if mutation == "missing":
                        path.unlink()
                    else:
                        path.write_bytes(b"X" * len(self.data[name]))
                    with self.assertRaises(mirror.MirrorError):
                        self.run_scan()
                    self.assertFalse((self.root / "invoked.json").exists())
                    self.assertFalse(self.output.exists())
                    self.assertFalse(json.loads(self.proof.read_text())["complete"])
                    path.write_bytes(self.data[name])
                    self.assert_stopped()

    def test_wrong_reference_platform_or_incomplete_proof_cannot_scan(self):
        original = self.stage_proof.read_bytes()
        for key, value in (("ref", stage.REPOSITORY + "@sha256:" + "f" * 64), ("platform", "linux/arm64"), ("complete", False)):
            with self.subTest(key=key):
                proof = json.loads(original)
                proof[key] = value
                self.stage_proof.write_text(json.dumps(proof))
                with self.assertRaises(mirror.MirrorError):
                    self.run_scan()
                self.assertFalse((self.root / "invoked.json").exists())
        self.stage_proof.write_bytes(original)

    def test_selected_config_must_really_match_requested_platform(self):
        document = json.loads(self.data["config"])
        document["architecture"] = "arm64"
        config = encoded(document)
        manifest = json.loads(self.data["manifest"])
        manifest["config"] = descriptor(config, "application/vnd.oci.image.config.v1+json")
        raw = encoded(manifest)
        root = json.loads(self.data["root"])
        root["manifests"][0].update(descriptor(raw, "application/vnd.oci.image.manifest.v1+json"))
        root_raw = encoded(root)
        ref = stage.REPOSITORY + "@" + digest(root_raw)
        for body in (config, raw, root_raw):
            (self.layout / "blobs/sha256" / digest(body)[7:]).write_bytes(body)
        identity = {"schema": stage.SCHEMA, "ref": ref, "platform": "linux/amd64"}
        (self.layout / stage.MARKER).write_text(json.dumps(identity))
        proof = json.loads(self.stage_proof.read_text())
        proof.update(identity, root=descriptor(root_raw, "application/vnd.oci.image.index.v1+json"),
                     selected=root["manifests"][0], config=manifest["config"])
        self.stage_proof.write_text(json.dumps(proof))
        with self.assertRaisesRegex(mirror.MirrorError, "actual config platform"):
            mirror.VerifiedLayout(ref, "linux/amd64", self.layout, self.stage_proof)

    def test_full_scan_keeps_original_command_platform_and_raw_report(self):
        receipt = self.run_scan()
        self.assertTrue(receipt["complete"])
        self.assertEqual(self.output.read_bytes(), REPORT)
        observed = json.loads((self.root / "invoked.json").read_text())
        self.assertEqual(observed["argv"][2:], self.command[1:])
        self.assertEqual(observed["platform"], "linux/amd64")
        self.assertFalse(Path(observed["config"]).exists())
        self.assertEqual(receipt["root_digest"], self.ref.split("@")[1])
        self.assert_stopped()

    def test_arm64_scan_uses_its_actual_selected_config_under_original_index(self):
        layout, proof = self.root / "arm-layout", self.root / "arm-stage.json"
        stage.Stager(self.ref, "linux/arm64", layout, proof, SourceRegistry(self.data), sleep=lambda _: None).run()
        with mock.patch.dict(os.environ, dict(self.env, TRIVY_PLATFORM="linux/arm64"), clear=True):
            receipt = mirror.scan(self.ref, "linux/arm64", layout, proof, self.proof,
                                  self.command, os.getppid())
        self.assertEqual(receipt["config_digest"], digest(self.data["other_config"]))
        self.assertEqual(receipt["selected_manifest"], digest(self.data["other_manifest"]))
        self.assertEqual(receipt["root_digest"], self.ref.split("@")[1])
        self.assertEqual(self.output.read_bytes(), REPORT)
        self.assert_stopped()
    def test_failed_scan_origin_fallback_and_timeout_cannot_leave_success_report(self):
        for mode in ("fail", "fallback", "bypass", "hang"):
            with self.subTest(mode=mode):
                with self.assertRaises(mirror.MirrorError):
                    self.run_scan(mode, scan_seconds=0.3)
                self.assertFalse(self.output.exists())
                self.assertFalse(json.loads(self.proof.read_text())["complete"])
                self.assert_stopped()

    def test_config_cleanup_error_after_scan_cannot_leave_complete_receipt_or_report(self):
        original = tempfile.TemporaryDirectory

        class FailedCleanup(original):
            def __exit__(self, *args):
                super().__exit__(*args)
                raise OSError("controlled temporary config cleanup failure")

        with mock.patch.object(mirror.tempfile, "TemporaryDirectory", FailedCleanup):
            with self.assertRaises(mirror.MirrorError):
                self.run_scan()
        self.assertFalse(json.loads(self.proof.read_text())["complete"])
        self.assertFalse(self.output.exists())
        self.assert_stopped()

    def test_group_gone_paths_still_wait_for_the_owned_child(self):
        for missing_at in (0, 1):
            with self.subTest(missing_at=missing_at):
                process = mock.Mock(pid=123)
                process.wait.return_value = -15
                effects = [None] * missing_at + [ProcessLookupError()]
                with mock.patch.object(mirror.os, "killpg", side_effect=effects):
                    mirror.stop_process(process, grace=0.2)
                process.wait.assert_called_once_with(timeout=0.2)

    def test_existing_scan_config_is_not_silently_discarded(self):
        with mock.patch.dict(os.environ, dict(self.env, TRIVY_CONFIG="existing-config.yaml"), clear=True):
            with self.assertRaisesRegex(mirror.MirrorError, "existing scan config"):
                mirror.scan(self.ref, "linux/amd64", self.layout, self.stage_proof, self.proof,
                            self.command, os.getppid())
        self.assertFalse((self.root / "invoked.json").exists())

    def test_startup_failures_close_listener_restore_signals_and_never_spawn_scan(self):
        # An owned subprocess timeout makes the original shutdown deadlock a
        # bounded failing control, rather than hanging this test process.
        probe = r'''
import http.client,json,os,pathlib,signal,sys,threading
sys.path.insert(0,sys.argv[1])
import test_native_oci_mirror as controls
fixture=controls.MirrorFixture();fixture.setUp()
original_start=threading.Thread.start;original_signal=signal.signal
handlers={s:signal.getsignal(s) for s in (signal.SIGTERM,signal.SIGINT)}
original_registry=controls.mirror.Registry;ports=[];calls=0
mode=sys.argv[2]
class Registry(original_registry):
 def __init__(self,layout):
  super().__init__(layout);ports.append(self.server_port)
controls.mirror.Registry=Registry
def start(thread):
 global calls
 calls+=1
 failing=calls==(1 if mode.startswith('watcher') else 2)
 if failing and mode.endswith('before'):raise RuntimeError('controlled startup failure')
 result=original_start(thread)
 if failing and mode.endswith('after'):raise RuntimeError('controlled partial startup failure')
 return result
def set_signal(signum,handler):
 if mode=='signal-after-install' and signum==signal.SIGTERM and getattr(handler,'__name__','')=='cancelled':
  result=original_signal(signum,handler);os.kill(os.getpid(),signal.SIGTERM);return result
 if mode=='signal-registration' and signum==signal.SIGINT and getattr(handler,'__name__','')=='cancelled':
  raise OSError('controlled signal registration failure')
 return original_signal(signum,handler)
threading.Thread.start=start;signal.signal=set_signal
try:
 try:fixture.run_scan()
 except controls.mirror.MirrorError:pass
 else:raise AssertionError('startup failure was accepted')
 assert not (fixture.root/'invoked.json').exists()
 assert json.loads(fixture.proof.read_text())['complete'] is False
 assert not list(fixture.work.iterdir())
 assert all(signal.getsignal(s)==h for s,h in handlers.items())
 assert threading.enumerate()==[threading.main_thread()]
 for port in ports:
  connection=http.client.HTTPConnection('127.0.0.1',port,timeout=0.2)
  try:connection.request('GET','/v2/')
  except OSError:pass
  else:raise AssertionError('allocated listener survived startup failure')
  finally:connection.close()
 print(json.dumps({'mode':mode,'listeners_closed':len(ports),'signals_restored':True,'scanner_spawned':False}))
finally:
 threading.Thread.start=original_start;signal.signal=original_signal;fixture.doCleanups()
'''
        for mode in ("watcher-before", "watcher-after", "server-before", "server-after", "signal-registration", "signal-after-install"):
            with self.subTest(mode=mode):
                result = subprocess.run([sys.executable, "-B", "-c", probe, str(Path(__file__).parent), mode],
                                        capture_output=True, text=True, timeout=4)
                self.assertEqual(result.returncode, 0, result.stderr)
                observed = json.loads(result.stdout)
                self.assertTrue(observed["signals_restored"])
                self.assertFalse(observed["scanner_spawned"])
                self.assertEqual(observed["listeners_closed"], int(mode.startswith("server")))

    def test_cancel_and_parent_death_stop_only_owned_child_and_listener(self):
        script = Path(__file__).with_name("native-oci-mirror.py")
        for mode in ("cancel", "parent-death"):
            with self.subTest(mode=mode):
                (self.root / "ready").unlink(missing_ok=True)
                (self.root / "parent-child.json").unlink(missing_ok=True)
                self.proof.unlink(missing_ok=True)
                args = [sys.executable, str(script), "--ref", self.ref, "--platform", "linux/amd64",
                        "--layout", str(self.layout), "--stage-proof", str(self.stage_proof), "--proof", str(self.proof)]
                driver = ("import json,os,pathlib,subprocess,sys,time; "
                          "p=subprocess.Popen(json.loads(os.environ['CONTROL_COMMAND'])+['--parent-pid',str(os.getpid()),'--']+json.loads(os.environ['CONTROL_SCAN'])); "
                          "pathlib.Path(os.environ['CONTROL_ROOT'],'parent-child.json').write_text(json.dumps({'pid':p.pid})); "
                          "p.wait()")
                env = dict(self.env, CONTROL_MODE="hang", CONTROL_COMMAND=json.dumps(args), CONTROL_SCAN=json.dumps(self.command))
                parent = subprocess.Popen([sys.executable, "-c", driver], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                try:
                    deadline = time.monotonic() + 5
                    while not (self.root / "ready").exists() and time.monotonic() < deadline:
                        time.sleep(0.02)
                    self.assertTrue((self.root / "ready").exists())
                    helper = json.loads((self.root / "parent-child.json").read_text())["pid"]
                    if mode == "cancel":
                        os.kill(helper, signal.SIGTERM)
                    else:
                        parent.kill()
                    parent.wait(timeout=5)
                    deadline = time.monotonic() + 5
                    while time.monotonic() < deadline:
                        if self.proof.exists() and not json.loads(self.proof.read_text())["complete"] and not list(self.work.iterdir()):
                            break
                        time.sleep(0.02)
                    self.assert_stopped()
                    child = json.loads((self.root / "invoked.json").read_text())["pid"]
                    with self.assertRaises(ProcessLookupError):
                        os.kill(child, 0)
                finally:
                    if parent.poll() is None:
                        parent.kill()
                        parent.wait(timeout=5)


class WorkflowTransportControls(MirrorFixture):
    def setUp(self):
        super().setUp()
        self.scripts = self.root / ".github/scripts"
        self.scripts.mkdir(parents=True)
        for name in ("native-oci-stage.py", "native-oci-mirror.py"):
            (self.scripts / name).write_bytes(Path(__file__).with_name(name).read_bytes())
        (self.root / "fixture.json").write_text(json.dumps({k: v.hex() for k, v in self.data.items()}))
        (self.root / "stage-fixture.py").write_text(r'''
import argparse,importlib.util,io,json,os,pathlib,sys
root=pathlib.Path(os.environ['CONTROL_ROOT'])
spec=importlib.util.spec_from_file_location('stage',root/'.github/scripts/native-oci-stage.py');stage=importlib.util.module_from_spec(spec);spec.loader.exec_module(stage)
p=argparse.ArgumentParser()
for name in ('ref','platform','output','proof'):p.add_argument('--'+name,required=True)
a=p.parse_args();data={k:bytes.fromhex(v) for k,v in json.loads((root/'fixture.json').read_text()).items()}
class Response(io.BytesIO):
 def __init__(self,body):super().__init__(body);self.status=200;self.headers={'Content-Length':str(len(body))}
class Registry:
 def open(self,kind,key,offset):
  if os.environ.get('CONTROL_STAGE_FAIL')=='1':raise stage.StageError('controlled staging transport failure')
  return Response(next(v for v in data.values() if 'sha256:'+stage.hashlib.sha256(v).hexdigest()==key))
try:stage.Stager(a.ref,a.platform,a.output,a.proof,Registry(),sleep=lambda _:None).run()
except stage.StageError:sys.exit(23)
''')
        binaries = self.root / "bin"
        binaries.mkdir()
        controls = {
            "python3": '#!/bin/sh\nif test "$1" = .github/scripts/native-oci-stage.py; then shift; exec "$REAL_PYTHON" "$CONTROL_ROOT/stage-fixture.py" "$@"; fi\nexec "$REAL_PYTHON" "$@"\n',
            "df": '#!' + sys.executable + '\nimport os,sys\nn=1 if os.environ.get("CONTROL_LOW_DISK")=="1" else 40*1024*1024\nprint("Filesystem 1024-blocks Used Available Capacity Mounted on")\nfor path in sys.argv[2:]:print("controlled 999999999 1",n,"1%",path)\n',
            "timeout": '#!' + sys.executable + '\nimport json,os,pathlib,subprocess,sys\na=sys.argv[1:];assert a[0]=="--kill-after=10s" and a[1] in ("21m","30m")\nwith pathlib.Path(os.environ["CONTROL_ROOT"],"timeouts.jsonl").open("a") as f:f.write(json.dumps(a)+"\\n")\nsys.exit(subprocess.call(a[2:]))\n',
            "sleep": "#!/bin/sh\nexit 0\n",
            "trivy": TRIVY,
        }
        for name, body in controls.items():
            path = binaries / name
            path.write_text(body)
            path.chmod(0o700)
        self.output = self.root / "native-amd64.trivy.json"
        self.env.update(PATH=str(binaries) + os.pathsep + os.defpath, REAL_PYTHON=sys.executable,
                        RUNNER_TEMP=str(self.work), TRIVY_CACHE_DIR=str(self.root / "cache"),
                        TRIVY_OUTPUT=str(self.output), GODEBUG="http2client=0")
        (self.root / "cache").mkdir()
        (self.root / "cache/foreign-cache").write_text("retain")
        (self.work / "foreign-sentinel").write_text("retain")

    def wrapper(self, **controls):
        from test_native_oci_workflow import run_block
        env = dict(self.env, **controls)
        prepared = subprocess.run(["/bin/bash", "-euo", "pipefail", "-c", run_block("name: prepare bounded full-scan retries")],
                                  env=env, cwd=self.root, capture_output=True, timeout=5)
        self.assertEqual(prepared.returncode, 0, prepared.stderr.decode())
        result = subprocess.run(["/bin/bash", str(self.work / "native-trivy"), "image", self.ref],
                                env=env, cwd=self.root, capture_output=True, timeout=15)
        self.assertEqual((self.work / "foreign-sentinel").read_text(), "retain")
        self.assertEqual((self.root / "cache/foreign-cache").read_text(), "retain")
        self.assertEqual(list(self.work.glob("native-trivy.*")), [])
        return result

    def test_actual_workflow_wrapper_stages_then_scans_original_ref_and_keeps_db_budgets(self):
        result = self.wrapper()
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual(self.output.read_bytes(), REPORT)
        proof = json.loads((self.root / "native-amd64-trivy-oci-stage.json").read_text())
        self.assertTrue(proof["complete"])
        mirror_proof = json.loads((self.root / "native-amd64-trivy-mirror-1.json").read_text())
        self.assertTrue(mirror_proof["complete"])
        self.assertEqual(mirror_proof["ref"], self.ref)
        calls = [json.loads(line) for line in (self.root / "timeouts.jsonl").read_text().splitlines()]
        self.assertEqual([(row[1], row[2], row[-1]) for row in calls[:2]],
                         [("21m", "trivy", "--download-db-only"), ("30m", "trivy", "--download-java-db-only")])
        self.assertEqual(calls[2][1:3], ["30m", "python3"])
        self.assertEqual(mirror.SCAN_SECONDS, 21 * 60)
        self.assertEqual(mirror.STOP_SECONDS, 10)

    def test_staging_failure_stops_before_any_scan_or_success_report(self):
        result = self.wrapper(CONTROL_STAGE_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "invoked.json").exists())
        self.assertFalse(self.output.exists())
        self.assertFalse(json.loads((self.root / "native-amd64-trivy-oci-stage.json").read_text())["complete"])
        self.assertEqual(list(self.root.glob("native-amd64-trivy-mirror-*.json")), [])

    def test_three_failed_full_scans_remove_partial_reports_and_never_approve(self):
        result = self.wrapper(CONTROL_MODE="fail")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.output.exists())
        calls = (self.root / "scan-calls.jsonl").read_text().splitlines()
        self.assertEqual(len(calls), 3)
        for attempt in (1, 2, 3):
            receipt = json.loads((self.root / f"native-amd64-trivy-mirror-{attempt}.json").read_text())
            self.assertFalse(receipt["complete"])
            self.assertEqual(receipt["scan_exit"], 17)

    def test_original_28gib_capacity_guard_stops_before_downloads(self):
        result = self.wrapper(CONTROL_LOW_DISK="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "db-calls").exists())
        self.assertFalse((self.root / "invoked.json").exists())
        self.assertFalse((self.root / "native-amd64-trivy-oci-stage.json").exists())


if __name__ == "__main__":
    unittest.main()
