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
import http.client
import io
import json
import math
import os
from pathlib import Path
import queue
import re
import signal
import subprocess
import sys
import threading
import time
from urllib.parse import urlencode


ROOT = Path(__file__).resolve().parent.parent
PROOF = Path("deploy/hcloud-csi/fixture-proof.json")
SC = "hcloud-volumes-luks-bounded"
ACTIONS = ("bind", "inspect", "backup", "backup-status", "backup-cleanup", "cleanup-observe")
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
IDENTITY = {**META, "owner": OWNER, "app": r"{.metadata.labels.app\.bex\.co/app}", "appUid": r"{.metadata.labels.app\.bex\.co/app-uid}", "workspace": r"{.metadata.labels.app\.bex\.co/workspace}", "namespace": "{.metadata.namespace}"}
FIELDS["cleanup_workload"] = IDENTITY
SNAPSHOT_LOCATION = ("BEX_DISK_SNAPSHOT_ENDPOINT", "BEX_DISK_SNAPSHOT_BUCKET", "BEX_DISK_SNAPSHOT_PREFIX", "BEX_DISK_SNAPSHOT_REGION")
CLEANUP_SECONDS = 100 * 60
CHANNEL_ACTIONS = ("observer-start", "observer-supervise", "observer-wait", "observer-stop")
CHANNEL_BYTES = 2 * 1024**2
CHANNEL_START_SECONDS = 180
CHANNEL_FINISH_SECONDS = CLEANUP_SECONDS + 60


def observer_directory():
    value = os.environ.get("RUNNER_TEMP", "")
    require(value and Path(value).is_absolute() and Path(value).is_dir(), "explicit existing RUNNER_TEMP required")
    directory = Path(value).resolve() / "bex-fixture-observer"
    require(not directory.is_symlink(), "observer channel must not be a symlink")
    return directory


def observer_scope(proof):
    run_id, attempt = os.environ.get("GITHUB_RUN_ID", ""), os.environ.get("GITHUB_RUN_ATTEMPT", "")
    require(re.fullmatch(r"[1-9][0-9]*", run_id) and re.fullmatch(r"[1-9][0-9]*", attempt), "observer channel requires its exact Actions run and attempt")
    return {"run": proof["run"], "serviceId": proof["service_id"], "namespace": proof["namespace"], "appUid": proof["app_uid"], "proofDigest": digest(proof), "workflowRunId": run_id, "workflowRunAttempt": attempt}


def channel_write(directory, name, value):
    temporary = directory / (name + ".tmp-" + str(os.getpid()))
    with temporary.open("x") as output:
        json.dump(value, output, separators=(",", ":"))
        output.write("\n")
    temporary.replace(directory / name)


def channel_read(directory, name):
    path = directory / name
    if not path.exists():
        return None
    require(path.is_file() and not path.is_symlink() and path.stat().st_size <= 16384, "invalid bounded observer channel file")
    return json.loads(path.read_text())


def observer_session(directory, proof):
    session = channel_read(directory, "session.json")
    require(session and session["scope"] == observer_scope(proof), "observer channel belongs to another proof or workflow attempt")
    return session


def observer_ready(receipt, proof, session):
    require(receipt["run"] == proof["run"] and receipt["serviceId"] == proof["service_id"], "observer readiness belongs to another fixture")
    observation = receipt["cleanupObservation"]
    expected = {"state": "observerReady", "maximumSeconds": CLEANUP_SECONDS, "fixtureExpiresAt": proof["expires_at"], "namespace": proof["namespace"], "appUid": proof["app_uid"], "purgeJobName": purge_job_name(proof)}
    require(all(observation[key] == value for key, value in expected.items()), "observer readiness scope changed")
    started, collected = instant(observation["startedAt"]), instant(receipt["collectedAt"])
    require(instant(session["startedAt"]) <= started <= collected <= now() and collected < instant(proof["expires_at"]), "observer readiness timestamp is stale or expired")
    remaining = observation["fixtureBudgetRemainingSeconds"]
    require(type(remaining) is int and 0 < remaining <= 7200, "observer readiness has no remaining fixture budget")
    versions = observation["watchResourceVersions"]
    require(all(isinstance(versions[key], str) and re.fullmatch(r"[1-9][0-9]*", versions[key]) for key in ("app", "purgeJob")), "observer readiness has no established watch resourceVersions")
    provider = observation["providerBaseline"]
    handle = proof["binding"]["volume_handle"]
    match = re.fullmatch(r"bounded-ext4-v1:([1-9][0-9]*)", handle)
    require(match and provider["volumeHandle"] == handle and type(provider["volumeId"]) is int and provider["volumeId"] == int(match[1]) and provider["http"] == 200 and provider["state"] == "present", "observer readiness has no exact positive provider baseline")
    return {"run": proof["run"], "serviceId": proof["service_id"], "collectedAt": receipt["collectedAt"], "cleanupObservation": {**expected, "startedAt": observation["startedAt"], "fixtureBudgetRemainingSeconds": remaining, "watchResourceVersions": {key: versions[key] for key in ("app", "purgeJob")}, "providerBaseline": {key: provider[key] for key in ("volumeHandle", "volumeId", "http", "state")}}}


def stop_observer_child(process):
    # The unreaped Popen handle owns this process group; no PID file is trusted.
    if process.poll() is None:
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
    process.wait(timeout=5)


def supervise_observer(directory, proof, session, phase):
    stopped = threading.Event()
    previous = {sig: signal.signal(sig, lambda *_: stopped.set()) for sig in (signal.SIGINT, signal.SIGTERM)}
    process, reader = None, None
    lines = queue.Queue(maxsize=32)
    reason, exit_code, ready, complete = "startup-failed", None, False, False
    deadline = time.monotonic() + CHANNEL_FINISH_SECONDS

    def status(state):
        channel_write(directory, "status.json", {"scope": session["scope"], "state": state, "updatedAt": now().isoformat(), "ready": ready, "exitCode": exit_code, "reason": reason})

    def read_output():
        try:
            while not stopped.is_set():
                raw = process.stdout.readline(262145)
                if not raw:
                    break
                require(len(raw) <= 262144 and raw.endswith(b"\n"), "observer line exceeded its fixed bound")
                while not stopped.is_set():
                    try:
                        lines.put(raw, timeout=0.1)
                        break
                    except queue.Full:
                        continue
        except (ValueError, OSError):
            stopped.set()

    try:
        status("starting")
        process = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), "cleanup-observe", "--phase", phase], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, start_new_session=True)
        reader = threading.Thread(target=read_output, daemon=True)
        reader.start()
        reason, written, heartbeat, reported_ready = "observing", 0, 0, False
        with (directory / "observer.jsonl").open("xb") as output:
            while True:
                stop = channel_read(directory, "stop.json")
                if stop is not None:
                    require(stop == {"scope": session["scope"]}, "observer stop request belongs to another workflow attempt")
                    stopped.set()
                if stopped.is_set() or time.monotonic() >= deadline:
                    reason = "stopped-or-deadline"
                    break
                try:
                    raw = lines.get(timeout=0.2)
                except queue.Empty:
                    raw = None
                if raw is not None:
                    receipt = json.loads(raw)
                    require(receipt["run"] == proof["run"] and receipt["serviceId"] == proof["service_id"] and isinstance(receipt["cleanupObservation"], dict), "observer output has a foreign scope")
                    state = receipt["cleanupObservation"].get("state")
                    if state == "observerReady":
                        require(not ready, "observer emitted duplicate readiness")
                        receipt = observer_ready(receipt, proof, session)
                    raw = (json.dumps({key: receipt[key] for key in ("run", "serviceId", "collectedAt", "cleanupObservation")}) + "\n").encode()
                    written += len(raw)
                    require(written <= CHANNEL_BYTES, "observer artifact exceeded its fixed output bound")
                    output.write(raw)
                    output.flush()
                    if state == "observerReady":
                        channel_write(directory, "ready.json", receipt)
                        ready = True
                    complete |= state == "complete"
                if ready != reported_ready or time.monotonic() - heartbeat >= 1:
                    status("running")
                    heartbeat, reported_ready = time.monotonic(), ready
                if process.poll() is not None and not reader.is_alive() and lines.empty():
                    exit_code = process.wait()
                    reason = "observer-exited" if ready and complete else "exited-without-complete-proof"
                    break
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError, json.JSONDecodeError):
        reason = "invalid-or-failed-observer-channel"
    finally:
        stopped.set()
        if process is not None:
            try:
                stop_observer_child(process)
            except (OSError, subprocess.SubprocessError):
                reason, exit_code = "child-reap-unproved", None
            if process.stdout:
                process.stdout.close()
        if reader:
            reader.join(timeout=2)
        status("finished")
        for sig, handler in previous.items():
            signal.signal(sig, handler)
    return exit_code == 0 and ready and reason == "observer-exited"


def wait_observer_channel(directory, session, readiness=False, timeout=None):
    deadline = time.monotonic() + (timeout if timeout is not None else CHANNEL_START_SECONDS if readiness else CHANNEL_FINISH_SECONDS)
    while time.monotonic() < deadline:
        status = channel_read(directory, "status.json")
        if status:
            require(status["scope"] == session["scope"], "observer status belongs to another workflow attempt")
            if status["state"] == "finished":
                require(not readiness and status["ready"] is True and type(status["exitCode"]) is int and status["exitCode"] == 0 and status["reason"] == "observer-exited", "observer ended without complete approved cleanup evidence")
                return status
            require(status["state"] in ("starting", "running"), "unknown observer supervisor state")
            require((now() - instant(status["updatedAt"])).total_seconds() <= 20, "observer supervisor heartbeat is stale; cleanup is unknown")
            if readiness and status["ready"] and status["state"] == "running":
                return channel_read(directory, "ready.json")
        time.sleep(0.2)
    raise ValueError("observer channel wait expired; readiness or cleanup remains unproved")


def observer_channel(action, proof, phase):
    directory = observer_directory()
    process, created = None, False
    if action == "observer-start":
        session = {"scope": observer_scope(proof), "startedAt": now().isoformat(), "phase": phase}
    else:
        if action == "observer-stop" and not directory.exists():
            return {"observerChannel": "not-started"}
        session = observer_session(directory, proof)
    if action == "observer-supervise":
        require(session["phase"] == phase, "observer supervisor phase changed")
        require(not (directory / "status.json").exists(), "observer supervisor cannot adopt an existing channel")
        require(supervise_observer(directory, proof, session, phase), "observer cleanup remains unproved")
        return {"observerChannel": "finished"}
    if action == "observer-stop":
        channel_write(directory, "stop.json", {"scope": session["scope"]})
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            status = channel_read(directory, "status.json")
            if status and status["scope"] == session["scope"] and status["state"] == "finished":
                require(status["reason"] != "child-reap-unproved", "observer child shutdown is unproved")
                return {"observerChannel": "stopped", "exitCode": status["exitCode"]}
            time.sleep(0.2)
        raise ValueError("observer stop was not acknowledged; no foreign process was signalled")
    try:
        if action == "observer-start":
            directory.mkdir(mode=0o700)
            created = True
            channel_write(directory, "session.json", session)
            process = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), "observer-supervise", "--phase", phase], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        result = wait_observer_channel(directory, session, readiness=action == "observer-start")
        if action == "observer-start":
            require(process.poll() is None, "observer supervisor exited before readiness handoff")
            return observer_ready(result, proof, session)
        return result
    except BaseException:
        if action != "observer-start" or created:
            channel_write(directory, "stop.json", {"scope": session["scope"]})
        if process is not None:
            try:
                process.wait(timeout=12)
            except subprocess.TimeoutExpired:
                stop_observer_child(process)
        raise


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

    def watch_cleanup(self, resource, kind, name, namespace):
        require((resource, kind) in (("apps.app.bex.co", "cleanup_app"), ("jobs", "cleanup_job")), "unknown fixed cleanup watch")
        endpoint = "/apis/" + ("app.bex.co/v1alpha1" if resource == "apps.app.bex.co" else "batch/v1") + "/namespaces/" + namespace + "/" + ("apps" if resource == "apps.app.bex.co" else "jobs")
        # kubectl's aggregated JSONPath list can discard collection metadata
        # for an empty result. The exact API list retains the replay version.
        raw = self.run(["get", "--raw", endpoint + "?" + urlencode({"fieldSelector": "metadata.name=" + name})])
        require(len(raw) <= 262144, "initial cleanup list exceeded its fixed size bound")
        result = json.loads(raw)
        version = result["metadata"]["resourceVersion"]
        require(re.fullmatch(r"[0-9]+", version), "cleanup watch resourceVersion unavailable")
        require(len(result["items"]) <= 1, "fixed cleanup list returned multiple identities")
        initial = [cleanup_watch_row(row, kind) for row in result["items"]]
        require(all(row["name"] == name for row in initial), "fixed cleanup list returned another name")
        return CleanupWatch(endpoint, name, kind, version, initial)


def cleanup_watch_row(obj, kind):
    metadata = obj["metadata"]
    labels, annotations = metadata.get("labels", {}), metadata.get("annotations", {})
    row = {"name": metadata["name"], "uid": metadata["uid"], "deleting": metadata.get("deletionTimestamp", ""), "resourceVersion": metadata["resourceVersion"]}
    if kind == "cleanup_app":
        disk = obj.get("spec", {}).get("disk") or {}
        return {**row, "service": labels.get("bex.co/app-id", ""), "workspace": labels.get("app.bex.co/workspace", ""), "size": str(disk.get("sizeGB", "")), "purgeComplete": annotations.get("app.bex.co/disk-snapshot-purge-complete", "")}
    owners = [ref for ref in metadata.get("ownerReferences", []) if ref.get("controller")]
    pod = obj.get("spec", {}).get("template", {}).get("spec", {})
    containers = pod.get("containers", [])
    container = next((item for item in containers if item.get("name") == "snapshot"), {})
    env = {item["name"]: item.get("value", "") for item in container.get("env", [])}
    return {**row, "namespace": metadata.get("namespace", ""), "owner": "".join(f"{ref.get('kind')}/{ref.get('name')}/{ref.get('uid')}" for ref in owners), "app": labels.get("app.bex.co/app", ""), "appUid": labels.get("app.bex.co/app-uid", ""), "workspace": labels.get("app.bex.co/workspace", ""), "appNamespace": labels.get("app.bex.co/app-namespace", ""), "component": labels.get("app.bex.co/component", ""), "complete": "".join(item.get("status", "") for item in obj.get("status", {}).get("conditions", []) if item.get("type") == "Complete"), "image": container.get("image", ""), "command": json.dumps(container.get("command")), "args": json.dumps(container.get("args")), "containerNames": "".join(item.get("name", "") + ";" for item in containers), "initNames": "".join(item.get("name", "") + ";" for item in pod.get("initContainers", [])), "volumes": json.dumps(pod.get("volumes", [])), "workspaceEnv": env.get("BEX_DISK_WORKSPACE", ""), "diskEnv": env.get("BEX_DISK_ID", ""), **{key: env.get(key, "") for key in SNAPSHOT_LOCATION}}


class CleanupWatch:
    """Replay from the initial list version, including short-lived delete events."""

    def __init__(self, endpoint, name, kind, version, initial):
        self.version, self.initial = version, initial
        self.events, self.stopped, self.started = queue.Queue(maxsize=256), threading.Event(), threading.Event()
        self.failure, self.process = None, None
        self.process_lock = threading.Lock()
        def collect():
            current = version
            event_count = 0
            initial_sync = True
            process = None
            try:
                while not self.stopped.is_set():
                    parameters = {"fieldSelector": "metadata.name=" + name, "resourceVersion": current, "watch": "true", "allowWatchBookmarks": "true", "timeoutSeconds": "60"}
                    if initial_sync:
                        parameters.update(sendInitialEvents="true", resourceVersionMatch="NotOlderThan")
                    query = urlencode(parameters)
                    argv = ["kubectl", "--request-timeout=75s", "get", "--raw", endpoint + "?" + query]
                    process = subprocess.Popen(argv, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
                    with self.process_lock:
                        self.process = process
                        cancelled = self.stopped.is_set()
                    # Popen itself may finish after close() has already returned.
                    # The publishing thread owns cleanup of that late child.
                    if cancelled:
                        break
                    while line := process.stdout.readline(262145):
                        require(len(line) <= 262144, "cleanup watch response exceeded its fixed size bound")
                        event = json.loads(line)
                        event_count += 1
                        require(event_count <= 4096, "cleanup watch exceeded its fixed event bound")
                        require(event.get("type") in ("ADDED", "MODIFIED", "DELETED", "BOOKMARK"), "cleanup watch failed or returned an unsupported event")
                        current = event["object"]["metadata"]["resourceVersion"]
                        require(re.fullmatch(r"[0-9]+", current), "cleanup watch resourceVersion unavailable")
                        if event["type"] == "BOOKMARK" and initial_sync and event["object"]["metadata"].get("annotations", {}).get("k8s.io/initial-events-end") == "true":
                            initial_sync = False
                            self.version = current
                            self.started.set()
                        if event["type"] != "BOOKMARK":
                            self.events.put_nowait({"event": event["type"], **cleanup_watch_row(event["object"], kind)})
                    require(self.stopped.is_set() or process.wait() == 0, "cleanup watch interrupted; evidence is incomplete")
                    process.stdout.close()
            except (ValueError, KeyError, TypeError, OSError, queue.Full):
                self.failure = "cleanup watch failed or exceeded its bounded event buffer; evidence is incomplete"
            finally:
                self.stop_process(process)
                if process is not None:
                    process.stdout.close()
                self.started.set()

        self.thread = threading.Thread(target=collect, daemon=True)
        self.thread.start()
        if not self.started.wait(25) or self.failure:
            self.close()
            raise ValueError("cleanup watch could not start")

    def drain(self):
        require(not self.failure, self.failure)
        rows = []
        while True:
            try:
                rows.append(self.events.get_nowait())
            except queue.Empty:
                return rows

    @staticmethod
    def stop_process(process):
        if process is not None and process.poll() is None:
            try:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=2)
            except ProcessLookupError:
                pass

    def close(self):
        self.stopped.set()
        with self.process_lock:
            process = self.process
        self.stop_process(process)
        self.thread.join(timeout=2)


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


def purge_job_name(proof):
    suffix = hashlib.sha256(proof["app_uid"].encode()).hexdigest()
    name = "dskpurge-" + proof["app_name"] + "-" + suffix[:8]
    return name if len(name) <= 63 else "dskpurge-" + proof["app_name"][:41] + "-" + suffix[:12]


def provider_volume(proof, token=None):
    match = re.fullmatch(r"bounded-ext4-v1:([1-9][0-9]*)", proof["binding"]["volume_handle"])
    require(match is not None, "exact reviewed provider handle required")
    token = token or os.environ.get("HCLOUD_TOKEN")
    require(token, "cleanup observation requires the existing HCLOUD_TOKEN")
    connection = http.client.HTTPSConnection("api.hetzner.cloud", timeout=10)
    try:
        connection.request("GET", "/v1/volumes/" + match[1], headers={"Authorization": "Bearer " + token})
        response = connection.getresponse()
        status = response.status
        volume_id = None
        if status == 200:
            raw = response.read(65537)
            if len(raw) <= 65536:
                volume_id = json.loads(raw).get("volume", {}).get("id")
        response.close()
        return {"volumeHandle": proof["binding"]["volume_handle"], "volumeId": volume_id, "http": status, "state": "absent" if status == 404 else "present" if status == 200 and type(volume_id) is int and volume_id == int(match[1]) else "unknown"}
    except (ValueError, TypeError, AttributeError, OSError, http.client.HTTPException):
        return {"volumeHandle": proof["binding"]["volume_handle"], "http": None, "state": "unknown"}
    finally:
        connection.close()


def verify_purge_event(row, proof, snapshot_location):
    require((row["name"], row["namespace"], row["app"], row["appUid"], row["workspace"], row["appNamespace"], row["component"]) == (purge_job_name(proof), proof["namespace"], proof["app_name"], proof["app_uid"], proof["namespace"], proof["namespace"], "disk-snapshot-purge"), "purge Job identity or canonical workspace changed")
    owner(row, "App", proof["app_name"], proof["app_uid"])
    require(re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", row["uid"]), "actual purge Job UID is unavailable")
    require(re.fullmatch(r"ghcr\.io/bex-co/bex-operator@sha256:[0-9a-f]{64}", proof["binding"]["backup_image"]), "reviewed purge helper digest required")
    require(re.fullmatch(r"ghcr\.io/bex-co/bex-operator@sha256:[0-9a-f]{64}", row["image"]), "purge helper must name an exact operator digest")
    require(row["containerNames"] == "snapshot;" and not row["initNames"] and row["volumes"] in ("", "[]") and json.loads(row["command"]) == ["/disk-snapshot"] and json.loads(row["args"]) == ["purge"] and (row["workspaceEnv"], row["diskEnv"]) == (proof["namespace"], proof["app_name"]), "purge Job does not run the exact own-prefix helper policy")
    require(all(row[key] == value for key, value in snapshot_location.items()), "purge Job snapshot location differs from the committed own backup template")
    return row["image"] == proof["binding"]["backup_image"]


class PurgeAttempts:
    def __init__(self, proof, snapshot_location):
        self.proof, self.snapshot_location = proof, snapshot_location
        self.current, self.attempts = None, {}

    def observe(self, row):
        approved = verify_purge_event(row, self.proof, self.snapshot_location)
        previous = self.current
        if previous is not None and previous["uid"] != row["uid"]:
            require(previous["deletedObserved"] and row["uid"] not in self.attempts, "purge Job replacement requires its prior UID's observed DELETED event")
            self.current = None
        if self.current is None:
            require(len(self.attempts) < 128, "cleanup observer exceeded its fixed purge-attempt bound")
            self.current = {"name": row["name"], "uid": row["uid"], "namespace": self.proof["namespace"], "ownerAppUid": self.proof["app_uid"], "image": row["image"], "trust": "APPROVED" if approved else "UNAPPROVED", "jobCompleteObserved": False, "deletedObserved": False}
            self.attempts[row["uid"]] = self.current
        require(self.current["image"] == row["image"], "purge helper image changed within one Job lifetime")
        require(not self.current["deletedObserved"] or row["event"] == "DELETED", "deleted purge Job lifetime reappeared")
        self.current["resourceVersion"] = row["resourceVersion"]
        self.current["jobCompleteObserved"] |= row["complete"] == "True"
        self.current["deletedObserved"] |= row["event"] == "DELETED"
        return previous is not None and previous["uid"] != row["uid"]

    def completion(self):
        if self.current and self.current["trust"] == "APPROVED" and self.current["jobCompleteObserved"]:
            return {**self.current, "prefixEmptyEvidence": "reviewed /disk-snapshot purge exits successfully only after its unfiltered own-prefix empty check"}
        return None


class AwaitPurgeWatch(ValueError):
    pass


def cleanup_inventory(kube, proof, known):
    ns, app, bound = proof["namespace"], proof["app_name"], proof["binding"]
    pending = []

    def remember(resource, row):
        key = (resource, row["name"])
        previous = known.get(key)
        if previous is not None and previous != row["uid"] and key == ("jobs", purge_job_name(proof)):
            raise AwaitPurgeWatch("await policy-verified purge Job watch lifetime")
        require(previous in (None, row["uid"]) or (resource == "volumeattachments" and row["pv"] == bound["pv_name"]), "cleanup refused a replacement resource lifetime")
        known[key] = row["uid"]
        identity = {"resource": resource, "name": row["name"], "uid": row["uid"]}
        if previous is not None and previous != row["uid"]:
            identity["priorUid"] = previous
        pending.append(identity)

    fixed = (("apps.app.bex.co", "app", app, proof["app_uid"], ns), ("persistentvolumeclaim", "pvc", proof["pvc_name"], proof["pvc_uid"], ns), ("persistentvolume", "pv", bound["pv_name"], bound["pv_uid"], None), ("secret", "secret", proof["pvc_name"] + "-luks", bound["secret_uid"], ns), ("cronjobs", "workload", bound["backup_cronjob"], bound["backup_cronjob_uid"], ns))
    for resource, kind, name, uid, namespace in fixed:
        row = kube.project(resource, kind, name, namespace, allow_missing=True)
        if row is None:
            continue
        require(row["uid"] == uid, "cleanup refused a replacement App/PVC/PV/Secret/CronJob lifetime")
        if kind == "app":
            require((row["service"], row["workspace"]) == (proof["service_id"], ns), "cleanup App identity changed")
        elif kind == "pv":
            require((row["handle"], row["driver"], row["namespace"], row["claim"], row["claimUid"], row["class"]) == (bound["volume_handle"], "csi.hetzner.cloud", ns, proof["pvc_name"], proof["pvc_uid"], SC), "cleanup PV provider or exact claim binding changed")
        else:
            owner(row, "App", app, proof["app_uid"])
        remember(resource, row)

    # Keep every observed owner lifetime, even after its label or parent vanishes.
    owners = {f"App/{app}/{proof['app_uid']}", f"CronJob/{bound['backup_cronjob']}/{bound['backup_cronjob_uid']}"}
    kinds = {"deployments": "Deployment", "replicasets": "ReplicaSet", "cronjobs": "CronJob", "jobs": "Job"}
    owners.update(f"{kinds[resource]}/{name}/{uid}" for (resource, name), uid in known.items() if resource in kinds)
    observed = set()
    for resource in ("deployments", "replicasets", "cronjobs", "jobs", "pods"):
        # Owner UID chains must still find a newly created child whose labels
        # drifted before its first sample. Unrelated metadata never leaves memory.
        rows = kube.project(resource, "cleanup_workload", namespace=ns)
        for row in rows:
            if row["app"] != app and row["appUid"] != proof["app_uid"] and row["owner"] not in owners:
                continue
            require(row["namespace"] == ns and row["app"] in ("", app) and row["appUid"] in ("", proof["app_uid"]) and row["workspace"] in ("", ns) and row["owner"] in owners, "cleanup workload has a foreign lifetime or workspace")
            if resource == "jobs" and row["name"] == job_name(proof) and proof.get("backup_job_uid"):
                require(row["uid"] == proof["backup_job_uid"], "recorded backup Job lifetime changed")
            remember(resource, row)
            observed.add((resource, row["name"]))
            if resource in kinds:
                owners.add(f"{kinds[resource]}/{row['name']}/{row['uid']}")
    for row in kube.project("volumeattachments", "attachment"):
        if row["pv"] == bound["pv_name"]:
            remember("volumeattachments", row)
            observed.add(("volumeattachments", row["name"]))
    fixed_names = {(resource, name) for resource, _, name, _, _ in fixed}
    for (resource, name), uid in list(known.items()):
        if (resource, name) in fixed_names or (resource, name) in observed:
            continue
        row = kube.project(resource, "attachment" if resource == "volumeattachments" else "workload", name, None if resource == "volumeattachments" else ns, allow_missing=True)
        if row is not None:
            if resource == "volumeattachments":
                require(row["pv"] == bound["pv_name"], "observed attachment changed its exact PV binding")
            require(row["uid"] == uid, "cleanup refused a replacement observed resource lifetime")
            remember(resource, row)
    return sorted(pending, key=lambda row: (row["resource"], row["name"]))


def sample_inspection(kube, proof, size, image):
    sample = run_action(kube, proof, "inspect", "initial-1g" if size == "1" else "grown-5g", image)
    result = sample["result"]
    return {"state": "inspection", "requestedSizeGiB": int(size), "geometry": result["geometry"], "kubeletMetrics": result["kubeletMetrics"], "servicePod": {key: result["servicePod"][key] for key in ("name", "uid", "node", "started")}, "attachment": {key: result["attachment"][key] for key in ("name", "uid", "node")}, "csiPod": {key: result["csiPod"][key] for key in ("name", "uid", "image", "imageId", "containerId")}}


def observe_cleanup(kube, proof, phase, image):
    require(proof.get("binding"), "commit the reviewed fixture binding before observing cleanup")
    token = os.environ.get("HCLOUD_TOKEN")
    require(token, "cleanup observation requires the existing HCLOUD_TOKEN")
    require(image == proof["binding"]["csi_image"], "observer CSI image must match the committed reviewed binding")
    deadline = time.monotonic() + CLEANUP_SECONDS
    binding(kube, proof, PHASE_GIB[phase])
    cron, _ = own_cronjob(kube, proof)
    env = {item["name"]: item.get("value", "") for item in cron["spec"]["jobTemplate"]["spec"]["template"]["spec"]["containers"][0].get("env", [])}
    snapshot_location = {key: env.get(key, "") for key in SNAPSHOT_LOCATION}
    require(snapshot_location[SNAPSHOT_LOCATION[0]] and snapshot_location[SNAPSHOT_LOCATION[1]], "own committed snapshot location is unavailable")
    watches = []
    known = {}
    observed_lifetimes = set()
    purge_attempts = PurgeAttempts(proof, snapshot_location)
    completed_job, completed_app, deletion_started, purge_seen, last = None, False, False, False, None
    live_size, probe_key, probe_at, emitted_bytes = str(PHASE_GIB[phase]), None, 0, 0

    def emit(receipt):
        nonlocal emitted_bytes
        encoded = json.dumps({"cleanupObservation": receipt, "run": proof["run"], "serviceId": proof["service_id"], "collectedAt": now().isoformat()})
        emitted_bytes += len(encoded)
        require(emitted_bytes <= CHANNEL_BYTES, "cleanup observer exceeded its fixed output bound")
        print(encoded, flush=True)

    def inventory():
        pending = cleanup_inventory(kube, proof, known)
        for item in pending:
            observed_lifetimes.add((item["resource"], item["name"], item["uid"]))
            if "priorUid" in item:
                observed_lifetimes.add((item["resource"], item["name"], item["priorUid"]))
        require(len(observed_lifetimes) <= 1024, "cleanup observer exceeded its fixed resource-lifetime bound")
        return pending

    def capture_events():
        nonlocal completed_app, completed_job, deletion_started, purge_seen, live_size
        for row in job_watch.drain():
            if purge_attempts.observe(row):
                completed_app = False
            purge_seen = True
            key = ("jobs", row["name"])
            known[key] = row["uid"]
            observed_lifetimes.add(("jobs", row["name"], row["uid"]))
            completed_job = purge_attempts.completion()
            emit({"watch": "purgeJob", "event": row["event"], **purge_attempts.current})
        for row in app_watch.drain():
            require(row["uid"] == proof["app_uid"] and (row["service"], row["workspace"]) == (proof["service_id"], proof["namespace"]), "cleanup App watch saw a replacement or foreign workspace")
            deletion_started = deletion_started or bool(row["deleting"]) or row["event"] == "DELETED"
            live_size = row["size"]
            require(row["purgeComplete"] in ("", proof["app_uid"]), "cleanup completion belongs to another App lifetime")
            completed_app = row["purgeComplete"] == proof["app_uid"]
            emit({"watch": "app", "event": row["event"], "uid": row["uid"], "resourceVersion": row["resourceVersion"], "purgeComplete": completed_app})

    try:
        for resource, kind, name in (("apps.app.bex.co", "cleanup_app", proof["app_name"]), ("jobs", "cleanup_job", purge_job_name(proof))):
            watches.append(kube.watch_cleanup(resource, kind, name, proof["namespace"]))
        app_watch, job_watch = watches
        require(len(app_watch.initial) == 1 and not app_watch.initial[0]["deleting"] and not app_watch.initial[0]["purgeComplete"], "cleanup observer must start before DELETE and before purge completion")
        require(app_watch.initial[0]["uid"] == proof["app_uid"] and not job_watch.initial, "cleanup observer initial App/purge lifetime changed")
        pending = inventory()
        baseline = provider_volume(proof, token)
        require(baseline["state"] == "present", "cleanup provider baseline is unproved; exact bound volume must return HTTP200 with its matching numeric ID before readiness")
        capture_events()
        require(not deletion_started and not completed_app and not purge_seen, "cleanup observer must be ready before DELETE")
        emit({"state": "observerReady", "startedAt": now().isoformat(), "maximumSeconds": CLEANUP_SECONDS, "fixtureExpiresAt": proof["expires_at"], "fixtureBudgetRemainingSeconds": max(0, int((instant(proof["expires_at"]) - now()).total_seconds())), "namespace": proof["namespace"], "appUid": proof["app_uid"], "purgeJobName": purge_job_name(proof), "providerBaseline": baseline, "watchResourceVersions": {"app": app_watch.version, "purgeJob": job_watch.version}})
        while time.monotonic() < deadline:
            capture_events()
            try:
                pending = inventory()
            except AwaitPurgeWatch:
                capture_events()
                emit({"state": "pending", "reason": "awaiting exact purge Job lifetime verification from its continuous watch"})
                time.sleep(min(2, max(0, deadline - time.monotonic())))
                continue
            current_probe_key = (live_size, tuple(item["uid"] for item in pending if item["resource"] == "pods"))
            if not deletion_started and now() < instant(proof["expires_at"]) and (current_probe_key != probe_key or time.monotonic() - probe_at >= 60):
                probe_key, probe_at = current_probe_key, time.monotonic()
                try:
                    require(live_size in ("1", "5"), "observer only inspects the fixed 1Gi/5Gi fixture")
                    emit(sample_inspection(kube, proof, live_size, image))
                except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
                    emit({"state": "inspection-unproved", "requestedSizeGiB": live_size, "reason": type(error).__name__})
            provider = provider_volume(proof, token) if not pending else {"state": "not-yet-checked"}
            capture_events()
            cleanup_absent = not pending and provider["state"] == "absent" and completed_app
            unapproved_complete = purge_attempts.current and purge_attempts.current["trust"] == "UNAPPROVED" and purge_attempts.current["jobCompleteObserved"]
            if cleanup_absent and (completed_job or unapproved_complete):
                # Include any short-lived identities just replayed by the watches
                # in a final exact-name absence sweep.
                try:
                    pending = inventory()
                except AwaitPurgeWatch:
                    capture_events()
                    emit({"state": "pending", "reason": "purge lifetime changed during the final absence sweep; awaiting continuous watch verification"})
                    time.sleep(min(2, max(0, deadline - time.monotonic())))
                    continue
            state = {"state": "complete" if not pending and provider["state"] == "absent" and completed_app and completed_job else "UNAPPROVED" if not pending and provider["state"] == "absent" and completed_app and unapproved_complete else "pending", "remaining": pending, "provider": provider, "purgeJob": completed_job, "purgeAttempts": list(purge_attempts.attempts.values()), "appCompletionObserved": completed_app}
            if state["state"] in ("complete", "UNAPPROVED"):
                state["observedLifetimes"] = [{"resource": resource, "name": name, "uid": uid} for resource, name, uid in sorted(observed_lifetimes)]
            if state["state"] == "UNAPPROVED":
                state["reason"] = "actual helper digest requires strict offline signature/source review; no prefix-empty or completed-cleanup authority is granted"
            if state != last:
                emit(state)
                last = state
            if state["state"] == "complete":
                return state
            if state["state"] == "UNAPPROVED":
                raise ValueError("cleanup has unapproved helper evidence; strict offline image signature/source review is required")
            time.sleep(min(2 if deletion_started else 10, max(0, deadline - time.monotonic())))
        emit({"state": "unknown", "reason": "bounded cleanup observer expired without every required proof", "lastObservation": last})
        raise ValueError("cleanup is unproved after the bounded observation window")
    finally:
        for watch in watches:
            watch.close()


def run_action(kube, proof, action, phase, image=None):
    require(action in ACTIONS and phase in PHASE_GIB, "unknown fixture action/phase")
    if action == "backup-cleanup":
        return {"run": proof["run"], "serviceId": proof["service_id"], "result": cleanup_backup(kube, proof)}
    if action == "backup-status":
        return {"run": proof["run"], "serviceId": proof["service_id"], "result": recorded_backup(kube, proof)}
    require(now() < instant(proof["expires_at"]), "active fixture window expired")
    if action == "cleanup-observe":
        return observe_cleanup(kube, proof, phase, image)
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
    parser.add_argument("action", choices=("validate", "validate-status", "validate-cleanup", "validate-observer", *CHANNEL_ACTIONS, *ACTIONS))
    parser.add_argument("--phase", choices=PHASE_GIB, default="initial-1g")
    args = parser.parse_args()
    if args.action in (*CHANNEL_ACTIONS, "cleanup-observe") and args.action != "observer-supervise":
        signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
    try:
        proof = load_proof(cleanup=args.action in ("validate-status", "validate-cleanup", "backup-status", "backup-cleanup", "observer-wait", "observer-stop"))
        if args.action in CHANNEL_ACTIONS:
            print(json.dumps(observer_channel(args.action, proof, args.phase)))
            return
        if args.action in ("validate-status", "validate-cleanup", "validate-observer"):
            require(proof.get("binding"), "commit the reviewed own backup binding first")
            if args.action == "validate-cleanup":
                require(proof.get("backup_job_uid"), "commit the actual owned backup Job UID before cleanup")
        image = command(["bash", str(ROOT / "scripts/install-hcloud-csi.sh"), "validate-pin"]).strip() if args.action in ("validate", "bind", "inspect", "backup", "validate-observer", "cleanup-observe") else None
        if args.action in ("validate", "validate-status", "validate-cleanup", "validate-observer"):
            print(json.dumps({"serviceId": proof["service_id"], "run": proof["run"], "proof": str(PROOF)}))
            return
        require(os.environ.get("KUBECONFIG") and Path(os.environ["KUBECONFIG"]).is_file(), "explicit fetched KUBECONFIG required")
        kube = Kube()
        result = run_action(kube, proof, args.action, args.phase, image)
        if args.action != "cleanup-observe":
            print(json.dumps(result, indent=2))
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        parser.exit(1, f"disk-fixture: {type(error).__name__}: {error}\n")


if __name__ == "__main__":
    main()
