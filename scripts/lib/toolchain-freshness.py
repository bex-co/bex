#!/usr/bin/env python3
"""Build-toolchain freshness inventory: validate, resolve, issue decisions.

A changed upstream digest is a review request: nothing here commits or merges
a pin, and only go-apply edits one, the native Go line that follows each new
stable Go release (w4/719, go-line-bump.yml). Public registry metadata only;
no cluster or push credentials.
"""
from __future__ import annotations

import hashlib
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
KINDS = frozenset({"builder", "stack", "native-base", "helper", "npm-package", "release-asset"})
# An npm-package pin (w8/m73 pnpm) is a registry tarball, not an image: its
# upstream is registry.npmjs.org/<name>:<dist-tag> and its digest is the sha256
# of the tarball that dist-tag currently names.
NPM_REGISTRY = "registry.npmjs.org"
# A release-asset pin (w8/m106 Hugo) is one file of a GitHub release: its
# upstream is github.com/<owner>/<repo>/<asset>:<tag> and its digest is the
# sha256 of that asset as the release serves it now.
GITHUB = "github.com"
DIGEST_RE = re.compile(r"sha256:[a-f0-9]{64}")
ID_RE = re.compile(r"^[a-z][a-z0-9-]{0,62}$")

CANONICAL_PIN_SITES = (
    "lego/operator/internal/build/native.go",
    "lego/types/toolchains/python.go",
    "lego/operator/internal/build/nativebases.go",
    "lego/operator/internal/build/nativepnpm.go",
    "lego/operator/internal/build/nativeffmpeg.go",
    "lego/operator/internal/build/nativehugo.go",
    "lego/operator/internal/build/build.go",
    "lego/operator/cmd/manager/main.go",
    "lego/operator/config/manager/manager.yaml",
    "lego/operator/internal/publish/publish.go",
    "deploy/gitops/charts/kpack/platform.yaml",
    "deploy/gitops/base/build-image-prewarm.yaml",
    "deploy/cnb-run/Dockerfile",
    "deploy/native-base/Dockerfile",
    "deploy/native-common/Dockerfile",
)

REGISTRY_API = {
    "docker.io": "registry-1.docker.io",
    "gcr.io": "gcr.io",
    "ghcr.io": "ghcr.io",
    "quay.io": "quay.io",
}
REGISTRY_AUTH = {
    "docker.io": ("https://auth.docker.io/token", "registry.docker.io"),
    "gcr.io": ("https://gcr.io/v2/token", "gcr.io"),
    "ghcr.io": ("https://ghcr.io/token", "ghcr.io"),
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
        # An npm scope (registry.npmjs.org/@pnpm/linux-x64) is a name, not a digest.
        upstream = image["upstream"].replace(NPM_REGISTRY + "/@", NPM_REGISTRY + "/", 1) if isinstance(image["upstream"], str) else None
        if upstream is None or ":" not in upstream or "@" in upstream:
            fail(f"{prefix}.upstream must be a host/name:tag reference")
        # The resolver routes by upstream host, so a kind's upstream must name
        # the host its kind resolves through.
        hosts = {"npm-package": NPM_REGISTRY + "/", "release-asset": GITHUB + "/"}
        want_host = hosts.get(image["kind"])
        if want_host and not image["upstream"].startswith(want_host):
            fail(f"{prefix}.upstream of a {image['kind']} must start with {want_host}")
        if not want_host and image["upstream"].startswith(tuple(hosts.values())):
            fail(f"{prefix}.upstream of a {image['kind']} must be an image reference")
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


class NotFound(RuntimeError):
    """The registry has no such tag (yet)."""


def http_digest(url: str, headers: dict[str, str]) -> str:
    req = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            digest = resp.headers.get("Docker-Content-Digest", "")
    except urllib.error.HTTPError as err:
        raise (NotFound if err.code == 404 else RuntimeError)(f"{url}: {err}") from err
    except urllib.error.URLError as err:
        raise RuntimeError(f"{url}: {err}") from err
    if not DIGEST_RE.fullmatch(digest.strip()):
        raise RuntimeError(f"{url}: missing Docker-Content-Digest")
    return digest.strip()


def http_sha256(url: str) -> str:
    digest = hashlib.sha256()
    try:
        with urllib.request.urlopen(urllib.request.Request(url), timeout=120) as resp:
            for chunk in iter(lambda: resp.read(1 << 16), b""):
                digest.update(chunk)
    except (urllib.error.URLError, TimeoutError) as err:
        raise RuntimeError(f"{url}: {err}") from err
    return "sha256:" + digest.hexdigest()


def resolve_npm_package(ref: str) -> str:
    name, tag = split_name_tag(ref.removeprefix(NPM_REGISTRY + "/"))
    meta = http_json(f"https://{NPM_REGISTRY}/{urllib.parse.quote(name, safe='@')}/{urllib.parse.quote(tag)}")
    tarball = (meta.get("dist") or {}).get("tarball")
    if not isinstance(tarball, str) or not tarball.startswith(f"https://{NPM_REGISTRY}/"):
        raise RuntimeError(f"{ref}: no {NPM_REGISTRY} tarball in dist metadata")
    return http_sha256(tarball)


def resolve_release_asset(ref: str) -> str:
    name, tag = split_name_tag(ref.removeprefix(GITHUB + "/"))
    parts = name.split("/")
    if len(parts) != 3 or not all(parts):
        raise RuntimeError(f"{ref}: want {GITHUB}/<owner>/<repo>/<asset>:<tag>")
    owner, repo, asset = (urllib.parse.quote(part) for part in parts)
    return http_sha256(f"https://{GITHUB}/{owner}/{repo}/releases/download/{urllib.parse.quote(tag)}/{asset}")


def resolve_upstream(ref: str) -> str:
    fixtures = fixture_digests()
    if fixtures is not None:
        if ref not in fixtures:
            # As the registry answers a tag it does not have.
            raise NotFound(f"FRESHNESS_DIGESTS has no entry for {ref}")
        try:
            return digest_of(fixtures[ref])
        except ValueError:
            fail(f"FRESHNESS_DIGESTS digest for {ref} is malformed")
    if ref.startswith(NPM_REGISTRY + "/"):
        return resolve_npm_package(ref)
    if ref.startswith(GITHUB + "/"):
        return resolve_release_asset(ref)
    registry, repo, tag = parse_upstream(ref)
    return http_digest(f"https://{REGISTRY_API.get(registry, registry)}/v2/{repo}/manifests/{tag}",
                       registry_headers(registry, repo))


def registry_headers(registry: str, repo: str) -> dict[str, str]:
    """Manifest Accept plus an anonymous pull token where the registry wants one."""
    headers = {"Accept": MANIFEST_ACCEPT}
    auth = REGISTRY_AUTH.get(registry)
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
    return headers


# LINE_SOURCES are the native toolchains whose pinned LINE (not just digest)
# must keep up with upstream, because Render builds with the newest one (w5/m144):
# Go with "latest stable", Bun with its newest release as the default line. A
# same-tag digest check never notices a new line, which is how Go sat on 1.24
# while go.mod files moved to 1.25+.
LINE_SOURCES = {
    "native-go": ("https://go.dev/dl/?mode=json", "go"),
    "native-bun": ("https://api.github.com/repos/oven-sh/bun/releases/latest", "bun"),
}


def line_of(version: str) -> tuple[int, int] | None:
    match = re.search(r"(\d+)\.(\d+)", version)
    return (int(match.group(1)), int(match.group(2))) if match else None


def fixture_version(key: str) -> tuple[bool, str | None]:
    """FRESHNESS_LATEST_LINES's newest version for key: (fixture set, version)."""
    fixture = os.environ.get("FRESHNESS_LATEST_LINES")
    if not fixture:
        return False, None
    try:
        data = json.loads(Path(fixture).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        fail(f"FRESHNESS_LATEST_LINES is not readable JSON: {err}")
    version = data.get(key) if isinstance(data, dict) else None
    if version is not None and not isinstance(version, str):
        raise RuntimeError(f"unexpected release feed shape: {version!r}")
    return True, version


def latest_line(image_id: str) -> str | None:
    """Upstream's newest major.minor for a LINE_SOURCES toolchain, or None."""
    fixed, version = fixture_version(image_id)
    if not fixed:
        version = upstream_version(image_id)
        if not isinstance(version, str):
            raise RuntimeError(f"unexpected release feed shape: {version!r}")
    line = line_of(version) if version else None
    return f"{line[0]}.{line[1]}" if line else None


def upstream_version(image_id: str) -> object:
    url, kind = LINE_SOURCES[image_id]
    # Unauthenticated GitHub API calls share a 60/hour budget per runner IP.
    token = os.environ.get("GITHUB_TOKEN")
    data = http_json(url, {"Authorization": f"Bearer {token}"} if token and "api.github.com" in url else None)
    return data[0]["version"] if kind == "go" else data.get("tag_name", "")


# RELEASE_LINE_SOURCES are the runtimes with one native line per upstream
# release line, as many as Render builds: any released version (w8/335). A
# line released upstream but newer than every pinned one is missing, and a
# service asking for it fails at version selection, which is how Python 3.15
# shipped four days after the lines stopped at 3.14. The finding rides on the
# newest pinned entry. Each feed's version key, its prefix, and the line depth.
RELEASE_LINE_SOURCES = {
    "docker.io/library/python": (
        "https://www.python.org/api/v2/downloads/release/?is_published=true&pre_release=false",
        "name", "Python ", 2,
    ),
    "docker.io/library/node": ("https://nodejs.org/dist/index.json", "version", "v", 1),
}


def release_line(version: str, depth: int) -> tuple[int, ...] | None:
    """The first depth numbers of a version or tag: 3.14-bookworm → (3, 14)."""
    match = re.match(r"\d+(?:\.\d+)*", version)
    parts = tuple(int(part) for part in match.group(0).split(".")) if match else ()
    return parts[:depth] if len(parts) >= depth else None


def newest_release_line(repo: str) -> tuple[int, ...] | None:
    """Upstream's newest released line of a RELEASE_LINE_SOURCES runtime."""
    url, key, prefix, depth = RELEASE_LINE_SOURCES[repo]
    fixed, version = fixture_version(repo)
    if fixed:
        return release_line(version, depth) if version else None
    # Final releases only: Node's index leads with its alphas and Python's
    # feed is filtered but checked here too.
    lines = [release_line(item[key].removeprefix(prefix), depth) for item in http_json(url)
             if re.fullmatch(r"\d+\.\d+\.\d+", item[key].removeprefix(prefix))]
    return max(lines, default=None)


def missing_release_lines(images: list[dict]) -> dict[str, str]:
    """Inventory id of each runtime's newest pinned line → the newer released line."""
    newest: dict[str, tuple[str, tuple[int, ...]]] = {}
    for image in images:
        repo, _, tag = image["upstream"].rpartition(":")
        if repo in RELEASE_LINE_SOURCES:
            line = release_line(tag, RELEASE_LINE_SOURCES[repo][3])
            if line and (repo not in newest or line > newest[repo][1]):
                newest[repo] = (image["id"], line)
    missing = {}
    for repo, (ident, pinned) in sorted(newest.items()):
        # Advisory, like LINE_SOURCES: an unreachable feed must not fail the
        # digest check this resolve exists for.
        try:
            released = newest_release_line(repo)
        except (RuntimeError, KeyError, TypeError, AttributeError) as err:
            print(f"warning: {repo}: newest released line unavailable: {err}", file=sys.stderr)
            continue
        if released and released > pinned:
            missing[ident] = ".".join(str(part) for part in released)
    return missing


def resolve(path: Path) -> dict:
    inventory = validate(path)
    missing = missing_release_lines(inventory["images"])
    images = []
    for image in sorted(inventory["images"], key=lambda item: item["id"]):
        try:
            observed = resolve_upstream(image["upstream"])
        except RuntimeError as err:
            fail(f"{image['id']}: resolve {image['upstream']}: {err}")
        committed = digest_of(image["committed"])
        entry = {
            "changed": observed != committed,
            "committed": committed,
            "files": list(image["files"]),
            "id": image["id"],
            "kind": image["kind"],
            "observed": observed,
            "upstream": image["upstream"],
        }
        if image["id"] in LINE_SOURCES:
            # Advisory: an unreachable release feed must not fail the digest
            # check this resolve exists for.
            try:
                newest = latest_line(image["id"])
            except (RuntimeError, KeyError, IndexError, TypeError, AttributeError) as err:
                print(f"warning: {image['id']}: latest line unavailable: {err}", file=sys.stderr)
                newest = None
            pinned = line_of(split_name_tag(image["upstream"])[1])
            if newest and pinned and line_of(newest) > pinned:
                entry["changed"] = True
                entry["newer_line"] = newest
        if image["id"] in missing:
            entry["changed"] = True
            entry["missing_line"] = missing[image["id"]]
        images.append(entry)
    return {"images": images, "schema": SCHEMA}


# The native Go line moves itself (w4/719). Render's native Go "automatically
# updates to the latest stable Go 1.x version … usually within 24 hours of a
# new release" (changelog 2026-04-16), so go-line-bump.yml runs go-release
# every few hours and, on a "bump" plan, smoke-builds the image on both
# architectures and runs go-apply, the one command here that edits the tree.
# A digest that moves without a new Go release stays a reviewed change.
GO_ID = "native-go"
GO_TAG = "golang:{line}-bookworm"
GO_PLATFORMS = ("linux/amd64", "linux/arm64")
# native.go's Go entry: one line, which is also the default and its image's tag.
GO_ENTRY_RE = re.compile(
    r'(\t"go": \{label: "Go", defaultLine: ")[0-9.]+(", lines: \[\]nativeLine\{\n'
    r'\t\t\{line: ")[0-9.]+(", image: ")golang:[0-9.]+-bookworm@sha256:[a-f0-9]{64}("\},\n\t\}\},)'
)


def go_release(version: str) -> tuple[int, int, int]:
    """go1.27.4, 1.27.4 or go1.20 (Go's name for 1.20.0) as numbers."""
    match = re.fullmatch(r"(?:go)?(\d+)\.(\d+)(?:\.(\d+))?", version.strip())
    if not match:
        raise RuntimeError(f"not a stable Go release: {version!r}")
    return int(match.group(1)), int(match.group(2)), int(match.group(3) or 0)


def go_name(release: tuple[int, int, int]) -> str:
    return ".".join(str(part) for part in release)


def latest_go_release() -> tuple[int, int, int]:
    """go.dev's newest stable release (FRESHNESS_LATEST_LINES native-go in tests)."""
    fixed, version = fixture_version(GO_ID)
    if fixed:
        if not version:
            raise RuntimeError("FRESHNESS_LATEST_LINES has no native-go release")
        return go_release(version)
    feed = http_json(LINE_SOURCES[GO_ID][0])
    stable = [go_release(item["version"]) for item in feed if isinstance(item, dict) and item.get("stable")]
    if not stable:
        raise RuntimeError("go.dev lists no stable release")
    return max(stable)


def image_go_versions(upstream: str, digest: str) -> dict[str, str]:
    """GOLANG_VERSION of each GO_PLATFORMS image under one golang index digest.

    FRESHNESS_GO_VERSIONS (tests) maps a digest to one version for every
    platform, or to a platform → version object.
    """
    fixture = os.environ.get("FRESHNESS_GO_VERSIONS")
    if fixture:
        try:
            value = json.loads(Path(fixture).read_text(encoding="utf-8")).get(digest, {})
        except (OSError, json.JSONDecodeError, AttributeError) as err:
            fail(f"FRESHNESS_GO_VERSIONS is not a readable JSON object: {err}")
        return {platform: value for platform in GO_PLATFORMS} if isinstance(value, str) else dict(value)
    registry, repo, _ = parse_upstream(upstream)
    base = f"https://{REGISTRY_API.get(registry, registry)}/v2/{repo}"
    headers = registry_headers(registry, repo)
    versions: dict[str, str] = {}
    for entry in http_json(f"{base}/manifests/{digest}", headers).get("manifests") or []:
        platform = entry.get("platform") or {}
        key = f"{platform.get('os')}/{platform.get('architecture')}"
        if key not in GO_PLATFORMS or key in versions:
            continue
        config = http_json(f"{base}/manifests/{entry['digest']}", headers)["config"]["digest"]
        for env in (http_json(f"{base}/blobs/{config}", headers).get("config") or {}).get("Env") or []:
            if env.startswith("GOLANG_VERSION="):
                versions[key] = env.split("=", 1)[1]
    return versions


def inventory_entry(inventory: dict, ident: str) -> dict:
    entry = next((image for image in inventory["images"] if image["id"] == ident), None)
    if entry is None:
        fail(f"inventory has no {ident!r} entry")
    return entry


def go_plan(path: Path) -> dict:
    """Whether the native Go pin is behind go.dev's newest stable release.

    action is "current", "wait" (the official image does not carry the
    release on every platform yet) or "bump" with the image to pin.
    """
    entry = inventory_entry(validate(path), GO_ID)
    committed = entry["committed"]
    pinned = image_go_versions(entry["upstream"], digest_of(committed))
    if set(pinned) != set(GO_PLATFORMS):
        fail(f"{committed}: no GOLANG_VERSION for {sorted(set(GO_PLATFORMS) - set(pinned))}")
    current = min(go_release(version) for version in pinned.values())
    latest = latest_go_release()
    plan = {"action": "current", "committed": committed, "current": go_name(current),
            "id": GO_ID, "latest": go_name(latest)}
    if latest <= current:
        plan["reason"] = f"native Go is on {go_name(current)}, go.dev's newest stable release"
        return plan
    line = f"{latest[0]}.{latest[1]}"
    upstream = "docker.io/library/" + GO_TAG.format(line=line)
    plan["action"] = "wait"
    try:
        digest = resolve_upstream(upstream)
    except NotFound:
        plan["reason"] = (f"Go {go_name(latest)} is out, but {upstream} is not published yet "
                          f"(native Go stays on {go_name(current)})")
        return plan
    carried = image_go_versions(upstream, digest)
    behind = sorted(p for p in GO_PLATFORMS if carried.get(p) is None or go_release(carried[p]) != latest)
    if behind:
        plan["reason"] = (f"Go {go_name(latest)} is out, but {upstream}@{digest} does not carry it on "
                          f"{', '.join(behind)} yet (native Go stays on {go_name(current)})")
        return plan
    plan.update({
        "action": "bump",
        "image": GO_TAG.format(line=line) + "@" + digest,
        "line": line,
        "reason": f"native Go {go_name(current)} → {go_name(latest)}, go.dev's newest stable release",
        "upstream": upstream,
    })
    return plan


def go_apply(plan_path: Path, path: Path) -> str:
    """Rewrite the native Go pin and its inventory entry to a "bump" plan."""
    try:
        plan = json.loads(plan_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        fail(f"cannot read the Go plan {plan_path}: {err}")
    if not isinstance(plan, dict) or plan.get("action") != "bump":
        fail("go-apply needs a go-release plan whose action is bump")
    image, line, upstream = plan.get("image", ""), plan.get("line", ""), plan.get("upstream", "")
    tag = GO_TAG.format(line=line)
    if (not re.fullmatch(r"\d+\.\d+", line) or not DIGEST_RE.search(image)
            or image != tag + "@" + digest_of(image) or upstream != "docker.io/library/" + tag):
        fail(f"malformed Go plan: image {image!r}, line {line!r}, upstream {upstream!r}")
    inventory = validate(path)
    entry = inventory_entry(inventory, GO_ID)
    if entry["committed"] == image:
        return f"noop: native Go already pins {image}"
    if entry["committed"] != plan.get("committed"):
        return f"noop: native Go moved to {entry['committed']} since the plan; the next run plans again"
    if len(entry["files"]) != 1:
        fail(f"{GO_ID} must list native.go alone, not {entry['files']}")
    target = repo_root() / entry["files"][0]
    rewritten, count = GO_ENTRY_RE.subn(
        lambda m: f"{m.group(1)}{line}{m.group(2)}{line}{m.group(3)}{image}{m.group(4)}",
        target.read_text(encoding="utf-8"))
    if count != 1:
        fail(f"{entry['files'][0]}: want one native Go entry in GO_ENTRY_RE's shape, found {count}")
    target.write_text(rewritten, encoding="utf-8")
    now = os.environ.get("FRESHNESS_NOW") or datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    entry.update({
        "upstream": upstream,
        "committed": image,
        "resolved_at": now,
        "source": (f"Go {plan['latest']} (was {plan['current']}), go.dev's newest stable release, applied "
                   f"{now[:10]} by go-line-bump.yml (w4/719): official {tag} carries "
                   f"GOLANG_VERSION={plan['latest']} on the linux/amd64 and linux/arm64 image configs"),
    })
    # As committed: insertion order, two-space indent (dump sorts keys).
    path.write_text(json.dumps(inventory, indent=2) + "\n", encoding="utf-8")
    validate(path)
    return f"native Go {plan['current']} → {plan['latest']}: {image}"


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
        section = [
            f"### `{image['id']}`",
            "",
            f"- upstream: `{image['upstream']}`",
            f"- committed: `{image['committed']}`",
            f"- observed: `{image['observed']}`",
        ]
        if image.get("newer_line"):
            section.append(
                f"- **newer line upstream: `{image['newer_line']}`**: move the pin to that"
                " line's tag and digest (Render builds with the newest), not to the observed digest"
            )
        if image["id"] == GO_ID:
            section.append(
                "- `go-line-bump.yml` moves this pin by itself once the official image carries a new"
                " Go release; a new digest with the same Go (a Debian rebuild) still needs review"
            )
        if image.get("missing_line"):
            section.append(
                f"- **released line missing: `{image['missing_line']}`**: add it as a new reviewed"
                " line beside this one (Render builds any released version); a service asking"
                " for it fails at version selection until then"
            )
        section.extend(["- files:", files, ""])
        lines.extend(section)
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
            "       toolchain-freshness.py issue-action <resolve.json>\n"
            "       toolchain-freshness.py go-release [inventory]\n"
            "       toolchain-freshness.py go-apply <go-release.json> [inventory]",
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
    if cmd == "go-release":
        try:
            sys.stdout.write(dump(go_plan(inventory_path(rest[0] if rest else None))))
        except (RuntimeError, KeyError, TypeError) as err:
            fail(f"go-release: {err}")
        return
    if cmd == "go-apply":
        if not rest:
            fail("go-apply requires a go-release JSON path")
        print(go_apply(Path(rest[0]), inventory_path(rest[1] if len(rest) > 1 else None)))
        return
    fail(f"unknown command {cmd}")


if __name__ == "__main__":
    main(sys.argv)
