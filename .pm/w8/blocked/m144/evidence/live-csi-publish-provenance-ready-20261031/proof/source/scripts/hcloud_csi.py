#!/usr/bin/env python3
"""Validate reviewed CSI image pins and the completed, scoped driver rollout."""

import argparse
import json
import re
from pathlib import Path


REPOSITORIES = {
    "official": "docker.io/hetznercloud/hcloud-csi-driver",
    "reviewed": "ghcr.io/bex-co/bex-operator",
}


def read_json(path):
    with Path(path).open() as source:
        return json.load(source)


def validate_pin(values, kind):
    try:
        image = values["controller"]["image"]["hcloudCSIDriver"]["name"]
    except (KeyError, TypeError) as error:
        raise ValueError("CSI values must contain controller/node driver image pins") from error
    if not isinstance(image, str) or not re.fullmatch(re.escape(REPOSITORIES[kind]) + r"@sha256:[0-9a-f]{64}", image):
        raise ValueError(f"{kind} CSI image must use its reviewed repository and a sha256 digest")
    expected = {
        role: {"image": {"hcloudCSIDriver": {"name": image, "tag": ""}}}
        for role in ("controller", "node")
    }
    if values != expected:
        raise ValueError("CSI values must pin only the same controller/node image with empty tags")
    return image


def driver_workloads(document):
    if not isinstance(document, dict) or not isinstance(document.get("items"), list):
        raise ValueError("expected a Kubernetes workload List")
    by_role = {}
    for workload in document.get("items", []):
        role = workload.get("metadata", {}).get("labels", {}).get("app.kubernetes.io/component")
        if role not in ("controller", "node"):
            continue
        expected_kind = "Deployment" if role == "controller" else "DaemonSet"
        if workload.get("kind") != expected_kind or role in by_role:
            raise ValueError(f"expected exactly one CSI {expected_kind}")
        if workload["metadata"]["labels"].get("app.kubernetes.io/instance") != "hcloud-csi":
            raise ValueError("CSI workload belongs to another Helm release")
        by_role[role] = workload
    if set(by_role) != {"controller", "node"}:
        raise ValueError("CSI controller Deployment and node DaemonSet must both exist")
    return by_role


def driver_image(spec):
    drivers = [container for container in spec.get("containers", []) if container["name"] == "hcloud-csi-driver"]
    if len(drivers) != 1:
        raise ValueError("expected exactly one hcloud-csi-driver container")
    return drivers[0]["image"]


def verify_ready(workload_document, pod_document, image):
    workloads = driver_workloads(workload_document)
    desired = {}
    for role, workload in workloads.items():
        if driver_image(workload["spec"]["template"]["spec"]) != image:
            raise ValueError(f"CSI {role} template does not select the reviewed image")
        status = workload.get("status", {})
        if status.get("observedGeneration", 0) < workload["metadata"]["generation"]:
            raise ValueError(f"CSI {role} generation has not been observed")
        if role == "controller":
            desired[role] = workload["spec"].get("replicas", 1)
            ready_fields = ("updatedReplicas", "readyReplicas", "availableReplicas")
        else:
            desired[role] = status.get("desiredNumberScheduled", 0)
            ready_fields = ("updatedNumberScheduled", "numberReady")
            if status.get("numberUnavailable", 0) or status.get("numberMisscheduled", 0):
                raise ValueError("CSI node DaemonSet has unavailable or misscheduled pods")
        if desired[role] < 1 or any(status.get(field, 0) != desired[role] for field in ready_fields):
            raise ValueError(f"CSI {role} has not completed its rollout")

    active = {"controller": 0, "node": 0}
    for pod in pod_document.get("items", []):
        metadata = pod.get("metadata", {})
        labels = metadata.get("labels", {})
        role = labels.get("app.kubernetes.io/component")
        if role not in active:
            continue
        if metadata.get("deletionTimestamp"):
            raise ValueError(f"CSI pod {metadata.get('name')} is still terminating")
        if labels.get("app.kubernetes.io/instance") != "hcloud-csi":
            raise ValueError("CSI pod belongs to another Helm release")
        if driver_image(pod["spec"]) != image:
            raise ValueError(f"CSI pod {metadata.get('name')} is still using another driver image")
        status = pod.get("status", {})
        ready = any(condition.get("type") == "Ready" and condition.get("status") == "True" for condition in status.get("conditions", []))
        driver_ready = any(container.get("name") == "hcloud-csi-driver" and container.get("ready") for container in status.get("containerStatuses", []))
        if status.get("phase") != "Running" or not ready or not driver_ready:
            raise ValueError(f"CSI pod {metadata.get('name')} is not Ready")
        active[role] += 1
    if active != desired:
        raise ValueError(f"CSI active pod counts {active} do not match desired counts {desired}")
    return {"image": image, "readyControllerPods": active["controller"], "readyNodePods": active["node"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    pin = commands.add_parser("pin")
    pin.add_argument("kind", choices=REPOSITORIES)
    pin.add_argument("values")
    resources = commands.add_parser("resources")
    resources.add_argument("workloads")
    ready = commands.add_parser("ready")
    ready.add_argument("image")
    ready.add_argument("workloads")
    ready.add_argument("pods")
    args = parser.parse_args()
    try:
        if args.command == "pin":
            print(validate_pin(read_json(args.values), args.kind))
        elif args.command == "resources":
            for workload in driver_workloads(read_json(args.workloads)).values():
                print(f"{workload['kind'].lower()}/{workload['metadata']['name']}")
        else:
            print(json.dumps(verify_ready(read_json(args.workloads), read_json(args.pods), args.image)))
    except (ValueError, KeyError, TypeError, OSError) as error:
        parser.exit(1, f"hcloud-csi: {error}\n")


if __name__ == "__main__":
    main()
