"""Additional legacy family proof; no original utility or hosted spec changes."""
import ctypes
import ctypes.util
import importlib.util
import json
import os
from pathlib import Path
import ssl
import subprocess
import sys
import sysconfig


PRIVATE_MODULES = ("poetry", "virtualenv", "cleo", "dulwich", "findpython")


def validate_identity(data, expected, project):
    if data["python"] != expected or data["python_env"] != expected or data["base_prefix"] != "/usr/local":
        raise RuntimeError("selected legacy application version/environment/base changed")
    if project:
        if (data["prefix"] == data["base_prefix"] or data["executable"].startswith("/opt/bex/poetry/")
                or not data["executable"].startswith(data["prefix"].rstrip("/") + "/")):
            raise RuntimeError("Poetry did not create a distinct application interpreter environment")
    elif data["prefix"] != "/usr/local" or not data["executable"].startswith("/usr/local/"):
        raise RuntimeError("system Python substituted for application Python")
    if data["private_imports"] or any(path.startswith("/opt/bex/poetry") for path in data["sys_path"]):
        raise RuntimeError("private Poetry dependencies leaked into application imports")
    if (data["headers"] != expected or data["libpython"] != expected or data["number"] != 107
            or data["label"] != "Crème brûlée & 李" or data["ffi_bytes"] != len("Crème brûlée & 李".encode("utf-8"))):
        raise RuntimeError("application C-header/loader/FFI/Unicode result differs")


def observe(expected, project):
    import _ctypes
    import _ssl
    import legacy_probe
    if "VERSION_CODENAME=bookworm" not in Path("/etc/os-release").read_text().splitlines():
        raise RuntimeError("legacy runtime left the common Bookworm foundation")
    linked = {}
    for path in (sys.executable, _ssl.__file__, _ctypes.__file__):
        result = subprocess.run(["ldd", path], check=True, capture_output=True, text=True, timeout=15).stdout
        if "not found" in result:
            raise RuntimeError("unresolved legacy runtime dependency")
        linked[path] = result
    library = sysconfig.get_config_var("INSTSONAME") or sysconfig.get_config_var("LDLIBRARY")
    if not library:
        raise RuntimeError("selected Python has no shared-library SONAME")
    ctypes.CDLL(library)  # Resolve the real selected SONAME via the loader/cache.
    python = ctypes.PyDLL(library)
    python.Py_GetVersion.restype = ctypes.c_char_p
    libc_name = ctypes.util.find_library("c")
    if not libc_name:
        raise RuntimeError("libc SONAME lookup failed")
    libc = ctypes.CDLL(libc_name)
    libc.strlen.argtypes, libc.strlen.restype = [ctypes.c_char_p], ctypes.c_size_t
    context = ssl.create_default_context()
    outgoing = ssl.MemoryBIO()
    client = context.wrap_bio(ssl.MemoryBIO(), outgoing, server_hostname="legacy.invalid")
    try:
        client.do_handshake()
    except ssl.SSLWantReadError:
        pass
    if len(ssl.RAND_bytes(32)) != 32 or outgoing.pending == 0:
        raise RuntimeError("selected SSL extension did not execute a local TLS handshake step")
    data = dict(legacy_probe.answer(), python=sys.version.split()[0], python_env=os.environ.get("PYTHON_VERSION"),
                prefix=sys.prefix, base_prefix=sys.base_prefix, executable=sys.executable, sys_path=sys.path,
                private_imports=[name for name in PRIVATE_MODULES if importlib.util.find_spec(name) is not None],
                libpython=python.Py_GetVersion().decode().split()[0], libpython_soname=library,
                ffi_bytes=libc.strlen("Crème brûlée & 李".encode("utf-8")), openssl=ssl.OPENSSL_VERSION,
                linked=linked, extension=str(Path(legacy_probe.__file__).resolve()))
    validate_identity(data, expected, project)
    return data


def logged(command, path, timeout=120):
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=timeout)
    except subprocess.TimeoutExpired as error:
        text = b"".join(part for part in (error.stdout, error.stderr) if part).decode(errors="replace")
        path.write_text(text)
        print(text, file=sys.stderr, flush=True)
        raise
    path.write_text(result.stdout + result.stderr)
    print(result.stdout + result.stderr, file=sys.stderr, flush=True)
    result.check_returncode()
    return result


def main():
    phase, expected = sys.argv[1:]
    output = Path("qa-output")
    output.mkdir(exist_ok=True)
    if phase == "project":
        print(json.dumps(observe(expected, True)))
        return
    if phase not in ("build", "runtime"):
        raise RuntimeError("expected build/runtime/project phase")
    if any(os.environ.get(key) for key in ("VIRTUAL_ENV", "CONDA_PREFIX", "POETRY_ACTIVE",
                                           "POETRY_VIRTUALENVS_USE_POETRY_PYTHON", "POETRY_VIRTUALENVS_CREATE")):
        raise RuntimeError("automatic interpreter control must start without forcing an environment")
    if phase == "build":
        logged([sys.executable, "setup.py", "build_ext", "--inplace"], output / "compile.log")
    app = observe(expected, False)
    tool = logged(["/opt/bex/poetry/bin/python", "-I", "check-tool.py"], output / ("tool-" + phase + ".log"), 30)
    # No poetry env use, explicit interpreter flags, install, dependencies, or
    # project-interpreter configuration: exercise actual automatic selection.
    project = logged(["poetry", "run", "python", "check.py", "project", expected], output / ("poetry-" + phase + ".log"))
    result = {"phase": phase, "app": app, "project": json.loads(project.stdout),
              "private_tool": json.loads(tool.stdout), "automatic_poetry_run_executed": True, "pass": True}
    (output / ("legacy-" + phase + ".json")).write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
