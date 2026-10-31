"""Separate offline Go/Rust family qualification; no original utility payloads."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import zlib

TEXT = "Bex 雪 λ🙂"
TREES = {"go": ("go",), "rust": ("cargo", "rustup")}


def require(value, message):
    if not value:
        raise RuntimeError(message)


def sha(path):
    value = hashlib.sha256()
    with path.open("rb") as source:
        while block := source.read(1024 * 1024):
            value.update(block)
    return value.hexdigest()


def tree(root):
    require(root.is_dir(), "primary tree is missing: " + str(root))
    result = {}
    for path in [root, *sorted(root.rglob("*"))]:
        info = path.lstat()
        item = {"mode": stat.S_IMODE(info.st_mode)}
        if path.is_symlink():
            item.update(kind="symlink", target=os.readlink(path))
        elif path.is_dir():
            item["kind"] = "directory"
        elif path.is_file():
            item.update(kind="file", bytes=info.st_size, sha256=sha(path))
        else:
            raise RuntimeError("unexpected primary tree file type")
        result[str(path.relative_to(root))] = item
    return result


def equal_trees(reference, actual):
    expected, found = tree(reference), tree(actual)
    require(expected == found, "installed primary bytes, modes or symlinks differ from pinned donor")
    return found


def env_map(entries):
    result = {}
    for entry in entries:
        key, separator, value = entry.partition("=")
        require(separator and key and key not in result, "invalid expected primary ENV")
        result[key] = value
    return result


def validate_primary(expected, actual):
    runtime, version = expected["runtime"], expected["version"]
    environment = env_map(expected["env"])
    require(runtime in TREES, "unknown family")
    require(all(actual["env"].get(key) == value for key, value in environment.items()), "selected primary ENV changed")
    if runtime == "go":
        require(actual["executable"] == "/usr/local/go/bin/go", "Go executable is not the selected primary")
        require(actual["version"] == "go" + version, "Go patch version changed")
        require(actual["goroot"] == "/usr/local/go" and actual["gopath"] == "/go", "Go installation roots changed")
        require(actual["toolchain"] == "local", "Go toolchain could download or substitute another compiler")
    else:
        require(actual["executable"] == "/usr/local/cargo/bin/cargo", "Cargo executable is not the selected primary")
        require(actual["version"].startswith("rustc " + version + " "), "Rust patch version changed")
        require(actual["sysroot"].startswith("/usr/local/rustup/toolchains/"), "Rust sysroot escaped the primary tree")
        require(actual["rustc"] == actual["sysroot"] + "/bin/rustc", "rustup selected another compiler")
        require(actual["target_libdir"].startswith(actual["sysroot"] + "/lib/rustlib/"), "Rust standard library tree changed")


def validate_outputs(runtime, outputs):
    payload = TEXT.encode()
    if runtime == "go":
        expected = "go-pure|8|14|33153|" + TEXT
        native = ["go-cgo", f"{zlib.adler32(payload):08x}", "5346"]
    else:
        require(runtime == "rust", "unknown family")
        expected = "rust-pure|8|14|65536|🙂λ 雪 xeB"
        native = ["rust-native", f"{zlib.crc32(payload):08x}", "2566"]
    require(outputs["pure"].strip() == expected, "independent Unicode/arithmetic result differs")
    fields = outputs["native"].strip().split("|")
    require(len(fields) == 5 and fields[:3] == native, "native checksum/arithmetic result differs")
    require(fields[3] and fields[3] == fields[4], "native header and linked library versions differ")


def command(arguments, env=None, timeout=300):
    return subprocess.run(arguments, check=True, capture_output=True, text=True, env=env, timeout=timeout).stdout.strip()


def primary_facts(expected):
    runtime = expected["runtime"]
    facts = {"env": {key: os.environ.get(key) for key in env_map(expected["env"])}}
    if runtime == "go":
        facts.update(executable=shutil.which("go"), **json.loads(command(["go", "env", "-json", "GOVERSION", "GOROOT", "GOPATH", "GOTOOLCHAIN", "GOTOOLDIR"])))
        facts.update(version=facts.pop("GOVERSION"), goroot=facts.pop("GOROOT"), gopath=facts.pop("GOPATH"), toolchain=facts.pop("GOTOOLCHAIN"))
        critical = [Path(facts["executable"]), Path(facts["GOTOOLDIR"]) / "compile", Path("/usr/local/go/pkg/include/textflag.h")]
    else:
        facts.update(executable=shutil.which("cargo"), version=command(["rustc", "--version"]), cargo=command(["cargo", "--version"]), sysroot=command(["rustc", "--print", "sysroot"]), rustc=command(["rustup", "which", "rustc"]), target_libdir=command(["rustc", "--print", "target-libdir"]))
        libraries = list(Path(facts["target_libdir"]).glob("libstd-*.rlib"))
        require(libraries, "selected Rust standard library is missing")
        critical = [Path(facts["executable"]), Path(facts["rustc"]), *libraries]
    validate_primary(expected, facts)
    facts["critical_sha256"] = {str(path): sha(path) for path in critical}
    return facts


def compile_programs(runtime, output):
    with tempfile.TemporaryDirectory(prefix="bex-native-gorust-") as temporary:
        work = Path(temporary)
        environment = dict(os.environ, GOPROXY="off", GOSUMDB="off", GOWORK="off", GOTOOLCHAIN="local", GOMAXPROCS="2", GOCACHE=str(work / "go-cache"), CARGO_NET_OFFLINE="true", CARGO_BUILD_JOBS="1")
        if runtime == "go":
            for name, cgo in (("pure", "0"), ("cgo", "1")):
                command(["go", "build", "-p", "1", "-mod=readonly", "-trimpath", "-o", str(output / ("go-" + name)), "./go-" + name], dict(environment, CGO_ENABLED=cgo))
        else:
            target = work / "target"
            command(["cargo", "build", "--offline", "--frozen", "--release", "--jobs", "1", "--target-dir", str(target)], environment)
            for name in ("pure", "native"):
                shutil.copy2(target / "release" / ("bex-qualify-" + name), output / ("rust-" + name))
            archives = list(target.glob("release/build/bex-qualify-native-*/out/libbex_probe.a"))
            require(len(archives) == 1, "Rust native static archive is missing or ambiguous")
            shutil.copy2(archives[0], Path("qa-output") / "rust-static.a")
            require(command(["ar", "t", str(archives[0])]).splitlines() == ["probe.o"], "Rust build did not use the expected C static object")


def main():
    phase, runtime = sys.argv[1:3]
    require(runtime in TREES and phase in ("tree", "build", "runtime"), "unknown profile phase")
    root = Path(__file__).resolve().parent
    os.chdir(root)
    expected = json.loads(Path("expected-primary.json").read_text())
    require(expected["runtime"] == runtime, "expected primary belongs to another family")
    proofs, binaries = Path("qa-output"), Path("qa-bin")
    proofs.mkdir(exist_ok=True)
    binaries.mkdir(exist_ok=True)
    if phase == "tree":
        require(len(sys.argv) == 4 and sys.argv[3] == "/opt/bex-test-primary", "expected fixed readonly donor mount")
        manifests = {name: equal_trees(Path(sys.argv[3]) / name, Path("/usr/local") / name) for name in TREES[runtime]}
        Path("qa-output/primary-tree.json").write_text(json.dumps({"image": expected["image"], "runtime": runtime, "trees": manifests}, indent=2) + "\n")
        return
    require(len(sys.argv) == 3, "unexpected phase arguments")
    require('VERSION_CODENAME=bookworm' in Path('/etc/os-release').read_text().splitlines(), "common Bookworm foundation changed")
    baseline = json.loads(Path("qa-output/primary-tree.json").read_text())
    require(baseline["image"] == expected["image"] and baseline["runtime"] == runtime, "donor tree proof identity changed")
    facts = primary_facts(expected)
    if phase == "build":
        compile_programs(runtime, binaries.resolve())
    else:
        built = json.loads(Path("qa-output/build.json").read_text())
        require(facts == built["primary"], "primary executable, tree or ENV changed after build")
    pure = binaries / (runtime + "-pure")
    native = binaries / (runtime + ("-cgo" if runtime == "go" else "-native"))
    outputs = {"pure": command([str(pure.resolve())]), "native": command([str(native.resolve())])}
    validate_outputs(runtime, outputs)
    linked = command(["ldd", str(native.resolve())])
    require("libz.so" in linked and "not found" not in linked, "native runtime linkage is missing")
    result = {"phase": phase, "runtime": runtime, "primary": facts, "outputs": outputs, "native_ldd": linked, "compiled_offline_in_this_phase": phase == "build", "pass": True}
    (proofs / (phase + ".json")).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
