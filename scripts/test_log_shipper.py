"""Exercise the Helm-rendered app pipeline in the chart's actual Alloy image.

Run: python3 -m unittest scripts/test_log_shipper.py -v
Requires Docker, Helm, and yq v4; no cluster or credentials.
"""

import json
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
        }
        for value, expected in {
            "err": "error", "fatal": "error", "panic": "error",
            "critical": "error", "crit": "error", "warn": "warn",
            "WARNING": "warn", "info": "info", "notice": "info",
            "DEBUG": "debug", "trace": "debug",
        }.items():
            for key in ("level", "severity"):
                cases[f'{key}={value} msg="normalization"'] = expected

        with tempfile.TemporaryDirectory(prefix="bex-log-levels-") as tmp:
            tmp = Path(tmp)
            chart = run("bash", "scripts/helm-artifact.sh", "pull", "alloy", str(tmp))
            values = tmp / "values.yaml"
            values.write_text(run("yq", "-r", ".spec.source.helm.values",
                                  "deploy/gitops/base/log-shipper.yaml"))
            rendered = tmp / "rendered.yaml"
            rendered.write_text(run("helm", "template", "log-shipper", chart,
                                    "-n", "monitoring", "-f", str(values)))
            config = run("yq", "-r", 'select(.kind == "ConfigMap") | .data["config.alloy"]', str(rendered))
            image = run("yq", "-r", 'select(.kind == "DaemonSet") | .spec.template.spec.containers[] | select(.name == "alloy") | .image', str(rendered))
            pipeline = re.search(r'^loki\.process "app_logs" \{\n.*?^\}', config, re.M | re.S)
            self.assertIsNotNone(pipeline, "rendered chart is missing the app pipeline")
            # Keep every production stage; replace only the input and sink.
            pipeline = pipeline.group().replace("loki.write.default.receiver", "loki.echo.test.receiver")
            (tmp / "config.alloy").write_text('''logging {
  format = "json"
}
loki.source.file "test" {
  targets = [{__path__ = "/tmp/lines.log"}]
  forward_to = [loki.process.app_logs.receiver]
}
loki.echo "test" {}
''' + pipeline)
            (tmp / "lines.log").write_text("\n".join(cases) + "\n")
            name = "bex-log-levels-" + uuid.uuid4().hex[:12]
            run("docker", "create", "--name", name, "--network", "none", image,
                "run", "--storage.path=/tmp/alloy", "/tmp/config.alloy")
            try:
                # docker cp also works when CI uses a separate Docker daemon.
                for filename in ("config.alloy", "lines.log"):
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
                        if entry.get("receiver") == "loki.echo.test":
                            labels = dict(re.findall(r'(\w+)="([^"\\]*)"', entry["labels"]))
                            self.assertEqual(labels.get("type"), "app")
                            actual[entry["entry"]] = labels.get("level")
                    if len(actual) >= len(cases):
                        break
                    if run("docker", "inspect", "-f", "{{.State.Running}}", name) != "true":
                        self.fail(f"Alloy exited before processing the fixture:\n{output}")
                    time.sleep(0.25)
                self.assertEqual(set(actual), set(cases), f"missing or changed log lines:\n{output}")
                for line, expected in cases.items():
                    with self.subTest(line=line):
                        self.assertEqual(actual[line], expected)
                self.assertEqual(set(actual.values()), {"error", "warn", "info", "debug", "unknown"})
            finally:
                run("docker", "rm", "-f", name)


if __name__ == "__main__":
    unittest.main()
