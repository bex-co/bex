"""Exercise the Helm-rendered app pipeline in the chart's actual Alloy image.

Run: python3 -m unittest scripts/test_log_shipper.py -v
Requires Docker, Helm, and yq v4; no cluster or credentials.
"""

import json
from datetime import datetime, timezone
from pathlib import Path
import re
import subprocess
import tempfile
import time
import unittest
import uuid


ROOT = Path(__file__).resolve().parents[1]


def run(*args, **kwargs):
    return subprocess.check_output(args, cwd=ROOT, text=True, **kwargs).strip()


def render_alloy(tmp):
    chart = run("bash", "scripts/helm-artifact.sh", "pull", "alloy", str(tmp))
    values = tmp / "values.yaml"
    values.write_text(run("yq", "-r", ".spec.source.helm.values",
                          "deploy/gitops/base/log-shipper.yaml"))
    rendered = tmp / "rendered.yaml"
    rendered.write_text(run("helm", "template", "log-shipper", chart,
                            "-n", "monitoring", "-f", str(values)))
    config = run("yq", "-r", 'select(.kind == "ConfigMap") | .data["config.alloy"]', str(rendered))
    image = run("yq", "-r", 'select(.kind == "DaemonSet") | .spec.template.spec.containers[] | select(.name == "alloy") | .image', str(rendered))
    return config, image


def run_pipeline(test, pipeline_name, lines, receiver_type, target='database = "dpg-test"', relabel=None, expect=None):
    """Render the chart, keep one production loki.process block, feed it lines
    in the chart's own Alloy image, and return {entry: labels} per echoed line.

    relabel=(block_name, [target meta labels]) instead routes one line per
    synthetic pod target through that production discovery.relabel block."""
    with tempfile.TemporaryDirectory(prefix="bex-log-shipper-") as tmp:
        tmp = Path(tmp)
        config, image = render_alloy(tmp)
        pipeline = re.search(r'^loki\.process "%s" \{\n.*?^\}' % pipeline_name, config, re.M | re.S)
        test.assertIsNotNone(pipeline, f"rendered chart is missing the {pipeline_name} pipeline")
        # Keep every production stage; replace only the input and sink.
        pipeline = pipeline.group().replace("loki.write.default.receiver", "loki.echo.test.receiver")
        files = {"lines.log": "\n".join(lines) + "\n"}
        if relabel:
            block_name, pods = relabel
            block = re.search(r'^discovery\.relabel "%s" \{\n.*?^\}' % block_name, config, re.M | re.S)
            test.assertIsNotNone(block, f"rendered chart is missing discovery.relabel {block_name}")
            targets = []
            for i, (meta, line) in enumerate(zip(pods, lines)):
                files[f"pod{i}.log"] = line + "\n"
                fields = dict(meta, __path__=f"/tmp/pod{i}.log")
                targets.append("{" + ", ".join(f"{k} = {json.dumps(v)}" for k, v in fields.items()) + "}")
            source = block.group().replace("discovery.kubernetes.pods.targets", "[" + ", ".join(targets) + "]")
            source += '''
loki.source.file "test" {
  targets = discovery.relabel.%s.output
  forward_to = [loki.process.%s.receiver]
}
''' % (block_name, pipeline_name)
        else:
            source = '''loki.source.file "test" {
  targets = [{__path__ = "/tmp/lines.log", %s}]
  forward_to = [loki.process.%s.receiver]
}
''' % (target, pipeline_name)
        (tmp / "config.alloy").write_text('''logging {
  format = "json"
}
loki.echo "test" {}
''' + source + pipeline)
        for filename, content in files.items():
            (tmp / filename).write_text(content)
        expect = len(lines) if expect is None else expect
        name = "bex-log-shipper-" + uuid.uuid4().hex[:12]
        run("docker", "create", "--name", name, "--network", "none", image,
            "run", "--storage.path=/tmp/alloy", "/tmp/config.alloy")
        try:
            # docker cp also works when CI uses a separate Docker daemon.
            for filename in ["config.alloy", *files]:
                run("docker", "cp", str(tmp / filename), f"{name}:/tmp/{filename}")
            run("docker", "start", name)
            deadline = time.monotonic() + 30
            actual = {}
            output = ""
            while time.monotonic() < deadline:
                output = run("docker", "logs", name, stderr=subprocess.STDOUT)
                for line in output.splitlines():
                    try:
                        entry = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    # Alloy names the echo component `receiver` (<=1.11) or `component_id`.
                    if (entry.get("receiver") or entry.get("component_id")) == "loki.echo.test":
                        labels = dict(re.findall(r'(\w+)="([^"\\]*)"', entry["labels"]))
                        test.assertEqual(labels.get("type"), receiver_type)
                        actual[entry["entry"]] = labels
                if len(actual) >= expect:
                    break
                if run("docker", "inspect", "-f", "{{.State.Running}}", name) != "true":
                    test.fail(f"Alloy exited before processing the fixture:\n{output}")
                time.sleep(0.25)
            # A pipeline that drops lines never reaches len(lines), so the
            # full deadline passes first — long enough for a line that should
            # have been dropped to show up.
            return actual, output
        finally:
            run("docker", "rm", "-f", name)


class AppLogLevelsTest(unittest.TestCase):
    def test_structured_severity_and_bounded_labels(self):
        cases = {
            '{"level":"error","msg":"json error"}': "error",
            '{"severity":"ERROR","msg":"json severity"}': "error",
            '{"level":"info","msg":"level=error is only a message"}': "info",
            '{"msg":"level=error is only a message"}': "unknown",
            'level=error msg=logfmt-error': "error",
            'time=2026-09-22T00:41:37.000Z level=ERROR msg="request failed" path=/x': "error",
            'severity=ERROR msg="severity alias"': "error",
            'level="ERROR" msg="quoted severity"': "error",
            'level=info severity=error msg="level takes precedence"': "info",
            'msg="level=error is only a message"': "unknown",
            'level=info msg="error in an info message"': "info",
            'ERROR plaintext error': "unknown",
            'plain text mentioning error': "unknown",
            'msg="no severity"': "unknown",
            'level=custom-value msg="unrecognized severity"': "unknown",
            # w7/m155: quoted text and JSON are not logfmt. Before the logfmt
            # stage was gated these produced Alloy's "failed to decode logfmt"
            # error per line (8,979 in one production day).
            'GET "/healthz" 200 12ms': "unknown",
            '{"level":"warn","msg":"quoted \\"json\\" value"}': "warning",
            '{"level":"error","msg":"truncated json"': "unknown",
            '  {"msg":"indented json without a severity"}': "unknown",
            'ts=2026-09-28T06:09:24Z level=warn msg="spaced \\"escaped\\" quotes"': "warning",
        }
        # Render's level names (w8/031): warnings ship as `warning`, the value
        # the pinned CLI's --level sends.
        for value, expected in {
            "err": "error", "fatal": "error", "panic": "error",
            "critical": "error", "crit": "error", "warn": "warning",
            "WARNING": "warning", "info": "info", "notice": "info",
            "DEBUG": "debug", "trace": "debug",
        }.items():
            for key in ("level", "severity"):
                cases[f'{key}={value} msg="normalization"'] = expected

        # discovery.relabel.app_pods drops every pod without app.bex.co/app,
        # so every production app_logs entry carries these labels.
        actual, output = run_pipeline(self, "app_logs", list(cases), "app",
                                      'namespace = "tenant", app = "web", pod = "web-1", container = "app"')
        self.assertEqual(set(actual), set(cases), f"missing or changed log lines:\n{output}")
        for line, expected in cases.items():
            with self.subTest(line=line):
                self.assertEqual(actual[line].get("level"), expected)
        self.assertEqual({labels.get("level") for labels in actual.values()},
                         {"error", "warning", "info", "debug", "unknown"})
        # Every supported format parses without a stage diagnostic: expected
        # mismatches must not bury genuine ingestion errors.
        diagnostics = [line for line in output.splitlines()
                       if '"component_id":"loki.process.app_logs"' in line
                       and re.search(r'"level":"(error|warn)"', line)]
        self.assertEqual(diagnostics, [], "app_logs emitted parse diagnostics")


class PostgresLogsTest(unittest.TestCase):
    """w8/030: CNPG's instance-manager chatter is dropped, and PostgreSQL's own
    records are unwrapped into its stderr shape with a level."""

    def test_cnpg_records_unwrapped_and_chatter_dropped(self):
        lines = [
            '{"level":"info","logger":"instance-manager","msg":"Starting EventSource"}',
            '{"level":"info","logger":"cluster-resource","msg":"Defaulting for Cluster"}',
            '{"logger":"postgres","msg":"record","record":{"log_time":"2026-09-27 01:00:02.123 UTC","process_id":"42","error_severity":"LOG","message":"database system is ready to accept connections"}}',
            '{"logger":"postgres","msg":"record","record":{"log_time":"t","process_id":"77","error_severity":"FATAL","message":"password authentication failed","detail":"pg_hba line 5"}}',
            '{"logger":"postgres","msg":"record","record":{"log_time":"t","process_id":"78","error_severity":"WARNING","message":"checkpoints too frequent","hint":"raise max_wal_size"}}',
            'plain line kept verbatim',
        ]
        want = {
            "2026-09-27 01:00:02.123 UTC [42] LOG:  database system is ready to accept connections": "info",
            "t [77] FATAL:  password authentication failed DETAIL:  pg_hba line 5": "error",
            "t [78] WARNING:  checkpoints too frequent HINT:  raise max_wal_size": "warning",
            "plain line kept verbatim": None,
        }
        actual, output = run_pipeline(self, "database_logs", lines, "postgres")
        self.assertEqual(set(actual), set(want), f"unexpected Postgres lines:\n{output}")
        for line, level in want.items():
            with self.subTest(line=line):
                self.assertEqual(actual[line].get("level"), level)
                self.assertNotIn("cnpg_logger", actual[line])


class PlatformLogsTest(unittest.TestCase):
    """w7/m157: bex-api and the operator manager ship as type=platform under a
    closed service= vocabulary; other bex-system components stay out."""

    def test_core_services_collected_with_closed_labels(self):
        def pod(ns, name, container, **labels):
            meta = {"__meta_kubernetes_namespace": ns, "__meta_kubernetes_pod_name": name,
                    "__meta_kubernetes_pod_container_name": container}
            meta.update({"__meta_kubernetes_pod_label_" + k: v for k, v in labels.items()})
            return meta
        pods = [
            (pod("bex-system", "bex-api-1", "api", app_kubernetes_io_name="bex-api"),
             "2026/09/28 05:08:12 usage: rolled up window m157-api-marker"),
            (pod("bex-system", "bex-controller-manager-1", "manager",
                 app_kubernetes_io_name="control-plane", control_plane="controller-manager"),
             '2026-09-28T01:09:31Z\tERROR\tm157-operator-marker\t{"controller": "app"}'),
            (pod("bex-system", "bex-static-server-1", "static-server", app_bex_co_component="static-server"),
             '{"level":"warn","msg":"m157-static-marker"}'),
            (pod("dashboard", "dashboard-1", "dashboard"), "m157-dashboard-marker"),
            (pod("bex-system", "bex-activator-1", "activator", app_bex_co_component="activator"),
             "m157-activator-must-not-ship"),
            (pod("bex-system", "bex-ssh-gateway-1", "ssh-gateway", app_kubernetes_io_name="bex-ssh-gateway"),
             "m157-ssh-gateway-must-not-ship"),
            (pod("opensandbox-system", "osb-controller-manager-1", "manager", control_plane="controller-manager"),
             "m157-foreign-operator-must-not-ship"),
        ]
        actual, output = run_pipeline(self, "platform_logs", [line for _, line in pods], "platform",
                                      relabel=("platform_pods", [meta for meta, _ in pods]), expect=4)
        want = {
            pods[0][1]: ("bex-api", "bex-system", "bex-api-1", "api", "unknown"),
            pods[1][1]: ("operator", "bex-system", "bex-controller-manager-1", "manager", "error"),
            pods[2][1]: ("static-server", "bex-system", "bex-static-server-1", "static-server", "warning"),
            pods[3][1]: ("dashboard", "dashboard", "dashboard-1", "dashboard", "unknown"),
        }
        self.assertEqual(set(actual), set(want), f"unexpected platform lines:\n{output}")
        for line, (service, ns, pod_name, container, level) in want.items():
            with self.subTest(line=line):
                labels = actual[line]
                self.assertEqual((labels.get("service"), labels.get("namespace"), labels.get("pod"),
                                  labels.get("container"), labels.get("level")),
                                 (service, ns, pod_name, container, level))
                self.assertNotIn("app", labels, "platform streams must stay outside tenant selectors")
        diagnostics = [line for line in output.splitlines()
                       if '"component_id":"loki.process.platform_logs"' in line
                       and re.search(r'"level":"(error|warn)"', line)]
        self.assertEqual(diagnostics, [], "platform_logs emitted parse diagnostics")


class ZotGCMetricsTest(unittest.TestCase):
    def test_fresh_failure_replay_reload_and_idle_expiry(self):
        heartbeat = "bex_zot_log_last_observed_timestamp_seconds"
        failure = "bex_zot_gc_failure_last_observed_timestamp_seconds"
        signature = {"level": "error", "module": "gc", "message": "failed to get repoMeta",
                     "error": "repo metadata not found for given repo name",
                     "repository": "fixture/repository"}
        with tempfile.TemporaryDirectory(prefix="bex-zot-metrics-") as directory:
            tmp = Path(directory)
            config, image = render_alloy(tmp)
            platform = re.search(r'^loki\.process "platform_logs" \{\n.*?^\}', config, re.M | re.S).group()
            metrics = re.search(r'^loki\.process "zot_gc_metrics" \{\n.*?^\}', config, re.M | re.S).group()
            native_source = re.search(r'^loki\.source\.kubernetes "platform_pods" \{\n.*?^\}', config, re.M | re.S).group()
            fanout = re.search(r'forward_to\s*=\s*\[.*?\]', native_source).group()
            self.assertEqual(metrics.count('max_idle_duration = "5m"'), 2)
            # Shorten only idle expiry in the isolated fixture. The same locked
            # binary's expiry/reload behavior runs without a five-minute CI wait.
            metrics = metrics.replace('max_idle_duration = "5m"', 'max_idle_duration = "5s"')
            # Echo sinks expose completion of each branch, so negative assertions
            # don't race queued entries; production metrics forward to no sink.
            metrics = metrics.replace('forward_to = []', 'forward_to = [loki.echo.metrics_done.receiver]')
            platform = platform.replace('loki.write.default.receiver', 'loki.echo.test.receiver')
            fixture = '''logging { format = "json" }
loki.echo "test" {}
loki.echo "metrics_done" {}
loki.source.file "test" {
  targets = [
    {__path__ = "/tmp/zot.log", service = "zot", namespace = "bex-registry", pod = "zot-0", container = "zot", repository = "fixture/repository"},
    {__path__ = "/tmp/other.log", service = "bex-api", namespace = "bex-system", pod = "api-0", container = "api"},
  ]
  forward_to = [loki.process.fixture_clock.receiver]
}
loki.process "fixture_clock" {
  stage.json { expressions = { fixture_time = "fixture_time" } }
  stage.timestamp {
    source = "fixture_time"
    format = "RFC3339Nano"
  }
  %s
}
''' % fanout
            fixture += platform + "\n" + metrics + "\n"
            (tmp / "config.alloy").write_text(fixture)
            (tmp / "zot.log").write_text("")
            (tmp / "other.log").write_text("")
            name = "bex-zot-metrics-" + uuid.uuid4().hex[:12]
            run("docker", "create", "--name", name, "--network", "none", image,
                "run", "--storage.path=/tmp/alloy", "/tmp/config.alloy")
            try:
                for filename in ["config.alloy", "zot.log", "other.log"]:
                    run("docker", "cp", str(tmp / filename), f"{name}:/tmp/{filename}")
                run("docker", "start", name)

                def scrape():
                    # Read inside the container: works with a remote Docker
                    # daemon, needs no public port and no second helper image.
                    response = run("docker", "exec", name, "bash", "-c",
                                   "exec 3<>/dev/tcp/127.0.0.1/12345; "
                                   "printf 'GET /metrics HTTP/1.0\\r\\nHost: localhost\\r\\n\\r\\n' >&3; cat <&3")
                    self.assertIn("200 OK", response.splitlines()[0])
                    samples = {}
                    for metric_name, labels, value in re.findall(r'^(bex_zot_\w+)\{([^}]+)\} ([^\s]+)$', response, re.M):
                        label_set = dict(re.findall(r'(\w+)="([^"\\]*)"', labels))
                        self.assertLessEqual(set(label_set), {"service", "component_id", "component_path"})
                        self.assertEqual(label_set.get("service"), "zot")
                        self.assertNotIn(metric_name, samples, "unbounded labels split a metric")
                        samples[metric_name] = float(value)
                    return samples

                def emit(fields, age=0, other=False):
                    stamp = datetime.fromtimestamp(time.time() - age, timezone.utc).isoformat()
                    entry = json.dumps(dict(fields, fixture_time=stamp, fixture_id=uuid.uuid4().hex))
                    filename = "other.log" if other else "zot.log"
                    run("docker", "exec", "-i", name, "sh", "-c", f"cat >> /tmp/{filename}", input=entry + "\n")
                    wanted = {"loki.echo.test"}
                    if not other and age < 300:
                        wanted.add("loki.echo.metrics_done")
                    deadline = time.monotonic() + 20
                    while time.monotonic() < deadline:
                        output = run("docker", "logs", name, stderr=subprocess.STDOUT)
                        for line in output.splitlines():
                            try:
                                record = json.loads(line)
                            except json.JSONDecodeError:
                                continue
                            receiver = record.get("receiver") or record.get("component_id")
                            if receiver in wanted and record.get("entry") == entry:
                                if receiver == "loki.echo.test":
                                    labels = dict(re.findall(r'(\w+)="([^"\\]*)"', record["labels"]))
                                    self.assertEqual(labels["type"], "platform")
                                    self.assertEqual(labels["service"], "bex-api" if other else "zot")
                                    self.assertEqual(labels["pod"], "api-0" if other else "zot-0")
                                    self.assertEqual(labels["namespace"], "bex-system" if other else "bex-registry")
                                    self.assertEqual(labels["container"], "api" if other else "zot")
                                    self.assertEqual(labels["level"], fields.get("level", "unknown"))
                                wanted.discard(receiver)
                        if not wanted:
                            return
                        self.assertEqual(run("docker", "inspect", "-f", "{{.State.Running}}", name), "true", output)
                        time.sleep(0.1)
                    self.fail(f"Alloy did not finish branches {wanted}:\n{output}")

                healthy = {"level": "info", "module": "gc", "message": "garbage collected blobs", "count": 0}
                emit(signature, age=3600)  # Retained by Loki, excluded from metrics.
                emit(signature, other=True)
                emit(healthy)  # Queue barrier also proves the heartbeat pipeline works.
                self.assertEqual(set(scrape()), {heartbeat})
                # Quoted error text, another module, another level, and a
                # different failure are all outside this one known GC defect.
                for control in [dict(healthy, request=json.dumps(signature)),
                                dict(signature, level="info"), dict(signature, module="http"),
                                dict(signature, message="some other failure"),
                                dict(signature, error="permission denied")]:
                    emit(control)
                    self.assertNotIn(failure, scrape())

                before = int(time.time())
                emit(signature)
                observed = scrape()
                self.assertEqual(set(observed), {heartbeat, failure})
                self.assertGreaterEqual(observed[failure], before)
                self.assertLessEqual(observed[failure], time.time())
                emit(signature, age=3600)
                emit(healthy)
                self.assertEqual(scrape()[failure], observed[failure], "old replay refreshed the failure")
                emit(signature, age=120)
                self.assertGreaterEqual(scrape()[failure], observed[failure])

                # A real component update resets custom gauges. Fresh entries
                # recreate the first sample; no synthetic initial zero exists.
                changed = fixture.replace('Last fresh Zot log observed by this collector.',
                                          'Last fresh Zot log observed by this collector after reload.')
                (tmp / "config.alloy").write_text(changed)
                run("docker", "cp", str(tmp / "config.alloy"), f"{name}:/tmp/config.alloy")
                run("docker", "kill", "--signal=HUP", name)
                deadline = time.monotonic() + 10
                while time.monotonic() < deadline and scrape():
                    time.sleep(0.1)
                self.assertEqual(scrape(), {}, "component reload retained its custom metrics")
                emit(healthy)
                self.assertEqual(set(scrape()), {heartbeat})
                emit(signature)
                last = scrape()[failure]

                # The pinned collector exports once before pruning idle metrics.
                # A resumed scrape therefore sees the OLD timestamp, never now.
                expiry = time.monotonic() + 7
                while time.monotonic() < expiry:
                    time.sleep(min(0.2, expiry - time.monotonic()))
                self.assertEqual(scrape()[failure], last)
                self.assertEqual(scrape(), {})
            finally:
                run("docker", "rm", "-f", name)


if __name__ == "__main__":
    unittest.main()
