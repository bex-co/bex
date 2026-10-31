"""Additional implicit-gyp projects; existing nine Node projects stay unchanged."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parent
OUT = ROOT / "qa-output"
PRIVATE = "/opt/bex/native-node-python/bin/python3.9"
EXPLICIT_PRIVATE = "/opt/bex/native-node-python/bin/python3"
GYP = "/usr/local/lib/node_modules/npm/node_modules/node-gyp"
SELECTORS = ("NODE_GYP_FORCE_PYTHON", "npm_config_python", "PYTHON")
BASE_KEYS = ("PATH", "NODE_VERSION", "PYTHON_VERSION", "PYTHONHOME", "PYTHONPATH", "LD_LIBRARY_PATH", *SELECTORS)
MANAGERS = ("npm", "yarn", "pnpm")
ORACLE = ROOT / "node-family-oracles.py"
if not ORACLE.exists():  # Host oracle controls use the unchanged repository source.
    ORACLE = ROOT.parent / "native-node-qualification/check.py"
_spec = importlib.util.spec_from_file_location("unchanged_node_family_oracles", ORACLE)
oracle = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(oracle)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def retain(name, value):
    (OUT / (name + ".json")).write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def command(name, argv, cwd=ROOT, env=None):
    try:
        result = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, text=True, timeout=90)
        record = {"argv": argv, "cwd": str(cwd), "returncode": result.returncode,
                  "stdout": result.stdout, "stderr": result.stderr, "timed_out": False}
    except subprocess.TimeoutExpired as error:
        def decoded(value):
            return value.decode(errors="replace") if isinstance(value, bytes) else value or ""
        record = {"argv": argv, "cwd": str(cwd), "returncode": None,
                  "stdout": decoded(error.stdout), "stderr": decoded(error.stderr), "timed_out": True}
    retain(name, record)
    print(json.dumps({"command_result": name, **record}), flush=True)
    return record


def successful(record):
    assert not record["timed_out"] and record["returncode"] == 0, "command did not complete successfully"
    return record["stdout"].strip()


def proofs(record):
    return [json.loads(line[len("BEX_NODE_ADDON_PROOF="):]) for line in record["stdout"].splitlines()
            if line.startswith("BEX_NODE_ADDON_PROOF=")]


def validate_proof(proof, expected, cwd, lifecycle=False):
    oracle.validate_identity(proof, expected["node_contract"])
    assert proof["cwd"] == str(cwd) and proof["platform"] == "linux"
    assert proof["arch"] == expected["arch"]
    if lifecycle:
        assert proof["phase"] == "lifecycle" and proof["lifecycle"] == "postinstall"
        for name in ("NODE", "npm_node_execpath"):
            assert proof[name] and os.path.realpath(proof[name]) == "/usr/local/bin/node", name
        assert proof["npm_execpath"], "missing actual manager entrypoint"


def variants(expected):
    rows = [
        ("default", {}, "success"),
        ("force-valid", {"NODE_GYP_FORCE_PYTHON": EXPLICIT_PRIVATE, "npm_config_python": PRIVATE, "PYTHON": PRIVATE}, "success"),
        ("config-before-env", {"npm_config_python": EXPLICIT_PRIVATE, "PYTHON": PRIVATE}, "success"),
        ("env-valid", {"PYTHON": EXPLICIT_PRIVATE}, "success"),
        ("ordinary-invalid-fallback", {"npm_config_python": "/bex-addon-invalid-config", "PYTHON": "/bex-addon-invalid-env"}, "success"),
        ("force-invalid", {"NODE_GYP_FORCE_PYTHON": "/bex-addon-invalid-force", "npm_config_python": PRIVATE, "PYTHON": PRIVATE}, "forced-error"),
    ]
    if expected["app_python"]:
        # A valid explicit modern interpreter is accepted by the real finder;
        # GYP's later distutils incompatibility must not cause silent fallback.
        rows.append(("app-python-explicit", {"PYTHON": "/usr/local/bin/python3"}, "app-success" if expected["common"] else "distutils-error"))
    return rows


def validate_install(record, kind, selectors, expected, metadata, addon_exists):
    assert not record["timed_out"], "UNPROVED: timed out install"
    assert isinstance(record["returncode"], int) and record["returncode"] >= 0, "UNPROVED: signal or incomplete install"
    log = record["stdout"] + record["stderr"]
    # Bind the actual invoked gyp, not merely a same-number global installation.
    assert GYP + "/bin/node-gyp.js" in log, "UNPROVED: selected npm gyp path absent"
    assert re.search(r"using node-gyp@" + re.escape(metadata["gyp"]) + r"(?:\s|$)", log), "wrong node-gyp metadata"
    selected = re.search(r'using Python version\s+(\S+)\s+found at\s+[\"\']([^\"\']+)[\"\']', log)
    if kind == "forced-error":
        assert record["returncode"] == 1 and not addon_exists
        assert "NODE_GYP_FORCE_PYTHON" in log and selectors["NODE_GYP_FORCE_PYTHON"] in log
        assert "Could not find any Python installation" in log, "UNPROVED: unrelated forced failure"
        assert selected is None, "invalid FORCE fell back to another interpreter"
        return
    assert selected is not None, "UNPROVED: actual selected Python absent"
    if kind == "distutils-error":
        assert record["returncode"] == 1 and not addon_exists
        assert selected.groups() == (expected["app_python"], "/usr/local/bin/python3")
        assert re.search(r"(?:ModuleNotFoundError|ImportError): No module named ['\"]distutils['\"]", log), "UNPROVED: unrelated app-Python failure"
        return
    assert kind in ("success", "app-success") and record["returncode"] == 0 and addon_exists
    if kind == "app-success":
        assert expected["common"] and selected.groups() == (expected["app_python"], "/usr/local/bin/python3")
    else:
        wanted = next((selectors[key] for key in SELECTORS if key in selectors and selectors[key] in (PRIVATE, EXPLICIT_PRIVATE)), PRIVATE)
        assert selected.groups() == (expected["private_python"], wanted)
    assert GYP + "/gyp/gyp_main.py" in log, "actual bundled GYP consumer absent"
    assert "-Dnode_root_dir=/usr/local" in log, "GYP did not use the selected local header prefix"
    if "NODE_GYP_FORCE_PYTHON" in selectors:
        assert "NODE_GYP_FORCE_PYTHON" in log
    elif "npm_config_python" in selectors and selectors["npm_config_python"] == EXPLICIT_PRIVATE:
        assert "command line or npm configuration" in log and EXPLICIT_PRIVATE in log
    elif selectors.get("PYTHON") == EXPLICIT_PRIVATE:
        assert 'environment variable PYTHON' in log


def project(manager, name):
    work = ROOT / "projects" / (manager + "-" + name)
    work.mkdir(parents=True)  # Existing output/build trees may never satisfy a new row.
    template = json.loads((ROOT / "package-template.json").read_text())
    assert template.get("gypfile") is True and not template.get("dependencies")
    assert not template.get("scripts"), "template unexpectedly defines an install bypass"
    template["name"] = "bex-node-addon-" + manager + "-" + name
    template["scripts"] = {"postinstall": "node ../../addon-check.js lifecycle"}
    (work / "package.json").write_text(json.dumps(template, ensure_ascii=False) + "\n")
    for filename in ("binding.gyp", "addon.c"):
        shutil.copyfile(ROOT / filename, work / filename)
    assert not (work / "build").exists() and not (work / "node_modules").exists()
    retain(manager + "-" + name + "-inputs", {filename: digest(work / filename)
        for filename in ("package.json", "binding.gyp", "addon.c")})
    return work


def manager_args(manager, npm):
    if manager == "npm":
        args = ["npm", "install", "--offline", "--build-from-source", "--nodedir=/usr/local",
                "--no-audit", "--package-lock=false", "--loglevel=verbose"]
        if int(npm.split(".")[0]) < 7 and os.geteuid() == 0:
            args.append("--unsafe-perm")
        return args
    if manager == "yarn":
        return ["yarn", "install", "--offline", "--no-lockfile", "--non-interactive", "--verbose"]
    assert manager == "pnpm"
    return ["pnpm", "install", "--offline", "--reporter=append-only"]


def metadata():
    npm_path = Path("/usr/local/lib/node_modules/npm/package.json")
    gyp_path = Path(GYP) / "package.json"
    npm, gyp = json.loads(npm_path.read_text()), json.loads(gyp_path.read_text())
    assert npm["name"] == "npm" and gyp["name"] == "node-gyp"
    return {"npm": npm["version"], "gyp": gyp["version"], "npm_sha256": digest(npm_path), "gyp_sha256": digest(gyp_path)}


def retain_tool_packages(tools):
    for name, path, key in (("selected-npm.package.json", "/usr/local/lib/node_modules/npm/package.json", "npm_sha256"),
                            ("selected-node-gyp.package.json", GYP + "/package.json", "gyp_sha256")):
        shutil.copyfile(path, OUT / name)
        assert digest(OUT / name) == tools[key], "retained selected package bytes changed"


def snapshot(expected, prefix):
    node = successful(command(prefix + "-node", ["node", "-p", "process.versions.node"]))
    assert node == expected["node_contract"]["node"]
    python = successful(command(prefix + "-app-python", ["python3", "-c", "import hashlib,json,ssl,sys,sysconfig; print(json.dumps([sys.version.split()[0],sys.prefix,sys.executable,sysconfig.get_path('stdlib'),{'ssl_file':ssl.__file__,'ssl_sha256':hashlib.sha256(open(ssl.__file__,'rb').read()).hexdigest(),'openssl':ssl.OPENSSL_VERSION}]))"]))
    if expected["app_python"]:
        actual = json.loads(python)
        assert actual[0] == expected["app_python"] and actual[1] == "/usr/local"
        assert os.path.realpath(actual[2]) == os.path.realpath("/usr/local/bin/python3")
    return {"node": node, "python": json.loads(python), "env": {key: os.environ.get(key) for key in BASE_KEYS}}


def private_closure(expected):
    record = command("private-python-closure", [PRIVATE, "-I", "-c",
        "import ctypes,distutils.version,json,ssl,sys; print(json.dumps([sys.version.split()[0],sys.prefix,sys.executable]))"])
    assert json.loads(successful(record)) == [expected["private_python"], "/opt/bex/native-node-python", PRIVATE]
    explicit = command("private-python-explicit-alias", [EXPLICIT_PRIVATE, "-I", "-c",
        "import ctypes,distutils.version,json,ssl,sys; print(json.dumps([sys.version.split()[0],sys.prefix,sys.executable]))"])
    assert json.loads(successful(explicit)) == [expected["private_python"], "/opt/bex/native-node-python", EXPLICIT_PRIVATE]
    modules = command("private-python-module-identities", [PRIVATE, "-I", "-c",
        "import _ctypes,_ssl,ctypes,distutils.version,hashlib,json,ssl; print(json.dumps({'openssl':ssl.OPENSSL_VERSION,'modules':{m.__name__:{'file':m.__file__,'sha256':hashlib.sha256(open(m.__file__,'rb').read()).hexdigest()} for m in (ctypes,_ctypes,ssl,_ssl,distutils.version)}}))"])
    identity = json.loads(successful(modules))
    assert identity["openssl"] and len(identity["modules"]) == 5
    assert all(row["file"].startswith("/opt/bex/native-node-python/") and re.fullmatch(r"[a-f0-9]{64}", row["sha256"])
               for row in identity["modules"].values())
    assert "/opt/bex/native-node-python/bin" not in os.environ["PATH"].split(":")
    assert all(not os.environ.get(key) for key in SELECTORS), "default row must not inherit explicit Python choices"


def build(expected):
    before = snapshot(expected, "before")
    private_closure(expected)
    if expected["common"] and expected["app_python"]:
        # The common packaging hook restores setuptools' distutils closure;
        # unlike raw historical Python3.13, this explicit interpreter must work.
        closure = command("app-distutils-closure", ["/usr/local/bin/python3", "-I", "-c",
            "import distutils.version,hashlib,importlib.metadata,json,sys,sysconfig; print(json.dumps([sys.version.split()[0],sys.prefix,distutils.version.__file__,{'distutils_sha256':hashlib.sha256(open(distutils.version.__file__,'rb').read()).hexdigest(),'setuptools':importlib.metadata.version('setuptools'),'stdlib':sysconfig.get_path('stdlib')}]))"])
        actual = json.loads(successful(closure))
        assert actual[:2] == [expected["app_python"], "/usr/local"] and actual[2].startswith("/usr/local/")
        assert actual[3]["setuptools"] and re.fullmatch(r"[a-f0-9]{64}", actual[3]["distutils_sha256"])
        shutil.copyfile("/usr/share/bex-native-python-tools.json", OUT / "selected-app-python-tools.json")
        restored = json.loads((OUT / "selected-app-python-tools.json").read_text())
        assert restored["python"] == expected["app_python"]
        selected_setuptools = [row for row in restored["artifacts"] if row["name"] == "setuptools"]
        assert len(selected_setuptools) == 1 and selected_setuptools[0]["version"] == actual[3]["setuptools"]
    tools = metadata()
    for manager in MANAGERS:
        version = successful(command(manager + "-version", [manager, "--version"]))
        assert version == (tools["npm"] if manager == "npm" else expected[manager])
    retain_tool_packages(tools)
    retain("selected-tools", tools)
    installed = []
    for manager in MANAGERS:
        # Default must pass first; later explicit controls never heal it.
        for name, selectors, kind in variants(expected):
            work = project(manager, name)
            env = dict(os.environ)
            for key in (*SELECTORS, "npm_config_node_gyp"):
                env.pop(key, None)
            env.update(npm_config_nodedir="/usr/local", npm_config_build_from_source="true",
                       npm_config_loglevel="verbose", npm_config_offline="true", **selectors)
            record = command(manager + "-" + name, manager_args(manager, tools["npm"]), work, env)
            addon = work / "build/Release/bex_addon.node"
            validate_install(record, kind, selectors, expected, tools, addon.is_file())
            if kind in ("success", "app-success"):
                observed = proofs(record)
                assert len(observed) == 1, "missing actual implicit install/postinstall proof"
                validate_proof(observed[0], expected, work, lifecycle=True)
                artifact = manager + "-" + name + ".node"
                shutil.copyfile(addon, OUT / artifact)
                assert digest(OUT / artifact) == digest(addon)
                installed.append({"manager": manager, "variant": name, "work": str(work.relative_to(ROOT)), "artifact": artifact, "sha256": digest(addon)})
            assert snapshot(expected, manager + "-" + name + "-after") == before, "addon install changed base app Python/Node/env/PATH"
    retain("addon-build", {"pass": True, "before": before, "tools": tools, "installed": installed,
                          "scope": "actual implicit gypfile default and explicit Python controls; no hosted claim"})


def validate_summary(built, expected):
    assert built["pass"] is True
    wanted = [(manager, name) for manager in MANAGERS for name, _, kind in variants(expected) if kind in ("success", "app-success")]
    assert [(item["manager"], item["variant"]) for item in built["installed"]] == wanted, "missing default or explicit successful rows"
    assert all(re.fullmatch(r"[a-f0-9]{64}", item["sha256"]) for item in built["installed"])


def runtime(expected):
    built = json.loads((OUT / "addon-build.json").read_text())
    validate_summary(built, expected)
    assert built["tools"] == metadata()
    assert snapshot(expected, "runtime") == built["before"]
    assert digest(OUT / "selected-npm.package.json") == built["tools"]["npm_sha256"]
    assert digest(OUT / "selected-node-gyp.package.json") == built["tools"]["gyp_sha256"]
    for item in built["installed"]:
        work = ROOT / item["work"]
        assert digest(work / "build/Release/bex_addon.node") == item["sha256"]
        assert digest(OUT / item["artifact"]) == item["sha256"]
        record = command(item["manager"] + "-" + item["variant"] + "-runtime", ["node", str(ROOT / "addon-check.js"), "runtime"], work)
        successful(record)
        observed = proofs(record)
        assert len(observed) == 1
        validate_proof(observed[0], expected, work)
    result = {"pass": True, "case": expected["case"], "imported": len(built["installed"]),
              "scope": "build-created native addon imports in actual generated runtime; no fresh runtime reinstall/hosted claim"}
    retain("addon-runtime", result)
    print(json.dumps(result, indent=2), flush=True)


def main():
    OUT.mkdir(exist_ok=True)
    expected = json.loads((ROOT / "expected-addon.json").read_text())
    assert sys.argv[1] in ("build", "runtime")
    (build if sys.argv[1] == "build" else runtime)(expected)


if __name__ == "__main__":
    main()
