#!/usr/bin/env python3
"""Serve a verified native OCI graph to one unchanged-reference Trivy scan."""
import argparse
import http.server
import importlib.util
import json
import os
from pathlib import Path
import signal
import stat
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse

sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("native_oci_stage", Path(__file__).with_name("native-oci-stage.py"))
stage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(stage)
SCAN_SECONDS = 21 * 60
STOP_SECONDS = 10


class MirrorError(Exception):
    pass


def document(path):
    stage.regular(path)
    if path.stat().st_size > stage.MAX_METADATA:
        raise MirrorError("oversize staging metadata")
    return stage.json_document(path.read_bytes())


class VerifiedLayout:
    """Recheck the staged graph without fetching or modifying any staged byte."""
    def __init__(self, ref, platform, layout, proof):
        match = stage.REFERENCE.fullmatch(ref)
        if not match or platform not in ("linux/amd64", "linux/arm64"):
            raise MirrorError("expected immutable native repository and explicit supported platform")
        self.root = Path(layout).resolve(strict=True)
        info = self.root.stat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise MirrorError("layout must be private and owned")
        identity = {"schema": stage.SCHEMA, "ref": ref, "platform": platform}
        if document(self.root / stage.MARKER) != identity:
            raise MirrorError("staging marker identity mismatch")
        self.proof = document(Path(proof))
        if self.proof.get("complete") is not True or any(self.proof.get(k) != v for k, v in identity.items()) or self.proof.get("output") != str(self.root):
            raise MirrorError("staging proof is incomplete or has another identity")
        self.manifests, self.blobs = {}, {}
        self.root_digest = match[1]
        root = self.checked(self.proof.get("root"), stage.MAX_METADATA)
        if root["digest"] != self.root_digest:
            raise MirrorError("staged root differs from immutable reference")
        raw_root = stage.json_document(root["path"].read_bytes())
        if raw_root.get("schemaVersion") != 2:
            raise MirrorError("unsupported root schema")
        root["media_type"] = raw_root.get("mediaType")
        self.manifests[self.root_digest] = root
        if root["media_type"] in stage.INDEX_TYPES:
            candidates = [d for d in raw_root.get("manifests", []) if d.get("platform", {}).get("os") == "linux" and d.get("platform", {}).get("architecture") == platform.split("/")[1]]
            if len(candidates) != 1:
                raise MirrorError("requested platform missing or ambiguous")
            selected = self.checked(candidates[0], stage.MAX_METADATA)
        elif root["media_type"] in stage.MANIFEST_TYPES:
            selected = root
        else:
            raise MirrorError("unsupported root media type")
        manifest = stage.json_document(selected["path"].read_bytes())
        if manifest.get("schemaVersion") != 2 or manifest.get("mediaType") not in stage.MANIFEST_TYPES:
            raise MirrorError("selected object is not an image manifest")
        if root["media_type"] in stage.INDEX_TYPES and candidates[0].get("mediaType") != manifest["mediaType"]:
            raise MirrorError("selected descriptor media type mismatch")
        selected["media_type"] = manifest["mediaType"]
        if selected["digest"] != self.proof.get("selected", {}).get("digest") or selected["size"] != self.proof["selected"].get("size"):
            raise MirrorError("selected manifest differs from staging proof")
        self.manifests[selected["digest"]] = selected
        config = self.checked(manifest.get("config"), stage.MAX_CONFIG)
        if manifest["config"].get("mediaType") not in stage.CONFIG_TYPES or config["digest"] != self.proof.get("config", {}).get("digest"):
            raise MirrorError("config differs from staging proof")
        actual = stage.json_document(config["path"].read_bytes())
        if actual.get("os") != "linux" or actual.get("architecture") != platform.split("/")[1]:
            raise MirrorError("actual config platform mismatch")
        self.blobs[config["digest"]] = config
        layers = manifest.get("layers")
        if not isinstance(layers, list) or len(layers) > 4096:
            raise MirrorError("invalid layer inventory")
        total = 0
        for descriptor in layers:
            layer = self.checked(descriptor, stage.MAX_IMAGE)
            self.blobs[layer["digest"]] = layer
            total += layer["size"]
        if total > stage.MAX_IMAGE or total != self.proof.get("compressed_layer_bytes"):
            raise MirrorError("compressed image size differs from complete staging proof")
        self.selected_digest, self.config_digest = selected["digest"], config["digest"]

    def checked(self, descriptor, maximum):
        if not isinstance(descriptor, dict) or not isinstance(descriptor.get("digest"), str) or not stage.DIGEST.fullmatch(descriptor["digest"]) or type(descriptor.get("size")) is not int or not 0 <= descriptor["size"] <= maximum or descriptor.get("urls"):
            raise MirrorError("invalid staged descriptor")
        path = self.root / "blobs/sha256" / descriptor["digest"][7:]
        stage.regular(path)
        if path.stat().st_size != descriptor["size"] or stage.file_hash(path) != descriptor["digest"]:
            raise MirrorError("staged blob size or SHA256 mismatch")
        return {"digest": descriptor["digest"], "size": descriptor["size"], "path": path}


class Registry(http.server.ThreadingHTTPServer):
    daemon_threads = True
    block_on_close = False

    def __init__(self, layout):
        self.layout, self.events = layout, []
        self.native_failure, self.root_served = False, False
        super().__init__(("127.0.0.1", 0), Handler)


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_HEAD(self):
        self.read(False)

    def do_GET(self):
        self.read(True)

    def read(self, body):
        parsed = urllib.parse.urlsplit(self.path)
        prefix = "/v2/bex-co/bex-cnb-run/"
        path = urllib.parse.unquote(parsed.path)
        if path == "/v2/" and not parsed.query:
            self.send_response(200)
            self.send_header("Content-Length", "2")
            self.send_header("Docker-Distribution-Api-Version", "registry/2.0")
            self.end_headers()
            if body:
                self.wfile.write(b"{}")
            return
        kind, _, digest = path.removeprefix(prefix).partition("/")
        entries = self.server.layout.manifests if kind == "manifests" else self.server.layout.blobs if kind == "blobs" else {}
        entry = entries.get(digest) if path.startswith(prefix) and not parsed.query else None
        event = {"method": self.command, "kind": kind if kind in ("manifests", "blobs") else "unknown", "digest": digest if stage.DIGEST.fullmatch(digest) else None, "status": 404}
        self.server.events.append(event)
        if entry is None:
            if path.startswith(prefix):
                self.server.native_failure = True
            self.send_error(404)
            return
        try:
            stage.regular(entry["path"])
            if entry["path"].stat().st_size != entry["size"]:
                raise MirrorError("staged blob changed size")
            with entry["path"].open("rb") as source:
                self.send_response(200)
                self.send_header("Content-Type", entry.get("media_type", "application/octet-stream"))
                self.send_header("Content-Length", str(entry["size"]))
                self.send_header("Docker-Content-Digest", digest)
                self.end_headers()
                event["status"] = 200
                if body:
                    while block := source.read(stage.CHUNK):
                        self.wfile.write(block)
            if body and digest == self.server.layout.root_digest:
                self.server.root_served = True
        except (OSError, MirrorError, stage.StageError):
            event["status"] = 500
            self.server.native_failure = True
            self.close_connection = True


def stop_process(process, grace=STOP_SECONDS):
    if process is None:
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        process.wait(timeout=grace)
        return
    deadline = time.monotonic() + grace
    while time.monotonic() < deadline:
        process.poll()
        try:
            os.killpg(process.pid, 0)
        except ProcessLookupError:
            process.wait(timeout=grace)
            return
        time.sleep(0.05)
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    process.wait(timeout=grace)


def scan(ref, platform, layout, stage_proof, proof, command, parent_pid, scan_seconds=SCAN_SECONDS, stop_seconds=STOP_SECONDS):
    if len(command) < 3 or command[1:3] != ["image", ref] or os.environ.get("TRIVY_PLATFORM") != platform:
        raise MirrorError("scan command must retain its original immutable reference and platform")
    if any(arg in ("--config", "-c", "--input") or arg.startswith(("--config=", "--input=")) for arg in command[3:]) or os.environ.get("TRIVY_CONFIG") or os.environ.get("TRIVY_INPUT") or Path("trivy.yaml").exists():
        raise MirrorError("transport config cannot replace an existing scan config or archive input")
    proof = Path(proof)
    if proof.resolve() == Path(stage_proof).resolve() or proof.resolve().is_relative_to(Path(layout).resolve()):
        raise MirrorError("mirror receipt must be separate from staged inputs")
    receipt = {"ref": ref, "platform": platform, "complete": False, "requests": []}
    server, server_thread, watcher, process = None, None, None, None
    watch_stop = threading.Event()
    previous = {}
    spawning, interrupted = False, False

    def cancelled(signum, _):
        nonlocal interrupted
        interrupted = True
        if spawning:
            return  # Finish owned startup so cleanup can observe the resource.
        raise MirrorError("scan cancelled by signal " + str(signum))

    def start_thread(thread):
        nonlocal spawning
        spawning = True
        try:
            thread.start()
        finally:
            spawning = False
        if interrupted:
            raise MirrorError("scan cancelled during thread startup")

    def watch_parent():
        while not watch_stop.wait(0.1):
            if os.getppid() != parent_pid:
                os.kill(os.getpid(), signal.SIGTERM)
                return

    try:
        for signum in (signal.SIGTERM, signal.SIGINT):
            previous[signum] = signal.getsignal(signum)
            signal.signal(signum, cancelled)
        watcher = threading.Thread(target=watch_parent, daemon=True)
        start_thread(watcher)
        if os.getppid() != parent_pid:
            raise MirrorError("scan parent already exited")
        verified = VerifiedLayout(ref, platform, layout, stage_proof)
        receipt.update(root_digest=verified.root_digest, selected_manifest=verified.selected_digest, config_digest=verified.config_digest, stage_proof_sha256=stage.file_hash(Path(stage_proof)))
        server = Registry(verified)
        receipt["loopback_port"] = server.server_port
        server_thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.05}, daemon=True)
        start_thread(server_thread)
        with tempfile.TemporaryDirectory(prefix="native-oci-mirror-", dir=os.environ.get("TMPDIR")) as work:
            config = Path(work) / "trivy-mirror.json"
            stage.atomic_json(config, {"registry": {"mirrors": {"ghcr.io": ["127.0.0.1:" + str(server.server_port)]}}})
            # Trivy keeps the original reference for ArtifactName/RepoDigests.
            # Only its registry transport receives this per-command mirror.
            spawning = True
            try:
                process = subprocess.Popen(command[:1] + ["--config", str(config)] + command[1:], start_new_session=True)
            finally:
                spawning = False
            if interrupted:
                raise MirrorError("scan cancelled during child startup")
            receipt["scan_pid"] = process.pid
            try:
                status = process.wait(timeout=scan_seconds)
            except subprocess.TimeoutExpired as exc:
                raise MirrorError("complete scan exceeded 21 minute budget") from exc
            receipt["scan_exit"] = status
            if status != 0:
                raise MirrorError("Trivy scan failed with exit " + str(status))
            # Trivy may fall back to the origin after mirror lookup failures.
            # Never label such a scan as a successful staged-local transport.
            if server.native_failure or not server.root_served:
                raise MirrorError("scan did not complete through the verified native mirror")
            receipt["complete"] = True
    except (Exception, KeyboardInterrupt) as exc:
        receipt["complete"] = False
        receipt["error"] = str(exc) if isinstance(exc, MirrorError) else "mirror scan failed (" + type(exc).__name__ + ")"
        raise MirrorError(receipt["error"]) from None
    finally:
        watch_stop.set()
        for signum in previous:
            signal.signal(signum, signal.SIG_IGN)
        cleanup_error = None
        try:
            stop_process(process, stop_seconds)
        except (OSError, subprocess.TimeoutExpired):
            cleanup_error = "owned scan process cleanup failed"
            receipt.update(complete=False, error=cleanup_error)
        finally:
            if server:
                try:
                    if server_thread is not None and server_thread.is_alive():
                        server.shutdown()
                finally:
                    server.server_close()
                if server_thread is not None and server_thread.ident is not None:
                    server_thread.join(timeout=1)
                receipt["requests"] = server.events
                if server.native_failure and receipt["complete"]:
                    cleanup_error = "native mirror request failed before shutdown"
                    receipt.update(complete=False, error=cleanup_error)
            if watcher is not None and watcher.ident is not None:
                watcher.join(timeout=1)
            if not receipt["complete"] and os.environ.get("TRIVY_OUTPUT"):
                Path(os.environ["TRIVY_OUTPUT"]).unlink(missing_ok=True)
            for signum, handler in previous.items():
                signal.signal(signum, handler)
            stage.atomic_json(proof, receipt)
        if cleanup_error:
            raise MirrorError(cleanup_error)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--platform", required=True)
    parser.add_argument("--layout", type=Path, required=True)
    parser.add_argument("--stage-proof", type=Path, required=True)
    parser.add_argument("--proof", type=Path, required=True)
    parser.add_argument("--parent-pid", type=int, required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    try:
        scan(args.ref, args.platform, args.layout, args.stage_proof, args.proof, command, args.parent_pid)
    except (MirrorError, OSError, stage.StageError):
        parser.exit(1, "native verified-mirror scan failed; no scan approval\n")


if __name__ == "__main__":
    main()
