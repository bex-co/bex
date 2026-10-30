"""Ownership refusal, fixed node protocol and exact backup provenance regressions."""

import copy
from datetime import datetime, timedelta, timezone
import importlib.util
import io
import itertools
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import Mock, patch

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
        container["env"] += [{"name": key, "value": value} for key, value in zip(fixture.SNAPSHOT_LOCATION, ("https://snapshots.invalid", "snapshots", "disks", "fsn1"))]
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


class FakeCleanupWatch:
    def __init__(self, initial):
        self.initial, self.version, self.pending, self.closed = initial, "100", [], False
        self.failure = False

    def drain(self):
        if self.failure:
            raise ValueError("cleanup watch interrupted")
        result, self.pending = self.pending, []
        return result

    def close(self):
        self.closed = True


class CleanupKube(FakeKube):
    def __init__(self, target):
        super().__init__(target, 5)
        self.gone, self.keep_pv, self.hidden = False, False, set()
        self.app_watch = FakeCleanupWatch([{**self.rows["app"], "resourceVersion": "100", "purgeComplete": ""}])
        self.job_watch = FakeCleanupWatch([])

    def watch_cleanup(self, resource, kind, name, namespace):
        self.calls.append(("watch", resource, kind, name, namespace))
        return self.app_watch if resource == "apps.app.bex.co" else self.job_watch

    def project(self, resource, kind, name=None, namespace=None, selector=None, allow_missing=False, field_selector=None):
        self.calls.append(("project", resource, kind, name, namespace, selector, allow_missing, field_selector))
        keys = {"apps.app.bex.co": "app", "persistentvolumeclaim": "pvc", "persistentvolume": "pv", "secret": "secret", "storageclass": "class", "cronjobs": "cronjob", "deployments": "deployment", "replicasets": "replicaset", "pods": "servicepod", "volumeattachments": "attachment"}
        row = copy.deepcopy(self.rows.get(keys.get(resource)))
        if self.gone and not (resource == "persistentvolume" and self.keep_pv):
            row = None
        if row is not None and resource in ("cronjobs", "deployments", "replicasets", "pods"):
            row.update(namespace=self.target["namespace"], app=self.target["app_name"], appUid=self.target["app_uid"], workspace=self.target["namespace"])
        if name is not None:
            return row if row is not None and row["name"] == name else None
        return [] if row is None or resource in self.hidden else [row]

    def finish(self, complete=True):
        self.gone = True
        self.app_watch.pending = [{**self.app_watch.initial[0], "event": "DELETED", "resourceVersion": "103", "purgeComplete": self.target["app_uid"]}]
        values = dict(name=fixture.purge_job_name(self.target), uid=uid(14), namespace=self.target["namespace"], app=self.target["app_name"], appUid=self.target["app_uid"], workspace=self.target["namespace"], appNamespace=self.target["namespace"], component="disk-snapshot-purge", owner=f"App/{self.target['app_name']}/{self.target['app_uid']}", complete="True" if complete else "", image=IMAGE, command='["/disk-snapshot"]', args='["purge"]', containerNames="snapshot;", workspaceEnv=self.target["namespace"], diskEnv=self.target["app_name"], resourceVersion="102")
        values.update({key: value for key, value in zip(fixture.SNAPSHOT_LOCATION, ("https://snapshots.invalid", "snapshots", "disks", "fsn1"))})
        self.job_watch.pending = [{"deleting": "", "initNames": "", "volumes": "", **values, "event": "DELETED"}]


def cleanup_fixture():
    target, _ = bound_fixture(5)
    kube = CleanupKube(target)
    kube.calls.clear()
    return target, kube


class CleanupTests(unittest.TestCase):
    def test_observer_captures_ephemeral_completion_then_exact_absence_after_expiry(self):
        target, kube = cleanup_fixture()
        clock = [fixture.instant(target["created_at"])]
        output = io.StringIO()

        def deleted(_):
            kube.finish()
            clock[0] = fixture.instant(target["expires_at"]) + timedelta(minutes=1)

        with patch.dict("os.environ", HCLOUD_TOKEN="private-canary"), patch.object(fixture, "now", side_effect=lambda: clock[0]), patch.object(fixture.time, "sleep", side_effect=deleted), patch.object(fixture, "provider_volume", side_effect=[{"state": "present", "http": 200, "volumeId": 42}, {"state": "absent", "http": 404}]) as provider, patch("sys.stdout", new=output):
            result = fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        self.assertEqual(result["state"], "complete")
        self.assertEqual(result["purgeJob"]["uid"], uid(14))
        self.assertTrue(kube.app_watch.closed and kube.job_watch.closed)
        receipts = [json.loads(line)["cleanupObservation"] for line in output.getvalue().splitlines()]
        self.assertEqual(receipts[0]["state"], "observerReady")
        self.assertEqual(receipts[0]["watchResourceVersions"], {"app": "100", "purgeJob": "100"})
        self.assertNotIn("private-canary", output.getvalue())
        self.assertEqual(dangerous_calls(kube), [])
        self.assertEqual(provider.call_count, 2)
        self.assertTrue(all(call.args == (target, "private-canary") for call in provider.call_args_list))

    def test_pvc_absence_does_not_hide_live_pv_or_previously_observed_pods(self):
        target, kube = cleanup_fixture()
        known = {}
        fixture.cleanup_inventory(kube, target, known)
        kube.hidden.add("pods")
        self.assertTrue(any(item["resource"] == "pods" for item in fixture.cleanup_inventory(kube, target, known)))
        kube.gone, kube.keep_pv = True, True
        remaining = fixture.cleanup_inventory(kube, target, known)
        self.assertEqual([(item["resource"], item["uid"]) for item in remaining], [("persistentvolume", uid(3))])

    def test_owner_chain_finds_unseen_pod_with_missing_labels_and_filters_foreign_metadata(self):
        target, kube = cleanup_fixture()
        original = kube.project

        def project(resource, *args, **kwargs):
            rows = original(resource, *args, **kwargs)
            if resource == "pods" and kwargs.get("name") is None:
                rows[0].update(app="", appUid="")
                rows.append(row("cleanup_workload", name="PRIVATE_CANARY", uid=uid(99), owner="Job/foreign/" + uid(99), namespace=target["namespace"], app="foreign", appUid=uid(99), workspace=target["namespace"]))
            return rows

        with patch.object(kube, "project", side_effect=project):
            pending = fixture.cleanup_inventory(kube, target, {})
        self.assertTrue(any(item["resource"] == "pods" and item["uid"] == uid(9) for item in pending))
        self.assertNotIn("PRIVATE_CANARY", json.dumps(pending))

    def test_replacements_and_foreign_workspaces_are_refused(self):
        for key in ("app", "pvc", "pv", "secret", "cronjob"):
            target, kube = cleanup_fixture()
            kube.rows[key]["uid"] = uid(99)
            with self.subTest(key=key), self.assertRaisesRegex(ValueError, "replacement"):
                fixture.cleanup_inventory(kube, target, {})
        target, kube = cleanup_fixture()
        kube.finish()
        event = kube.job_watch.pending[0]
        scope = {key: event[key] for key in fixture.SNAPSHOT_LOCATION}
        for field, value in (("appUid", uid(99)), ("workspace", "tea-foreign"), ("owner", "App/foreign/" + uid(99)), ("image", "unreviewed"), ("workspaceEnv", "tea-foreign"), ("BEX_DISK_SNAPSHOT_BUCKET", "foreign-bucket")):
            changed = {**event, field: value}
            with self.subTest(field=field), self.assertRaises(ValueError):
                fixture.verify_purge_event(changed, target, scope)

    def test_missing_completion_and_watch_errors_never_claim_cleanup(self):
        for failure in (False, True):
            target, kube = cleanup_fixture()
            output = io.StringIO()

            def deleted(_):
                kube.finish(complete=False)
                kube.job_watch.failure = failure

            with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture, "CLEANUP_SECONDS", 10), patch.object(fixture.time, "monotonic", side_effect=itertools.count().__next__), patch.object(fixture.time, "sleep", side_effect=deleted), patch.object(fixture, "provider_volume", side_effect=lambda *_: {"state": "absent" if kube.gone else "present", "http": 404 if kube.gone else 200, "volumeId": 42}), patch("sys.stdout", new=output), self.subTest(failure=failure), self.assertRaises(ValueError):
                fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
            self.assertNotIn('"state": "complete"', output.getvalue())
            self.assertTrue(kube.app_watch.closed and kube.job_watch.closed)

    def test_observer_must_start_before_delete(self):
        target, kube = cleanup_fixture()
        kube.app_watch.initial[0]["deleting"] = "2026-10-30T00:00:00Z"
        with patch.dict("os.environ", HCLOUD_TOKEN="test"), self.assertRaisesRegex(ValueError, "before DELETE"):
            fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        self.assertTrue(kube.app_watch.closed and kube.job_watch.closed)

    def test_only_exact_provider_404_is_absence_and_unrelated_response_fields_are_never_retained(self):
        target, _ = cleanup_fixture()
        for status, expected in ((404, "absent"), (200, "present"), (403, "unknown"), (500, "unknown"), (302, "unknown")):
            connection = Mock()
            connection.getresponse.return_value.status = status
            connection.getresponse.return_value.read.return_value = b'{"volume":{"id":42,"labels":{"private-canary":"not-emitted"}}}'
            with patch.dict("os.environ", HCLOUD_TOKEN="private-canary"), patch.object(fixture.http.client, "HTTPSConnection", return_value=connection) as connect:
                result = fixture.provider_volume(target)
            self.assertEqual(result["state"], expected)
            connect.assert_called_once_with("api.hetzner.cloud", timeout=10)
            connection.request.assert_called_once_with("GET", "/v1/volumes/42", headers={"Authorization": "Bearer private-canary"})
            if status != 200:
                connection.getresponse.return_value.read.assert_not_called()
            connection.close.assert_called_once()
            self.assertNotIn("private-canary", json.dumps(result))

    def test_wrong_provider_project_and_mismatched_numeric_id_never_emit_readiness(self):
        for status, body in ((404, b""), (403, b""), (500, b""), (200, b'{"volume":{"id":99}}')):
            target, kube = cleanup_fixture()
            connection, output = Mock(), io.StringIO()
            connection.getresponse.return_value.status = status
            connection.getresponse.return_value.read.return_value = body
            with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture.http.client, "HTTPSConnection", return_value=connection), patch("sys.stdout", new=output), self.subTest(status=status), self.assertRaisesRegex(ValueError, "baseline is unproved"):
                fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
            self.assertNotIn("observerReady", output.getvalue())
            self.assertTrue(kube.app_watch.closed and kube.job_watch.closed)

    def test_fixed_sample_reuses_geometry_and_node_metrics_without_claiming_http_phase(self):
        target, kube = bound_fixture(5)
        sample = fixture.sample_inspection(kube, target, "5", IMAGE)
        self.assertEqual(sample["geometry"]["filesystemBytes"], 5 * 1024**3)
        self.assertEqual(sample["kubeletMetrics"]["node"], "own-node")
        self.assertEqual(sample["servicePod"]["uid"], uid(9))
        self.assertNotIn("phase", sample)
        self.assertNotIn("PRIVATE_CANARY", json.dumps(sample))

    def test_sampler_retries_once_per_minute_or_identity_change_and_stops_at_expiry(self):
        target, kube = cleanup_fixture()
        utc, tick, sleeps = [fixture.instant(target["created_at"])], [0], [0]
        output = io.StringIO()

        def advance(_):
            sleeps[0] += 1
            if sleeps[0] == 1:
                tick[0] = 30
            elif sleeps[0] == 2:
                tick[0] = 60
            elif sleeps[0] == 3:
                tick[0] = 61
                kube.rows["servicepod"]["uid"] = uid(20)
                # A same-name Pod replacement intentionally ends authority;
                # exercise a real rollout's fresh Pod name instead.
                kube.rows["servicepod"]["name"] = "new-service-pod"
            else:
                utc[0] = fixture.instant(target["expires_at"]) + timedelta(seconds=1)
                kube.finish()

        with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture, "now", side_effect=lambda: utc[0]), patch.object(fixture.time, "monotonic", side_effect=lambda: tick[0]), patch.object(fixture.time, "sleep", side_effect=advance), patch.object(fixture, "sample_inspection", side_effect=ValueError("transient not Ready")) as sample, patch.object(fixture, "provider_volume", side_effect=[{"state": "present", "http": 200, "volumeId": 42}, {"state": "absent", "http": 404}]), patch("sys.stdout", new=output):
            result = fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        self.assertEqual(result["state"], "complete")
        self.assertEqual(sample.call_count, 3)
        self.assertEqual(output.getvalue().count('"state": "inspection-unproved"'), 3)

    def test_attachment_remount_records_both_lifetimes_on_the_same_bound_pv(self):
        target, kube = cleanup_fixture()
        known = {}
        fixture.cleanup_inventory(kube, target, known)
        kube.rows["attachment"]["uid"] = uid(30)
        remaining = fixture.cleanup_inventory(kube, target, known)
        remount = next(item for item in remaining if item["resource"] == "volumeattachments")
        self.assertEqual((remount["uid"], remount["priorUid"]), (uid(30), uid(10)))

    def test_purge_retry_requires_prior_delete_and_resets_completed_attempt(self):
        target, kube = cleanup_fixture()
        kube.finish()
        event = kube.job_watch.pending[0]
        attempts = fixture.PurgeAttempts(target, {key: event[key] for key in fixture.SNAPSHOT_LOCATION})
        attempts.observe({**event, "event": "ADDED"})
        self.assertEqual(attempts.completion()["uid"], uid(14))
        replacement = {**event, "uid": uid(15), "event": "ADDED", "complete": "", "resourceVersion": "104"}
        with self.assertRaisesRegex(ValueError, "observed DELETED"):
            attempts.observe(replacement)
        attempts.observe(event)
        with self.assertRaises(ValueError):
            attempts.observe({**replacement, "workspaceEnv": "tea-foreign"})
        self.assertTrue(attempts.observe(replacement))
        self.assertIsNone(attempts.completion())
        attempts.observe({**replacement, "event": "MODIFIED", "complete": "True", "resourceVersion": "105"})
        self.assertEqual(attempts.completion()["uid"], uid(15))
        self.assertEqual(list(attempts.attempts), [uid(14), uid(15)])
        self.assertTrue(attempts.attempts[uid(14)]["deletedObserved"])

    def test_changed_helper_digest_preserves_owner_bound_unapproved_evidence_only(self):
        target, kube = cleanup_fixture()
        output = io.StringIO()
        actual_image = "ghcr.io/bex-co/bex-operator@sha256:" + "c" * 64

        def deleted(_):
            kube.finish()
            kube.job_watch.pending[0]["image"] = actual_image

        with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture.time, "sleep", side_effect=deleted), patch.object(fixture, "sample_inspection", return_value={"state": "inspection"}), patch.object(fixture, "provider_volume", side_effect=[{"state": "present", "http": 200, "volumeId": 42}, {"state": "absent", "http": 404}]), patch("sys.stdout", new=output), self.assertRaisesRegex(ValueError, "strict offline"):
            fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        receipts = [json.loads(line)["cleanupObservation"] for line in output.getvalue().splitlines()]
        final = receipts[-1]
        self.assertEqual(final["state"], "UNAPPROVED")
        self.assertIsNone(final["purgeJob"])
        self.assertEqual(final["remaining"], [])
        self.assertEqual(final["provider"]["http"], 404)
        attempt = final["purgeAttempts"][0]
        self.assertEqual((attempt["uid"], attempt["image"], attempt["ownerAppUid"]), (uid(14), actual_image, target["app_uid"]))
        self.assertTrue(attempt["jobCompleteObserved"])
        self.assertNotIn("prefixEmptyEvidence", output.getvalue())
        self.assertNotIn('"state": "complete"', output.getvalue())
        self.assertEqual(dangerous_calls(kube), [])

    def test_observer_retains_failed_purge_attempt_before_successful_retry(self):
        target, kube = cleanup_fixture()
        kube.finish(complete=False)
        old = kube.job_watch.pending[0]
        kube.gone, kube.job_watch.pending, kube.app_watch.pending = False, [], []
        sleeps = []

        def advance(seconds):
            sleeps.append(seconds)
            if len(sleeps) == 1:
                kube.job_watch.pending = [{**old, "event": "ADDED"}]
                kube.app_watch.pending = [{**kube.app_watch.initial[0], "event": "MODIFIED", "deleting": "2026-10-30T00:00:00Z", "resourceVersion": "103"}]
            elif len(sleeps) == 2:
                kube.job_watch.pending = [{**old, "event": "DELETED", "resourceVersion": "104"}, {**old, "uid": uid(15), "event": "ADDED", "resourceVersion": "105"}]
            else:
                kube.finish()
                kube.job_watch.pending[0].update(uid=uid(15), resourceVersion="106")
                kube.app_watch.pending[0]["resourceVersion"] = "107"

        with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture.time, "sleep", side_effect=advance), patch.object(fixture, "sample_inspection", return_value={"state": "inspection"}), patch.object(fixture, "provider_volume", side_effect=[{"state": "present", "http": 200, "volumeId": 42}, {"state": "absent", "http": 404}]), patch("sys.stdout", new=io.StringIO()):
            result = fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        self.assertEqual(result["state"], "complete")
        self.assertEqual(result["purgeJob"]["uid"], uid(15))
        self.assertEqual([attempt["uid"] for attempt in result["purgeAttempts"]], [uid(14), uid(15)])
        self.assertFalse(result["purgeAttempts"][0]["jobCompleteObserved"])
        self.assertTrue(result["purgeAttempts"][0]["deletedObserved"])
        self.assertEqual(sleeps, [10, 2, 2])

    def test_inventory_uses_ten_seconds_before_delete_and_two_seconds_after(self):
        target, kube = cleanup_fixture()
        sleeps = []

        def advance(seconds):
            sleeps.append(seconds)
            if len(sleeps) == 1:
                kube.app_watch.pending = [{**kube.app_watch.initial[0], "event": "MODIFIED", "deleting": "2026-10-30T00:00:00Z", "resourceVersion": "101"}]
            else:
                kube.finish()

        with patch.dict("os.environ", HCLOUD_TOKEN="test"), patch.object(fixture.time, "sleep", side_effect=advance), patch.object(fixture, "sample_inspection", return_value={"state": "inspection"}), patch.object(fixture, "provider_volume", side_effect=[{"state": "present", "http": 200, "volumeId": 42}, {"state": "absent", "http": 404}]), patch("sys.stdout", new=io.StringIO()):
            result = fixture.run_action(kube, target, "cleanup-observe", "grown-5g", IMAGE)
        self.assertEqual(result["state"], "complete")
        self.assertEqual(sleeps, [10, 2])

    def test_watch_initial_api_list_preserves_empty_collection_version(self):
        kube = fixture.Kube()
        kube.run = Mock(return_value=json.dumps({"metadata": {"resourceVersion": "501"}, "items": []}))
        with patch.object(fixture, "CleanupWatch") as watch:
            self.assertIs(kube.watch_cleanup("jobs", "cleanup_job", "own-purge", "tea-owned"), watch.return_value)
            self.assertEqual(watch.call_args.args, ("/apis/batch/v1/namespaces/tea-owned/jobs", "own-purge", "cleanup_job", "501", []))
        self.assertEqual(kube.run.call_args.args[0], ["get", "--raw", "/apis/batch/v1/namespaces/tea-owned/jobs?fieldSelector=metadata.name%3Down-purge"])

    def test_watch_initial_api_list_filters_private_fields_and_rejects_other_names(self):
        target, _ = cleanup_fixture()
        obj = {"metadata": {"name": target["app_name"], "uid": target["app_uid"], "resourceVersion": "502"}, "spec": {"env": [{"name": "SECRET", "value": "PRIVATE_CANARY"}]}}
        kube = fixture.Kube()
        kube.run = Mock(return_value=json.dumps({"metadata": {"resourceVersion": "503"}, "items": [obj]}))
        with patch.object(fixture, "CleanupWatch") as watch:
            kube.watch_cleanup("apps.app.bex.co", "cleanup_app", target["app_name"], "tea-owned")
            self.assertEqual(watch.call_args.args[3], "503")
            self.assertEqual(watch.call_args.args[4][0]["uid"], target["app_uid"])
            self.assertNotIn("PRIVATE_CANARY", json.dumps(watch.call_args.args[4]))
            with self.assertRaisesRegex(ValueError, "another name"):
                kube.watch_cleanup("apps.app.bex.co", "cleanup_app", "another-app", "tea-owned")
            with self.assertRaisesRegex(ValueError, "unknown fixed"):
                kube.watch_cleanup("secrets", "cleanup_job", "own-purge", "tea-owned")

    def test_watch_filters_private_spec_fields_and_reconnects_from_bookmark(self):
        target, kube = cleanup_fixture()
        stopped, received = threading.Event(), threading.Event()
        bookmark = json.dumps({"type": "BOOKMARK", "object": {"metadata": {"resourceVersion": "501", "annotations": {"k8s.io/initial-events-end": "true"}}}}) + "\n"
        event = {"type": "MODIFIED", "object": {"metadata": {"name": target["app_name"], "uid": target["app_uid"], "resourceVersion": "502", "labels": {"bex.co/app-id": target["service_id"], "app.bex.co/workspace": target["namespace"]}}, "spec": {"disk": {"sizeGB": 5}, "env": [{"name": "SECRET", "value": "PRIVATE_CANARY"}]}}}
        first, second = Mock(), Mock()
        first.stdout.readline.side_effect = [bookmark, ""]
        first.wait.return_value = 0
        second.wait.return_value, second.poll.return_value = 0, None
        second.terminate.side_effect = stopped.set
        delivered = [False]

        def read(_):
            if not delivered[0]:
                delivered[0] = True
                return json.dumps(event) + "\n"
            received.set()
            stopped.wait(5)
            return ""

        second.stdout.readline.side_effect = read
        with patch.object(fixture.subprocess, "Popen", side_effect=[first, second]) as popen:
            watcher = fixture.CleanupWatch("/apis/app.bex.co/v1alpha1/namespaces/tea-owned/apps", target["app_name"], "cleanup_app", "100", kube.app_watch.initial)
            try:
                self.assertTrue(received.wait(1))
                rows = watcher.drain()
                self.assertEqual([item["resourceVersion"] for item in rows], ["502"])
                self.assertNotIn("PRIVATE_CANARY", json.dumps(rows))
                urls = [call.args[0][-1] for call in popen.call_args_list]
                self.assertIn("resourceVersion=100", urls[0])
                self.assertIn("resourceVersion=501", urls[1])
                self.assertIn("sendInitialEvents=true", urls[0])
                self.assertNotIn("sendInitialEvents", urls[1])
                self.assertTrue(all("fieldSelector=metadata.name%3D" in url and "allowWatchBookmarks=true" in url for url in urls))
            finally:
                watcher.close()
        self.assertTrue(stopped.is_set())

    def test_watch_410_error_is_unproved_without_printing_response(self):
        process = Mock()
        process.stdout.readline.side_effect = [json.dumps({"type": "ERROR", "object": {"code": 410, "message": "PRIVATE_CANARY"}}) + "\n", ""]
        process.poll.return_value, process.wait.return_value = 0, 0
        with patch.object(fixture.subprocess, "Popen", return_value=process):
            try:
                watcher = fixture.CleanupWatch("/apis/batch/v1/namespaces/tea-owned/jobs", "own-purge", "cleanup_job", "100", [])
            except ValueError as error:
                self.assertNotIn("PRIVATE_CANARY", str(error))
            else:
                try:
                    with self.assertRaises(ValueError):
                        watcher.drain()
                finally:
                    watcher.close()

    def test_watch_close_terminates_child_published_after_reconnect_cancellation(self):
        reconnecting, release = threading.Event(), threading.Event()
        first, late = Mock(), Mock()
        first.stdout.readline.side_effect = [json.dumps({"type": "BOOKMARK", "object": {"metadata": {"resourceVersion": "101", "annotations": {"k8s.io/initial-events-end": "true"}}}}) + "\n", ""]
        first.wait.return_value, first.poll.return_value = 0, 0
        late.poll.return_value, late.wait.return_value = None, 0
        late.stdout.readline.return_value = ""

        first_started = [False]

        def popen(*_args, **_kwargs):
            if not first_started[0]:
                first_started[0] = True
                return first
            reconnecting.set()
            release.wait(5)
            return late

        with patch.object(fixture.subprocess, "Popen", side_effect=popen):
            watcher = fixture.CleanupWatch("/apis/batch/v1/namespaces/tea-owned/jobs", "own-purge", "cleanup_job", "100", [])
            self.assertTrue(reconnecting.wait(1))
            try:
                watcher.close()
                self.assertTrue(watcher.thread.is_alive())
                late.terminate.assert_not_called()
            finally:
                release.set()
                watcher.thread.join(timeout=1)
            self.assertFalse(watcher.thread.is_alive())
            late.terminate.assert_called_once()
            late.stdout.readline.assert_not_called()


if __name__ == "__main__":
    unittest.main()
