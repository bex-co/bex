#!/usr/bin/env python3
"""Temporary m114 proof: restored backup versus read-only live SQL/OpenBao.

Original unseal/root credentials go only to network-none Docker. Live reads use
exactly the private independent auditor's Pod-bound bex-api Kubernetes login.
No values, names, map keys, hashes, credentials or raw errors leave private scratch.
"""
import concurrent.futures
import contextlib
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import signal
import tarfile
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

EXPECTED_KEY = "openbao-snapshots/openbao-20261018-072623.snap.gz.age"
EXPECTED_CIPHERTEXT_SHA256 = "140f42f8aace422c7a398cc27f1e68f35cb701d4b815dfa90b6a03dc1bf12e11"
EXPECTED_OWNERS = {
    "evg-d9qtbdluqtoc73bv1d40": "tea-d9qt8kluqtoc73bv1chg",
    "evg-db0s94c48ccs739jikr0": "tea-d98210cbbpdc73dcrkvg",
}
IMAGE = "quay.io/openbao/openbao:2.5.5@sha256:6150c4a6b62067db6141c8da7a6a6b5763f4f47c315343d0c848b40fecdfd452"
JOB = "openbao-backup-m114-072612"
DEADLINE = time.monotonic() + 12 * 60
# Never forward original restore credentials to kubectl, docker exec or child tools.
TOOL_ENV = {k: v for k, v in os.environ.items() if not re.search(r"TOKEN|SECRET|PASSWORD|KEY|^AGE_|^AWS_|^TF_STATE_", k)}
KUBE = ["kubectl", "--request-timeout=25s"]


def private_json(path, value):
    path = Path(path)
    temporary = path.with_name(path.name + ".writing")
    try:
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(fd, "w") as out:
            json.dump(value, out, sort_keys=True)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)


def record(**fields):
    if globals().get("TERMINATED"):
        fields["terminated"] = True
    path = os.environ.get("M114_REPORT")
    if path:
        current = json.loads(Path(path).read_text()) if Path(path).exists() else {}
        private_json(path, {**current, **fields})


class AuditTerminated(Exception):
    pass


TERMINATED = False
CLEANING = 0


def terminate(signum, frame):
    global TERMINATED
    TERMINATED = True
    if not CLEANING:
        raise AuditTerminated()


@contextlib.contextmanager
def cleaning():
    global CLEANING
    CLEANING += 1
    try:
        yield
    finally:
        CLEANING -= 1
        if not CLEANING and TERMINATED:
            raise AuditTerminated()


def remove_container(name):
    # Inspect must explicitly report absence. A daemon/transport failure is not
    # evidence that the secret-bearing disposable layer has been destroyed.
    try:
        subprocess.run(["docker", "rm", "-f", "-v", name], capture_output=True, timeout=25, env=TOOL_ENV)
    except subprocess.TimeoutExpired:
        pass
    inspected = subprocess.run(["docker", "container", "inspect", name], capture_output=True, timeout=8, env=TOOL_ENV)
    error = inspected.stderr.lower()
    removed = inspected.returncode != 0 and (b"no such container" in error or b"no such object" in error)
    record(localContainerRemoved=removed)
    if not removed:
        raise RuntimeError("local container removal unverified")


def run(args, data=None, timeout=35, check=True):
    if time.monotonic() > DEADLINE:
        raise RuntimeError("audit time budget exceeded")
    result = subprocess.run(args, input=data, capture_output=True, env=TOOL_ENV, timeout=timeout)
    if check and result.returncode:
        raise RuntimeError("private subprocess failed")
    return result


def kube(*args):
    return run([*KUBE, *args]).stdout.decode().strip()


def database():
    primary = kube("-n", "bex-system", "get", "cluster.postgresql.cnpg.io", "bex-db", "-o", "jsonpath={.status.currentPrimary}")
    sql = """BEGIN READ ONLY; SET LOCAL statement_timeout='20s';
    SELECT jsonb_build_object(
      'tenants',(SELECT coalesce(jsonb_agg(id ORDER BY id),'[]') FROM tenants),
      'groups',(SELECT coalesce(jsonb_agg(to_jsonb(g) ORDER BY g.id),'[]') FROM env_groups g),
      'environments',(SELECT coalesce(jsonb_agg(jsonb_build_object('id',id,'tenant',tenant_id) ORDER BY id),'[]') FROM environments)); COMMIT;"""
    return json.loads(kube("-n", "bex-system", "exec", primary, "-c", "postgres", "--", "psql", "-X", "-q", "-A", "-t", "-U", "postgres", "-d", "bex", "-v", "ON_ERROR_STOP=1", "-c", sql))


class LiveBao:
    """Same Pod-bound auth and revoke-self lifecycle as independent auditor."""
    def __enter__(self):
        self.token = None
        self.proc = None
        self.http = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        try:
            pods = json.loads(kube("-n", "bex-system", "get", "pods", "-l", "app.kubernetes.io/name=bex-api", "-o", "json"))["items"]
            pod = next(p for p in pods if p["status"].get("phase") == "Running" and any(c["type"] == "Ready" and c["status"] == "True" for c in p["status"].get("conditions", [])))
            jwt = kube("-n", "bex-system", "create", "token", "bex-api", "--duration=10m", "--bound-object-kind=Pod", "--bound-object-name=" + pod["metadata"]["name"], "--bound-object-uid=" + pod["metadata"]["uid"])
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                port = sock.getsockname()[1]
            self.url = "http://127.0.0.1:%d" % port
            self.proc = subprocess.Popen([*KUBE, "-n", "secrets", "port-forward", "--address=127.0.0.1", "service/openbao", "%d:8200" % port], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, env=TOOL_ENV)
            for _ in range(100):
                if self.proc.poll() is not None:
                    raise RuntimeError("private port-forward failed")
                try:
                    with socket.create_connection(("127.0.0.1", port), timeout=.2):
                        break
                except OSError:
                    time.sleep(.2)
            else:
                raise RuntimeError("private port-forward timeout")
            self.token = self.request("auth/kubernetes/login", {"role": "bex-api", "jwt": jwt})["auth"]["client_token"]
            del jwt
            return self
        except Exception:
            self.__exit__()
            raise

    def __exit__(self, *exc):
        with cleaning():
            revoked = self.token is None
            stopped = self.proc is None
            try:
                if self.token:
                    try:
                        self.request("auth/token/revoke-self", {}, timeout=10)
                        revoked = True
                    except Exception:
                        pass
            finally:
                self.token = None
                if self.proc:
                    if self.proc.poll() is None:
                        self.proc.terminate()
                    with contextlib.suppress(subprocess.TimeoutExpired):
                        self.proc.wait(timeout=5)
                    if self.proc.poll() is None:
                        self.proc.kill()
                        self.proc.wait(timeout=5)
                    stopped = self.proc.poll() is not None
                record(liveTokenRevoked=revoked, liveForwardStopped=stopped)
            if not revoked or not stopped:
                raise RuntimeError("live audit credential cleanup failed")

    def request(self, path, payload=None, timeout=20):
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["X-Vault-Token"] = self.token
        data = None if payload is None else json.dumps(payload).encode()
        req = urllib.request.Request(self.url + "/v1/" + path, data=data, headers=headers)
        try:
            with self.http.open(req, timeout=timeout) as response:
                body = response.read()
                return json.loads(body) if body else {}
        except urllib.error.HTTPError as error:
            if error.code == 404:
                return {}
            raise RuntimeError("private OpenBao read failed") from None

    def metadata(self, tenant, path):
        return self.request("tenants/data/" + urllib.parse.quote(tenant + "/" + path, safe="/")).get("data", {}).get("data", {})


class LocalBao:
    def __init__(self, scratch):
        self.scratch = scratch
        self.name = os.environ["M114_CONTAINER"]

    def copy_file(self, source, destination):
        if destination not in {"m114-config.hcl", "m114-snapshot.snap", "m114-root.token"}:
            raise RuntimeError("unexpected private copy destination")
        # Docker cp can preserve client UID even without -a (Docker Desktop).
        # Explicit root ownership in a private archive keeps 0600 files readable
        # with ALL capabilities dropped, including with a separate daemon.
        archive = self.scratch / "docker-copy-private.tar"
        try:
            with tarfile.open(archive, "w") as tar:
                info = tar.gettarinfo(str(source), arcname=destination)
                info.uid, info.gid, info.uname, info.gname, info.mode = 0, 0, "", "", 0o600
                with source.open("rb") as content:
                    tar.addfile(info, content)
            with archive.open("rb") as content:
                result = subprocess.run(["docker", "cp", "-a", "-", self.name + ":/tmp"], stdin=content, capture_output=True, timeout=60, env=TOOL_ENV)
            if result.returncode:
                raise RuntimeError("private Docker copy failed")
        finally:
            archive.unlink(missing_ok=True)

    def set_root(self, token):
        source = self.scratch / "local-root.token"
        source.write_text(token)
        self.copy_file(source, "m114-root.token")
        source.unlink()

    def cli(self, *args, data=None, allow_absent=False):
        # Root exists only in a 0600 file in isolated Docker and process memory;
        # its value never appears in argv, Docker env configuration or output.
        result = run(["docker", "exec", "-i", self.name, "sh", "-ec", 'export BAO_TOKEN="$(cat /tmp/m114-root.token)"; exec bao "$@"', "sh", *args], data, check=False)
        if allow_absent and result.returncode and (b"No value found at" in result.stderr or (result.returncode == 2 and not result.stderr and result.stdout.strip() in (b"{}", b"null"))):
            return {}
        if result.returncode:
            raise RuntimeError("isolated OpenBao command failed")
        return json.loads(result.stdout) if result.stdout and "-format=json" in args else {}

    def create_server(self):
        config = self.scratch / "config.hcl"
        config.write_text('''ui=false
 disable_mlock=true
 api_addr="http://127.0.0.1:8200"
 cluster_addr="https://127.0.0.1:8201"
 listener "tcp" { address="127.0.0.1:8200" cluster_address="127.0.0.1:8201" tls_disable=1 }
 storage "raft" { path="/tmp/m114-data" node_id="local-m114-proof" }
''')
        run(["docker", "pull", IMAGE], timeout=180)
        # All state stays in this disposable writable layer and survives only
        # this container's restart. There are no host mounts or published ports.
        run(["docker", "create", "--name", self.name, "--label", "bex.co/m114-preservation=" + os.environ.get("GITHUB_RUN_ID", "local"), "--network", "none", "--user", "0:0", "--cap-drop", "ALL", "--memory", "512m", "--cpus", "1", "--pids-limit", "128", "-e", "BAO_ADDR=http://127.0.0.1:8200", "--entrypoint", "sh", IMAGE, "-ec", "umask 077; mkdir -p /tmp/m114-data; exec bao server -config=/tmp/m114-config.hcl"])
        self.copy_file(config, "m114-config.hcl")
        run(["docker", "start", self.name])

    def start_restore(self):
        self.create_server()
        self.copy_file(self.scratch / "snapshot.snap", "m114-snapshot.snap")
        initialized = None
        for _ in range(40):
            result = run(["docker", "exec", self.name, "bao", "operator", "init", "-key-shares=1", "-key-threshold=1", "-format=json"], check=False)
            if result.returncode == 0:
                initialized = json.loads(result.stdout)
                break
            time.sleep(.25)
        if initialized is None:
            raise RuntimeError("local initialization failed")
        self.set_root(initialized["root_token"])
        self.cli("write", "-format=json", "sys/unseal", "key=-", data=initialized["unseal_keys_b64"][0].encode())
        del initialized
        self.cli("operator", "raft", "snapshot", "restore", "-force", "/tmp/m114-snapshot.snap")
        run(["docker", "restart", self.name])
        self.set_root(os.environ["BAO_ROOT_TOKEN"])
        for _ in range(40):
            status = run(["docker", "exec", self.name, "bao", "status", "-format=json"], check=False)
            if status.stdout:
                break
            time.sleep(.25)
        for number in range(1, 4):
            status = self.cli("write", "-format=json", "sys/unseal", "key=-", data=os.environ["BAO_UNSEAL_KEY_%d" % number].encode())
        if status.get("data", {}).get("sealed", True):
            raise RuntimeError("restored snapshot did not unseal")
        ins = json.loads(run(["docker", "inspect", self.name]).stdout)[0]
        if ins["HostConfig"]["NetworkMode"] != "none" or ins["HostConfig"].get("PortBindings") or ins["HostConfig"].get("Binds") or ins["Config"]["User"] != "0:0":
            raise RuntimeError("local isolation check failed")

    def keys(self, tenant=None, path=""):
        logical = "" if tenant is None else tenant + "/" + path
        result = self.cli("list", "-format=json", "tenants/metadata/" + logical, allow_absent=True)
        return result if isinstance(result, list) else result.get("data", {}).get("keys", [])

    def metadata(self, tenant, path):
        return self.cli("read", "-format=json", "tenants/data/" + tenant + "/" + path, allow_absent=True).get("data", {}).get("data", {})

    def close(self):
        with cleaning():
            remove_container(self.name)


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()).digest()


def timestamp(value):
    # PGX writes timestamps at microsecond precision; source RFC3339Nano may
    # have more digits. Python truncates them as the PG encoder does.
    value = re.sub(r"(\.\d{6})\d+", r"\1", value)
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(dt.timezone.utc)


def inspect_source(bao, tenants):
    prefixes = sorted(set(tenants) | {"default"} | {key.rstrip("/") for key in bao.keys() if key.endswith("/")})
    listed = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        for tenant, keys in zip(prefixes, pool.map(lambda t: bao.keys(t, "env-groups"), prefixes)):
            for key in keys:
                listed.setdefault(key.rstrip("/"), []).append(tenant)
    source = {}
    ambiguous = 0
    for gid, owners in sorted(listed.items()):
        editable = lambda m: bool(m.get("name") and m.get("locator") != "1" and m.get("tombstoned") != "1")
        copies = [(tenant, bao.metadata(tenant, "env-groups/" + gid + "/meta")) for tenant in owners if tenant != "default"]
        copies = [(tenant, meta) for tenant, meta in copies if editable(meta)]
        ambiguous += len(copies) > 1
        if copies:
            owner, meta = copies[0]
        else:
            meta = bao.metadata("default", "env-groups/" + gid + "/meta")
            if not editable(meta):
                continue
            owner = meta.get("workspace") or "default"
        maps = {}
        for leaf in ("env", "files", "revision"):
            path = "env-groups/" + gid + "/" + leaf
            maps[leaf] = bao.metadata(owner, path)
            if not maps[leaf] and owner != "default":
                maps[leaf] = bao.metadata("default", path)
        source[gid] = {"owner": owner, "meta": meta, **maps}
    return source, len(prefixes), len(listed), ambiguous


def resolved_links(gid, owner, meta, apps):
    links = list(dict.fromkeys(s.strip() for s in meta.get("links", "").split(",") if s.strip()))
    if meta.get("linksByID") == "1" or not links:
        return links, 0
    mounting = [a for a in apps if a["metadata"].get("labels", {}).get("bex.co/tenant") == owner and (gid + "-env" in a.get("spec", {}).get("envFromSecrets", []) or gid + "-files" in a.get("spec", {}).get("filesFromSecrets", []))]
    out, unresolved = [], 0
    for link in links:
        if link.startswith("srv-"):
            if link not in out:
                out.append(link)
            continue
        found = [a for a in mounting if link in (a["metadata"].get("labels", {}).get("bex.co/app-id"), a["metadata"]["name"], a["metadata"].get("labels", {}).get("bex.co/service-name"), a.get("spec", {}).get("displayName"))]
        if len(found) != 1:
            unresolved += 1
        else:
            resolved = found[0]["metadata"].get("labels", {}).get("bex.co/app-id") or found[0]["metadata"]["name"]
            if resolved not in out:
                out.append(resolved)
    for app in mounting:
        public_id = app["metadata"].get("labels", {}).get("bex.co/app-id") or app["metadata"]["name"]
        if public_id not in out:
            out.append(public_id)
    return out, unresolved


def compare_groups(source, rows, apps, live):
    owners = {gid: group["owner"] for gid, group in source.items()}
    owner_equal = owners == {gid: row["tenant_id"] for gid, row in rows.items()} == EXPECTED_OWNERS
    metadata_equal = values_equal = owner_equal
    env_count = file_count = unresolved = 0
    for gid, src in source.items():
        row = rows.get(gid)
        if row is None:
            metadata_equal = values_equal = False
            continue
        meta = src["meta"]
        links, missing = resolved_links(gid, src["owner"], meta, apps)
        unresolved += missing
        checks = [
            meta["name"] == row["name"],
            (meta.get("environment") or None) == row["environment_id"],
            links == row["service_links"],
            timestamp(meta["createdAt"]) == timestamp(row["created_at"]),
            timestamp(meta.get("updatedAt") or meta["createdAt"]) == timestamp(row["updated_at"]),
            sorted(src["env"]) == sorted(row["env_keys"]),
            sorted(src["files"]) == sorted(row["file_names"]),
            src["revision"].get("state", "idle") in ("", "idle"),
            row["content_state"] == "idle", row["op_id"] is None,
            max(int(src["revision"].get("generation", "1") or "1"), 1) == row["content_generation"],
        ]
        metadata_equal = metadata_equal and all(checks) and missing == 0
        for leaf in ("env", "files"):
            current = live.metadata(src["owner"], "env-groups/" + gid + "/" + leaf)
            values_equal = values_equal and digest(current) == digest(src[leaf])
        env_count += len(src["env"])
        file_count += len(src["files"])
    return {"metadataEqual": bool(metadata_equal), "valuesEqual": bool(values_equal), "envValueCount": env_count, "secretFileCount": file_count, "unresolvedLinkCount": unresolved}


def verify(scratch, report_path):
    if report_path != Path(os.environ["M114_REPORT"]):
        raise RuntimeError("report path differs from this run")
    before = database()
    job = json.loads(kube("-n", "secrets", "get", "job", JOB, "-o", "json"))
    obj = json.loads((scratch / "object.json").read_text())
    provenance = job["status"].get("succeeded") == 1 and timestamp(job["status"]["startTime"]) <= timestamp(obj["LastModified"]) <= timestamp(job["status"]["completionTime"]) and obj["ContentLength"] > 0
    bao = LocalBao(scratch)
    try:
        bao.start_restore()
        record(snapshotRestored=True)
        source, prefixes, listed, ambiguous = inspect_source(bao, before["tenants"])
        record(sourceScanned=True, snapshotLiveGroups=len(source), snapshotListedIDs=listed)
        rows = {group["id"]: group for group in before["groups"]}
        needs_apps = any(group["meta"].get("links") and group["meta"].get("linksByID") != "1" for group in source.values())
        apps = json.loads(kube("get", "apps.app.bex.co", "-A", "-o", "json"))["items"] if needs_apps else []
        with LiveBao() as live:
            comparison = compare_groups(source, rows, apps, live)
        unchanged = before == database()
        report = {**comparison, "metadataEqual": bool(comparison["metadataEqual"] and unchanged and provenance and ambiguous == 0), "valuesEqual": bool(comparison["valuesEqual"] and unchanged), "comparisonCompleted": True, "completed": False, "backupProvenanceEqual": provenance, "sqlStable": unchanged, "snapshotPrefixes": prefixes, "snapshotListedIDs": listed, "snapshotLiveGroups": len(source), "sqlGroups": len(rows), "ambiguousOwnerCount": ambiguous, "groups": [{"id": gid, "owner": group["owner"]} for gid, group in sorted(source.items())]}
        record(**report)
        return report["metadataEqual"] and report["valuesEqual"]
    finally:
        bao.close()


def main():
    os.umask(0o077)
    if sys.argv[1] == "--validate-uri":
        uri, bucket = sys.argv[2:]
        if uri != "s3://" + bucket + "/" + EXPECTED_KEY:
            raise RuntimeError("snapshot URI differs from reviewed object")
        return
    if sys.argv[1] == "--validate-ciphertext":
        hasher = hashlib.sha256()
        with Path(sys.argv[2]).open("rb") as source:
            for chunk in iter(lambda: source.read(1024 * 1024), b""):
                hasher.update(chunk)
        if hasher.hexdigest() != EXPECTED_CIPHERTEXT_SHA256:
            raise RuntimeError("snapshot ciphertext changed")
        record(snapshotCiphertextEqual=True)
        return
    if sys.argv[1] == "--cleanup-container":
        with cleaning():
            remove_container(sys.argv[2])
        return
    if sys.argv[1] == "--finalize-report":
        status, removed = int(sys.argv[2]), sys.argv[3] == "true"
        path = Path(os.environ["M114_REPORT"])
        report = json.loads(path.read_text())
        report["plaintextCleanupComplete"] = removed
        report["completed"] = status == 0 and not report.get("terminated") and all(report.get(key) for key in ("comparisonCompleted", "liveTokenRevoked", "liveForwardStopped", "localContainerRemoved", "plaintextCleanupComplete"))
        private_json(path, report)
        if not (report["completed"] and report["metadataEqual"] and report["valuesEqual"]):
            raise RuntimeError("audit did not finish successfully")
        return
    scratch, report_path = map(Path, sys.argv[1:])
    if not verify(scratch, report_path):
        raise RuntimeError("preservation comparison failed")


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, terminate)
    signal.signal(signal.SIGINT, terminate)
    try:
        main()
    except Exception:
        # No exception traceback may accidentally disclose data or credentials.
        sys.exit(1)
