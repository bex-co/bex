#!/usr/bin/env python3
"""Replay the fast-exit cron log probe on a throwaway kind cluster (w4/m174).

The Docker harness (scripts/test_log_shipper.py) runs each production
loki.process block against fixed input; it cannot see what pod churn does to
loki.source.file. This replays the real thing: the GitOps-rendered Loki and
log-shipper (Alloy) charts on a single-node kind cluster, then N Jobs labelled
like a tenant App running a probe command (--probe: the m174 DoD's fast exit,
or a321's delayed prefix), then every Job pod's CRI file compared line by line
with what Loki holds for that pod.

On 2026-10-18 it found Alloy v1.11.2 losing 4 of 800 lines and v1.11.3 none
(see deploy/log-shipper/Dockerfile). Run it before pinning a new bex-alloy
build or a chart bump; BEX_ALLOY_IMAGE replaces the pinned image, and a
locally built one is loaded into the cluster:

    BEX_ALLOY_IMAGE=bex-alloy:candidate scripts/log-shipper-kind-replay.py --jobs 160

The cluster has its own kubeconfig (never the caller's) and is deleted at the
end unless --keep. Exit status is non-zero when any line is missing.

Requires: kind, kubectl, helm, yq v4, docker.
"""
import argparse
import collections
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
import urllib.parse

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from test_log_shipper import render_chart, run  # noqa: E402

NAMESPACE = "tea-m174-replay"
NODE_IMAGE = "kindest/node:v1.36.1"  # as scripts/verify-egress-meter.sh
# Each probe is a command per run and the lines it writes:
#   fast — the m174 DoD (stdout, stdout, stderr, JSON stdout, stdout), exits at once;
#   delayed — a321's prefix, a 90-second wait, then a suffix.
PROBES = {
    "fast": ('echo ALPHA-{tag} one; echo BETA-{tag} two; echo "error: GAMMA-{tag} three" 1>&2; '
             'echo "{{\\"level\\":\\"warn\\",\\"msg\\":\\"DELTA-{tag}\\"}}"; echo 100%-{tag} under_score-{tag}', 5),
    "delayed": ('printf "begin-{tag}\\n"; sleep 90; printf "end-{tag}\\n"', 2),
}
# Loki's default max_entries_limit_per_query; one query holds every run.
MAX_LOKI_ENTRIES = 5000


class Cluster:
    def __init__(self, name, kubeconfig):
        self.name, self.kubeconfig, self.node = name, kubeconfig, f"{name}-control-plane"

    def kubectl(self, *args, **kwargs):
        return run("kubectl", "--kubeconfig", self.kubeconfig, *args, **kwargs)


def bring_up(cluster, tmp, image):
    cluster.kubectl("label", "node", cluster.node, "bex.co/pool=platform")  # Loki's pool
    cluster.kubectl("create", "namespace", "monitoring")
    for rendered in (render_chart(tmp, "loki", "loki.yaml", "loki", "--set", "singleBinary.persistence.size=2Gi"),
                     render_chart(tmp, "alloy", "log-shipper.yaml", "log-shipper")):
        cluster.kubectl("apply", "-n", "monitoring", "-f", str(rendered))
    if image:
        # A locally built candidate has no registry to pull from.
        if subprocess.run(["docker", "image", "inspect", image], capture_output=True).returncode == 0:
            run("kind", "load", "docker-image", image, "--name", cluster.name)
        cluster.kubectl("-n", "monitoring", "set", "image", "ds/log-shipper", f"alloy={image}")
    cluster.kubectl("-n", "monitoring", "rollout", "status", "ds/log-shipper", "--timeout=300s")
    cluster.kubectl("-n", "monitoring", "rollout", "status", "statefulset/loki", "--timeout=300s")
    print(cluster.kubectl("-n", "monitoring", "exec", "ds/log-shipper", "-c", "alloy", "--",
                          "/bin/alloy", "--version").splitlines()[0])
    run("docker", "exec", cluster.node, "crictl", "pull", "docker.io/library/alpine:3")


def run_jobs(cluster, command, jobs, spacing):
    cluster.kubectl("create", "namespace", NAMESPACE)
    for i in range(1, jobs + 1):
        tag = f"r{i:04d}"
        job = {"apiVersion": "batch/v1", "kind": "Job",
               "metadata": {"name": f"run-{tag}", "namespace": NAMESPACE, "labels": {"app.bex.co/app": "replay-cron"}},
               "spec": {"backoffLimit": 0, "template": {
                   "metadata": {"labels": {"app.bex.co/app": "replay-cron"}},
                   "spec": {"restartPolicy": "Never", "containers": [{
                       "name": "app", "image": "alpine:3", "imagePullPolicy": "IfNotPresent",
                       "command": ["sh", "-c", command.format(tag=tag)]}]}}}}
        cluster.kubectl("apply", "-f", "-", input=json.dumps(job))
        if i < jobs:
            time.sleep(spacing)
    cluster.kubectl("-n", NAMESPACE, "wait", "--for=condition=complete", "job", "--all", "--timeout=600s")


def loki_lines(cluster, since):
    """Every line Loki holds for the namespace, by pod, read through the API
    server's service proxy (no port-forward to race)."""
    query = urllib.parse.urlencode({"query": f'{{namespace="{NAMESPACE}"}}', "limit": MAX_LOKI_ENTRIES,
                                    "direction": "forward", "start": int(since * 1e9), "end": int(time.time() * 1e9)})
    data = json.loads(cluster.kubectl(
        "get", "--raw", f"/api/v1/namespaces/monitoring/services/loki:3100/proxy/loki/api/v1/query_range?{query}"))
    by_pod = collections.defaultdict(list)
    for stream in data["data"]["result"]:
        by_pod[stream["stream"].get("pod")].extend(line for _, line in stream["values"])
    return by_pod


def compare(cluster, lines_per_run, jobs, since, settle=120):
    pods = json.loads(cluster.kubectl("-n", NAMESPACE, "get", "pods", "-o", "json"))["items"]
    written = {}
    for pod in pods:
        name, uid = pod["metadata"]["name"], pod["metadata"]["uid"]
        raw = run("docker", "exec", cluster.node, "cat", f"/var/log/pods/{NAMESPACE}_{name}_{uid}/app/0.log")
        written[name] = [line.split(" ", 3)[3] for line in raw.splitlines()]
    # A file is picked up on the shipper's next discovery pass and pushed in
    # one-second batches: wait for every line, then judge what is still missing.
    deadline = time.time() + settle
    while True:
        stored = loki_lines(cluster, since)
        missing = {name: [line for line in lines if line not in stored.get(name, [])] for name, lines in written.items()}
        if not any(missing.values()) or time.time() > deadline:
            break
        time.sleep(5)
    total = sum(len(lines) for lines in written.values())
    lost = sum(len(lines) for lines in missing.values())
    for name in sorted(written):
        if missing[name] or len(written[name]) != lines_per_run:
            print(f"{name}: wrote {len(written[name])}, Loki holds {len(stored.get(name, []))}, missing {missing[name]}")
    print(f"{len(pods)} runs, {total} lines written, {lost} missing from Loki")
    return lost == 0 and len(pods) == jobs and total == lines_per_run * jobs


def main():
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--probe", choices=PROBES, default="fast")
    parser.add_argument("--jobs", type=int, default=160)
    parser.add_argument("--spacing", type=float, default=5, help="seconds between Job creations")
    parser.add_argument("--name", default="bex-log-replay")
    parser.add_argument("--keep", action="store_true", help="leave the cluster up afterwards")
    args = parser.parse_args()
    command, lines_per_run = PROBES[args.probe]
    if args.jobs * lines_per_run > MAX_LOKI_ENTRIES:
        parser.error(f"--jobs is at most {MAX_LOKI_ENTRIES // lines_per_run}: one Loki query holds every line")
    if args.name in run("kind", "get", "clusters").splitlines():
        parser.error(f"kind cluster {args.name} already exists; pick another --name or delete it")
    with tempfile.TemporaryDirectory() as tmp:
        tmp = pathlib.Path(tmp)
        kubeconfig = str(pathlib.Path.cwd() / f".{args.name}.kubeconfig") if args.keep else str(tmp / "kubeconfig")
        cluster = Cluster(args.name, kubeconfig)
        run("kind", "create", "cluster", "--name", args.name, "--image", NODE_IMAGE, "--kubeconfig", kubeconfig, "--wait", "180s")
        try:
            bring_up(cluster, tmp, os.environ.get("BEX_ALLOY_IMAGE"))
            since = time.time() - 60
            run_jobs(cluster, command, args.jobs, args.spacing)
            ok = compare(cluster, lines_per_run, args.jobs, since)
        finally:
            if args.keep:
                print(f"kept: kubectl --kubeconfig {kubeconfig} …; remove with "
                      f"kind delete cluster --name {args.name} && rm {kubeconfig}")
            else:
                subprocess.run(["kind", "delete", "cluster", "--name", args.name, "--kubeconfig", kubeconfig],
                               capture_output=True)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
