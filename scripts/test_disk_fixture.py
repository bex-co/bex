"""Ownership refusal, fixed node protocol and exact backup provenance regressions."""

import copy
from datetime import datetime, timedelta, timezone
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("disk_fixture", Path(__file__).with_name("disk_fixture.py"))
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)
IMAGE = "ghcr.io/bex-co/bex-operator@sha256:" + "a" * 64


def uid(number):
    return "00000000-0000-0000-0000-" + str(number).zfill(12)


def proof():
    current = datetime.now(timezone.utc)
    name = "qa-" + current.strftime("%Y%m%d") + "-w8-m144-0123456789abcdef"
    result = {
        "schema": "bex-m144-fixture-v1", "run": "0123456789abcdef", "service_name": name,
        "disk_name": name + "-disk", "workspace": "tea-owned", "namespace": "tea-owned",
        "service_id": "srv-new", "disk_id": "dsk-new", "app_name": "tea-owned-" + name,
        "app_uid": uid(1), "pvc_name": "disk-truncated-hash", "pvc_uid": uid(2),
        "created_at": current.isoformat(), "expires_at": (current + timedelta(hours=2)).isoformat(),
        "baseline_ids": ["srv-foreign"], "baseline_disks": ["dsk-foreign"],
        "budget": {"max_services": 1, "max_disks": 1, "max_size_gib": 5, "max_runtime_seconds": 7200, "max_estimated_usage_usd": 0.03},
    }
    result["create_proof"] = {"HTTP": 201, "service_id": result["service_id"], "workspace": result["workspace"], "service_name": name, "disk_name": result["disk_name"], "initial_size_gib": 1, "mount_path": "/var/data", "plan": "starter", "request_body_sha256": "b" * 64}
    return result


def row(kind, **values):
    return {key: values.get(key, "") for key in fixture.FIELDS[kind]}


def owner(kind, name, number):
    return [{"apiVersion": "app.bex.co/v1alpha1" if kind == "App" else "batch/v1", "kind": kind, "name": name, "uid": uid(number), "controller": True}]


class FakeKube:
    def __init__(self, target, size=1):
        self.target, self.size, self.calls, self.job = target, size, [], None
        name = target["app_name"]
        self.rows = {
            "app": row("app", name=name, uid=uid(1), service="srv-new", workspace="tea-owned", diskName=target["disk_name"], mount="/var/data", size=str(size)),
            "pvc": row("pvc", name=target["pvc_name"], uid=uid(2), owner=f"App/{name}/{uid(1)}", **{"class": fixture.SC}, request=f"{size}Gi", capacity=f"{size}Gi", phase="Bound", pv="pvc-own"),
            "pv": row("pv", name="pvc-own", uid=uid(3), **{"class": fixture.SC}, namespace="tea-owned", claim=target["pvc_name"], claimUid=uid(2), driver="csi.hetzner.cloud", handle="bounded-ext4-v1:42", capacity=f"{size}Gi"),
            "class": row("class", name=fixture.SC, uid=uid(4), provisioner="csi.hetzner.cloud", mode="ext4-v1", filesystem="ext4", publishSecret="${pvc.name}-luks", expandSecret="${pvc.name}-luks"),
            "secret": row("secret", name=target["pvc_name"] + "-luks", uid=uid(5), owner=f"App/{name}/{uid(1)}"),
            "cronjob": row("workload", name="dskbak-truncated-hash", uid=uid(6), owner=f"App/{name}/{uid(1)}"),
            "deployment": row("workload", name="own-service", uid=uid(7), owner=f"App/{name}/{uid(1)}"),
            "replicaset": row("workload", name="own-replicaset", uid=uid(8), owner=f"Deployment/own-service/{uid(7)}"),
            "servicepod": row("pod", name="own-service-pod", uid=uid(9), owner=f"ReplicaSet/own-replicaset/{uid(8)}", node="own-node", phase="Running", ready="True", claims=target["pvc_name"] + ";", started=target["created_at"]),
            "attachment": row("attachment", name="own-attachment", uid=uid(10), pv="pvc-own", node="own-node", attacher="csi.hetzner.cloud", attached="true", device="/dev/disk/by-id/scsi-0HC_Volume_42"),
            "daemonset": row("workload", name="own-csi", uid=uid(11)),
            "csipod": row("pod", name="own-csi-pod", uid=uid(12), owner=f"DaemonSet/own-csi/{uid(11)}", node="own-node", phase="Running", ready="True", driverReady="true", image=IMAGE, imageId=IMAGE, containerId="containerd://own-csi"),
        }
        container = {"name": "snapshot", "image": IMAGE, "command": ["/disk-snapshot"], "args": ["backup"], "volumeMounts": [{"name": "disk", "mountPath": "/disk", "readOnly": True}], "env": [{"name": "BEX_DISK_WORKSPACE", "value": target["workspace"]}, {"name": "BEX_DISK_ID", "value": name}]}
        self.cron = {"metadata": {"name": "dskbak-truncated-hash", "uid": uid(6), "ownerReferences": owner("App", name, 1)}, "spec": {"jobTemplate": {"metadata": {"labels": {"app.kubernetes.io/name": "disk-backup"}}, "spec": {"backoffLimit": 2, "template": {"spec": {"containers": [container], "volumes": [{"name": "disk", "persistentVolumeClaim": {"claimName": target["pvc_name"]}}], "restartPolicy": "Never"}}}}}}
        self.inflight = []
        self.ambiguous_create = False
        self.post_create_policy_drift = False

    def project(self, resource, kind, name=None, namespace=None, selector=None, allow_missing=False, field_selector=None):
        self.calls.append(("project", resource, kind, name, namespace, selector, allow_missing, field_selector))
        singular = {"apps.app.bex.co": "app", "persistentvolumeclaim": "pvc", "persistentvolume": "pv", "storageclass": "class", "secret": "secret", "cronjob": "cronjob"}
        if resource in singular:
            return copy.deepcopy(self.rows[singular[resource]])
        plural = {"cronjobs": "cronjob", "deployments": "deployment", "replicasets": "replicaset", "volumeattachments": "attachment", "daemonsets": "daemonset"}
        if resource == "jobs":
            return [row("job_status", completed=job.get("status", {}).get("completionTime", ""), failed="True" if any(c.get("type") == "Failed" and c.get("status") == "True" for c in job.get("status", {}).get("conditions", [])) else "") for job in self.inflight]
        if resource == "pods":
            return [copy.deepcopy(self.rows["csipod" if namespace == "kube-system" else "servicepod"])]
        return [copy.deepcopy(self.rows[plural[resource]])]

    def cronjob(self, name, namespace):
        self.calls.append(("cronjob", name, namespace))
        return copy.deepcopy(self.cron)

    def run(self, args, body=None, timeout=30):
        self.calls.append(("run", args, body))
        if "exec" in args:
            program = args[args.index("--") + 1]
            if program == "cryptsetup":
                return "/dev/mapper/scsi-0HC_Volume_42 is active and is in use.\n  type: LUKS1\n  device: /dev/sdb\n  sector size: 512 [bytes]\n  offset: 4096 [512-byte units] (2097152 [bytes])\n  size: 20967424 [512-byte units] (10735321088 [bytes])\n"
            if program == "blkid":
                return "crypto_LUKS\n"
            if program == "dumpe2fs":
                return f"Block count: {self.size * 262144}\nBlock size: 4096\nFilesystem volume name: PRIVATE_CANARY_NOT_RETAINED\n"
            raise AssertionError("unexpected executable")
        if args[:2] == ["get", "--raw"]:
            return '\n'.join(f'kubelet_volume_stats_{key}_bytes{{namespace="tea-owned",persistentvolumeclaim="{self.target["pvc_name"]}"}} {value}' for key, value in (("used", 1024), ("capacity", self.size * 1024**3 * 0.95), ("available", self.size * 1024**3 * 0.9))) + '\nkubelet_volume_stats_used_bytes{namespace="foreign",persistentvolumeclaim="private-canary"} 99999\n'
        if "create" in args:
            self.job = json.loads(body)
            self.job["metadata"]["uid"] = uid(13)
            if self.post_create_policy_drift:
                self.job["spec"]["template"]["spec"]["containers"][0]["image"] = "private-canary:unreviewed"
            if self.ambiguous_create:
                raise ValueError("simulated transport lost after server POST")
            return json.dumps(self.job)
        if "delete" in args:
            return "{}"
        if "job" in args:
            if "jsonpath={.metadata.uid}" in args:
                return self.job["metadata"]["uid"] if self.job else ""
            return json.dumps(self.job) if self.job else ""
        raise AssertionError("unexpected kubectl protocol")


def bound_fixture(size=1):
    target = proof()
    kube = FakeKube(target, size)
    target["binding"] = fixture.run_action(kube, target, "bind", "initial-1g" if size == 1 else "grown-5g", IMAGE)["candidateBinding"]
    target["snapshot_not_before"] = target["created_at"]
    kube.calls.clear()
    return target, kube


def dangerous_calls(kube):
    return [call for call in kube.calls if call[0] == "run" and any(op in call[1] for op in ("exec", "create", "delete"))]


class ProofTests(unittest.TestCase):
    def test_proof_requires_real_creation_unique_ids_utc_date_and_two_hour_budget(self):
        original = proof()
        fixture.validate_proof(original)
        changes = [lambda p: p["create_proof"].update(HTTP=200), lambda p: p["baseline_ids"].append(p["service_id"]), lambda p: p.update(app_uid=""), lambda p: p.update(service_name="existing-customer"), lambda p: p.update(expires_at=(fixture.instant(p["created_at"]) + timedelta(hours=24)).isoformat()), lambda p: p["budget"].update(max_services=2)]
        for change in changes:
            target = copy.deepcopy(original)
            change(target)
            with self.subTest(change=change), self.assertRaises((ValueError, KeyError)):
                fixture.validate_proof(target)

    def test_expired_proof_only_authorizes_cleanup(self):
        target = proof()
        later = fixture.instant(target["created_at"]) + timedelta(hours=3)
        with self.assertRaisesRegex(ValueError, "expired"):
            fixture.validate_proof(target, later)
        fixture.validate_proof(target, later, cleanup=True)

    def test_missing_untracked_or_dirty_proof_refused_before_kubernetes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(fixture, "command") as execute, self.assertRaisesRegex(ValueError, "absent"):
                fixture.load_proof(root)
            execute.assert_not_called()
            path = root / fixture.PROOF
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps(proof()))
            for failure in (0, 1):
                def call(args, **kwargs):
                    if ("ls-files" in args and failure == 0) or ("diff" in args and failure == 1):
                        raise ValueError("not committed and clean")
                    return ""
                with patch.object(fixture, "command", side_effect=call), self.assertRaises(ValueError):
                    fixture.load_proof(root)


class InspectTests(unittest.TestCase):
    def test_live_geometry_and_matching_node_metrics_use_only_fixed_commands(self):
        target, kube = bound_fixture()
        result = fixture.run_action(kube, target, "inspect", "initial-1g", IMAGE)
        self.assertEqual(result["result"]["geometry"]["filesystemBytes"], 1024**3)
        self.assertEqual(result["result"]["kubeletMetrics"]["bytes"]["used"], 1024)
        encoded = json.dumps(result)
        self.assertNotIn("PRIVATE_CANARY", encoded)
        self.assertNotIn("private-canary", encoded)
        commands = [call[1][call[1].index("--") + 1:] for call in dangerous_calls(kube)]
        self.assertEqual(commands, [["cryptsetup", "status", "scsi-0HC_Volume_42"], ["blkid", "-p", "-o", "value", "-s", "TYPE", "/dev/disk/by-id/scsi-0HC_Volume_42"], ["dumpe2fs", "-h", "/dev/mapper/scsi-0HC_Volume_42"]])
        self.assertTrue(all(".data" not in expression for expression in fixture.FIELDS["secret"].values()))

    def test_wrong_ownership_binding_device_or_driver_refuses_before_exec(self):
        for key, field, value in (("app", "uid", uid(99)), ("app", "service", "srv-foreign"), ("pvc", "uid", uid(99)), ("pvc", "owner", "App/foreign/" + uid(99)), ("pv", "claimUid", uid(99)), ("pv", "uid", uid(99)), ("attachment", "device", "/dev/disk/by-id/scsi-0HC_Volume_99"), ("servicepod", "owner", "Job/foreign/" + uid(99)), ("csipod", "image", "foreign:latest"), ("csipod", "imageId", "containerd://sha256:foreign")):
            target, kube = bound_fixture()
            kube.rows[key][field] = value
            with self.subTest(key=key, field=field), self.assertRaises(ValueError):
                fixture.run_action(kube, target, "inspect", "initial-1g", IMAGE)
            self.assertEqual(dangerous_calls(kube), [])

    def test_unreviewed_binding_and_template_changes_refuse_exec_or_create(self):
        target, kube = bound_fixture(5)
        del target["binding"]
        with self.assertRaisesRegex(ValueError, "review and commit"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        self.assertEqual(dangerous_calls(kube), [])
        target, kube = bound_fixture(5)
        kube.cron["spec"]["jobTemplate"]["spec"]["backoffLimit"] = 3
        with self.assertRaisesRegex(ValueError, "template"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        self.assertEqual(dangerous_calls(kube), [])

    def test_geometry_and_metrics_do_not_accept_provider_floor_or_foreign_series(self):
        target, kube = bound_fixture()
        status = kube.run(["exec", "--", "cryptsetup"])
        with self.assertRaisesRegex(ValueError, "geometry"):
            fixture.parse_geometry(status, "crypto_LUKS", "Block count: 2621440\nBlock size: 4096\n", "42", 1)
        with self.assertRaisesRegex(ValueError, "missing"):
            fixture.metrics('kubelet_volume_stats_used_bytes{namespace="foreign",persistentvolumeclaim="private"} 0', target["namespace"], target["pvc_name"], 1024**3)

    def test_current_and_legacy_sector_formats_require_consistent_units(self):
        self.assertEqual(fixture.sectors("4096 sectors"), 4096)
        self.assertEqual(fixture.sectors("4096 [512-byte units] (2097152 [bytes])"), 4096)
        with self.assertRaisesRegex(ValueError, "inconsistent"):
            fixture.sectors("4096 [512-byte units] (4096 [bytes])")


class BackupTests(unittest.TestCase):
    def created(self):
        target, kube = bound_fixture(5)
        with patch("sys.stdout", new=io.StringIO()):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        target["backup_job_uid"] = kube.job["metadata"]["uid"]
        return target, kube

    def test_creates_once_from_exact_template_then_requires_recorded_uid(self):
        target, kube = self.created()
        self.assertEqual(kube.job["spec"], kube.cron["spec"]["jobTemplate"]["spec"])
        self.assertEqual(len([call for call in dangerous_calls(kube) if "create" in call[1]]), 1)
        with self.assertRaisesRegex(ValueError, "already recorded"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        result = fixture.run_action(kube, target, "backup-status", "grown-5g", IMAGE)
        self.assertEqual(result["result"]["uid"], uid(13))

    def test_unknown_existing_or_inflight_backup_is_not_replayed(self):
        target, kube = self.created()
        del target["backup_job_uid"]
        with self.assertRaisesRegex(ValueError, "already exists"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        target, kube = bound_fixture(5)
        kube.inflight = [{"status": {"active": 1}}]
        with self.assertRaisesRegex(ValueError, "in flight"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        self.assertEqual(dangerous_calls(kube), [])

    def test_ambiguous_create_reconciles_only_proven_job_without_uid_or_replay(self):
        target, kube = bound_fixture(5)
        kube.ambiguous_create = True
        with self.assertRaisesRegex(ValueError, "transport lost"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        self.assertNotIn("backup_job_uid", target)
        result = fixture.run_action(kube, target, "backup-status", "grown-5g", IMAGE)
        self.assertEqual(result["result"]["uid"], uid(13))
        self.assertTrue(result["result"]["podPolicyVerified"])
        self.assertEqual(len([call for call in dangerous_calls(kube) if "create" in call[1]]), 1)
        with self.assertRaisesRegex(ValueError, "actual owned backup Job UID"):
            fixture.run_action(kube, target, "backup-cleanup", "grown-5g", IMAGE)
        with self.assertRaisesRegex(ValueError, "already exists"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        kube.job["metadata"]["annotations"].clear()
        with self.assertRaisesRegex(ValueError, "provenance"):
            fixture.run_action(kube, target, "backup-status", "grown-5g", IMAGE)

    def test_post_create_policy_drift_emits_identity_but_never_adopts_policy(self):
        target, kube = bound_fixture(5)
        kube.post_create_policy_drift = True
        output = io.StringIO()
        with patch("sys.stdout", new=output), self.assertRaisesRegex(ValueError, "pod policy"):
            fixture.run_action(kube, target, "backup", "grown-5g", IMAGE)
        identity = json.loads(output.getvalue())["createdBackupJobIdentity"]
        self.assertEqual(identity["uid"], uid(13))
        self.assertNotIn("private-canary", output.getvalue())
        output = io.StringIO()
        with patch("sys.stdout", new=output), self.assertRaisesRegex(ValueError, "pod policy"):
            fixture.run_action(kube, target, "backup-status", "grown-5g", IMAGE)
        self.assertFalse(json.loads(output.getvalue())["podPolicyVerified"])
        self.assertNotIn("backup_job_uid", target)
        self.assertEqual(len([call for call in dangerous_calls(kube) if "create" in call[1]]), 1)
        target["backup_job_uid"] = identity["uid"]
        with patch("sys.stdout", new=io.StringIO()):
            cancellation = fixture.run_action(kube, target, "backup-cleanup", "grown-5g", IMAGE)
        self.assertFalse(cancellation["result"]["podPolicyVerified"])
        deletion = [call for call in dangerous_calls(kube) if "delete" in call[1]][0]
        self.assertEqual(json.loads(deletion[2])["preconditions"], {"uid": uid(13)})

    def test_absent_backup_status_is_read_only(self):
        target, kube = bound_fixture(5)
        result = fixture.run_action(kube, target, "backup-status", "grown-5g", IMAGE)
        self.assertIn("absent", result["result"]["state"])
        self.assertIsNone(result["result"]["uid"])
        self.assertEqual(dangerous_calls(kube), [])

    def test_expired_cleanup_uses_exact_uid_even_after_parent_teardown(self):
        target, kube = self.created()
        later = fixture.instant(target["created_at"]) + timedelta(hours=3)
        fixture.validate_proof(target, later, cleanup=True)
        kube.rows["app"] = kube.rows["pvc"] = kube.rows["cronjob"] = None
        kube.calls.clear()
        result = fixture.run_action(kube, target, "backup-cleanup", "grown-5g", IMAGE)
        deletions = [call for call in dangerous_calls(kube) if "delete" in call[1]]
        self.assertEqual(len(deletions), 1)
        options = json.loads(deletions[0][2])
        self.assertEqual(options["preconditions"], {"uid": uid(13)})
        self.assertEqual(options["propagationPolicy"], "Foreground")
        self.assertIn("requested", result["result"]["cleanup"])

    def test_cleanup_refuses_wrong_uid_or_unproven_job_and_replacement_parent(self):
        for mutate in (lambda t, k: k.job["metadata"].update(uid=uid(99)), lambda t, k: k.job["metadata"]["annotations"].clear(), lambda t, k: k.rows["app"].update(uid=uid(99)), lambda t, k: k.job["metadata"].update(ownerReferences=owner("CronJob", "foreign", 99))):
            target, kube = self.created()
            mutate(target, kube)
            kube.calls.clear()
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                fixture.run_action(kube, target, "backup-cleanup", "grown-5g", IMAGE)
            self.assertEqual(dangerous_calls(kube), [])


if __name__ == "__main__":
    unittest.main()
