#!/usr/bin/env python3
"""Check Helm-rendered platform scrapes and independent inventories per environment."""

import io
import json
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile
import time


ROOT = Path(__file__).resolve().parent.parent


def yaml_objects(source, selector="."):
    result = subprocess.check_output(
        ["yq", "-o=json", "-I=0", selector], input=source, text=True, cwd=ROOT
    )
    return [json.loads(line) for line in result.splitlines() if line.strip()]


def render_application(app, chart, temporary):
    helm = app["spec"]["source"]["helm"]
    values_file = temporary / "values.yaml"
    # Argo chooses one inline representation; valuesObject replaces values.
    values_file.write_text(json.dumps(helm["valuesObject"]) if helm.get("valuesObject") is not None
                           else helm.get("values", ""))
    command = ["helm", "template", helm["releaseName"], chart, "-n", "monitoring",
               "-f", str(values_file)]
    for parameter in helm.get("parameters", []):
        command.extend(["--set-string" if parameter.get("forceString") else "--set",
                        f'{parameter["name"]}={parameter["value"]}'])
    return subprocess.check_output(command, text=True, cwd=ROOT)


def verify_log_workloads(apps, charts, temporary):
    alloy = next(app for app in apps if app["metadata"]["name"] == "log-shipper")
    objects = yaml_objects(render_application(alloy, charts["alloy"], temporary))
    daemonset = next(obj for obj in objects if obj["kind"] == "DaemonSet")
    pod = daemonset["spec"]["template"]["spec"]
    assert not pod.get("nodeSelector") and not pod.get("affinity"), \
        "Expected log collectors cover every node; update inventory if placement changes"
    assert {"operator": "Exists"} in pod["tolerations"]
    container = next(c for c in pod["containers"] if c["name"] == "alloy")
    assert any(port["name"] == "http-metrics" and port["containerPort"] == 12345
               for port in container["ports"])
    assert daemonset["spec"]["template"]["metadata"]["labels"]["app.kubernetes.io/name"] == "alloy"
    assert daemonset["spec"]["template"]["metadata"]["labels"]["app.kubernetes.io/instance"] == "log-shipper"

    loki = next(app for app in apps if app["metadata"]["name"] == "loki")
    objects = yaml_objects(render_application(loki, charts["loki"], temporary))
    statefulset = next(obj for obj in objects if obj["kind"] == "StatefulSet" and obj["metadata"]["name"] == "loki")
    assert statefulset["spec"]["replicas"] == 1, "A service scrape cannot account for multiple Loki replicas"
    service = next(obj for obj in objects if obj["kind"] == "Service" and obj["metadata"]["name"] == "loki")
    assert service["spec"]["type"] == "ClusterIP"
    assert any(port["port"] == 3100 for port in service["spec"]["ports"])
    config = next(obj["data"]["config.yaml"] for obj in objects
                  if obj["kind"] == "ConfigMap" and "config.yaml" in obj.get("data", {}))
    config = yaml_objects(config)[0]
    assert config["auth_enabled"] is False, "Scrape's fake tenant assumes single-tenant Loki"
    assert not config.get("limits_config", {}).get("policy_stream_mapping"), \
        "Loki scrape selects the default policy; update it when introducing named policies"
    runtime = next(obj["data"]["runtime-config.yaml"] for obj in objects
                   if obj["kind"] == "ConfigMap" and "runtime-config.yaml" in obj.get("data", {}))
    runtime = yaml_objects(runtime)[0] or {}
    assert all(not limits.get("policy_stream_mapping")
               for limits in runtime.get("overrides", {}).values()), \
        "Named runtime policies require updating Loki's scrape scope"


def verify_application_size(app, environment):
    # Argo retains inline source values in history plus comparedTo, operation
    # and syncResult. Last-applied is JSON inside a string, so include escaping.
    # Reserve 64KiB for other status/metadata and leave another 256KiB below
    # etcd's default 1.5MiB request limit; this is a growth budget, not wire size.
    history_limit = app["spec"].get("revisionHistoryLimit", 10)
    assert isinstance(history_limit, int) and history_limit >= 0
    application_json = json.dumps(app, separators=(",", ":"))
    source_bytes = len(json.dumps(app["spec"]["source"], separators=(",", ":")).encode())
    modeled_bytes = (
        len(application_json.encode()) + len(json.dumps(application_json).encode())
        + (history_limit + 3) * source_bytes + 64 * 1024
    )
    budget = 1280 * 1024
    assert modeled_bytes <= budget, (
        f"{environment}: Prometheus Application projects {modeled_bytes:,} bytes with "
        f"{history_limit} history entries, exceeding the {budget:,}-byte budget; "
        "reduce retained history or move inline values out of the Application"
    )
    print(f"PASS: {environment} Prometheus Application size budget ({modeled_bytes:,}/{budget:,} bytes)", flush=True)


def verify_log_metric_filtering(alloy, loki):
    """Exercise rendered keep/drop behavior with native and adversarial labels.

    Only keep and labelkeep are used here. These regexes share Python/RE2
    syntax; unknown relabel actions fail rather than being silently simulated.
    """
    def retained(job, labels):
        labels = dict(labels)
        for rule in job["metric_relabel_configs"]:
            if rule["action"] == "keep":
                value = rule.get("separator", ";").join(labels.get(k, "") for k in rule["source_labels"])
                if re.fullmatch(rule["regex"], value) is None:
                    return None
            elif rule["action"] == "labelkeep":
                labels = {k: v for k, v in labels.items() if re.fullmatch(rule["regex"], k)}
            else:
                raise AssertionError(f"Unexercised metric relabel action: {rule['action']}")
        return labels

    identity = {"job": "alloy", "instance": "pod:12345", "node": "node-a", "pod": "alloy-a"}
    writer = dict(identity, component_id="loki.write.default", component_path="/",
                  host="loki.monitoring.svc:3100", tenant="")
    for name in ("loki_write_sent_entries_total", "loki_write_batch_retries_total",
                 "loki_write_dropped_entries_total", "loki_write_request_duration_seconds_count"):
        labels = dict(writer, __name__=name, reason="ingester_error", status_code="400")
        assert retained(alloy, labels) == dict(identity, __name__=name, reason="ingester_error", status_code="400")
        for key, wrong in (("component_id", "loki.write.other"), ("component_path", "/nested"),
                           ("host", "other:3100"), ("tenant", "customer")):
            assert retained(alloy, dict(labels, **{key: wrong})) is None, (name, key)
    for health in ("healthy", "unhealthy", "unknown"):
        labels = dict(identity, __name__="alloy_component_controller_running_components",
                      controller_id="", controller_path="/", health_type=health)
        assert retained(alloy, labels) == dict(identity, __name__=labels["__name__"], health_type=health)
        assert retained(alloy, dict(labels, controller_id="nested")) is None
        assert retained(alloy, dict(labels, controller_path="/nested")) is None
    marker = dict(identity, __name__="alloy_config_last_load_successful")
    assert retained(alloy, marker) == marker
    for name in ("bex_zot_gc_failure_last_observed_timestamp_seconds", "bex_zot_log_last_observed_timestamp_seconds"):
        # Native stage.metrics identity, verified against the pinned image and
        # deployed endpoint; omitting these labels would miss a scrape regression.
        labels = dict(identity, __name__=name, component_id="loki.process.zot_gc_metrics",
                      component_path="/", service="zot", repository="unbounded")
        assert retained(alloy, labels) == dict(identity, __name__=name, service="zot")
    assert retained(alloy, dict(writer, __name__="loki_process_dropped_lines_total", reason="cnpg_instance_manager")) is None

    identity = {"job": "loki", "instance": "loki.monitoring.svc:3100"}
    rejection = dict(identity, __name__="loki_discarded_samples_total", tenant="fake", policy="",
                     reason="greater_than_max_sample_age", retention_hours="504", format="loki")
    assert retained(loki, rejection) == {k: v for k, v in rejection.items() if k not in {"tenant", "policy"}}
    assert retained(loki, dict(rejection, tenant="customer")) is None
    assert retained(loki, dict(rejection, policy="new-policy")) is None
    # Distinct native dimensions must not collapse to duplicate sample identities.
    assert retained(loki, dict(rejection, retention_hours="24")) != retained(loki, rejection)
    assert retained(loki, dict(rejection, format="otlp")) != retained(loki, rejection)
    for name in ("process_start_time_seconds", "loki_ingester_wal_disk_full_failures_total"):
        labels = dict(identity, __name__=name)
        assert retained(loki, labels) == labels
    for internal in ("true", "false"):
        labels = dict(identity, __name__="loki_distributor_lines_received_total", tenant="fake",
                      policy="", format="loki", is_internal_stream=internal)
        assert retained(loki, labels) == {k: v for k, v in labels.items() if k not in {"tenant", "policy"}}
    request = dict(identity, __name__="loki_request_duration_seconds_count", route="loki_api_v1_push",
                   method="POST", status_code="500", ws="false")
    assert retained(loki, request) == dict(identity, __name__=request["__name__"], status_code="500")
    assert retained(loki, dict(request, route="loki_api_v1_query_range")) is None
    assert retained(loki, dict(identity, __name__="loki_ingester_wal_discarded_samples_total", reason="duplicate")) is None


def verify_filesystem_collector(resources, prometheus, environment):
    objects = yaml_objects(resources)
    daemonset = next(obj for obj in objects if obj["kind"] == "DaemonSet"
                     and obj["metadata"]["name"] == "prometheus-prometheus-node-exporter")
    pod = daemonset["spec"]["template"]["spec"]
    assert pod["nodeSelector"] == {"kubernetes.io/os": "linux", "bex.co/pool": "tenant"}
    assert pod["tolerations"] == [{
        "key": "bex.co/build-only", "operator": "Equal", "value": "true", "effect": "NoSchedule",
    }], "Build nodes are in scope; platform/control-plane taints must not be tolerated"
    assert not any(pod.get(flag, False) for flag in ("hostNetwork", "hostPID", "hostIPC"))
    assert pod["automountServiceAccountToken"] is False
    assert pod["securityContext"]["runAsNonRoot"] is True
    assert len(pod["containers"]) == 1
    container = pod["containers"][0]
    assert all(not port.get("hostPort") for port in container["ports"])
    security = container["securityContext"]
    assert security["readOnlyRootFilesystem"] and not security["allowPrivilegeEscalation"]
    assert security["capabilities"] == {"drop": ["ALL"]}
    assert container["resources"] == {
        "requests": {"cpu": "10m", "memory": "32Mi"},
        "limits": {"cpu": "100m", "memory": "64Mi"},
    }
    hosts = {volume["name"]: volume["hostPath"]["path"] for volume in pod["volumes"]}
    assert hosts == {"proc": "/proc", "sys": "/sys", "root": "/"}
    mounts = {mount["name"]: mount for mount in container["volumeMounts"]}
    assert all(mount["readOnly"] for mount in mounts.values())
    assert mounts["root"]["mountPropagation"] == "HostToContainer"
    assert mounts["proc"]["mountPath"] == "/host/proc"
    assert mounts["root"]["mountPath"] == "/host/root"
    arguments = container["args"]
    assert "--collector.disable-defaults" in arguments
    assert [arg for arg in arguments if arg.startswith("--collector.") and "=" not in arg] == [
        "--collector.disable-defaults", "--collector.filesystem",
    ], "Do not enable an unrelated host collector suite"
    assert "--web.disable-exporter-metrics" in arguments
    fs_filter = next(arg.split("=", 1)[1] for arg in arguments
                     if arg.startswith("--collector.filesystem.fs-types-exclude="))
    assert bool(re.search(fs_filter, "overlay")) == (environment == "prod")
    services = [obj for obj in objects if obj["kind"] == "Service"
                and obj["metadata"]["name"] == "prometheus-prometheus-node-exporter"]
    assert len(services) == 1 and services[0]["spec"]["type"] == "ClusterIP"
    assert not any(obj["kind"] == "Ingress" and "node-exporter" in str(obj) for obj in objects)
    ksm = next(obj for obj in objects if obj["kind"] == "Deployment"
               and obj["metadata"]["name"] == "prometheus-kube-state-metrics")
    assert "--metric-labels-allowlist=nodes=[bex.co/pool]" in ksm["spec"]["template"]["spec"]["containers"][0]["args"]

    jobs = [job for job in prometheus["scrape_configs"] if job["job_name"] == "tenant-node-filesystem"]
    assert len(jobs) == 1
    job = jobs[0]
    assert job["scrape_interval"] == "60s"
    assert job["kubernetes_sd_configs"] == [{"role": "pod", "namespaces": {"names": ["monitoring"]}}]
    target_rules = job["relabel_configs"]
    assert any(rule.get("action") == "keep" and rule.get("regex") ==
               "prometheus-node-exporter;prometheus;node-exporter;metrics" for rule in target_rules)
    assert {"source_labels": ["__meta_kubernetes_pod_node_name"], "target_label": "node"} in target_rules
    metric_rules = job["metric_relabel_configs"]
    kept_labels = next(rule["regex"] for rule in metric_rules if rule["action"] == "labelkeep")
    assert set(kept_labels.split("|")) == {
        "__name__", "job", "instance", "node", "mountpoint", "device", "fstype", "collector",
    }, "Filesystem identity must survive without tenant/path/error-message labels"
    # Exercise the rendered regex against independently chosen mount shapes.
    # These RE2 expressions use only syntax shared with Python's fullmatch.
    filters = [rule for rule in metric_rules if rule["action"] == "keep"]
    cases = [
        ("node_filesystem_avail_bytes", "ext4", "/", True),
        ("node_filesystem_files_free", "xfs", "/var/lib/containerd", True),
        ("node_filesystem_size_bytes", "ext4", "/var/lib/kubelet", True),
        ("node_filesystem_readonly", "xfs", "/var", True),
        ("node_filesystem_device_error", "ext4", "/var/lib", True),
        ("node_filesystem_avail_bytes", "overlay", "/", True),
        ("node_scrape_collector_success", "", "", True),
        ("node_filesystem_size_bytes", "tmpfs", "/", False),
        ("node_filesystem_size_bytes", "ext4", "/boot", False),
        ("node_filesystem_size_bytes", "ext4", "/var/lib/kubelet/pods/uid/volumes/pvc", False),
        ("node_filesystem_size_bytes", "overlay", "/var/lib/containerd/snapshots/1/fs", False),
        ("node_cpu_seconds_total", "", "", False),
    ]
    for name, fstype, mountpoint, expected in cases:
        labels = {"__name__": name, "fstype": fstype, "mountpoint": mountpoint}
        kept = all(re.fullmatch(rule["regex"], rule.get("separator", ";").join(
            labels.get(label, "") for label in rule["source_labels"])) for rule in filters)
        assert bool(kept) == expected, f"{environment}: wrong scrape filtering for {labels}"
    return container


def exercise_filesystem_binary(container, environment, temporary):
    """Use fake mount metadata and real statfs, without mounting the host disk.

    The fixed image reads /proc/1/mounts even without hostPID. This fixture
    demonstrates the declared collector filters, inode/byte metrics, deduping,
    and shared/separate/missing backing filesystems; it does not claim a host's
    live topology. Promtool fixtures separately test the required-mount alerts.
    """
    fixture = temporary / "filesystem-fixture"
    (fixture / "proc/1").mkdir(parents=True, exist_ok=True)
    root = fixture / "root"
    for mount in ("var/lib/containerd", "var/lib/kubelet/pods/uid/volumes/pvc",
                  "var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs",
                  "run", "boot"):
        (root / mount).mkdir(parents=True, exist_ok=True)
    fixture.chmod(0o755)
    arguments = [arg.replace("/host/", "/fixture/").replace("[$(HOST_IP)]", "127.0.0.1")
                 for arg in container["args"]]
    noise = [
        "tmpfs /run tmpfs rw 0 0",
        "tmpfs /var tmpfs rw 0 0",
        "/dev/pvc /var/lib/kubelet/pods/uid/volumes/pvc ext4 rw 0 0",
        "overlay /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs overlay rw 0 0",
    ]
    cases = [
        ("shared root", ["/dev/root / ext4 rw 0 0"] * 2, {"/"}),
        ("separate imagefs", ["/dev/root / ext4 rw 0 0", "/dev/image /var/lib/containerd xfs rw 0 0"],
         {"/", "/var/lib/containerd"}),
        ("required imagefs disappeared", ["/dev/root / ext4 rw 0 0"], {"/"}),
        ("CAPD overlay root", ["overlay / overlay rw 0 0"], {"/"} if environment == "local" else set()),
    ]
    for name, mounts, expected in cases:
        (fixture / "proc/1/mounts").write_text("\n".join(mounts + noise) + "\n")
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w") as tar:
            tar.add(fixture, arcname=".")
        identifier = subprocess.check_output([
            "docker", "create", "--network", "none", "--read-only", "--user", "65534:65534",
            "--tmpfs", "/fixture:rw,noexec,nosuid,size=1m,mode=0755,uid=65534,gid=65534",
            "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
            container["image"], *arguments,
        ], text=True).strip()
        try:
            subprocess.run(["docker", "start", identifier], check=True, stdout=subprocess.DEVNULL)
            subprocess.run(["docker", "exec", "-i", identifier, "tar", "xf", "-", "-C", "/fixture"],
                           input=archive.getvalue(), check=True)
            deadline = time.monotonic() + 10
            while True:
                response = subprocess.run([
                    "docker", "exec", identifier, "wget", "-qO-", "http://127.0.0.1:9100/metrics",
                ], capture_output=True, text=True)
                if response.returncode == 0:
                    break
                if time.monotonic() >= deadline:
                    logs = subprocess.check_output(["docker", "logs", identifier], stderr=subprocess.STDOUT, text=True)
                    raise AssertionError(f"{environment}/{name}: exporter failed to start: {logs}")
                time.sleep(0.1)
            metrics = response.stdout
            assert 'node_scrape_collector_success{collector="filesystem"} 1' in metrics
            collectors = re.findall(r'node_scrape_collector_success\{collector="([^"]+)"\}', metrics)
            assert collectors == ["filesystem"], f"Unrelated collectors enabled: {collectors}"
            assert not re.search(r"^(go_|process_|node_cpu_|node_memory_|node_network_)", metrics, re.M)
            for metric in ("size_bytes", "avail_bytes", "files", "files_free"):
                samples = re.findall(rf'^node_filesystem_{metric}\{{[^\n]*mountpoint="([^"]+)"[^\n]*\}} (\S+)$', metrics, re.M)
                assert {mount for mount, _ in samples} == expected, f"{environment}/{name}: {metric}: {samples}"
                assert len(samples) == len(expected), f"Duplicate mount samples: {samples}"
                assert all(float(value) > 0 for _, value in samples), f"statfs did not yield real {metric}"
            print(f"PASS: {environment} locked filesystem collector: {name}", flush=True)
        finally:
            subprocess.run(["docker", "rm", "-f", identifier], check=True, stdout=subprocess.DEVNULL)


def main():
    with tempfile.TemporaryDirectory(prefix="bex-gitops-metrics-") as directory:
        temporary = Path(directory)
        charts = {name: subprocess.check_output(
            ["bash", "scripts/helm-artifact.sh", "pull", name, directory], text=True, cwd=ROOT,
        ).strip() for name in ("prometheus", "alloy", "loki")}
        for environment in ("prod", "local"):
            rendered = subprocess.check_output(
                ["kubectl", "kustomize", f"deploy/gitops/overlays/{environment}"],
                text=True,
                cwd=ROOT,
            )
            apps = yaml_objects(rendered, 'select(.kind == "Application")')
            bootstrap = yaml_objects(
                (ROOT / f"deploy/gitops/bootstrap/{environment}.yaml").read_text(),
                'select(.kind == "Application")',
            )
            expected = sorted(
                (app["metadata"]["namespace"], app["metadata"]["name"])
                for app in apps + bootstrap
            )
            app = next(app for app in apps if app["metadata"]["name"] == "prometheus")
            verify_application_size(app, environment)
            resources = render_application(app, charts["prometheus"], temporary)
            verify_log_workloads(apps, charts, temporary)
            configmaps = yaml_objects(resources, 'select(.kind == "ConfigMap")')
            config = next(cm["data"] for cm in configmaps if cm["metadata"]["name"] == "prometheus-server")
            prometheus = yaml_objects(config["prometheus.yml"])[0]
            assert prometheus["global"]["evaluation_interval"] == "1m", \
                "Filesystem forecast history guard requires one-minute rule evaluations"
            collector = verify_filesystem_collector(resources, prometheus, environment)
            if "--collector" in sys.argv:
                exercise_filesystem_binary(collector, environment, temporary)
            assert "/etc/config/platform_gitops_expected.yml" in prometheus["rule_files"]
            jobs = [job for job in prometheus["scrape_configs"] if job["job_name"] == "argocd-applications"]
            assert len(jobs) == 1, "Argo needs exactly one private scrape"
            assert jobs[0]["static_configs"] == [{"targets": ["argocd-metrics.argocd.svc:8082"]}]
            assert jobs[0]["metric_relabel_configs"] == [
                {"source_labels": ["__name__"], "regex": "argocd_app_info", "action": "keep"},
                {"regex": "__name__|job|instance|namespace|name|sync_status|health_status", "action": "labelkeep"},
            ], "Keep application state without repository/revision labels"
            alloy_jobs = [job for job in prometheus["scrape_configs"] if job["job_name"] == "alloy"]
            assert len(alloy_jobs) == 1, "Alloy needs one private per-pod scrape"
            alloy = alloy_jobs[0]
            assert alloy["kubernetes_sd_configs"] == [{
                "role": "pod", "namespaces": {"names": ["monitoring"]},
            }], "Alloy discovery must remain scoped to monitoring pods"
            assert alloy["scrape_interval"] == "30s", "Observe short-lived log evidence promptly"
            # The chart's config-reloader is a second container; keeping the
            # named Alloy port avoids duplicate samples from per-port discovery.
            assert any(rule.get("action") == "keep" and rule.get("regex") == "alloy;log-shipper;alloy;http-metrics"
                       for rule in alloy["relabel_configs"]), "Only scrape the intended collector and port"
            kept_labels = next(rule["regex"] for rule in alloy["metric_relabel_configs"]
                               if rule["action"] == "labelkeep").split("|")
            assert set(kept_labels) == {
                "__name__", "job", "instance", "node", "pod", "service", "reason", "status_code", "health_type",
            }, "No repository, path, tenant or credential labels in Alloy metrics"
            loki_jobs = [job for job in prometheus["scrape_configs"] if job["job_name"] == "loki"]
            assert len(loki_jobs) == 1
            loki = loki_jobs[0]
            assert loki["static_configs"] == [{"targets": ["loki.monitoring.svc:3100"]}]
            assert loki["scrape_interval"] == "30s"
            assert set(next(rule["regex"] for rule in loki["metric_relabel_configs"]
                            if rule["action"] == "labelkeep").split("|")) == {
                "__name__", "job", "instance", "reason", "retention_hours", "format", "is_internal_stream", "status_code",
            }
            verify_log_metric_filtering(alloy, loki)
            (temporary / "platform_gitops_expected.yml").write_text(config["platform_gitops_expected.yml"])
            fixture = {
                "rule_files": ["platform_gitops_expected.yml"],
                "evaluation_interval": "1m",
                "tests": [{
                    "name": f"{environment}: expected inventory survives total exporter absence",
                    "interval": "1m",
                    "promql_expr_test": [{
                        "expr": "bex:platform_gitops_expected",
                        "eval_time": "0m",
                        "exp_samples": [{
                            "labels": f'bex:platform_gitops_expected{{namespace="{namespace}",name="{name}"}}',
                            "value": 1,
                        } for namespace, name in expected],
                    }],
                }],
            }
            # Database identity comes from the declared Cluster objects, not
            # the observed scrape or the recording rules under test. Local
            # storage/backup differences do not disable pg_stat_archiver.
            databases = []
            for database_app in apps:
                if database_app["metadata"]["name"] not in {"auth-dbs", "bex-postgres"}:
                    continue
                clusters = subprocess.check_output(
                    ["kubectl", "kustomize", database_app["spec"]["source"]["path"]],
                    text=True, cwd=ROOT,
                )
                databases.extend(yaml_objects(clusters, 'select(.kind == "Cluster")'))
            assert databases, "Platform Cluster inventory must not be empty"
            (temporary / "alerting_rules.yml").write_text(config["alerting_rules.yml"])
            fixture["rule_files"].append("alerting_rules.yml")
            fixture["tests"][0]["promql_expr_test"].append({
                "expr": "bex:platform_database_backup_expected",
                "eval_time": "0m",
                "exp_samples": [{
                    "labels": 'bex:platform_database_backup_expected{namespace="%s",cnpg_io_cluster="%s"}'
                    % (cluster["metadata"]["namespace"], cluster["metadata"]["name"]),
                    "value": 1,
                } for cluster in databases],
            })
            (temporary / "inventory_test.yml").write_text(json.dumps(fixture))
            subprocess.run(["promtool", "test", "rules", "inventory_test.yml"], cwd=temporary, check=True)
            print(f"PASS: {environment} private scrape, {len(expected)} Application identities and {len(databases)} database identities", flush=True)


if __name__ == "__main__":
    main()
