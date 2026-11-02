#!/usr/bin/env python3
"""Local SHA-approved utility ZIP intake only; never downloads or admits a baseline."""
import argparse, hashlib, json, os, re, signal, stat
import struct, threading, unicodedata, zipfile, zlib
from pathlib import Path

CHUNK = 1 << 20
MAX_ZIP, MAX_OUTPUT, MAX_MEMBER = 256 << 20, 512 << 20, 256 << 20
MAX_MEMBERS, MAX_PATH, MAX_PARTS, MAX_MANIFEST = 10000, 1024, 32, 16 << 20
FLOOR, MARGIN = (1 << 30) + (128 << 20), 32 << 20
SHA = re.compile(r"[a-f0-9]{64}\Z")

class Refusal(Exception):
    pass
def need(ok, code):
    if not ok:
        raise Refusal(code)
def unique(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result, "duplicate-json-key")
        result[key] = value
    return result
def safe_name(name, directory=False):
    need(isinstance(name, str) and name and "\0" not in name and "\\" not in name, "member-name")
    need(len(name.encode("utf-8")) <= MAX_PATH and name.endswith("/") == directory, "member-path-bound")
    parts = (name[:-1] if directory else name).split("/")
    need(len(parts) <= MAX_PARTS and all(p not in ("", ".", "..") and ":" not in p for p in parts), "member-path")
    for part in parts:
        folded = unicodedata.normalize("NFC", part).casefold()
        need(not (folded == ".env" or folded.startswith(".env.") or folded.endswith(".kubeconfig") or folded in {
            ".docker", "docker_config", "dockerconfig.json", ".dockerconfigjson", ".kube", "kubeconfig",
            "render-env", "credentials", "credentials.json", "token", "token.json", "htpasswd",
            ".npmrc", ".pypirc", ".git-credentials", "id_rsa", "id_ed25519"}), "credential-scope")
    need(not any(p.casefold() == "manifest.json" for p in parts) or name == "MANIFEST.json", "nested-manifest")
    return parts
def regular_parents(path):
    for parent in reversed(path.parents):
        need(stat.S_ISDIR(parent.lstat().st_mode), "parent-not-directory")
def extra_fields(raw):
    while raw:
        need(len(raw) >= 4, "extra-header")
        tag, size = struct.unpack("<HH", raw[:4])
        field, raw = raw[4:4 + size], raw[4 + size:]
        need(len(field) == size and tag in (1, 0x5455, 0x7875), "unknown-extra-metadata")
        if tag == 1:
            need(size in (8, 16, 24, 28), "zip64-extra")
        elif tag == 0x5455:
            need(size >= 1 and field[0] & ~7 == 0 and size == 1 + 4 * bin(field[0]).count('1'), "timestamp-extra")
        else:
            need(size >= 5 and field[0] == 1, "unix-id-extra")
            uid = field[1]
            need(1 <= uid <= 8 and size > 2 + uid, "unix-id-extra")
            gid = field[2 + uid]
            need(1 <= gid <= 8 and size == 3 + uid + gid, "unix-id-extra")
def free_bytes(path):
    fs = os.statvfs(path)
    return fs.f_bavail * fs.f_frsize
class Budget:
    def __init__(self, artifact, cancelled):
        self.artifact, self.cancelled = artifact, cancelled
        self.stop, self.error, self.written = threading.Event(), None, 0
        self.minimum_free, self.peak_allocated, self.actual_file_bytes = None, 0, 0
        self.thread = threading.Thread(target=self.observe, name="owned-utility-zip-observer", daemon=True)
    def check(self, next_bytes=0, inventory=False):
        need(not self.cancelled[0] and self.error is None, "cancel-or-observer-failure")
        available = free_bytes(self.artifact)
        self.minimum_free = available if self.minimum_free is None else min(self.minimum_free, available)
        need(available > FLOOR + next_bytes, "free-space-bound")
        need(self.written + next_bytes <= MAX_OUTPUT, "observed-output-bound")
        if inventory:
            allocated, logical = 0, 0
            for base, dirs, files in os.walk(self.artifact, followlinks=False):
                for name in dirs + files:
                    st = (Path(base) / name).lstat()
                    need(stat.S_ISDIR(st.st_mode) or stat.S_ISREG(st.st_mode), "owned-nonregular")
                    allocated += st.st_blocks * 512
                    logical += st.st_size if stat.S_ISREG(st.st_mode) else 0
            self.peak_allocated = max(self.peak_allocated, allocated)
            self.actual_file_bytes = logical
            need(logical <= MAX_OUTPUT, "actual-output-bound")
            need(allocated <= MAX_OUTPUT + MARGIN, "allocated-output-bound")
    def observe(self):
        try:
            while not self.stop.wait(1):
                self.check(inventory=True)
        except BaseException as error:
            self.error = error.args[0] if isinstance(error, Refusal) else type(error).__name__
    def close(self):
        self.stop.set()
        if self.thread.ident is not None:
            self.thread.join(2)
        need(not self.thread.is_alive(), "observer-not-joined")
def extract(archive, expected_size, expected_sha256, output):
    archive, output = Path(archive), Path(output)
    need(type(expected_size) is int and 0 < expected_size <= MAX_ZIP, "zip-size-bound")
    need(isinstance(expected_sha256, str) and SHA.fullmatch(expected_sha256), "zip-sha-shape")
    need(archive.is_absolute() and output.is_absolute() and str(output).startswith("/private/tmp/"), "private-path")
    regular_parents(archive)
    regular_parents(output)
    need(stat.S_ISREG(archive.lstat().st_mode) and archive.stat().st_size == expected_size, "zip-not-regular-or-size")
    output.mkdir(mode=0o700)
    artifact = output / "artifact"
    artifact.mkdir(mode=0o700)
    state = {"complete": False, "zip_bytes": expected_size, "zip_sha256": expected_sha256,
             "completed_members": 0, "member": None, "observed_member_bytes": 0}
    cancelled, previous = [False], {}
    budget = Budget(artifact, cancelled)
    def record():
        temporary = output / "extraction.json.partial"
        temporary.write_text(json.dumps(state, indent=2) + "\n")
        temporary.chmod(0o600)
        temporary.replace(output / "extraction.json")
    try:
        record()
        for signum in (signal.SIGINT, signal.SIGTERM):
            previous[signum] = signal.getsignal(signum)
            signal.signal(signum, lambda *_: cancelled.__setitem__(0, True))
        budget.thread.start()
        budget.check()
        with os.fdopen(os.open(archive, os.O_RDONLY | os.O_NOFOLLOW), "rb") as raw:
            identity = os.fstat(raw.fileno())
            need(stat.S_ISREG(identity.st_mode) and identity.st_size == expected_size, "opened-zip-identity")
            digest = hashlib.sha256()
            for chunk in iter(lambda: raw.read(CHUNK), b""):
                budget.check()
                digest.update(chunk)
            need(digest.hexdigest() == expected_sha256, "zip-sha-mismatch")
            raw.seek(0)
            with zipfile.ZipFile(raw) as zipped:
                infos, nodes, seen, declared = zipped.infolist(), {}, set(), 0
                need(0 < len(infos) <= MAX_MEMBERS, "member-count")
                for info in infos:
                    need(info.orig_filename == info.filename, "original-name-differs")
                    parts = safe_name(info.filename, info.is_dir())
                    need(info.filename not in seen, "duplicate-member")
                    seen.add(info.filename)
                    need(info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED) and not info.flag_bits & ~0x80E, "zip-method-or-flags")
                    need(type(info.file_size) is int and 0 <= info.file_size <= MAX_MEMBER and type(info.compress_size) is int and 0 <= info.compress_size <= MAX_ZIP, "member-size")
                    kind = stat.S_IFMT(info.external_attr >> 16)
                    need(kind in (0, stat.S_IFDIR if info.is_dir() else stat.S_IFREG), "special-member")
                    need(not info.external_attr & 0xFF48 and (not info.external_attr & 0x10 or info.is_dir()), "special-dos-metadata")
                    need(info.create_system in (0, 3), "unknown-member-system")
                    extra_fields(info.extra)
                    for index in range(1, len(parts) + 1):
                        name = "/".join(parts[:index])
                        key = unicodedata.normalize("NFC", name).casefold()
                        is_dir = index < len(parts) or info.is_dir()
                        need(key not in nodes or nodes[key] == (name, is_dir), "path-collision")
                        nodes[key] = (name, is_dir)
                    need(not info.is_dir() or info.file_size == 0, "directory-payload")
                    declared += info.file_size
                need(declared <= MAX_OUTPUT and free_bytes(artifact) >= FLOOR + MARGIN + declared, "declared-output-fit")
                manifests = [i for i in infos if i.filename == "MANIFEST.json" and not i.is_dir()]
                need(len(manifests) == 1 and manifests[0].file_size <= MAX_MANIFEST, "root-manifest")
                with zipped.open(manifests[0]) as member:
                    content = bytearray()
                    for chunk in iter(lambda: member.read(CHUNK), b""):
                        budget.check()
                        content.extend(chunk)
                        need(len(content) <= MAX_MANIFEST, "manifest-bound")
                manifest = json.loads(content, object_pairs_hook=unique)
                need(isinstance(manifest, dict) and isinstance(manifest.get("files"), list), "manifest-shape")
                rows = {}
                for row in manifest["files"]:
                    need(isinstance(row, dict) and set(row) == {"path", "bytes", "sha256"}, "manifest-row")
                    safe_name(row["path"])
                    need(row["path"] != "MANIFEST.json" and row["path"] not in rows and type(row["bytes"]) is int and 0 <= row["bytes"] <= MAX_MEMBER and isinstance(row["sha256"], str) and SHA.fullmatch(row["sha256"]), "manifest-row-identity")
                    rows[row["path"]] = row
                need(set(rows) == {i.filename for i in infos if not i.is_dir()} - {"MANIFEST.json"}, "manifest-bijection")
                need(all(rows[i.filename]["bytes"] == i.file_size for i in infos if i.filename in rows), "manifest-declared-size")
                state.update(declared_bytes=declared, manifest_sha256=hashlib.sha256(content).hexdigest())
                for info in infos:
                    target = artifact / info.filename
                    parent = artifact
                    need(stat.S_ISDIR(parent.lstat().st_mode), "parent-not-directory")
                    for part in safe_name(info.filename, info.is_dir())[:-1]:
                        parent /= part
                        try:
                            parent.mkdir(mode=0o700)
                        except FileExistsError:
                            pass
                        need(stat.S_ISDIR(parent.lstat().st_mode), "parent-not-directory")
                    if info.is_dir():
                        with zipped.open(info) as member:
                            need(member.read(CHUNK) == b"" and info.CRC == 0, "directory-payload")
                        target.mkdir(mode=0o700, exist_ok=True)
                        continue
                    state.update(member=info.filename, observed_member_bytes=0)
                    record()
                    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW
                    with os.fdopen(os.open(target, flags, 0o600), "wb", buffering=0) as destination, zipped.open(info) as member:
                        digest, crc = hashlib.sha256(), 0
                        for chunk in iter(lambda: member.read(CHUNK), b""):
                            need(state["observed_member_bytes"] + len(chunk) <= info.file_size, "member-overlong")
                            budget.check(len(chunk))
                            written = destination.write(chunk)
                            budget.written += written
                            state["observed_member_bytes"] += written
                            need(written == len(chunk), "short-write")
                            digest.update(chunk)
                            crc = zlib.crc32(chunk, crc)
                            budget.check()
                    need(state["observed_member_bytes"] == info.file_size and crc & 0xFFFFFFFF == info.CRC, "member-size-or-crc")
                    need(info.filename == "MANIFEST.json" or digest.hexdigest() == rows[info.filename]["sha256"], "member-sha")
                    state["completed_members"] += 1
                    record()
                after = os.fstat(raw.fileno())
                need((after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns) == (identity.st_dev, identity.st_ino, identity.st_size, identity.st_mtime_ns), "zip-changed-during-read")
        budget.close()
        budget.check(inventory=True)
        state.update(complete=True, member=None)
    except BaseException as error:
        state.update(complete=False, failure=error.args[0] if isinstance(error, Refusal) else type(error).__name__)
        raise
    finally:
        try:
            try:
                budget.close()
                budget.check(inventory=True)
            except BaseException as error:
                state.update(complete=False, final_failure=error.args[0] if isinstance(error, Refusal) else type(error).__name__)
                raise
            finally:
                if cancelled[0] or budget.error:
                    state.update(complete=False, observer_error=budget.error, cancelled=cancelled[0])
                state.update(observed_output_bytes=budget.written, actual_file_bytes=budget.actual_file_bytes, minimum_free=budget.minimum_free, peak_allocated=budget.peak_allocated)
                record()
        finally:
            restore_errors = []
            for signum, handler in reversed(list(previous.items())):
                try:
                    signal.signal(signum, handler)
                except BaseException as error:
                    restore_errors.append(type(error).__name__)
            if cancelled[0] or budget.error:
                state.update(complete=False, cancelled=cancelled[0], observer_error=budget.error)
                record()
            if restore_errors:
                state.update(complete=False, restore_errors=restore_errors)
                record()
                raise Refusal("signal-restore-failure")
    need(state["complete"], "late-observer-failure")
    return state

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("zip")
    parser.add_argument("bytes", type=int)
    parser.add_argument("sha256")
    parser.add_argument("output")
    args = parser.parse_args()
    extract(args.zip, args.bytes, args.sha256, args.output)
