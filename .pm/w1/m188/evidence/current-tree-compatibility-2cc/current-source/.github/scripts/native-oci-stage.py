#!/usr/bin/env python3
"""Stage one verified public native supplier as an OCI layout; never extract it."""
import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

REPOSITORY = "ghcr.io/bex-co/bex-cnb-run"
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
REFERENCE = re.compile(re.escape(REPOSITORY) + r"@(sha256:[0-9a-f]{64})\Z")
MAX_IMAGE = 8 * 1024**3
MAX_METADATA = 4 * 1024**2
MAX_CONFIG = 1024**2
CHUNK = 1024**2
MARKER = ".bex-native-oci-stage.json"
SCHEMA = "bex.native-oci-stage/v1"
INDEX_TYPES = {"application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json"}
MANIFEST_TYPES = {"application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json"}
CONFIG_TYPES = {"application/vnd.oci.image.config.v1+json", "application/vnd.docker.container.image.v1+json"}
ACCEPT = ",".join(sorted(INDEX_TYPES | MANIFEST_TYPES))


class StageError(Exception):
    pass


def json_document(data):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise StageError("duplicate metadata key")
            result[key] = value
        return result
    try:
        return json.loads(data, object_pairs_hook=unique)
    except (ValueError, UnicodeError) as exc:
        raise StageError("invalid JSON metadata") from exc


def origin(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password:
        raise StageError("non-HTTPS or credential-bearing redirect rejected")
    return parsed.hostname.lower(), parsed.port or 443


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, new_url):
        cross_origin = origin(request.full_url) != origin(new_url)
        redirected = super().redirect_request(request, fp, code, message, headers, new_url)
        if redirected and cross_origin:
            for mapping in (redirected.headers, redirected.unredirected_hdrs):
                for key in list(mapping):
                    if key.lower() == "authorization":
                        del mapping[key]
        return redirected


class Registry:
    """Anonymous pull only. Tokens and signed redirect URLs never enter receipts."""
    def __init__(self):
        self.opener = urllib.request.build_opener(SafeRedirect())
        self.token = None
        self.expires = 0

    def open(self, kind, digest, offset):
        try:
            if not self.token or time.monotonic() >= self.expires:
                query = urllib.parse.urlencode({"service": "ghcr.io", "scope": "repository:bex-co/bex-cnb-run:pull"})
                with self.opener.open("https://ghcr.io/token?" + query, timeout=60) as response:
                    data = response.read(65537)
                if len(data) > 65536:
                    raise StageError("oversize anonymous token response")
                token = json_document(data)
                self.token = token.get("token") or token.get("access_token")
                if not isinstance(self.token, str) or not self.token:
                    raise StageError("missing anonymous pull token")
                lifetime = token.get("expires_in", 120)
                self.expires = time.monotonic() + min(lifetime, 300) * 0.8 if type(lifetime) is int and lifetime > 0 else 0
            url = "https://ghcr.io/v2/bex-co/bex-cnb-run/" + kind + "/" + digest
            headers = {"Authorization": "Bearer " + self.token, "Accept": ACCEPT}
            if offset:
                headers["Range"] = "bytes=" + str(offset) + "-"
            return self.opener.open(urllib.request.Request(url, headers=headers), timeout=60)
        except urllib.error.HTTPError as exc:
            if exc.code == 401:
                self.token = None
            raise StageError("registry HTTP " + str(exc.code)) from None
        except (OSError, urllib.error.URLError, http.client.HTTPException):
            raise StageError("registry transport failed") from None


def regular(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_nlink != 1:
        raise StageError("unsafe owned file")


def file_hash(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(CHUNK), b""):
            digest.update(block)
    return "sha256:" + digest.hexdigest()


def atomic_json(path, value):
    if path.exists() or path.is_symlink():
        regular(path)
    fd, temporary = tempfile.mkstemp(prefix=".native-oci-", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as target:
            json.dump(value, target, sort_keys=True, separators=(",", ":"))
            target.write("\n")
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def owned_directory(path):
    if path.exists() or path.is_symlink():
        info = path.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise StageError("output directory must be private and owned")
    else:
        path.mkdir(mode=0o700)


class Stager:
    def __init__(self, ref, platform, output, proof, registry=None, sleep=time.sleep):
        match = REFERENCE.fullmatch(ref)
        if not match or platform not in ("linux/amd64", "linux/arm64"):
            raise StageError("expected fixed public repository, immutable SHA256 and supported platform")
        self.platform, self.root_digest = platform, match[1]
        output = Path(output)
        self.output = output.parent.resolve(strict=True) / output.name
        owned_directory(self.output)
        proof = Path(proof)
        self.proof_path = proof.parent.resolve(strict=True) / proof.name
        identity = {"schema": SCHEMA, "ref": ref, "platform": platform}
        marker = self.output / MARKER
        if marker.exists() or marker.is_symlink():
            regular(marker)
            if json_document(marker.read_bytes()) != identity:
                raise StageError("output marker identity mismatch")
        elif any(self.output.iterdir()):
            raise StageError("nonempty unmarked output rejected")
        else:
            atomic_json(marker, identity)
        for relative in ("blobs", "blobs/sha256", "parts"):
            owned_directory(self.output / relative)
        forbidden = {marker, self.output / "index.json", self.output / "oci-layout"}
        if self.proof_path in forbidden or any(self.proof_path.is_relative_to(self.output / x) for x in ("blobs", "parts")):
            raise StageError("unsafe proof path")
        if self.proof_path.exists() or self.proof_path.is_symlink():
            regular(self.proof_path)
            previous = json_document(self.proof_path.read_bytes())
            if any(previous.get(key) != value for key, value in identity.items()) or previous.get("output") != str(self.output):
                raise StageError("existing proof identity mismatch")
        index = self.output / "index.json"
        if index.exists() or index.is_symlink():
            regular(index)
            index.unlink()
        self.registry = registry or Registry()
        self.sleep = sleep
        self.proof = dict(identity, output=str(self.output), complete=False, blobs=[])

    def blob(self, digest, size, limit, kind="blobs"):
        if not isinstance(digest, str) or not DIGEST.fullmatch(digest) or (size is not None and (type(size) is not int or not 0 <= size <= limit)):
            raise StageError("invalid or oversize descriptor")
        verified = self.output / "blobs/sha256" / digest[7:]
        partial = self.output / "parts" / (digest[7:] + ".part")
        record = {"digest": digest, "expected_size": size, "attempts": [], "verified": False}
        self.proof["blobs"].append(record)
        if verified.exists() or verified.is_symlink():
            regular(verified)
            actual = verified.stat().st_size
            if actual <= limit and (size is None or actual == size) and file_hash(verified) == digest:
                record.update(verified=True, reused=True, size=actual)
                return verified
            verified.unlink()
        if partial.exists() or partial.is_symlink():
            regular(partial)

        def recover_complete_partial():
            if not partial.exists() or size is None or partial.stat().st_size != size:
                return False
            if file_hash(partial) != digest:
                partial.unlink()
                return False
            os.replace(partial, verified)
            record.update(verified=True, reused=True, recovered_complete_partial=True, size=size)
            return True

        for attempt in range(1, 4):
            if recover_complete_partial():
                return verified
            if partial.exists() and (size is None or partial.stat().st_size > size):
                partial.unlink()
            offset = partial.stat().st_size if partial.exists() else 0
            item = {"attempt": attempt, "requested_offset": offset}
            record["attempts"].append(item)
            reset = False
            try:
                with self.registry.open(kind, digest, offset) as response:
                    status = response.status
                    if status == 200:
                        if offset:
                            item["range_ignored_restart"] = True
                        offset = 0
                    elif status == 206 and size is not None:
                        expected_range = "bytes " + str(offset) + "-" + str(size - 1) + "/" + str(size)
                        if response.headers.get("Content-Range") != expected_range:
                            reset = True
                            raise StageError("invalid resumed Content-Range")
                    else:
                        reset = True
                        raise StageError("unexpected blob response status")
                    length = response.headers.get("Content-Length")
                    if length is not None and (not length.isdecimal() or int(length) > limit - offset or size is not None and int(length) != size - offset):
                        reset = True
                        raise StageError("blob response length mismatch")
                    flags = os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW | (os.O_APPEND if offset else os.O_TRUNC)
                    with os.fdopen(os.open(partial, flags, 0o600), "wb") as target:
                        total = offset
                        while True:
                            try:
                                remaining = (size if size is not None else limit) - total
                                block = response.read(min(CHUNK, remaining + 1))
                            except http.client.IncompleteRead as exc:
                                block = exc.partial
                                if len(block) <= limit - total and (size is None or len(block) <= size - total):
                                    target.write(block)
                                raise StageError("truncated blob stream") from None
                            if not block:
                                break
                            if len(block) > limit - total or size is not None and len(block) > size - total:
                                reset = True
                                raise StageError("blob exceeds descriptor size")
                            target.write(block)
                            total += len(block)
                        target.flush()
                        os.fsync(target.fileno())
                actual = partial.stat().st_size
                item["received_total"] = actual
                if size is not None and actual != size:
                    raise StageError("truncated blob stream")
                if file_hash(partial) != digest:
                    reset = True
                    raise StageError("blob digest mismatch")
                os.replace(partial, verified)
                item["outcome"] = "verified"
                record.update(verified=True, reused=False, size=actual)
                return verified
            except (StageError, OSError, urllib.error.URLError, http.client.HTTPException) as exc:
                item["outcome"] = str(exc) if isinstance(exc, StageError) else "blob transport failed"
                if reset and partial.exists():
                    partial.unlink()
                if recover_complete_partial():
                    item.update(outcome="verified_complete_partial", received_total=size)
                    return verified
                item["retained_partial_size"] = partial.stat().st_size if partial.exists() else 0
                if attempt < 3:
                    self.sleep(attempt * 2)
        raise StageError("blob failed after three attempts: " + digest)

    def descriptor(self, value, limit):
        if not isinstance(value, dict) or not isinstance(value.get("digest"), str) or not DIGEST.fullmatch(value["digest"]) or type(value.get("size")) is not int or not 0 <= value["size"] <= limit or value.get("urls"):
            raise StageError("invalid or oversize OCI descriptor")
        return value

    def run(self):
        try:
            root = self.blob(self.root_digest, None, MAX_METADATA, "manifests")
            document = json_document(root.read_bytes())
            media_type = document.get("mediaType")
            if type(document.get("schemaVersion")) is not int or document["schemaVersion"] != 2:
                raise StageError("unsupported manifest schema")
            if media_type in INDEX_TYPES:
                matches = [d for d in document.get("manifests", []) if d.get("platform", {}).get("os") == "linux" and d.get("platform", {}).get("architecture") == self.platform.split("/")[1]]
                if len(matches) != 1:
                    raise StageError("requested platform missing or ambiguous")
                selected = self.descriptor(matches[0], MAX_METADATA)
                manifest = self.blob(selected["digest"], selected["size"], MAX_METADATA, "manifests")
                document = json_document(manifest.read_bytes())
            else:
                selected = {"digest": self.root_digest, "size": root.stat().st_size, "mediaType": media_type}
            if type(document.get("schemaVersion")) is not int or document["schemaVersion"] != 2 or document.get("mediaType") not in MANIFEST_TYPES or selected.get("mediaType") != document.get("mediaType"):
                raise StageError("selected descriptor is not an image manifest")
            config = self.descriptor(document.get("config"), MAX_CONFIG)
            if config.get("mediaType") not in CONFIG_TYPES:
                raise StageError("unsupported image config media type")
            layers = document.get("layers")
            if not isinstance(layers, list) or len(layers) > 4096:
                raise StageError("invalid layer inventory")
            layers = [self.descriptor(d, MAX_IMAGE) for d in layers]
            if sum(d["size"] for d in layers) > MAX_IMAGE:
                raise StageError("compressed image exceeds 8GiB")
            config_path = self.blob(config["digest"], config["size"], MAX_CONFIG)
            config_document = json_document(config_path.read_bytes())
            if config_document.get("os") != "linux" or config_document.get("architecture") != self.platform.split("/")[1]:
                raise StageError("raw config does not match requested platform")
            platform = {"os": "linux", "architecture": self.platform.split("/")[1]}
            variants = {v for v in (selected.get("platform", {}).get("variant"), config_document.get("variant")) if v}
            if len(variants) > 1 or any(not isinstance(v, str) or not re.fullmatch(r"v[0-9]+(?:\.[0-9]+)*", v) for v in variants):
                raise StageError("invalid or inconsistent platform variant")
            if variants:
                platform["variant"] = variants.pop()
            for layer in layers:
                self.blob(layer["digest"], layer["size"], MAX_IMAGE)
            selected = {key: selected[key] for key in ("mediaType", "digest", "size")}
            selected["platform"] = platform
            atomic_json(self.output / "oci-layout", {"imageLayoutVersion": "1.0.0"})
            atomic_json(self.output / "index.json", {"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": [selected]})
            self.proof.update(complete=True, root={"digest": self.root_digest, "size": root.stat().st_size}, selected=selected, config={"digest": config["digest"], "size": config["size"]}, compressed_layer_bytes=sum(d["size"] for d in layers))
        except Exception as exc:
            self.proof["error"] = str(exc) if isinstance(exc, StageError) else "staging failed (" + type(exc).__name__ + ")"
            raise StageError(self.proof["error"]) from None
        finally:
            atomic_json(self.proof_path, self.proof)
        return self.proof


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--platform", choices=("linux/amd64", "linux/arm64"), required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--proof", type=Path, required=True)
    args = parser.parse_args()
    try:
        proof = Stager(args.ref, args.platform, args.output, args.proof).run()
    except Exception:
        parser.exit(1, "native OCI staging failed; no verified layout approval\n")
    print(json.dumps({"complete": True, "selected": proof["selected"]["digest"], "proof": str(args.proof)}))


if __name__ == "__main__":
    main()
