#!/usr/bin/env python3
"""Check Helm-rendered platform scrapes and independent inventories per environment."""

import json
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parent.parent


def yaml_objects(source, selector="."):
    result = subprocess.check_output(
        ["yq", "-o=json", "-I=0", selector], input=source, text=True, cwd=ROOT
    )
    return [json.loads(line) for line in result.splitlines() if line.strip()]


def main():
    with tempfile.TemporaryDirectory(prefix="bex-gitops-metrics-") as directory:
        temporary = Path(directory)
        chart = subprocess.check_output(
            ["bash", "scripts/helm-artifact.sh", "pull", "prometheus", directory],
            text=True,
            cwd=ROOT,
        ).strip()
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
            helm = app["spec"]["source"]["helm"]
            values_file = temporary / "values.yaml"
            values_file.write_text(helm["values"])
            override_file = temporary / "override.json"
            override_file.write_text(json.dumps(helm.get("valuesObject", {})))
            command = [
                "helm", "template", "prometheus", chart, "-n", "monitoring",
                "-f", str(values_file), "-f", str(override_file),
            ]
            for parameter in helm.get("parameters", []):
                command.extend([
                    "--set-string" if parameter.get("forceString") else "--set",
                    f'{parameter["name"]}={parameter["value"]}',
                ])
            resources = subprocess.check_output(command, text=True, cwd=ROOT)
            configmaps = yaml_objects(resources, 'select(.kind == "ConfigMap")')
            config = next(cm["data"] for cm in configmaps if cm["metadata"]["name"] == "prometheus-server")
            prometheus = yaml_objects(config["prometheus.yml"])[0]
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
            assert set(kept_labels) == {"__name__", "job", "instance", "node", "pod", "service"}, \
                "No repository, path, tenant or credential labels in Alloy metrics"
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
