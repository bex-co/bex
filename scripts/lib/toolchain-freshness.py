#!/usr/bin/env python3
"""Build-toolchain freshness inventory: validate, resolve, issue decisions.

Does not edit, commit, or merge digest pins. A changed upstream digest is a
review request. Public registry metadata only; no cluster or push credentials.
"""
from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

SCHEMA = "bex.build-toolchain-freshness/v1"
CLUSTER_BUILDER_ID = "cnb-builder"
KINDS = frozenset({"builder", "stack", "native-base", "helper"})
DIGEST_RE = re.compile(r"sha256:[a-f0-9]{64}")
ID_RE = re.compile(r"^[a-z][a-z0-9-]{0,62}$")

CANONICAL_PIN_SITES = (
    "lego/operator/internal/build/native.go",
    "lego/operator/internal/build/build.go",
    "lego/operator/cmd/manager/main.go",
    "lego/operator/config/manager/manager.yaml",
    "lego/operator/internal/publish/publish.go",
    "deploy/gitops/charts/kpack/platform.yaml",
    "deploy/gitops/base/build-image-prewarm.yaml",
)

REGISTRY_API = {
    "docker.io": "registry-1.docker.io",
    "gcr.io": "gcr.io",
    "quay.io": "quay.io",
}
REGISTRY_AUTH = {
    "docker.io": ("https://auth.docker.io/token", "registry.docker.io"),
    "gcr.io": ("https://gcr.io/v2/token", "gcr.io"),
    "quay.io": ("https://quay.io/v2/auth", "quay.io"),
}
MANIFEST_ACCEPT = (
    "application/vnd.oci.image.index.v1+json,"
    "application/vnd.docker.distribution.manifest.list.v2+json,"
    "application/vnd.oci.image.manifest.v1+json,"
    "application/vnd.docker.distribution.manifest.v2+json"
)


def fail(message: str) -> None:
    print(f"FAIL: {message}", file=sys.stderr)
    raise SystemExit(1)


def repo_root() -> Path:
    override = os.environ.get("FRESHNESS_ROOT")
    if override:
        return Path(override).resolve()
    return Path(__file__).resolve().parents[2]


def inventory_path(explicit: str | None) -> Path:
    if explicit:
        return Path(explicit).resolve()
    env = os.environ.get("FRESHNESS_INVENTORY")
    if env:
        return Path(env).resolve()
    return repo_root() / "lego/operator/internal/build/toolchain-freshness.json"


def pin_sites() -> list[str]:
    env = os.environ.get("FRESHNESS_PIN_SITES")
    if env:
        return [p for p in env.split("\n") if p.strip()]
    return list(CANONICAL_PIN_SITES)


def dump(obj: object) -> str:
    return json.dumps(obj, indent=2, sort_keys=True) + "\n"


def digest_of(value: str) -> str:
    match = DIGEST_RE.search(value)
    if not match:
        raise ValueError(f"no sha256 digest in {value!r}")
    return match.group(0)


def parse_inventory(raw: str) -> dict:
    try:
        data = json.loads(raw)
    except json.JSONDecodeError as err:
        fail(f"inventory is not JSON: {err}")
    if not isinstance(data, dict):
        fail("inventory root must be an object")
    if data.get("schema") != SCHEMA:
        fail(f"inventory schema must be {SCHEMA!r}")
    images = data.get("images")
    if not isinstance(images, list) or not images:
        fail("inventory images must be a non-empty array")
    seen: set[str] = set()
    builder = False
    for i, image in enumerate(images):
        prefix = f"images[{i}]"
        if not isinstance(image, dict):
            fail(f"{prefix} must be an object")
        for key in ("id", "kind", "upstream", "committed", "resolved_at", "source", "files"):
            if key not in image:
                fail(f"{prefix} missing {key}")
        ident = image["id"]
        if not isinstance(ident, str) or not ID_RE.fullmatch(ident):
            fail(f"{prefix}.id is not a lowercase slug")
        if ident in seen:
            fail(f"duplicate inventory id {ident!r}")
        seen.add(ident)
        if image["kind"] not in KINDS:
            fail(f"{prefix}.kind must be one of {sorted(KINDS)}")
        if not isinstance(image["upstream"], str) or ":" not in image["upstream"] or "@" in image["upstream"]:
            fail(f"{prefix}.upstream must be a host/name:tag reference")
        try:
            digest_of(image["committed"])
        except ValueError:
            fail(f"{prefix}.committed must contain sha256:<64 hex>")
        if not isinstance(image["source"], str) or not image["source"].strip():
            fail(f"{prefix}.source must be a non-empty resolution source")
        try:
            datetime.strptime(image["resolved_at"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
        except (TypeError, ValueError):
            fail(f"{prefix}.resolved_at must be UTC RFC3339 YYYY-MM-DDTHH:MM:SSZ")
        files = image["files"]
        if not isinstance(files, list) or not files or any(not isinstance(f, str) or not f for f in files):
            fail(f"{prefix}.files must be a non-empty array of paths")
        if ident == CLUSTER_BUILDER_ID:
            builder = True
    if not builder:
        fail(f"inventory must include id {CLUSTER_BUILDER_ID!r} for ClusterBuilder age metrics")
    return data


def load_inventory(path: Path) -> dict:
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as err:
        fail(f"cannot read inventory {path}: {err}")
    return parse_inventory(raw)


def file_digests(path: Path) -> set[str]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as err:
        fail(f"cannot read pin site {path}: {err}")
    return set(DIGEST_RE.findall(text))


def validate(path: Path) -> dict:
    inventory = load_inventory(path)
    root = repo_root()
    committed: dict[str, str] = {}
    for image in inventory["images"]:
        digest = digest_of(image["committed"])
        committed[digest] = image["id"]
        for rel in image["files"]:
            target = root / rel
            if not target.is_file():
                fail(f"{image['id']}: listed file {rel} does not exist")
            if digest not in file_digests(target):
                fail(f"{image['id']}: digest {digest} missing from {rel}")
    uncovered: list[str] = []
    for rel in pin_sites():
        target = root / rel
        if not target.is_file():
            fail(f"pin site {rel} does not exist")
        for digest in sorted(file_digests(target)):
            if digest not in committed:
                uncovered.append(f"{rel}: {digest}")
    if uncovered:
        fail("pin-site digest not in inventory:\n  " + "\n  ".join(uncovered))
    return inventory


def split_name_tag(ref: str) -> tuple[str, str]:
    last_slash = ref.rfind("/")
    last_colon = ref.rfind(":")
    if last_colon <= last_slash:
        raise ValueError(f"missing tag in {ref!r}")
    return ref[:last_colon], ref[last_colon + 1 :]


def parse_upstream(ref: str) -> tuple[str, str, str]:
    name, tag = split_name_tag(ref)
    if "/" not in name:
        return "docker.io", f"library/{name}", tag
    first, rest = name.split("/", 1)
    if "." in first or ":" in first or first == "localhost":
        return first, rest, tag
    return "docker.io", name, tag


def fixture_digests() -> dict[str, str] | None:
    path = os.environ.get("FRESHNESS_DIGESTS")
    if not path:
        return None
    try:
        data = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        fail(f"FRESHNESS_DIGESTS is not readable JSON: {err}")
    if not isinstance(data, dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in data.items()):
        fail("FRESHNESS_DIGESTS must be an object of upstream → digest strings")
    return data


def http_json(url: str, headers: dict[str, str] | None = None) -> dict:
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except (urllib.error.URLError, json.JSONDecodeError, TimeoutError) as err:
        raise RuntimeError(f"{url}: {err}") from err


def http_digest(url: str, headers: dict[str, str]) -> str:
    req = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            digest = resp.headers.get("Docker-Content-Digest", "")
    except urllib.error.URLError as err:
        raise RuntimeError(f"{url}: {err}") from err
    if not DIGEST_RE.fullmatch(digest.strip()):
        raise RuntimeError(f"{url}: missing Docker-Content-Digest")
    return digest.strip()


def resolve_upstream(ref: str) -> str:
    fixtures = fixture_digests()
    if fixtures is not None:
        if ref not in fixtures:
            fail(f"FRESHNESS_DIGESTS has no entry for {ref}")
        try:
            return digest_of(fixtures[ref])
        except ValueError:
            fail(f"FRESHNESS_DIGESTS digest for {ref} is malformed")
    registry, repo, tag = parse_upstream(ref)
    api_host = REGISTRY_API.get(registry, registry)
    auth = REGISTRY_AUTH.get(registry)
    headers = {"Accept": MANIFEST_ACCEPT}
    if auth:
        token_url, service = auth
        token_qs = urllib.parse.urlencode(
            {"service": service, "scope": f"repository:{repo}:pull"}
        )
        payload = http_json(f"{token_url}?{token_qs}")
        token = payload.get("token") or payload.get("access_token")
        if not token:
            raise RuntimeError(f"no anonymous token for {registry}")
        headers["Authorization"] = f"Bearer {token}"
    return http_digest(f"https://{api_host}/v2/{repo}/manifests/{tag}", headers)


def resolve(path: Path) -> dict:
    inventory = validate(path)
    images = []
    for image in sorted(inventory["images"], key=lambda item: item["id"]):
        try:
            observed = resolve_upstream(image["upstream"])
        except RuntimeError as err:
            fail(f"{image['id']}: resolve {image['upstream']}: {err}")
        committed = digest_of(image["committed"])
        images.append(
            {
                "changed": observed != committed,
                "committed": committed,
                "files": list(image["files"]),
                "id": image["id"],
                "kind": image["kind"],
                "observed": observed,
                "upstream": image["upstream"],
            }
        )
    return {"images": images, "schema": SCHEMA}


def drifted(report: dict) -> list[dict]:
    return [image for image in report["images"] if image.get("changed")]


def issue_body(report: dict) -> str:
    lines = [
        "## Build toolchain digest drift",
        "",
        "Upstream registries published new digests for pinned build-toolchain images.",
        "This is a **review request**, not authorization to update. The scheduled",
        "workflow never edits, commits, or merges a pin.",
        "",
        "After reviewing an observed digest, update every listed file to that digest",
        "and set that inventory entry's `resolved_at` to the review time (UTC RFC3339).",
        "Procedure: `docs/ADR060-build-worker-reliability-and-performance.md` D7.",
        "",
    ]
    for image in drifted(report):
        files = "\n".join(f"  - `{path}`" for path in image["files"])
        lines.extend(
            [
                f"### `{image['id']}`",
                "",
                f"- upstream: `{image['upstream']}`",
                f"- committed: `{image['committed']}`",
                f"- observed: `{image['observed']}`",
                "- files:",
                files,
                "",
            ]
        )
    lines.append("Resolver: `python3 scripts/lib/toolchain-freshness.py resolve`.")
    lines.append("")
    return "\n".join(lines)


def issue_action(report: dict) -> str:
    existing = os.environ.get("EXISTING_ISSUE", "").strip()
    if drifted(report):
        return "update" if existing else "open"
    return "close" if existing else "noop"


def load_report(path: Path) -> dict:
    try:
        report = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        fail(f"cannot read resolve report {path}: {err}")
    if not isinstance(report, dict) or not isinstance(report.get("images"), list):
        fail("resolve report must contain an images array")
    return report


def main(argv: list[str]) -> None:
    if len(argv) < 2 or argv[1] in {"-h", "--help"}:
        print(
            "usage: toolchain-freshness.py validate [inventory]\n"
            "       toolchain-freshness.py resolve [inventory]\n"
            "       toolchain-freshness.py issue-body <resolve.json>\n"
            "       toolchain-freshness.py issue-action <resolve.json>",
            file=sys.stderr,
        )
        raise SystemExit(2)
    cmd = argv[1]
    rest = argv[2:]
    if cmd == "validate":
        validate(inventory_path(rest[0] if rest else None))
        print("PASS: build-toolchain freshness inventory")
        return
    if cmd == "resolve":
        sys.stdout.write(dump(resolve(inventory_path(rest[0] if rest else None))))
        return
    if cmd == "issue-body":
        if not rest:
            fail("issue-body requires a resolve JSON path")
        sys.stdout.write(issue_body(load_report(Path(rest[0]))))
        return
    if cmd == "issue-action":
        if not rest:
            fail("issue-action requires a resolve JSON path")
        print(issue_action(load_report(Path(rest[0]))))
        return
    fail(f"unknown command {cmd}")


if __name__ == "__main__":
    main(sys.argv)
