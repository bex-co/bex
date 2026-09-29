"""Exercise delivery loss and recovery in the locked Alloy/Loki images.

Run: python3 -m unittest scripts/test_log_delivery.py -v
Requires Docker, Helm and yq v4. No cluster, credentials or published ports.
"""

from datetime import datetime, timezone
import json
from pathlib import Path
import re
import subprocess
import tempfile
import time
import unittest
from urllib.parse import urlencode
import uuid

from scripts.test_log_shipper import render_alloy, run


def render_loki(tmp):
    chart = run("bash", "scripts/helm-artifact.sh", "pull", "loki", str(tmp))
    values = tmp / "loki-values.yaml"
    values.write_text(run("yq", "-r", ".spec.source.helm.values", "deploy/gitops/base/loki.yaml"))
    rendered = tmp / "loki-rendered.yaml"
    rendered.write_text(run("helm", "template", "loki", chart, "-n", "monitoring", "-f", str(values)))
    config = run("yq", "-r", 'select(.kind == "ConfigMap" and .metadata.name == "loki") | .data["config.yaml"]', str(rendered))
    config = json.loads(run("yq", "-o=json", "-I=0", input=config))
    image = run("yq", "-r", 'select(.kind == "StatefulSet") | .spec.template.spec.containers[] | select(.name == "loki") | .image', str(rendered))
    return config, image


class LogDeliveryTest(unittest.TestCase):
    def test_retries_final_loss_rejection_and_recovery(self):
        with tempfile.TemporaryDirectory(prefix="bex-log-delivery-") as directory:
            tmp = Path(directory)
            alloy_config, alloy_image = render_alloy(tmp)
            loki_config, loki_image = render_loki(tmp)
            writer = re.search(r'^loki\.write "default" \{\n.*?^\}', alloy_config, re.M | re.S).group()
            # Production uses the pinned v1.11.2 defaults: batch 1s/1MiB,
            # request timeout 10s, backoff 500ms..5m, ten attempts, retry 429,
            # no WAL. Only the isolated fixture shortens batching/backoff.
            # github.com/grafana/alloy/blob/v1.11.2/internal/component/loki/write/types.go
            self.assertNotRegex(writer, r"backoff|batch_wait|remote_timeout|retry_on_http_429|wal")
            writer = writer.replace("http://loki.monitoring.svc:3100", "http://127.0.0.1:3100")
            writer = writer.replace("endpoint {", '''endpoint {
    batch_wait = "100ms"
    min_backoff_period = "1s"
    max_backoff_period = "1s"
    max_backoff_retries = 10''')
            pipelines = [re.search(r'^loki\.process "%s" \{\n.*?^\}' % name,
                                   alloy_config, re.M | re.S).group()
                         for name in ("app_logs", "database_logs")]
            fixture = 'logging { format = "json" }\n' + writer + "\n" + "\n".join(pipelines)
            for port, pipeline in ((3500, "app_logs"), (3501, "database_logs")):
                fixture += '''
loki.source.api "%s" {
  http {
    listen_address = "127.0.0.1"
    listen_port = %s
  }
  use_incoming_timestamp = true
  forward_to = [loki.process.%s.receiver]
}
''' % (pipeline, port, pipeline)
            (tmp / "config.alloy").write_text(fixture)

            # Keep chart limits/schema/storage; replace cluster addressing with
            # one isolated process and enable a fixture-only reloadable fault.
            loki_config["common"].update(instance_addr="127.0.0.1", ring={"kvstore": {"store": "inmemory"}})
            loki_config["common"]["compactor_grpc_address"] = "127.0.0.1:9095"
            loki_config["memberlist"] = {"join_members": []}
            loki_config["runtime_config"] = {"file": "/tmp/runtime.json", "period": "100ms"}
            loki_config["ingester"] = {"lifecycler": {"min_ready_duration": "0s"}}
            loki_config["analytics"] = {"reporting_enabled": False}
            (tmp / "loki.json").write_text(json.dumps(loki_config).replace("/var/loki", "/tmp/loki"))
            (tmp / "runtime.json").write_text('{"overrides": {}}')
            prefix = "bex-log-delivery-" + uuid.uuid4().hex[:12]
            loki, alloy = prefix + "-loki", prefix + "-alloy"
            created = []
            try:
                run("docker", "create", "--name", loki, "--network", "none", loki_image,
                    "-config.file=/tmp/loki.json")
                created.append(loki)
                for filename in ("loki.json", "runtime.json"):
                    run("docker", "cp", str(tmp / filename), f"{loki}:/tmp/{filename}")
                run("docker", "start", loki)
                # Sharing only this isolated network namespace avoids host ports
                # and works when the Docker daemon is remote from the test runner.
                run("docker", "create", "--name", alloy, "--network", f"container:{loki}",
                    alloy_image, "run", "--storage.path=/tmp/alloy", "/tmp/config.alloy")
                created.append(alloy)
                run("docker", "cp", str(tmp / "config.alloy"), f"{alloy}:/tmp/config.alloy")
                run("docker", "start", alloy)

                def http(port, path, body=None):
                    method, payload = ("GET", "") if body is None else ("POST", json.dumps(body))
                    request = (f"{method} {path} HTTP/1.0\r\nHost: localhost\r\n"
                               f"Content-Type: application/json\r\nContent-Length: {len(payload.encode())}\r\n\r\n{payload}")
                    response = run("docker", "exec", "-i", alloy, "bash", "-c",
                                   f"set -e; exec 3<>/dev/tcp/127.0.0.1/{port}; cat >&3; cat <&3",
                                   input=request, timeout=10, stderr=subprocess.DEVNULL)
                    head, _, content = response.partition("\n\n")
                    return int(head.split()[1]), content

                def wait_for(predicate, label, seconds=20):
                    deadline = time.monotonic() + seconds
                    while time.monotonic() < deadline:
                        try:
                            if predicate():
                                return
                        except (subprocess.CalledProcessError, IndexError, ValueError):
                            pass
                        time.sleep(0.1)
                    self.fail(f"Timed out waiting for {label}")

                def metrics(port):
                    status, text = http(port, "/metrics")
                    self.assertEqual(status, 200)
                    result = []
                    for name, labels, value in re.findall(r'^(\w+)(?:\{([^\n]*)\})? ([^\s]+)$', text, re.M):
                        if name.startswith(("loki_write_", "loki_discarded_", "loki_process_", "loki_runtime_config_")) or name in {
                            "alloy_resources_process_start_time_seconds", "alloy_config_last_load_successful",
                            "process_start_time_seconds", "alloy_build_info", "loki_build_info",
                        }:
                            result.append((name, dict(re.findall(r'(\w+)="([^"\\]*)"', labels)), float(value)))
                    return result

                def value(samples, name, **labels):
                    return sum(number for metric, actual, number in samples
                               if metric == name and all(actual.get(key) == wanted for key, wanted in labels.items()))

                def emit(line, age=0, database=False):
                    labels = {"namespace": "fixture", "pod": "fixture", "container": "fixture"}
                    labels.update({"database": "dpg-fixture"} if database else {"app": "fixture"})
                    body = {"streams": [{"stream": labels,
                                         "values": [[str(int((time.time() - age) * 1e9)), line]]}]}
                    self.assertEqual(http(3501 if database else 3500, "/loki/api/v1/push", body)[0], 204)

                def block_delivery(blocked):
                    previous = [labels for name, labels, _ in metrics(3100) if name == "loki_runtime_config_hash"]
                    self.assertEqual(len(previous), 1)
                    overrides = {"fake": {"block_ingestion_until": datetime.fromtimestamp(
                        time.time() + 3600, timezone.utc).isoformat(), "block_ingestion_status_code": 503}} if blocked else {}
                    (tmp / "runtime.json").write_text(json.dumps({"overrides": overrides}))
                    run("docker", "cp", str(tmp / "runtime.json"), f"{loki}:/tmp/runtime.json")
                    wait_for(lambda: [labels for name, labels, _ in metrics(3100)
                                      if name == "loki_runtime_config_hash"] != previous,
                             "Loki runtime fault configuration")
                    self.assertEqual(value(metrics(3100), "loki_runtime_config_last_reload_successful"), 1)

                def stored_lines(age=0):
                    query = urlencode({"query": '{namespace="fixture"}', "limit": 100,
                                       "start": str(int((time.time() - age - 600) * 1e9)),
                                       "end": str(int((time.time() - age + 60) * 1e9))})
                    status, body = http(3100, "/loki/api/v1/query_range?" + query)
                    self.assertEqual(status, 200, body)
                    return [line for stream in json.loads(body)["data"]["result"] for _, line in stream["values"]]

                wait_for(lambda: http(3100, "/ready")[0] == 200 and http(12345, "/-/ready")[0] == 200,
                         "both processes ready", seconds=45)
                quiet = metrics(12345)
                self.assertGreater(value(quiet, "alloy_resources_process_start_time_seconds"), 0)
                self.assertEqual(value(quiet, "alloy_config_last_load_successful"), 1)
                self.assertEqual(value(quiet, "alloy_build_info", version="v1.11.2"), 1)
                self.assertEqual(value(metrics(3100), "loki_build_info", version="3.6.7"), 1)
                self.assertGreater(value(metrics(3100), "process_start_time_seconds"), 0)
                self.assertEqual(value(quiet, "loki_write_dropped_entries_total"), 0)
                self.assertEqual(value(quiet, "loki_write_batch_retries_total"), 0)

                emit('{"logger":"instance-manager","msg":"intentional chatter"}', database=True)
                wait_for(lambda: value(metrics(12345), "loki_process_dropped_lines_total",
                                       reason="cnpg_instance_manager") == 1, "intentional CNPG filtering")
                self.assertEqual(value(metrics(12345), "loki_write_dropped_entries_total"), 0)
                self.assertEqual(stored_lines(), [])

                emit("healthy-before-fault")
                wait_for(lambda: "healthy-before-fault" in stored_lines(), "initial healthy delivery")
                baseline = metrics(12345)
                self.assertEqual(value(baseline, "loki_write_sent_entries_total"), 1)

                block_delivery(True)
                emit("transient-retry-delivered")
                wait_for(lambda: value(metrics(12345), "loki_write_batch_retries_total") > 0, "retriable 503")
                self.assertEqual(http(3100, "/ready")[0], 200)
                block_delivery(False)
                wait_for(lambda: "transient-retry-delivered" in stored_lines(), "buffered line after recovery")
                recovered = metrics(12345)
                self.assertEqual(value(recovered, "loki_write_sent_entries_total"), 2)
                self.assertEqual(value(recovered, "loki_write_dropped_entries_total"), 0)

                # Timestamp rejection is real Loki validation while readiness
                # stays green. Alloy must not retry this permanent HTTP 400.
                retries = value(recovered, "loki_write_batch_retries_total")
                emit("permanently-rejected-old-line", age=8 * 24 * 3600)
                wait_for(lambda: value(metrics(12345), "loki_write_dropped_entries_total") == 1, "permanent rejection")
                rejected = metrics(12345)
                self.assertEqual(value(rejected, "loki_write_batch_retries_total"), retries)
                self.assertEqual(value(rejected, "loki_write_dropped_entries_total", reason="ingester_error"), 1)
                self.assertEqual(value(metrics(3100), "loki_discarded_samples_total",
                                       reason="greater_than_max_sample_age"), 1)
                self.assertEqual(stored_lines(age=8 * 24 * 3600), [])
                self.assertEqual(http(3100, "/ready")[0], 200)
                self.assertEqual(http(12345, "/-/ready")[0], 200)
                self.assertEqual(value(rejected, "alloy_config_last_load_successful"), 1)

                block_delivery(True)
                emit("exhausted-retries-lost")
                wait_for(lambda: value(metrics(12345), "loki_write_dropped_entries_total") == 2, "exhausted retries")
                exhausted = metrics(12345)
                self.assertEqual(value(exhausted, "loki_write_batch_retries_total") - retries, 10)
                self.assertEqual(value(exhausted, "loki_write_sent_entries_total"), 2)
                block_delivery(False)
                emit("healthy-after-loss")
                wait_for(lambda: "healthy-after-loss" in stored_lines(), "delivery after final loss")
                self.assertCountEqual(stored_lines(), ["healthy-before-fault", "transient-retry-delivered", "healthy-after-loss"])
                final = metrics(12345)
                self.assertEqual(value(final, "loki_write_sent_entries_total"), 3)
                self.assertEqual(value(final, "loki_write_dropped_entries_total"), 2,
                                 "recovery must not claim that discarded batches were recovered")
                self.assertEqual(value(final, "alloy_resources_process_start_time_seconds"),
                                 value(quiet, "alloy_resources_process_start_time_seconds"))
                for name, labels, _ in final:
                    if name.startswith("loki_write_"):
                        self.assertLessEqual(set(labels), {"component_id", "component_path", "host", "tenant", "reason", "status_code", "le"})
                        self.assertEqual(labels.get("component_id"), "loki.write.default")
                for name, labels, _ in metrics(3100):
                    if name == "loki_discarded_samples_total":
                        self.assertLessEqual(set(labels), {"reason", "tenant", "retention_hours", "policy", "format"})

                # A real process restart resets transport counters. Subsequent
                # delivery works, but previously discarded batches do not replay.
                run("docker", "restart", alloy)
                wait_for(lambda: http(12345, "/-/ready")[0] == 200, "collector restart")
                restarted = metrics(12345)
                self.assertGreater(value(restarted, "alloy_resources_process_start_time_seconds"),
                                   value(quiet, "alloy_resources_process_start_time_seconds"))
                self.assertEqual(value(restarted, "loki_write_dropped_entries_total"), 0)
                self.assertEqual(value(restarted, "loki_write_sent_entries_total"), 0)
                emit("healthy-after-restart")
                wait_for(lambda: "healthy-after-restart" in stored_lines(), "delivery after counter reset")
                self.assertCountEqual(stored_lines(), ["healthy-before-fault", "transient-retry-delivered",
                                                      "healthy-after-loss", "healthy-after-restart"])
                self.assertEqual(value(metrics(12345), "loki_write_sent_entries_total"), 1)
            except Exception:
                for name in created:
                    print(run("docker", "logs", name, stderr=subprocess.STDOUT)[-12000:])
                raise
            finally:
                for name in reversed(created):
                    run("docker", "rm", "-f", name)


if __name__ == "__main__":
    unittest.main()
