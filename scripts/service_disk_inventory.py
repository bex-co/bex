#!/usr/bin/env python3
"""Project only service-disk binding metadata; never read Secrets or tenant files."""

import argparse
import csv
import json
from pathlib import Path
import re


FIELDS = {
    "apps": (
        ("namespace", "{.metadata.namespace}"),
        ("name", "{.metadata.name}"),
        ("uid", "{.metadata.uid}"),
        ("serviceId", r"{.metadata.labels.bex\.co/app-id}"),
        ("requestedSizeGB", "{.spec.disk.sizeGB}"),
        ("allocatedSizeGB", "{.status.disk.allocatedSizeGB}"),
        ("observedSizeGB", "{.status.disk.observedSizeGB}"),
        ("capacityGB", "{.status.disk.capacityGB}"),
    ),
    "claims": (
        ("namespace", "{.metadata.namespace}"),
        ("name", "{.metadata.name}"),
        ("uid", "{.metadata.uid}"),
        ("controller", '{range .metadata.ownerReferences[?(@.controller==true)]}{.kind}{"/"}{.name}{"/"}{.uid}{end}'),
        ("storageClass", "{.spec.storageClassName}"),
        ("requestedStorage", "{.spec.resources.requests.storage}"),
        ("capacityStorage", "{.status.capacity.storage}"),
        ("phase", "{.status.phase}"),
        ("volumeName", "{.spec.volumeName}"),
        ("deletingAt", "{.metadata.deletionTimestamp}"),
    ),
    "volumes": (
        ("name", "{.metadata.name}"),
        ("storageClass", "{.spec.storageClassName}"),
        ("capacityStorage", "{.spec.capacity.storage}"),
        ("phase", "{.status.phase}"),
        ("reclaimPolicy", "{.spec.persistentVolumeReclaimPolicy}"),
        ("claimNamespace", "{.spec.claimRef.namespace}"),
        ("claimName", "{.spec.claimRef.name}"),
        ("claimUid", "{.spec.claimRef.uid}"),
        ("csiDriver", "{.spec.csi.driver}"),
        ("volumeHandle", "{.spec.csi.volumeHandle}"),
    ),
    "deployments": (
        ("namespace", "{.metadata.namespace}"),
        ("name", "{.metadata.name}"),
        ("uid", "{.metadata.uid}"),
        ("generation", "{.metadata.generation}"),
        ("observedGeneration", "{.status.observedGeneration}"),
        ("replicas", "{.spec.replicas}"),
        ("updatedReplicas", "{.status.updatedReplicas}"),
        ("readyReplicas", "{.status.readyReplicas}"),
        ("availableReplicas", "{.status.availableReplicas}"),
        ("containerImages", '{range .spec.template.spec.containers[*]}{.name}{"="}{.image}{";"}{end}'),
    ),
    "pods": (
        ("namespace", "{.metadata.namespace}"),
        ("name", "{.metadata.name}"),
        ("uid", "{.metadata.uid}"),
        ("phase", "{.status.phase}"),
        ("deletingAt", "{.metadata.deletionTimestamp}"),
        ("containerImages", '{range .spec.containers[*]}{.name}{"="}{.image}{";"}{end}'),
        ("containerImageIds", '{range .status.containerStatuses[*]}{.name}{"="}{.imageID}{";"}{end}'),
        ("containerReady", '{range .status.containerStatuses[*]}{.name}{"="}{.ready}{";"}{end}'),
    ),
}


def projection(kind):
    fields = '{"\\t"}'.join(expression for _, expression in FIELDS[kind]) + '{"\\n"}'
    if kind == "apps":
        return "{range .items[?(@.spec.disk)]}" + fields + "{end}"
    if kind in ("claims", "deployments", "pods"):
        return "{range .items[*]}" + fields + "{end}"
    return fields


def read_rows(kind, path):
    names = [name for name, _ in FIELDS[kind]]
    result = []
    with Path(path).open(newline="") as source:
        for row in csv.reader(source, delimiter="\t", quoting=csv.QUOTE_NONE):
            if not row or not any(row):
                continue
            if len(row) != len(names):
                raise ValueError(f"unexpected {kind} metadata projection width")
            result.append(dict(zip(names, row)))
    return result


def app_claims(rows):
    result = []
    for row in rows:
        owner = row["controller"].split("/")
        if len(owner) != 3 or owner[0] != "App":
            continue
        claim = {key: value for key, value in row.items() if key != "controller"}
        claim["controller"] = {"kind": owner[0], "name": owner[1], "uid": owner[2]}
        result.append(claim)
    return result


def volume_names(claims):
    names = sorted({claim["volumeName"] for claim in app_claims(claims) if claim["volumeName"]})
    for name in names:
        if len(name) > 253 or not re.fullmatch(r"[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?", name):
            raise ValueError("invalid referenced persistent volume name")
    return names


def split_containers(value):
    return dict(item.split("=", 1) for item in value.split(";") if item)


def platform_images(deployments, pods):
    result = {"deployments": [], "pods": []}
    for row in deployments:
        deployment = {key: row[key] for key in ("namespace", "name", "uid")}
        for key in ("generation", "observedGeneration", "replicas", "updatedReplicas", "readyReplicas", "availableReplicas"):
            deployment[key] = int(row[key]) if row[key] else None
        deployment["containerImages"] = split_containers(row["containerImages"])
        result["deployments"].append(deployment)
    for row in pods:
        pod = {key: row[key] for key in ("namespace", "name", "uid", "phase", "deletingAt")}
        for key in ("containerImages", "containerImageIds"):
            pod[key] = split_containers(row[key])
        pod["containerReady"] = {name: value == "true" for name, value in split_containers(row["containerReady"]).items()}
        result["pods"].append(pod)
    return result


def inventory(apps, claim_rows, volumes, deployments=(), pods=()):
    claims = app_claims(claim_rows)
    by_volume = {volume["name"]: volume for volume in volumes}
    for claim in claims:
        volume = by_volume.get(claim["volumeName"])
        claim["volume"] = volume
        claim["bindingMatches"] = None if volume is None else (
            volume["claimNamespace"], volume["claimName"], volume["claimUid"]
        ) == (claim["namespace"], claim["name"], claim["uid"])
    disk_apps = []
    matched = set()
    for app in sorted(apps, key=lambda row: (row["namespace"], row["name"])):
        disk = {key: (int(app[key]) if app[key] else None) for key in (
            "requestedSizeGB", "allocatedSizeGB", "observedSizeGB", "capacityGB"
        )}
        owned = []
        for index, claim in enumerate(claims):
            if claim["namespace"] == app["namespace"] and claim["controller"]["uid"] == app["uid"] and claim["controller"]["name"] == app["name"]:
                owned.append(claim)
                matched.add(index)
        disk_apps.append({
            "namespace": app["namespace"], "name": app["name"], "uid": app["uid"],
            "serviceId": app["serviceId"], "disk": disk, "claims": owned,
        })
    return {
        "appsWithDisks": disk_apps,
        "claimsWhoseOwnerIsNotInDiskInventory": [claim for index, claim in enumerate(claims) if index not in matched],
        "platformImages": platform_images(deployments, pods),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    project = commands.add_parser("projection")
    project.add_argument("kind", choices=FIELDS)
    names = commands.add_parser("volume-names")
    names.add_argument("claims")
    report = commands.add_parser("report")
    for kind in FIELDS:
        report.add_argument(kind)
    args = parser.parse_args()
    try:
        if args.command == "projection":
            print(projection(args.kind))
        elif args.command == "volume-names":
            for name in volume_names(read_rows("claims", args.claims)):
                print(name)
        else:
            print(json.dumps(inventory(*(read_rows(kind, getattr(args, kind)) for kind in FIELDS)), indent=2))
    except (ValueError, KeyError, TypeError, OSError) as error:
        parser.exit(1, f"service-disk-inventory: {error}\n")


if __name__ == "__main__":
    main()
