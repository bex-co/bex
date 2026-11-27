#!/usr/bin/env python3
"""Build-toolchain freshness inventory: validate, resolve, issue decisions.

A changed upstream digest is a review request: nothing here commits or merges
a pin, and only go-apply and rust-apply edit one, the native Go and Rust lines
that follow each new stable release (w4/719 go-line-bump.yml, w3/m638
rust-line-bump.yml). Public registry metadata only; no cluster or push
credentials.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import sys
import tomllib
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Callable
from datetime import datetime, timezone
from pathlib import Path
from typing import NamedTuple

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


def http_text(url: str, headers: dict[str, str] | None = None) -> str:
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.read().decode("utf-8")
    except (urllib.error.URLError, UnicodeDecodeError, TimeoutError) as err:
        raise RuntimeError(f"{url}: {err}") from err


def http_json(url: str, headers: dict[str, str] | None = None) -> dict:
    try:
        return json.loads(http_text(url, headers))
    except json.JSONDecodeError as err:
        raise RuntimeError(f"{url}: {err}") from err


class NotFound(RuntimeError):
    """The registry has no such tag (yet)."""


def http_digest(url: str, headers: dict[str, str]) -> str:
    # HEAD answers the same Docker-Content-Digest without the manifest, and
    # Docker Hub does not count it against the anonymous pull limit, which
    # line_version_mismatches' reads spend instead.
    req = urllib.request.Request(url, headers=headers, method="HEAD")
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


def platform_configs(upstream: str, digest: str, platforms: tuple[str, ...]) -> dict[str, dict]:
    """The image config of each of platforms ("linux/amd64") under one index digest."""
    registry, repo, _ = parse_upstream(upstream)
    base = f"https://{REGISTRY_API.get(registry, registry)}/v2/{repo}"
    headers = registry_headers(registry, repo)
    configs: dict[str, dict] = {}
    for entry in http_json(f"{base}/manifests/{digest}", headers).get("manifests") or []:
        platform = entry.get("platform") or {}
        key = f"{platform.get('os')}/{platform.get('architecture')}"
        if key not in platforms or key in configs:
            continue
        config = http_json(f"{base}/manifests/{entry['digest']}", headers)["config"]["digest"]
        configs[key] = http_json(f"{base}/blobs/{config}", headers).get("config") or {}
    return configs


def config_env(config: dict, name: str) -> str | None:
    return next((env.split("=", 1)[1] for env in config.get("Env") or [] if env.startswith(name + "=")), None)


# A reviewed native line names the patch its image carries (`version:` in
# native.go, `Version:` in lego/types/toolchains/python.go). Version selection
# matches requests against it and the build log names it, so it must move with
# the digest. resolve reads the patch from each pinned image's linux/amd64
# config, where the upstream records it, and reports a line whose field says
# otherwise (w3/m635).
LINE_VERSION_SITES = ("lego/operator/internal/build/native.go", "lego/types/toolchains/python.go")
LINE_VERSION_RE = re.compile(
    r'\{[Ll]ine: "[^"]+", [Vv]ersion: "(?P<version>[^"]+)",(?: otp: "[^"]+",)? '
    r'[Ii]mage: "[^"@]+@(?P<digest>sha256:[a-f0-9]{64})"'
)
# Where each upstream repository's image config records its patch: an Env
# variable or a label. Elixir writes its own as v1.18.6.
IMAGE_VERSION_KEYS = {
    "library/elixir": ("Env", "ELIXIR_VERSION"),
    "library/node": ("Env", "NODE_VERSION"),
    "library/python": ("Env", "PYTHON_VERSION"),
    "library/ruby": ("Env", "RUBY_VERSION"),
    "library/rust": ("Env", "RUST_VERSION"),
    "oven/bun": ("Labels", "org.opencontainers.image.version"),
}


def pinned_line_versions() -> dict[str, str]:
    """digest → the version field of every reviewed line that names one."""
    root = repo_root()
    versions: dict[str, str] = {}
    for rel in LINE_VERSION_SITES:
        target = root / rel
        if target.is_file():
            for match in LINE_VERSION_RE.finditer(target.read_text(encoding="utf-8")):
                versions[match.group("digest")] = match.group("version")
    return versions


def image_version(upstream: str, digest: str) -> str | None:
    """The patch an image's linux/amd64 config records, or None if unrecorded.

    FRESHNESS_IMAGE_VERSIONS (tests) maps a digest to that patch; with the
    FRESHNESS_DIGESTS fixture alone nothing is read from a registry.
    """
    repo = parse_upstream(upstream)[1]
    if repo not in IMAGE_VERSION_KEYS:
        return None
    fixture = os.environ.get("FRESHNESS_IMAGE_VERSIONS")
    if fixture or os.environ.get("FRESHNESS_DIGESTS"):
        try:
            value = (json.loads(Path(fixture).read_text(encoding="utf-8")) if fixture else {}).get(digest)
        except (OSError, json.JSONDecodeError, AttributeError) as err:
            fail(f"FRESHNESS_IMAGE_VERSIONS is not a readable JSON object: {err}")
    else:
        config = platform_configs(upstream, digest, ("linux/amd64",)).get("linux/amd64", {})
        section, name = IMAGE_VERSION_KEYS[repo]
        value = config_env(config, name) if section == "Env" else (config.get("Labels") or {}).get(name)
    return value.removeprefix("v") if isinstance(value, str) else None


def line_version_mismatches(images: list[dict]) -> dict[str, dict[str, str]]:
    """Inventory id → {field, image} for each line whose version field is not its image's patch."""
    fields = pinned_line_versions()
    mismatches = {}
    for image in images:
        digest = digest_of(image["committed"])
        if digest not in fields:
            continue
        # Advisory, like LINE_SOURCES: an unreadable config must not fail the
        # digest check this resolve exists for.
        try:
            carried = image_version(image["upstream"], digest)
        except (RuntimeError, KeyError, TypeError, AttributeError) as err:
            print(f"warning: {image['id']}: image version unavailable: {err}", file=sys.stderr)
            continue
        if carried and carried != fields[digest]:
            mismatches[image["id"]] = {"field": fields[digest], "image": carried}
    return mismatches


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
    versions = line_version_mismatches(inventory["images"])
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
        if image["id"] in versions:
            entry["changed"] = True
            entry["version_mismatch"] = versions[image["id"]]
        images.append(entry)
    return {"images": images, "schema": SCHEMA}


# The native Go and Rust lines move themselves. Render's native Go
# "automatically updates to the latest stable Go 1.x version … usually within
# 24 hours of a new release" (changelog 2026-04-16, w4/719), and "by default,
# Render uses the latest stable Rust toolchain" (render.com/docs/rust-toolchain,
# w3/m638). go-line-bump.yml and rust-line-bump.yml run <cmd>-release every few
# hours and, on a "bump" plan, smoke-build the image on both architectures and
# run <cmd>-apply, the only commands here that edit the tree. A digest that
# moves without a new release stays a reviewed change.
PLATFORMS = ("linux/amd64", "linux/arm64")
Release = tuple[int, int, int]


class MovingLine(NamedTuple):
    cmd: str  # the <cmd>-release / <cmd>-apply prefix and <cmd>-line-bump.yml
    ident: str  # its inventory entry
    label: str
    env: str  # the image config Env that records the release
    feed: str  # where the newest stable release is read, for messages
    item: str  # the board item that made it move itself
    parse: Callable[[str], Release]
    latest: Callable[[], Release]
    tag: Callable[[Release], str]  # the official tag that carries a release
    # native.go's entry. Its groups are the fixed text around the values apply
    # writes: values(release) in order, then the digest-pinned image.
    entry_re: re.Pattern[str]
    values: Callable[[Release], tuple[str, ...]]

    @property
    def workflow(self) -> str:
        return f"{self.cmd}-line-bump.yml"


def release_name(release: Release) -> str:
    return ".".join(str(part) for part in release)


def go_release(version: str) -> Release:
    """go1.27.4, 1.27.4 or go1.20 (Go's name for 1.20.0) as numbers."""
    match = re.fullmatch(r"(?:go)?(\d+)\.(\d+)(?:\.(\d+))?", version.strip())
    if not match:
        raise RuntimeError(f"not a stable Go release: {version!r}")
    return int(match.group(1)), int(match.group(2)), int(match.group(3) or 0)


def rust_release(version: str) -> Release:
    """1.100.0 as numbers. The line is rust:1-bookworm, so a Rust 2 fails the
    plan for a person to review instead of waiting for a tag that never comes."""
    match = re.fullmatch(r"1\.(\d+)\.(\d+)", version.strip())
    if not match:
        raise RuntimeError(f"not a stable Rust 1.x release: {version!r}")
    return 1, int(match.group(1)), int(match.group(2))


def fixture_release(ident: str, parse: Callable[[str], Release]) -> Release | None:
    """FRESHNESS_LATEST_LINES's newest release for ident, None without the fixture."""
    fixed, version = fixture_version(ident)
    if not fixed:
        return None
    if not version:
        raise RuntimeError(f"FRESHNESS_LATEST_LINES has no {ident} release")
    return parse(version)


def latest_go_release() -> Release:
    """go.dev's newest stable release (FRESHNESS_LATEST_LINES native-go in tests)."""
    fixed = fixture_release("native-go", go_release)
    if fixed:
        return fixed
    feed = http_json(LINE_SOURCES["native-go"][0])
    stable = [go_release(item["version"]) for item in feed if isinstance(item, dict) and item.get("stable")]
    if not stable:
        raise RuntimeError("go.dev lists no stable release")
    return max(stable)


RUST_CHANNEL = "https://static.rust-lang.org/dist/channel-rust-stable.toml"


def latest_rust_release() -> Release:
    """The stable channel's rust (FRESHNESS_LATEST_LINES native-rust in tests).

    rustup installs `stable` from this manifest, whose [pkg.rust] version reads
    "1.100.0 (0ef1bc386 2026-11-09)".
    """
    fixed = fixture_release("native-rust", rust_release)
    if fixed:
        return fixed
    try:
        channel = tomllib.loads(http_text(RUST_CHANNEL))
    except tomllib.TOMLDecodeError as err:
        raise RuntimeError(f"{RUST_CHANNEL}: {err}") from err
    version = ((channel.get("pkg") or {}).get("rust") or {}).get("version")
    if not isinstance(version, str) or not version.strip():
        raise RuntimeError(f"{RUST_CHANNEL} names no [pkg.rust] version")
    return rust_release(version.split()[0])


GO = MovingLine(
    cmd="go", ident="native-go", label="Go", env="GOLANG_VERSION",
    feed="go.dev's newest stable release", item="w4/719",
    parse=go_release, latest=latest_go_release,
    tag=lambda release: f"golang:{release[0]}.{release[1]}-bookworm",
    # One line, which is also the default and its image's tag.
    entry_re=re.compile(
        r'(\t"go": \{label: "Go", defaultLine: ")[0-9.]+(", lines: \[\]nativeLine\{\n'
        r'\t\t\{line: ")[0-9.]+(", image: ")golang:[0-9.]+-bookworm@sha256:[a-f0-9]{64}("\},\n\t\}\},)'
    ),
    values=lambda release: (f"{release[0]}.{release[1]}",) * 2,
)
RUST = MovingLine(
    cmd="rust", ident="native-rust", label="Rust", env="RUST_VERSION",
    feed="static.rust-lang.org's stable channel", item="w3/m638",
    parse=rust_release, latest=latest_rust_release,
    tag=lambda release: "rust:1-bookworm",
    # One line "1", the default, on rust:1-bookworm: only the version the build
    # log names (w8/327) and the digest move.
    entry_re=re.compile(
        r'(\tnativeRustRuntime: \{label: "Rust", defaultLine: "1", lines: \[\]nativeLine\{\n'
        r'\t\t\{line: "1", version: ")[0-9.]+(", image: ")rust:1-bookworm@sha256:[a-f0-9]{64}("\},\n\t\}\},)'
    ),
    values=lambda release: (release_name(release),),
)
MOVING_LINES = {spec.cmd: spec for spec in (GO, RUST)}


def platform_versions(upstream: str, digest: str, env: str) -> dict[str, str]:
    """env of each PLATFORMS image under one index digest.

    FRESHNESS_PLATFORM_VERSIONS (tests) maps a digest to one version for every
    platform, or to a platform → version object.
    """
    fixture = os.environ.get("FRESHNESS_PLATFORM_VERSIONS")
    if fixture:
        try:
            value = json.loads(Path(fixture).read_text(encoding="utf-8")).get(digest, {})
        except (OSError, json.JSONDecodeError, AttributeError) as err:
            fail(f"FRESHNESS_PLATFORM_VERSIONS is not a readable JSON object: {err}")
        return {platform: value for platform in PLATFORMS} if isinstance(value, str) else dict(value)
    configs = platform_configs(upstream, digest, PLATFORMS)
    return {key: version for key, config in configs.items() if (version := config_env(config, env))}


def inventory_entry(inventory: dict, ident: str) -> dict:
    entry = next((image for image in inventory["images"] if image["id"] == ident), None)
    if entry is None:
        fail(f"inventory has no {ident!r} entry")
    return entry


def line_plan(spec: MovingLine, path: Path) -> dict:
    """Whether a moving line's pin is behind its newest stable release.

    action is "current", "wait" (the official image does not carry the
    release on every platform yet) or "bump" with the image to pin.
    """
    entry = inventory_entry(validate(path), spec.ident)
    committed = entry["committed"]
    pinned = platform_versions(entry["upstream"], digest_of(committed), spec.env)
    if set(pinned) != set(PLATFORMS):
        fail(f"{committed}: no {spec.env} for {sorted(set(PLATFORMS) - set(pinned))}")
    current = min(spec.parse(version) for version in pinned.values())
    latest = spec.latest()
    plan = {"action": "current", "committed": committed, "current": release_name(current),
            "id": spec.ident, "latest": release_name(latest)}
    if latest <= current:
        plan["reason"] = f"native {spec.label} is on {release_name(current)}, {spec.feed}"
        return plan
    tag = spec.tag(latest)
    upstream = "docker.io/library/" + tag
    out = f"{spec.label} {release_name(latest)} is out, but {upstream}"
    stays = f"(native {spec.label} stays on {release_name(current)})"
    plan["action"] = "wait"
    try:
        digest = resolve_upstream(upstream)
    except NotFound:
        plan["reason"] = f"{out} is not published yet {stays}"
        return plan
    carried = platform_versions(upstream, digest, spec.env)
    behind = sorted(p for p in PLATFORMS if carried.get(p) is None or spec.parse(carried[p]) != latest)
    if behind:
        plan["reason"] = f"{out}@{digest} does not carry it on {', '.join(behind)} yet {stays}"
        return plan
    plan.update({
        "action": "bump",
        "image": tag + "@" + digest,
        "reason": f"native {spec.label} {release_name(current)} → {release_name(latest)}, {spec.feed}",
        "upstream": upstream,
    })
    return plan


def line_apply(spec: MovingLine, plan_path: Path, path: Path) -> str:
    """Rewrite a moving line's pin and its inventory entry to a "bump" plan."""
    try:
        plan = json.loads(plan_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        fail(f"cannot read the {spec.label} plan {plan_path}: {err}")
    if not isinstance(plan, dict) or plan.get("action") != "bump":
        fail(f"{spec.cmd}-apply needs a {spec.cmd}-release plan whose action is bump")
    image, upstream = plan.get("image", ""), plan.get("upstream", "")
    try:
        latest, current = (spec.parse(str(plan.get(key, ""))) for key in ("latest", "current"))
    except RuntimeError as err:
        fail(f"malformed {spec.label} plan: {err}")
    tag = spec.tag(latest)
    if (not isinstance(image, str) or not DIGEST_RE.search(image)
            or image != tag + "@" + digest_of(image) or upstream != "docker.io/library/" + tag):
        fail(f"malformed {spec.label} plan: image {image!r}, latest {plan.get('latest')!r}, upstream {upstream!r}")
    inventory = validate(path)
    entry = inventory_entry(inventory, spec.ident)
    if entry["committed"] == image:
        return f"noop: native {spec.label} already pins {image}"
    if entry["committed"] != plan.get("committed"):
        return f"noop: native {spec.label} moved to {entry['committed']} since the plan; the next run plans again"
    if len(entry["files"]) != 1:
        fail(f"{spec.ident} must list native.go alone, not {entry['files']}")
    target = repo_root() / entry["files"][0]
    values = (*spec.values(latest), image, "")
    rewritten, count = spec.entry_re.subn(
        lambda m: "".join(text + value for text, value in zip(m.groups(), values, strict=True)),
        target.read_text(encoding="utf-8"))
    if count != 1:
        fail(f"{entry['files'][0]}: want one native {spec.label} entry in {spec.cmd}'s entry_re shape, found {count}")
    target.write_text(rewritten, encoding="utf-8")
    now = os.environ.get("FRESHNESS_NOW") or datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    latest_name, current_name = release_name(latest), release_name(current)
    entry.update({
        "upstream": upstream,
        "committed": image,
        "resolved_at": now,
        "source": (f"{spec.label} {latest_name} (was {current_name}), {spec.feed}, applied {now[:10]} by "
                   f"{spec.workflow} ({spec.item}): official {tag} carries {spec.env}={latest_name} on the "
                   "linux/amd64 and linux/arm64 image configs"),
    })
    # As committed: insertion order, two-space indent and literal non-ASCII
    # (dump sorts keys), so no other entry's line changes.
    path.write_text(json.dumps(inventory, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    validate(path)
    return f"native {spec.label} {current_name} → {latest_name}: {image}"


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
        moving = next((spec for spec in MOVING_LINES.values() if spec.ident == image["id"]), None)
        if moving:
            section.append(
                f"- `{moving.workflow}` moves this pin by itself once the official image carries a new"
                f" {moving.label} release; a new digest with the same {moving.label} (a Debian rebuild)"
                " still needs review"
            )
        if image.get("missing_line"):
            section.append(
                f"- **released line missing: `{image['missing_line']}`**: add it as a new reviewed"
                " line beside this one (Render builds any released version); a service asking"
                " for it fails at version selection until then"
            )
        if image.get("version_mismatch"):
            mismatch = image["version_mismatch"]
            section.append(
                f"- **version field `{mismatch['field']}`, but the committed image carries"
                f" `{mismatch['image']}`**: the build log and version selection name the field,"
                " so move it with the digest"
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
            "       toolchain-freshness.py go-release|rust-release [inventory]\n"
            "       toolchain-freshness.py go-apply|rust-apply <plan.json> [inventory]",
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
    line, _, verb = cmd.partition("-")
    if line in MOVING_LINES and verb == "release":
        try:
            sys.stdout.write(dump(line_plan(MOVING_LINES[line], inventory_path(rest[0] if rest else None))))
        except (RuntimeError, KeyError, TypeError) as err:
            fail(f"{cmd}: {err}")
        return
    if line in MOVING_LINES and verb == "apply":
        if not rest:
            fail(f"{cmd} requires a {line}-release JSON path")
        print(line_apply(MOVING_LINES[line], Path(rest[0]), inventory_path(rest[1] if len(rest) > 1 else None)))
        return
    fail(f"unknown command {cmd}")


if __name__ == "__main__":
    main(sys.argv)
