#!/usr/bin/env python3
"""Fixed m144 fixture metadata, node geometry and own-CronJob backup operations.

Authority comes only from the reviewed, committed fixture-proof.json, never a
workflow-supplied target, command, device, image, selector or manifest. Secret
data and tenant files are never requested. Full backup templates stay in memory.
"""

import argparse
import copy
import csv
from datetime import datetime, timezone, timedelta
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parent.parent
PROOF = Path("deploy/hcloud-csi/fixture-proof.json")
SC = "hcloud-volumes-luks-bounded"
ACTIONS = ("bind", "inspect", "backup", "backup-status", "backup-cleanup")
PHASE_GIB = {"initial-1g": 1, "full-1g": 1, "grown-5g": 5, "restart-5g": 5, "redeploy-5g": 5, "restored-5g": 5}
OWNER = '{range .metadata.ownerReferences[?(@.controller==true)]}{.kind}{"/"}{.name}{"/"}{.uid}{end}'
META = {"name": "{.metadata.name}", "uid": "{.metadata.uid}", "deleting": "{.metadata.deletionTimestamp}"}
FIELDS = {
    "app": {**META, "service": r"{.metadata.labels.bex\.co/app-id}", "workspace": r"{.metadata.labels.app\.bex\.co/workspace}", "diskName": "{.spec.disk.name}", "mount": "{.spec.disk.mountPath}", "size": "{.spec.disk.sizeGB}", "displayName": "{.spec.displayName}"},
    "pvc": {**META, "owner": OWNER, "class": "{.spec.storageClassName}", "request": "{.spec.resources.requests.storage}", "capacity": "{.status.capacity.storage}", "phase": "{.status.phase}", "pv": "{.spec.volumeName}"},
    "pv": {**META, "class": "{.spec.storageClassName}", "capacity": "{.spec.capacity.storage}", "namespace": "{.spec.claimRef.namespace}", "claim": "{.spec.claimRef.name}", "claimUid": "{.spec.claimRef.uid}", "driver": "{.spec.csi.driver}", "handle": "{.spec.csi.volumeHandle}"},
    "secret": {**META, "owner": OWNER},
    "class": {**META, "provisioner": "{.provisioner}", "mode": "{.parameters.boundedFilesystem}", "filesystem": r"{.parameters.csi\.storage\.k8s\.io/fstype}", "publishSecret": r"{.parameters.csi\.storage\.k8s\.io/node-publish-secret-name}", "expandSecret": r"{.parameters.csi\.storage\.k8s\.io/node-expand-secret-name}"},
    "attachment": {**META, "pv": "{.spec.source.persistentVolumeName}", "node": "{.spec.nodeName}", "attacher": "{.spec.attacher}", "attached": "{.status.attached}", "device": "{.status.attachmentMetadata.devicePath}"},
    "workload": {**META, "owner": OWNER},
    "job_status": {"completed": "{.status.completionTime}", "failed": '{range .status.conditions[?(@.type=="Failed")]}{.status}{end}'},
    "pod": {**META, "owner": OWNER, "node": "{.spec.nodeName}", "phase": "{.status.phase}", "ready": '{range .status.conditions[?(@.type=="Ready")]}{.status}{end}', "started": "{.status.startTime}", "claims": '{range .spec.volumes[*]}{.persistentVolumeClaim.claimName}{";"}{end}', "image": '{.spec.containers[?(@.name=="hcloud-csi-driver")].image}', "containerId": '{.status.containerStatuses[?(@.name=="hcloud-csi-driver")].containerID}', "imageId": '{.status.containerStatuses[?(@.name=="hcloud-csi-driver")].imageID}', "driverReady": '{.status.containerStatuses[?(@.name=="hcloud-csi-driver")].ready}'},
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def instant(value):
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(parsed.tzinfo is not None, "timestamps must include a timezone")
    return parsed


def now():
    return datetime.now(timezone.utc)


def validate_proof(proof, current=None, cleanup=False):
    current = current or now()
    require(proof.get("schema") == "bex-m144-fixture-v1", "fixture proof schema missing")
    run = proof["run"]
    require(re.fullmatch(r"[0-9a-f]{16}", run), "invalid unique fixture run")
    name = proof["service_name"]
    match = re.fullmatch(r"qa-(\d{8})-w8-m144-" + run, name)
    require(match is not None and proof["disk_name"] == name + "-disk", "dated fixture name mismatch")
    created, expiry = instant(proof["created_at"]), instant(proof["expires_at"])
    require(created.astimezone(timezone.utc).strftime("%Y%m%d") == match[1], "fixture name must match the actual UTC creation date")
    require(created <= current and created < expiry <= created + timedelta(hours=2), "fixture proof exceeds its concrete two-hour budget")
    require(cleanup or current < expiry, "fixture proof expired; only proven-own backup reconciliation/cleanup remains allowed")
    require(proof["budget"] == {"max_services": 1, "max_disks": 1, "max_size_gib": 5, "max_runtime_seconds": 7200, "max_estimated_usage_usd": 0.03}, "fixture budget does not match the reviewed one-service acceptance window")
    for key, prefix in (("workspace", "tea"), ("service_id", "srv"), ("disk_id", "dsk")):
        require(re.fullmatch(prefix + r"-[a-z0-9]+", proof[key]), "invalid fixture identity")
    require(proof["namespace"] == proof["workspace"], "fixture must be in its own workspace namespace")
    require(proof["app_name"] == proof["workspace"] + "-" + name, "App name does not match new REST creation")
    for key in ("app_name", "pvc_name"):
        require(re.fullmatch(r"[a-z0-9][-a-z0-9.]{0,252}", proof[key]), "invalid Kubernetes identity")
    for key in ("app_uid", "pvc_uid"):
        require(re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", proof[key]), "exact App/PVC UID proof required")
    require(proof["service_id"] not in proof["baseline_ids"] and proof["disk_id"] not in proof["baseline_disks"], "fixture identity existed before this run")
    creation = proof["create_proof"]
    expected = {"HTTP": 201, "service_id": proof["service_id"], "workspace": proof["workspace"], "service_name": name, "disk_name": proof["disk_name"], "initial_size_gib": 1, "mount_path": "/var/data", "plan": "starter"}
    require(all(creation.get(key) == value for key, value in expected.items()), "actual guarded HTTP201 creation proof required")
    require(re.fullmatch(r"[0-9a-f]{64}", creation["request_body_sha256"]), "creation request hash missing")
    if "binding" in proof:
        require(re.fullmatch(r"bounded-ext4-v1:[1-9][0-9]*", proof["binding"]["volume_handle"]), "invalid reviewed volume binding")
    if "backup_job_uid" in proof:
        require(re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", proof["backup_job_uid"]), "actual backup Job UID required")
    return proof


def command(args, body=None, timeout=30):
    result = subprocess.run(args, input=body, text=True, capture_output=True, timeout=timeout)
    # Never print subprocess stdout/stderr wholesale: they can contain a full
    # CronJob template, unrelated kubelet samples, or failed Job specifications.
    require(result.returncode == 0, f"fixed {args[0]} operation failed (exit {result.returncode}); no raw output retained")
    return result.stdout


def load_proof(root=ROOT, cleanup=False):
    path = root / PROOF
    require(path.is_file() and not path.is_symlink(), "committed fixture proof is absent")
    command(["git", "-C", str(root), "ls-files", "--error-unmatch", str(PROOF)])
    command(["git", "-C", str(root), "diff", "--quiet", "HEAD", "--", str(PROOF)])
    return validate_proof(json.loads(path.read_text()), cleanup=cleanup)


class Kube:
    def run(self, args, body=None, timeout=30):
        return command(["kubectl", "--request-timeout=20s", *args], body, timeout)

    def project(self, resource, kind, name=None, namespace=None, selector=None, allow_missing=False, field_selector=None):
        fields = FIELDS[kind]
        expression = '{"\\t"}'.join(fields.values()) + '{"\\n"}'
        if name is None:
            expression = "{range .items[*]}" + expression + "{end}"
        args = (["-n", namespace] if namespace else []) + ["get", resource]
        args += [name] if name else []
        if allow_missing:
            args += ["--ignore-not-found"]
        if selector:
            args += ["-l", selector]
        if field_selector:
            args += ["--field-selector", field_selector]
        raw = self.run([*args, "-o", "jsonpath=" + expression])
        rows = []
        for values in csv.reader(io.StringIO(raw), delimiter="\t", quoting=csv.QUOTE_NONE):
            if values:
                require(len(values) == len(fields), "unexpected projected metadata width")
                rows.append(dict(zip(fields, values)))
        if name:
            if allow_missing and not rows:
                return None
            require(len(rows) == 1, "exact fixture object not found")
            return rows[0]
        return rows

    def cronjob(self, name, namespace):
        # The exact owner-validated template is needed to preserve the operator
        # policy. It is parsed only in memory and never emitted or persisted.
        return json.loads(self.run(["-n", namespace, "get", "cronjob", name, "-o", "json"]))


def exact(row, uid):
    require(row["uid"] == uid and not row["deleting"], "fixture object lifetime changed or is deleting")


def owner(row, kind, name, uid):
    require(row["owner"] == f"{kind}/{name}/{uid}", "fixture controller owner UID mismatch")


def controller(metadata, kind, name, uid):
    owners = [ref for ref in metadata.get("ownerReferences", []) if ref.get("controller")]
    require(len(owners) == 1 and all(owners[0].get(key) == value for key, value in {"kind": kind, "name": name, "uid": uid}.items()), "fixture controller owner UID mismatch")


def binding(kube, proof, size):
    ns, app_name = proof["namespace"], proof["app_name"]
    app = kube.project("apps.app.bex.co", "app", app_name, ns)
    exact(app, proof["app_uid"])
    require((app["service"], app["workspace"], app["diskName"], app["mount"], app["size"]) == (proof["service_id"], proof["workspace"], proof["disk_name"], "/var/data", str(size)), "live App no longer matches proved fixture")
    require(not app["displayName"] or app["displayName"] == proof["service_name"], "fixture display name changed")
    claim = kube.project("persistentvolumeclaim", "pvc", proof["pvc_name"], ns)
    exact(claim, proof["pvc_uid"])
    owner(claim, "App", app_name, proof["app_uid"])
    require((claim["class"], claim["request"], claim["phase"]) == (SC, f"{size}Gi", "Bound"), "fixture claim is not admitted on the bounded class")
    require(re.fullmatch(r"[a-z0-9][-a-z0-9.]{0,252}", claim["pv"]), "invalid bound PV name")
    pv = kube.project("persistentvolume", "pv", claim["pv"])
    require(not pv["deleting"] and (pv["namespace"], pv["claim"], pv["claimUid"], pv["class"], pv["driver"]) == (ns, proof["pvc_name"], proof["pvc_uid"], SC, "csi.hetzner.cloud"), "PV does not bind this exact own claim")
    require(re.fullmatch(r"bounded-ext4-v1:[1-9][0-9]*", pv["handle"]), "fixture PV has no bounded provider handle")
    storage = kube.project("storageclass", "class", SC)
    require((storage["provisioner"], storage["mode"], storage["filesystem"], storage["publishSecret"], storage["expandSecret"]) == ("csi.hetzner.cloud", "ext4-v1", "ext4", "${pvc.name}-luks", "${pvc.name}-luks"), "bounded encrypted class changed")
    secret = kube.project("secret", "secret", proof["pvc_name"] + "-luks", ns)
    owner(secret, "App", app_name, proof["app_uid"])
    require(not secret["deleting"], "own LUKS Secret is deleting")
    result = {"app": app, "claim": claim, "volume": pv, "storageClass": storage, "secretMetadata": secret}
    if "binding" in proof:
        expected = proof["binding"]
        live = {"pv_name": pv["name"], "pv_uid": pv["uid"], "volume_handle": pv["handle"], "storage_class_uid": storage["uid"], "secret_uid": secret["uid"]}
        require(all(expected.get(key) == value for key, value in live.items()), "immutable fixture volume/class/Secret binding changed")
    return result


def own_cronjob(kube, proof):
    # Discover by the exact App identity; names may be truncated by the shared
    # Kubernetes naming contract, so neither the public srv ID nor a guessed
    # disk prefix is treated as ownership evidence.
    rows = kube.project("cronjobs", "workload", namespace=proof["namespace"], selector="app.bex.co/app=" + proof["app_name"] + ",app.kubernetes.io/name=disk-backup")
    require(len(rows) == 1, "expected exactly one own backup CronJob")
    row = rows[0]
    owner(row, "App", proof["app_name"], proof["app_uid"])
    require(not row["deleting"], "own backup CronJob is deleting")
    cron = kube.cronjob(row["name"], proof["namespace"])
    require(cron["metadata"]["uid"] == row["uid"], "backup CronJob lifetime changed")
    controller(cron["metadata"], "App", proof["app_name"], proof["app_uid"])
    spec = cron["spec"]["jobTemplate"]["spec"]
    pod = spec["template"]["spec"]
    containers = pod.get("containers", [])
    require(len(containers) == 1 and not pod.get("initContainers") and not pod.get("ephemeralContainers"), "unexpected backup containers")
    container = containers[0]
    require(container.get("name") == "snapshot" and container.get("command") == ["/disk-snapshot"] and container.get("args") == ["backup"], "backup command changed")
    require(re.fullmatch(r"ghcr\.io/bex-co/bex-operator@sha256:[0-9a-f]{64}", container["image"]), "backup helper must use a reviewed operator digest")
    claims = [volume.get("persistentVolumeClaim", {}).get("claimName") for volume in pod.get("volumes", [])]
    require(claims == [proof["pvc_name"]], "backup template must mount only the own fixture claim")
    require(container.get("volumeMounts") == [{"name": "disk", "mountPath": "/disk", "readOnly": True}], "backup must keep its sole disk bind read-only")
    env = {item["name"]: item.get("value") for item in container.get("env", [])}
    require(env.get("BEX_DISK_WORKSPACE") == proof["workspace"] and env.get("BEX_DISK_ID") == proof["app_name"], "backup object scope changed")
    info = {"backup_cronjob": row["name"], "backup_cronjob_uid": row["uid"], "backup_template_sha256": digest(spec), "backup_pod_spec_sha256": digest(pod), "backup_image": container["image"]}
    if "binding" in proof:
        require(all(proof["binding"].get(key) == value for key, value in info.items()), "reviewed backup template or lifetime changed")
    return cron, info


def mounted_pod(kube, proof, facts):
    ns, app = proof["namespace"], proof["app_name"]
    selector = "app.bex.co/app=" + app
    deployments = kube.project("deployments", "workload", namespace=ns, selector=selector)
    deployments = [row for row in deployments if row["owner"] == f"App/{app}/{proof['app_uid']}" and not row["deleting"]]
    require(len(deployments) == 1, "expected one own service Deployment")
    deployment = deployments[0]
    sets = kube.project("replicasets", "workload", namespace=ns, selector=selector)
    sets = {row["name"]: row for row in sets if row["owner"] == f"Deployment/{deployment['name']}/{deployment['uid']}"}
    pods = kube.project("pods", "pod", namespace=ns, selector=selector)
    own = [row for row in pods if any(row["owner"] == f"ReplicaSet/{name}/{rs['uid']}" for name, rs in sets.items()) and row["phase"] not in ("Succeeded", "Failed")]
    require(len(own) == 1 and own[0]["phase"] == "Running" and own[0]["ready"] == "True" and not own[0]["deleting"], "fixture must have exactly one Ready service pod and no old live pod")
    pod = own[0]
    require([claim for claim in pod["claims"].split(";") if claim] == [proof["pvc_name"]], "service pod does not mount only the own claim")
    attachments = kube.project("volumeattachments", "attachment")
    matches = [row for row in attachments if row["pv"] == facts["volume"]["name"]]
    require(len(matches) == 1, "fixture must have exactly one current volume attachment")
    attachment = matches[0]
    require((attachment["node"], attachment["attacher"], attachment["attached"], attachment["deleting"]) == (pod["node"], "csi.hetzner.cloud", "true", ""), "fixture attachment does not match its live pod node")
    provider = facts["volume"]["handle"].split(":")[1]
    require(attachment["device"] == "/dev/disk/by-id/scsi-0HC_Volume_" + provider, "attachment device does not match exact provider handle")
    return {"servicePod": pod, "attachment": attachment}


def csi_pod(kube, node, image):
    sets = kube.project("daemonsets", "workload", namespace="kube-system", selector="app.kubernetes.io/instance=hcloud-csi,app.kubernetes.io/component=node")
    require(len(sets) == 1 and not sets[0]["deleting"], "expected one reviewed CSI DaemonSet")
    daemon = sets[0]
    pods = kube.project("pods", "pod", namespace="kube-system", selector="app.kubernetes.io/instance=hcloud-csi,app.kubernetes.io/component=node", field_selector="spec.nodeName=" + node)
    pods = [row for row in pods if row["node"] == node]
    require(len(pods) == 1, "expected exactly one current CSI pod on fixture node")
    pod = pods[0]
    owner(pod, "DaemonSet", daemon["name"], daemon["uid"])
    require((pod["image"], pod["phase"], pod["ready"], pod["driverReady"], pod["deleting"]) == (image, "Running", "True", "true", ""), "bound-node CSI driver is not the reviewed Ready image")
    require(pod["containerId"] and "sha256:" in pod["imageId"], "CSI running container identity is unavailable")
    return pod


def sectors(value):
    match = re.fullmatch(r"(\d+) (?:sectors|\[512-byte units\](?: \((\d+) \[bytes\]\))?)", value)
    require(match is not None, "unsupported cryptsetup sector metadata")
    count = int(match[1])
    require(match[2] is None or int(match[2]) == count * 512, "inconsistent cryptsetup byte/sector metadata")
    return count


def parse_geometry(status, raw_type, filesystem, provider, size):
    require(raw_type.strip() == "crypto_LUKS", "underlying fixture device is not LUKS")
    values = dict(re.findall(r"^\s*(type|device|sector size|offset|size):\s*(.+?)\s*$", status, re.M))
    require(" is active" in status and values.get("type") in ("LUKS1", "LUKS2"), "fixture LUKS mapping is not active")
    device = values.get("device", "")
    require(device == "/dev/disk/by-id/scsi-0HC_Volume_" + provider or re.fullmatch(r"/dev/[a-zA-Z0-9]+", device), "unexpected mapped backing-device identity")
    fields = dict(re.findall(r"^(Block count|Block size):\s*(\d+)\s*$", filesystem, re.M))
    blocks, block_size = int(fields["Block count"]), int(fields["Block size"])
    require(block_size == 4096 and blocks * block_size == size * 1024**3, "actual ext4 geometry does not match the requested logical capacity")
    count = sectors(values["size"])
    sector_size = int(re.fullmatch(r"(\d+)(?:\s+\[?bytes\]?)?", values["sector size"])[1])
    offset = sectors(values["offset"])
    return {"activeLuks": True, "luksType": values["type"], "backingDevice": device, "mappedSectors": count, "sectorSize": sector_size, "offsetSectors": offset, "mappedBytes": count * 512, "blockCount": blocks, "blockSize": block_size, "filesystemBytes": blocks * block_size}


def metrics(raw, namespace, pvc, capacity):
    names = {"kubelet_volume_stats_" + key + "_bytes": key for key in ("used", "capacity", "available")}
    found = {}
    for line in raw.splitlines():
        match = re.fullmatch(r"(kubelet_volume_stats_(?:used|capacity|available)_bytes)\{([^}]*)\}\s+(\S+)(?:\s+\d+)?", line)
        if not match:
            continue
        labels = dict(re.findall(r'(\w+)="([^"\\]*)"', match[2]))
        if labels.get("namespace") != namespace or labels.get("persistentvolumeclaim") != pvc:
            continue
        name = names[match[1]]
        require(name not in found, "duplicate own-node volume metric sample")
        value = float(match[3])
        require(math.isfinite(value) and value >= 0, "invalid kubelet volume metric")
        found[name] = value
    require(set(found) == {"used", "capacity", "available"}, "own-node kubelet volume samples are missing")
    require(0.9 * capacity <= found["capacity"] <= capacity and found["used"] <= capacity and found["available"] <= capacity, "kubelet filesystem capacity disagrees with raw geometry")
    return found


def inspect(kube, proof, facts, mount, image, size):
    node = mount["attachment"]["node"]
    before = csi_pod(kube, node, image)
    device = mount["attachment"]["device"]
    mapping = device.rsplit("/", 1)[1]
    outputs = []
    for argv in (["cryptsetup", "status", mapping], ["blkid", "-p", "-o", "value", "-s", "TYPE", device], ["dumpe2fs", "-h", "/dev/mapper/" + mapping]):
        require(now() < instant(proof["expires_at"]), "fixture inspection window expired before exec")
        require(csi_pod(kube, node, image) == before, "CSI identity changed before fixed exec")
        outputs.append(kube.run(["-n", "kube-system", "exec", before["name"], "-c", "hcloud-csi-driver", "--", *argv], timeout=30))
        require(csi_pod(kube, node, image) == before, "CSI identity changed during fixed exec; discard evidence")
    geometry = parse_geometry(*outputs, facts["volume"]["handle"].split(":")[1], size)
    raw = kube.run(["get", "--raw", "/api/v1/nodes/" + node + "/proxy/metrics"])
    samples = metrics(raw, proof["namespace"], proof["pvc_name"], geometry["filesystemBytes"])
    require(binding(kube, proof, size) == facts and mounted_pod(kube, proof, facts) == mount, "fixture binding/mount changed during inspection; discard evidence")
    return {**mount, "csiPod": before, "geometry": geometry, "kubeletMetrics": {"node": node, "collectedAt": now().isoformat(), "source": "live-node /proxy/metrics; raw response was filtered in memory", "namespace": proof["namespace"], "persistentvolumeclaim": proof["pvc_name"], "bytes": samples}}


def ownership_digest(proof):
    return digest({key: value for key, value in proof.items() if key not in ("backup_job_uid", "expires_at")})


def job_name(proof):
    return "qa-m144-" + proof["run"] + "-backup"


def job_identity(job, proof):
    metadata = job["metadata"]
    require(metadata["name"] == job_name(proof) and metadata["namespace"] == proof["namespace"], "backup Job target mismatch")
    bound = proof["binding"]
    controller(metadata, "CronJob", bound["backup_cronjob"], bound["backup_cronjob_uid"])
    require(metadata.get("labels", {}).get("bex.co/m144-run") == proof["run"] and metadata.get("annotations", {}).get("bex.co/m144-ownership-sha256") == ownership_digest(proof), "backup Job has no matching creation provenance")
    if proof.get("backup_job_uid"):
        require(metadata["uid"] == proof["backup_job_uid"], "backup Job lifetime changed")
    return {"name": metadata["name"], "uid": metadata["uid"], "ownerCronJobUid": bound["backup_cronjob_uid"], "ownershipSha256": ownership_digest(proof)}


def verify_job(job, proof, cron):
    identity = job_identity(job, proof)
    bound = proof["binding"]
    require(digest(job["spec"]["template"]["spec"]) == bound["backup_pod_spec_sha256"], "backup Job pod policy differs from the committed own template")
    if cron is not None:
        original = cron["spec"]["jobTemplate"]["spec"]
        for key, value in original.items():
            if key not in ("template", "selector", "manualSelector"):
                require(job["spec"].get(key) == value, "backup Job policy differs from own CronJob")
    return {**identity, "podPolicyVerified": True, "startTime": job.get("status", {}).get("startTime"), "completionTime": job.get("status", {}).get("completionTime"), "succeeded": job.get("status", {}).get("succeeded", 0), "failed": job.get("status", {}).get("failed", 0), "conditions": [{key: condition.get(key) for key in ("type", "status", "reason")} for condition in job.get("status", {}).get("conditions", [])]}


def create_backup(kube, proof, cron):
    name, ns = job_name(proof), proof["namespace"]
    require(not proof.get("backup_job_uid"), "backup Job was already recorded; use backup-status")
    existing = kube.run(["-n", ns, "get", "job", name, "--ignore-not-found", "-o", "jsonpath={.metadata.uid}"])
    require(not existing, "backup Job name already exists; creation is never replayed")
    jobs = kube.project("jobs", "job_status", namespace=ns, selector="app.bex.co/app=" + proof["app_name"] + ",app.kubernetes.io/name=disk-backup")
    require(all(job["completed"] or job["failed"] == "True" for job in jobs), "another own backup Job is still in flight")
    template = copy.deepcopy(cron["spec"]["jobTemplate"])
    metadata = template.get("metadata", {})
    metadata.update(name=name, namespace=ns, ownerReferences=[{"apiVersion": "batch/v1", "kind": "CronJob", "name": cron["metadata"]["name"], "uid": cron["metadata"]["uid"], "controller": True, "blockOwnerDeletion": True}])
    metadata.setdefault("labels", {})["bex.co/m144-run"] = proof["run"]
    metadata.setdefault("annotations", {})["bex.co/m144-ownership-sha256"] = ownership_digest(proof)
    job = {"apiVersion": "batch/v1", "kind": "Job", "metadata": metadata, "spec": template["spec"]}
    # Exactly one POST. A transport failure is ambiguous: do not replay it.
    require(now() < instant(proof["expires_at"]), "fixture backup window expired before POST")
    created = json.loads(kube.run(["-n", ns, "create", "-f", "-", "-o", "json"], json.dumps(job)))
    print(json.dumps({"createdBackupJobIdentity": job_identity(created, proof), "podPolicyVerified": False, "next": "Identity only; full policy validation follows. Preserve this receipt even if validation fails."}), flush=True)
    receipt = verify_job(created, proof, cron)
    print(json.dumps({"createdBackupJob": receipt, "next": "Review and commit its exact backup_job_uid before cleanup; status can reconcile a lost response. Confirm the new snapshot through the owned REST API."}), flush=True)
    return receipt


def recorded_backup(kube, proof, cleanup=False):
    require(proof.get("binding"), "commit the reviewed own backup binding first")
    ns = proof["namespace"]
    # Cleanup survives expiry and normal REST/operator teardown. An absent old
    # object is safe; a replacement with the same name never inherits authority.
    for resource, kind, name, uid in (("apps.app.bex.co", "app", proof["app_name"], proof["app_uid"]), ("persistentvolumeclaim", "pvc", proof["pvc_name"], proof["pvc_uid"]), ("cronjob", "workload", proof["binding"]["backup_cronjob"], proof["binding"]["backup_cronjob_uid"])):
        row = kube.project(resource, kind, name, ns, allow_missing=True)
        if row is not None:
            require(row["uid"] == uid, "refuse cleanup across a replacement App/PVC/CronJob lifetime")
            if kind == "app":
                require((row["service"], row["workspace"]) == (proof["service_id"], proof["workspace"]), "cleanup App service/workspace identity changed")
            else:
                owner(row, "App", proof["app_name"], proof["app_uid"])
    name = job_name(proof)
    raw = kube.run(["-n", ns, "get", "job", name, "--ignore-not-found", "-o", "json"])
    if not raw.strip():
        return {"name": name, "uid": proof.get("backup_job_uid"), "state": "deterministic own backup Job is absent"}
    job = json.loads(raw)
    identity = job_identity(job, proof)
    try:
        receipt = verify_job(job, proof, None)
    except ValueError:
        # A real POST may precede a post-response validation failure. Preserve
        # only its provenance-checked identity, explicitly without adopting it.
        print(json.dumps({"unverifiedBackupJobIdentity": identity, "podPolicyVerified": False}), flush=True)
        if not cleanup:
            raise
        # A reviewed actual Job UID, original owner and creation annotation
        # authorize cancellation even if admission changed the pod policy.
        # This does not adopt that policy or permit exec/new work.
        receipt = {**identity, "podPolicyVerified": False, "cleanupOnly": "cancel the exact recorded own Job whose policy did not validate"}
    if not proof.get("backup_job_uid"):
        receipt["next"] = "Read-only reconciliation only; review and commit this actual backup_job_uid before cleanup. Never replay creation."
    return receipt


def cleanup_backup(kube, proof):
    require(proof.get("backup_job_uid"), "commit the actual owned backup Job UID before cleanup")
    receipt = recorded_backup(kube, proof, cleanup=True)
    if "state" in receipt:
        return receipt
    options = {"apiVersion": "v1", "kind": "DeleteOptions", "preconditions": {"uid": proof["backup_job_uid"]}, "propagationPolicy": "Foreground"}
    ns, name = proof["namespace"], job_name(proof)
    kube.run(["delete", "--raw", f"/apis/batch/v1/namespaces/{ns}/jobs/{name}", "-f", "-"], json.dumps(options))
    receipt["cleanup"] = "UID-preconditioned foreground deletion requested; final removal still needs verification"
    return receipt


def run_action(kube, proof, action, phase, image=None):
    require(action in ACTIONS and phase in PHASE_GIB, "unknown fixture action/phase")
    if action == "backup-cleanup":
        return {"run": proof["run"], "serviceId": proof["service_id"], "result": cleanup_backup(kube, proof)}
    if action == "backup-status":
        return {"run": proof["run"], "serviceId": proof["service_id"], "result": recorded_backup(kube, proof)}
    require(now() < instant(proof["expires_at"]), "active fixture window expired")
    size = PHASE_GIB[phase]
    facts = binding(kube, proof, size)
    cron, info = own_cronjob(kube, proof)
    mount = mounted_pod(kube, proof, facts)
    driver = csi_pod(kube, mount["attachment"]["node"], image)
    candidate = {"pv_name": facts["volume"]["name"], "pv_uid": facts["volume"]["uid"], "volume_handle": facts["volume"]["handle"], "storage_class_uid": facts["storageClass"]["uid"], "secret_uid": facts["secretMetadata"]["uid"], "csi_image": image, "csi_image_id": driver["imageId"], **info}
    if action == "bind":
        return {"scope": "selected metadata only; no exec or mutation", "candidateBinding": candidate, "facts": facts, **mount, "csiPod": driver}
    require(proof.get("binding") == candidate, "review and commit exact candidate binding before exec or mutation")
    if action == "inspect":
        result = inspect(kube, proof, facts, mount, image, size)
    else:
        require(size == 5, "manual backup operations are limited to the compacted 5Gi fixture")
        require(instant(proof["snapshot_not_before"]) >= instant(proof["created_at"]) and instant(proof["snapshot_not_before"]) <= now(), "commit the owned compact/hash snapshot-not-before proof first")
        result = create_backup(kube, proof, cron)
    return {"run": proof["run"], "serviceId": proof["service_id"], "phase": phase, "collectedAt": now().isoformat(), "facts": facts, "result": result}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("validate", "validate-status", "validate-cleanup", *ACTIONS))
    parser.add_argument("--phase", choices=PHASE_GIB, default="initial-1g")
    args = parser.parse_args()
    try:
        proof = load_proof(cleanup=args.action in ("validate-status", "validate-cleanup", "backup-status", "backup-cleanup"))
        if args.action in ("validate-status", "validate-cleanup"):
            require(proof.get("binding"), "commit the reviewed own backup binding first")
            if args.action == "validate-cleanup":
                require(proof.get("backup_job_uid"), "commit the actual owned backup Job UID before cleanup")
        image = command(["bash", str(ROOT / "scripts/install-hcloud-csi.sh"), "validate-pin"]).strip() if args.action in ("validate", "bind", "inspect", "backup") else None
        if args.action in ("validate", "validate-status", "validate-cleanup"):
            print(json.dumps({"serviceId": proof["service_id"], "run": proof["run"], "proof": str(PROOF)}))
            return
        require(os.environ.get("KUBECONFIG") and Path(os.environ["KUBECONFIG"]).is_file(), "explicit fetched KUBECONFIG required")
        kube = Kube()
        print(json.dumps(run_action(kube, proof, args.action, args.phase, image), indent=2))
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        parser.exit(1, f"disk-fixture: {type(error).__name__}: {error}\n")


if __name__ == "__main__":
    main()
