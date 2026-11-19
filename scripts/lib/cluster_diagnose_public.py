#!/usr/bin/env python3
"""Cluster diagnose for a public log: `scripts/cluster-diagnose.sh <srv-id> --public-log`.

Reads the same production state as the local run, as JSON, and prints only
what a public Actions log may carry (docs/ADR019-infra-credentials.md
§Decision 5, w5/m501): pod phases, waiting and termination reasons, restart
counts, scheduler and autoscaler verdicts, node readiness and allocation,
Machine and MachineDeployment state. Tenant namespaces, pod, workload and
service names, node names and addresses become keyed hashes
(scripts/lib/public_log.py), and a tenant pod's events print as their reason
only, except the platform's own scheduling, scale-up and volume verdicts.
Every line passes through the redactor once more on its way out.
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from public_log import (  # noqa: E402
    COMPONENT_NAMESPACES, TAGGED_RE, WORKLOAD_NAMESPACES, Redactor, cluster_key,
)

# Events whose message is the platform's own verdict about placement, scaling
# or volumes. Every other tenant pod event prints as its reason only: probe
# output, image references and container commands are the tenant's.
MESSAGE_REASONS = frozenset({
    "FailedScheduling", "NotTriggerScaleUp", "TriggeredScaleUp", "FailedAttachVolume",
    "FailedMount", "FailedCreatePodSandBox", "Evicted", "Preempted",
})
AUTOSCALER_LOG_RE = re.compile(
    r"scale.?up|unschedulable|max node|no node group|error|predicate|skipping", re.IGNORECASE
)
# "<namespace>/<pod>" in a log line; a dotted prefix (node.kubernetes.io/...)
# is a label key, not a namespace. Group 1 is the text just before it.
POD_REF_RE = re.compile(r"(.{0,6})(?<![A-Za-z0-9./_-])([a-z0-9][a-z0-9-]{0,62})/([a-z0-9][a-z0-9.-]{0,252})(?![A-Za-z0-9/_-])")
AUTOSCALER_SELECTOR = "app.kubernetes.io/instance=cluster-autoscaler"
SERVICE_ID_RE = re.compile(r"srv-[0-9a-v]{20}")
PENDING_LIMIT = 40
# Machine conditions where False is a problem (v1beta2 also reports Updating,
# Paused, Deleting, ... whose False is the healthy state).
MACHINE_POSITIVE_CONDITIONS = frozenset({
    "Available", "Ready", "UpToDate", "BootstrapConfigReady", "InfrastructureReady",
    "NodeReady", "NodeHealthy", "HealthCheckSucceeded",
})


def kubectl(*args: str, json_out: bool = True):
    """(result, None) or (None, an error line). Never raises: a traceback would
    print the arguments, tenant namespace and pod names among them."""
    try:
        proc = subprocess.run(
            ["kubectl", *args, "--request-timeout=30s"] + (["-o", "json"] if json_out else []),
            capture_output=True, text=True, timeout=90, check=False,
        )
    except (OSError, subprocess.SubprocessError) as exc:
        return None, f"kubectl {' '.join(args[:3])} failed: {type(exc).__name__}"
    if proc.returncode != 0:
        first = (proc.stderr.strip().splitlines() or ["no output"])[0]
        return None, f"kubectl {' '.join(args[:3])} failed: {first}"
    if not json_out:
        return proc.stdout, None
    try:
        return json.loads(proc.stdout or "{}"), None
    except json.JSONDecodeError as exc:
        return None, f"kubectl {' '.join(args[:3])}: unreadable JSON ({exc})"


def parse_time(value: str | None) -> datetime | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def age(value: str | None, now: datetime) -> str:
    t = parse_time(value)
    if t is None:
        return "-"
    s = max(0, int((now - t).total_seconds()))
    for unit, size in (("d", 86400), ("h", 3600), ("m", 60)):
        if s >= size:
            return f"{s // size}{unit}"
    return f"{s}s"


class Report:
    def __init__(self, redactor: Redactor, now: datetime):
        self.r = redactor
        self.now = now

    def emit(self, line: str = "") -> None:
        print(self.r.redact(line), flush=True)

    def section(self, title: str) -> None:
        self.emit()
        self.emit(f"== {title}")

    def namespace(self, ns: str) -> str:
        if ns in COMPONENT_NAMESPACES or ns in WORKLOAD_NAMESPACES:
            return ns
        red = self.r.redact(ns)
        return red if TAGGED_RE.fullmatch(red) else self.r.tag("ns", ns)

    def name(self, ns: str, name: str, kind: str = "pod") -> str:
        """ns/name with tenant parts hashed; fail closed on an unknown shape."""
        if ns in COMPONENT_NAMESPACES:
            return f"{ns}/{name}"
        red = self.r.redact(name)
        if not TAGGED_RE.fullmatch(red):
            red = self.r.tag(kind, f"{ns}/{name}")
        return f"{self.namespace(ns)}/{red}"

    def node(self, name: str | None) -> str:
        return self.r.tag("node", name) if name else "-"

    def pod_refs(self, line: str) -> str:
        """Hash the `namespace/name` pod references a log line makes (the
        autoscaler names every pod it considers): a tenant or workload
        namespace, or any pair right after "pod". Other pairs (0/3 nodes,
        cpu/memory) stay; emit() still redacts them."""
        def ref(m: re.Match) -> str:
            before, ns, name = m.groups()
            if ns in COMPONENT_NAMESPACES:
                return m.group(0)
            if ns in WORKLOAD_NAMESPACES or TAGGED_RE.fullmatch(self.r.redact(ns)) or re.search(r"pod\W*$", before, re.I):
                return before + self.name(ns, name)
            return m.group(0)
        return POD_REF_RE.sub(ref, line)

    def tenant(self, ns: str) -> bool:
        return ns not in COMPONENT_NAMESPACES


def container_lines(pod: dict) -> list[str]:
    out = []
    status = pod.get("status") or {}
    for group, key in (("init", "initContainerStatuses"), ("container", "containerStatuses")):
        for i, cs in enumerate(status.get(key) or []):
            state = cs.get("state") or {}
            if "waiting" in state:
                now = f"waiting reason={state['waiting'].get('reason', '-')}"
            elif "terminated" in state:
                t = state["terminated"]
                now = f"terminated reason={t.get('reason', '-')} exitCode={t.get('exitCode', '-')}"
            elif "running" in state:
                now = "running"
            else:
                now = "unknown"
            line = f"  {group}[{i}] {now} ready={cs.get('ready', False)} restarts={cs.get('restartCount', 0)}"
            last = (cs.get("lastState") or {}).get("terminated")
            if last:
                line += f"; last terminated reason={last.get('reason', '-')} exitCode={last.get('exitCode', '-')}"
            out.append(line)
    return out


def pod_ready(pod: dict) -> str:
    statuses = (pod.get("status") or {}).get("containerStatuses") or []
    return f"{sum(1 for c in statuses if c.get('ready'))}/{len(statuses) or len((pod.get('spec') or {}).get('containers') or [])}"


def scheduled_condition(pod: dict) -> dict | None:
    for cond in (pod.get("status") or {}).get("conditions") or []:
        if cond.get("type") == "PodScheduled" and cond.get("status") != "True":
            return cond
    return None


def service_pods(rep: Report, service_id: str) -> None:
    rep.section(f"Service pods ({service_id})")
    selector = f"bex.co/app-id={service_id}"
    pods, err = kubectl("get", "pods", "-A", "-l", selector)
    if err:
        rep.emit(err)
        return
    items = pods.get("items") or []
    phases: dict[str, int] = {}
    for pod in items:
        phase = (pod.get("status") or {}).get("phase", "Unknown")
        phases[phase] = phases.get(phase, 0) + 1
    rep.emit(f"pods: {len(items)}" + (" (" + ", ".join(f"{k} {v}" for k, v in sorted(phases.items())) + ")" if items else ""))
    for pod in items:
        meta, spec, status = pod.get("metadata") or {}, pod.get("spec") or {}, pod.get("status") or {}
        ns, name = meta.get("namespace", ""), meta.get("name", "")
        node = spec.get("nodeName")
        rep.emit(
            f"{rep.name(ns, name)}  phase={status.get('phase', 'Unknown')}  ready={pod_ready(pod)}"
            f"  node={rep.node(node)}  age={age(meta.get('creationTimestamp'), rep.now)}"
            + (f"  reason={status['reason']}" if status.get("reason") else "")
        )
        for cond in status.get("conditions") or []:
            if cond.get("status") != "True":
                rep.emit(f"  condition {cond.get('type')}={cond.get('status')} reason={cond.get('reason', '-')}")
        for line in container_lines(pod):
            rep.emit(line)
        events, err = kubectl("get", "events", "-n", ns, "--field-selector", f"involvedObject.kind=Pod,involvedObject.name={name}")
        if err:
            rep.emit(f"  {err}")
            continue
        evs = sorted(events.get("items") or [], key=lambda e: e.get("lastTimestamp") or e.get("eventTime") or "")
        for ev in evs[-15:]:
            reason = ev.get("reason", "-")
            line = f"  event {ev.get('type', '-')} {reason} x{ev.get('count') or 1} ({age(ev.get('lastTimestamp') or ev.get('eventTime'), rep.now)} ago)"
            if reason in MESSAGE_REASONS and ev.get("message"):
                line += ": " + " ".join(ev["message"].split())
            rep.emit(line)
    workloads, err = kubectl("get", "deploy,rs", "-A", "-l", selector)
    if err:
        rep.emit(err)
        return
    for w in workloads.get("items") or []:
        meta, spec, status = w.get("metadata") or {}, w.get("spec") or {}, w.get("status") or {}
        kind = w.get("kind", "?")
        if kind == "ReplicaSet" and not spec.get("replicas") and not status.get("replicas"):
            continue  # an old revision scaled to zero
        line = (
            f"{kind} {rep.name(meta.get('namespace', ''), meta.get('name', ''), kind.lower())}"
            f"  desired={spec.get('replicas', 0)} ready={status.get('readyReplicas', 0)}"
            f" available={status.get('availableReplicas', 0)}"
        )
        if kind == "Deployment":
            line += f" updated={status.get('updatedReplicas', 0)}"
            for cond in status.get("conditions") or []:
                line += f" {cond.get('type')}={cond.get('status')}({cond.get('reason', '-')})"
        else:
            rev = (meta.get("annotations") or {}).get("deployment.kubernetes.io/revision")
            if rev:
                line += f" revision={rev}"
        rep.emit(line)


def pending_pods(rep: Report) -> None:
    rep.section("Pending pods cluster-wide")
    pods, err = kubectl("get", "pods", "-A", "--field-selector=status.phase=Pending")
    if err:
        rep.emit(err)
        return
    items = pods.get("items") or []
    tenant = sum(1 for p in items if rep.tenant((p.get("metadata") or {}).get("namespace", "")))
    reasons: dict[str, int] = {}
    for p in items:
        cond = scheduled_condition(p)
        key = cond.get("reason", "-") if cond else "Scheduled"
        reasons[key] = reasons.get(key, 0) + 1
    rep.emit(
        f"pending: {len(items)} (tenant {tenant}, platform {len(items) - tenant})"
        + (" by scheduling: " + ", ".join(f"{k} {v}" for k, v in sorted(reasons.items())) if items else "")
    )
    for p in items[:PENDING_LIMIT]:
        meta, spec = p.get("metadata") or {}, p.get("spec") or {}
        node = spec.get("nodeName")
        line = (
            f"{rep.name(meta.get('namespace', ''), meta.get('name', ''))}  age={age(meta.get('creationTimestamp'), rep.now)}"
            f"  node={rep.node(node)}"
        )
        cond = scheduled_condition(p)
        if cond:
            line += f"  {cond.get('reason', '-')}: " + " ".join((cond.get("message") or "").split())
        waiting = [
            (cs.get("state") or {}).get("waiting", {}).get("reason")
            for cs in ((p.get("status") or {}).get("containerStatuses") or [])
        ]
        waiting = [w for w in waiting if w]
        if waiting:
            line += "  waiting=" + ",".join(sorted(set(waiting)))
        rep.emit(line)
    if len(items) > PENDING_LIMIT:
        rep.emit(f"... and {len(items) - PENDING_LIMIT} more")


def allocation(describe: str) -> dict[str, list[str]]:
    """Node name -> its "Allocated resources" rows, from `kubectl describe nodes`."""
    out: dict[str, list[str]] = {}
    node, inside = None, False
    for line in describe.splitlines():
        if line.startswith("Name:"):
            parts = line.split()
            node, inside = (parts[1] if len(parts) > 1 else None), False
        elif line.startswith("Allocated resources:"):
            inside = True
        elif inside and line.startswith("Events:"):
            inside = False
        elif inside and node:
            parts = line.split()
            if parts and parts[0] in ("cpu", "memory", "pods", "ephemeral-storage"):
                # "cpu  991m (24%)  1450m (36%)" -> requests and limits
                rest = line.split(None, 1)[1] if len(parts) > 1 else ""
                cols = re.findall(r"\S+ \(\d+%\)|\S+", rest)
                if len(cols) >= 2:
                    out.setdefault(node, []).append(f"{parts[0]} req {cols[0]} lim {cols[1]}")
    return out


def nodes(rep: Report, node_list: dict | None, node_err: str | None) -> None:
    rep.section("Nodes and allocation")
    if node_err:
        rep.emit(node_err)
        return
    describe, err = kubectl("describe", "nodes", json_out=False)
    alloc = allocation(describe or "")
    if err:
        rep.emit(err)
    items = node_list.get("items") or []
    ready = 0
    for n in items:
        meta, spec, status = n.get("metadata") or {}, n.get("spec") or {}, n.get("status") or {}
        labels = meta.get("labels") or {}
        pool = labels.get("bex.co/pool") or labels.get("nodepool") or (
            "control-plane" if "node-role.kubernetes.io/control-plane" in labels else "-"
        )
        conds = {c.get("type"): c for c in status.get("conditions") or []}
        is_ready = (conds.get("Ready") or {}).get("status", "Unknown")
        ready += is_ready == "True"
        line = (
            f"{rep.node(meta.get('name'))}  pool={pool}  Ready={is_ready}"
            f"  schedulable={'no' if spec.get('unschedulable') else 'yes'}"
            f"  kubelet={(status.get('nodeInfo') or {}).get('kubeletVersion', '-')}  age={age(meta.get('creationTimestamp'), rep.now)}"
        )
        pressure = [t for t, c in conds.items() if t != "Ready" and c.get("status") == "True"]
        if pressure:
            line += "  pressure=" + ",".join(sorted(pressure))
        taints = [f"{t.get('key')}{'=' + t['value'] if t.get('value') else ''}:{t.get('effect')}" for t in spec.get("taints") or []]
        if taints:
            line += "  taints=" + ",".join(taints)
        alloc_rows = alloc.get(meta.get("name", ""), [])
        if alloc_rows:
            line += "  " + "; ".join(alloc_rows)
        rep.emit(line)
    rep.emit(f"nodes: {len(items)} (Ready {ready})")


def machines(rep: Report, data: dict | None, err: str | None) -> None:
    rep.section("MachineDeployments and Machines (bex-capi)")
    if err:
        rep.emit(err)
        return
    for item in data.get("items") or []:
        meta, spec, status = item.get("metadata") or {}, item.get("spec") or {}, item.get("status") or {}
        if item.get("kind") == "MachineDeployment":
            counts = " ".join(
                f"{k}={status[k]}" for k in ("replicas", "readyReplicas", "availableReplicas", "upToDateReplicas", "updatedReplicas", "unavailableReplicas")
                if k in status
            )
            rep.emit(f"MachineDeployment {meta.get('name')}  desired={spec.get('replicas', '-')} {counts} phase={status.get('phase', '-')}")
        else:
            node = (status.get("nodeRef") or {}).get("name")
            labels = meta.get("labels") or {}
            pool = labels.get("cluster.x-k8s.io/deployment-name") or (
                "control-plane" if "cluster.x-k8s.io/control-plane" in labels else "-"
            )
            line = (
                f"Machine {rep.node(meta.get('name'))}  pool={pool}"
                f"  phase={status.get('phase', '-')}  node={rep.node(node)}"
                f"  version={spec.get('version', '-')}  age={age(meta.get('creationTimestamp'), rep.now)}"
            )
            bad = [
                f"{c.get('type')}({c.get('reason', '-')})" for c in status.get("conditions") or []
                if c.get("status") == "False" and c.get("type") in MACHINE_POSITIVE_CONDITIONS
            ]
            if bad:
                line += "  not=" + ",".join(bad)
            if status.get("failureReason"):
                line += f"  failure={status['failureReason']}"
            rep.emit(line)


def autoscaler(rep: Report) -> None:
    rep.section("Cluster autoscaler")
    cms, err = kubectl("get", "configmap", "-A", "--field-selector=metadata.name=cluster-autoscaler-status")
    if err:
        rep.emit(err)
    for cm in (cms or {}).get("items") or []:
        if (cm.get("metadata") or {}).get("namespace") not in COMPONENT_NAMESPACES:
            continue
        for line in ((cm.get("data") or {}).get("status") or "").splitlines()[:80]:
            rep.emit(line)
    pods, err = kubectl("get", "pods", "-A", "-l", AUTOSCALER_SELECTOR)
    if err:
        rep.emit(err)
        return
    for pod in (pods or {}).get("items") or []:
        meta = pod.get("metadata") or {}
        ns, name = meta.get("namespace", ""), meta.get("name", "")
        if ns not in COMPONENT_NAMESPACES:
            continue
        rep.emit(f"===== {ns}/{name}")
        logs, err = kubectl("logs", "-n", ns, name, "--tail=400", json_out=False)
        if err:
            rep.emit(err)
            continue
        for line in [l for l in (logs or "").splitlines() if AUTOSCALER_LOG_RE.search(l)][-60:]:
            rep.emit(rep.pod_refs(line))


def main(argv: list[str]) -> int:
    if len(argv) != 1 or not SERVICE_ID_RE.fullmatch(argv[0]):
        print("usage: cluster_diagnose_public.py srv-<20 chars>", file=sys.stderr)
        return 2
    # During an outage every read below would wait out its timeout; say so once.
    _, err = kubectl("get", "--raw=/readyz", json_out=False)
    if err:
        print(f"the API server did not answer: {err}")
        return 1
    key, _ = cluster_key()
    node_list, node_err = kubectl("get", "nodes")
    capi, capi_err = kubectl("get", "machinedeployments.cluster.x-k8s.io,machines.cluster.x-k8s.io", "-n", "bex-capi")
    named = list((node_list or {}).get("items") or [])
    named += [i for i in (capi or {}).get("items") or [] if i.get("kind") == "Machine"]
    names = [(i.get("metadata") or {}).get("name", "") for i in named]
    rep = Report(Redactor(key, names), datetime.now(timezone.utc))
    rep.emit("Cluster diagnose (public log): tenant names, node names and addresses are keyed hashes.")
    rep.emit("Full detail: bash scripts/fetch-app-kubeconfig.sh <path>, then bash scripts/cluster-diagnose.sh <srv-id>.")
    service_pods(rep, argv[0])
    pending_pods(rep)
    nodes(rep, node_list, node_err)
    machines(rep, capi, capi_err)
    autoscaler(rep)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
