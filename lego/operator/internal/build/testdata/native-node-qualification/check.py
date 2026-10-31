"""Separate common Node family gate; original 13/28 fixtures are untouched."""
import hashlib
import gzip
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parent
OUT = ROOT / "qa-output"
PAYLOAD = "bex Node café λ 雪".encode()
ENGINE_SENTINEL = "bex-node-engine-sentinel"


def validate_identity(proof, expected):
    assert proof["node"] == expected["node"], "selected app Node changed"
    assert proof["executable"] == proof["path_node"] == "/usr/local/bin/node"
    assert proof["node_env"] == expected["node_env"], "canonical NODE_VERSION changed"
    assert "/usr/local/bin" in proof["path"].split(":")
    assert proof["addon"]["headers"] == expected["node"], "wrong selected headers"
    assert proof["addon"]["module_abi"] == proof["modules"]
    assert proof["addon"]["answer"] == "native Grüße λ 雪"


def validate_core(proof):
    assert proof["openssl"] and proof["brotli"] and proof["napi"]
    assert proof["sha256"] == hashlib.sha256(PAYLOAD).hexdigest()
    assert all(proof[key] is True for key in (
        "brotli_roundtrip", "gzip_roundtrip", "aes_roundtrip", "bad_tag_rejected"))
    assert proof["roots"] > 0


def validate_tools(proof, versions, expected):
    assert proof["npm_package"]["name"] == "npm"
    assert proof["npm_package"]["version"] == versions["npm"]
    assert len(proof["npm_package_sha256"]) == 64
    assert proof["npm_cli"] == "/usr/local/lib/node_modules/npm/bin/npm-cli.js"
    assert proof["yarn_package"]["name"] == "yarn"
    assert proof["yarn_package"]["version"] == versions["yarn"] == expected["yarn"]
    assert proof["yarn_cli"].endswith("/yarn-v" + expected["yarn"] + "/bin/yarn.js")
    assert proof["pnpm_package"]["name"] == "pnpm"
    assert proof["pnpm_package"]["version"] == versions["pnpm"] == expected["pnpm"]


def validate_lifecycle(proof, expected, manager, event):
    validate_identity(proof, expected)
    validate_core(proof)
    assert proof["manager"] == manager and proof["lifecycle"] == event
    for key in ("NODE", "npm_node_execpath"):
        assert proof[key] is not None and os.path.realpath(proof[key]) == "/usr/local/bin/node", key
    assert proof["npm_execpath"], "missing actual package-manager entrypoint"


def command(name, args, cwd=ROOT, success=True):
    timed_out = False
    try:
        result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=90)
    except subprocess.TimeoutExpired as error:
        timed_out = True
        def text(value):
            return value.decode(errors="replace") if isinstance(value, bytes) else value or ""
        result = subprocess.CompletedProcess(args, None, text(error.stdout), text(error.stderr))
    # Keep the real command status/output before any assertion, including the
    # established pnpm14/16 startup error. A failed build is never called pass.
    record = {"command": args, "cwd": str(cwd.relative_to(ROOT)),
              "returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr, "timed_out": timed_out}
    (OUT / (name + ".json")).write_text(json.dumps(record, indent=2) + "\n")
    print(json.dumps({"command_result": name, **record}), flush=True)
    assert not timed_out, name + " timed out"
    if success:
        assert result.returncode == 0, name + " failed"
    else:
        assert result.returncode != 0, name + " unexpectedly accepted incompatible engines"
    return result


def proofs(output):
    return [json.loads(line[len("BEX_NODE_PROOF="):])
            for line in output.splitlines() if line.startswith("BEX_NODE_PROOF=")]


def node_proof(name, mode):
    result = command(name, ["node", str(ROOT / "check.js"), mode])
    actual = proofs(result.stdout)
    assert len(actual) == 1
    return actual[0]


def project(manager, expected, wrong=False):
    work = OUT / (manager + ("-wrong-engine" if wrong else "-project"))
    work.mkdir(exist_ok=True)
    major = int(expected["node"].split(".")[0])
    manifest = {"name": "bex-profile-" + manager, "version": "1.0.0", "private": True,
                "engines": {"node": str(major) + ".x"},
                "scripts": {"preinstall": "node ../../check.js lifecycle " + manager,
                            "postinstall": "node ../../check.js lifecycle " + manager,
                            "probe": "node ../../check.js script " + manager}}
    if manager == "pnpm":
        manifest["packageManager"] = "pnpm@" + expected["pnpm"]
    if wrong:
        # npm6's no-args install does not reliably reject the root's engines.
        # Reach the actual dependency check with a tiny offline local tarball;
        # the positive project above remains entirely dependency-free.
        required = ">=" + str(major + 1)
        dependency = {"name": ENGINE_SENTINEL, "version": "1.0.0", "engines": {"node": required}}
        data = (json.dumps(dependency, sort_keys=True) + "\n").encode()
        buffer = io.BytesIO()
        with gzip.GzipFile(filename="", fileobj=buffer, mode="wb", mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
                member = tarfile.TarInfo("package/package.json")
                member.size, member.mode, member.mtime = len(data), 0o644, 0
                archive.addfile(member, io.BytesIO(data))
        body = buffer.getvalue()
        (work / (ENGINE_SENTINEL + ".tgz")).write_bytes(body)
        manifest["dependencies"] = {ENGINE_SENTINEL: "file:./" + ENGINE_SENTINEL + ".tgz"}
        (OUT / (manager + "-engine-input.json")).write_text(json.dumps({
            "package": dependency, "tarball": ENGINE_SENTINEL + ".tgz", "bytes": len(body),
            "sha256": hashlib.sha256(body).hexdigest(),
            "archive_members": ["package/package.json"]}, indent=2) + "\n")
    (work / "package.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return work


def validate_engine_rejection(result, manager, required, npm_version):
    assert result.returncode == 1, manager + " did not report an engine rejection exit"
    output = result.stdout + result.stderr
    assert ENGINE_SENTINEL in output and required in output, "wrong engine package/range"
    if manager == "npm":
        diagnostic = "ENOTSUP" if int(npm_version.split(".")[0]) < 7 else "EBADENGINE"
        assert diagnostic in output, "npm failed for a different reason"
    elif manager == "yarn":
        diagnostic = 'The engine "node" is incompatible'
        assert diagnostic in output, "Yarn failed for a different reason"
    else:
        assert manager == "pnpm"
        diagnostic = "ERR_PNPM_UNSUPPORTED_ENGINE"
        assert diagnostic in output, "pnpm failed for a different reason"
    return {"manager": manager, "package": ENGINE_SENTINEL, "range": required,
            "diagnostic": diagnostic, "returncode": result.returncode}


def package_managers(expected, versions, phase, managers):
    for manager in managers:
        work = project(manager, expected)
        if phase == "build":
            args = {"npm": ["npm", "install", "--offline", "--no-audit", "--package-lock=false", "--engine-strict"],
                    "yarn": ["yarn", "install", "--offline", "--non-interactive", "--no-lockfile"],
                    "pnpm": ["pnpm", "install", "--offline", "--lockfile=false", "--engine-strict"]}[manager]
            # npm6 suppresses root lifecycle by default. Derive that route from
            # the actual selected package metadata/version, never an exactpatch guess.
            if manager == "npm" and int(versions["npm"].split(".")[0]) < 7 and os.geteuid() == 0:
                args.append("--unsafe-perm")
            result = command(manager + "-install", args, work)
            actual = proofs(result.stdout)
            for event in ("preinstall", "postinstall"):
                matches = [p for p in actual if p["lifecycle"] == event]
                assert len(matches) == 1, manager + " did not run " + event
                validate_lifecycle(matches[0], expected, manager, event)
            wrong = project(manager, expected, wrong=True)
            rejected = command(manager + "-wrong-engine", args, wrong, success=False)
            proof = validate_engine_rejection(rejected, manager,
                ">=" + str(int(expected["node"].split(".")[0]) + 1), versions["npm"])
            (OUT / (manager + "-engine-proof.json")).write_text(json.dumps(proof, indent=2) + "\n")
        result = command(manager + "-run-" + phase, [manager, "run", "probe"], work)
        actual = proofs(result.stdout)
        assert len(actual) == 1
        validate_lifecycle(actual[0], expected, manager, "probe")
        if manager == "pnpm":
            result = command("pnpm-exec-" + phase, ["pnpm", "exec", "node", str(ROOT / "check.js"), "exec"], work)
            actual = proofs(result.stdout)
            assert len(actual) == 1
            validate_identity(actual[0], expected)
            validate_core(actual[0])


def main():
    phase = sys.argv[1]
    assert phase in ("build", "runtime")
    OUT.mkdir(exist_ok=True)
    expected = json.loads((ROOT / "expected-node.json").read_text())
    if phase == "build":
        command("compile-addon", ["cc", "-shared", "-fPIC", "-I/usr/local/include/node",
            "-DNODE_GYP_MODULE_NAME=bex_node_qualification", str(ROOT / "addon.c"),
            "-o", str(OUT / "bex_node.node")])
    primary = node_proof("node-" + phase, phase)
    validate_identity(primary, expected)
    validate_core(primary)
    linked = command("ldd-" + phase, ["ldd", "/usr/local/bin/node"])
    assert "not found" not in linked.stdout + linked.stderr
    tools = node_proof("tools-" + phase, "tools")
    versions = {}
    for manager in ("npm", "yarn"):
        versions[manager] = command(manager + "-version-" + phase, [manager, "--version"]).stdout.strip()
    package_managers(expected, versions, phase, ("npm", "yarn"))
    # Intentionally expose pnpm14/16, retaining the earlier actual npm/Yarn
    # observations. No private-host Node, downgrade or fake success is used.
    versions["pnpm"] = command("pnpm-version-" + phase, ["pnpm", "--version"]).stdout.strip()
    validate_tools(tools, versions, expected)
    if phase == "runtime":
        assert versions == json.loads((OUT / "build-tool-versions.json").read_text())
    package_managers(expected, versions, phase, ("pnpm",))
    if phase == "build":
        (OUT / "build-tool-versions.json").write_text(json.dumps(versions, indent=2) + "\n")
    result = {"pass": True, "phase": phase, "node": primary, "tool_versions": versions,
              "npm_package_sha256": tools["npm_package_sha256"],
              "scope": "common Node generated image, matching-header C import and offline dependency-free tool projects; no native-init/Kubernetes/hosted claim"}
    (OUT / ("node-" + phase + ".json")).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2), flush=True)


if __name__ == "__main__":
    main()
