"""Exercise primary Python packaging/headers independently of Debian Python."""
from email.parser import Parser
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import sysconfig
import tempfile


def run(*args, cwd, env=None, fails=False):
    result = subprocess.run(args, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if (result.returncode != 0) != fails:
        raise RuntimeError((args, result.returncode, result.stdout.decode(errors="replace"), result.stderr.decode(errors="replace")))
    return result.stdout


executable = sys.executable
version = sys.version.split()[0]
assert executable.startswith("/usr/local/")
assert (Path(sysconfig.get_path("include")) / "Python.h").is_file()
with tempfile.TemporaryDirectory(prefix="bex-primary-python-") as directory:
    root = Path(directory)
    project = root / "project"
    project.mkdir()
    (project / "setuptools.py").write_text("value = 'project setuptools'\n")
    (project / "wheel.py").write_text("value = 'project wheel'\n")
    environment = dict(os.environ, PYTHONPATH=str(project), PATH="/project-path:" + os.environ["PATH"])
    record = Path("/usr/share/bex-native-python-tools.json")
    before = record.read_bytes()
    site = Path(sysconfig.get_path("purelib"))
    selected = json.loads(before)["artifacts"]
    metadata = [site / (item["name"] + "-" + item["version"] + ".dist-info") / "RECORD" for item in selected]
    before_times = [path.stat().st_mtime_ns for path in metadata]
    for _ in range(2):
        run("/usr/bin/bex-native-python-tools", cwd=project, env=environment)
    assert record.read_bytes() == before
    assert [path.stat().st_mtime_ns for path in metadata] == before_times
    assert run(executable, "-c", "import setuptools,wheel; assert setuptools.value == 'project setuptools'; assert wheel.value == 'project wheel'", cwd=project, env=environment) == b""
    assert sys.executable == executable and sys.version.split()[0] == version

    run(executable, "-c", "import setuptools,pkg_resources,wheel,distutils.sysconfig; assert pkg_resources.get_distribution('setuptools').version == " + repr(selected[0]["version"]), cwd=root)
    (root / "primary_data.py").write_text("LABEL = 'Crème brûlée & 李'\n", encoding="utf-8")
    (root / "primary_math.c").write_text(r'''
#include <Python.h>
static PyObject *weighted(PyObject *self, PyObject *args) {
    long a, b;
    if (!PyArg_ParseTuple(args, "ll", &a, &b)) return NULL;
    return PyLong_FromLong(7 * a + 3 * b);
}
static PyObject *dessert(PyObject *self, PyObject *args) {
    return PyUnicode_FromString("Crème brûlée & 李");
}
static PyMethodDef methods[] = {
    {"weighted", weighted, METH_VARARGS, "Compute a weighted sum."},
    {"dessert", dessert, METH_NOARGS, "Return known UTF-8."},
    {NULL, NULL, 0, NULL}
};
static struct PyModuleDef module = {PyModuleDef_HEAD_INIT, "primary_math", NULL, -1, methods};
PyMODINIT_FUNC PyInit_primary_math(void) { return PyModule_Create(&module); }
''')
    setup = "from setuptools import Extension, setup\nsetup(name='bex-primary-proof', version='1.2.3', py_modules=['primary_data'], ext_modules=[Extension('primary_math', ['primary_math.c'])])\n"
    (root / "setup.py").write_text(setup)
    run(executable, "setup.py", "egg_info", cwd=root)
    info = Parser().parsestr((root / "bex_primary_proof.egg-info/PKG-INFO").read_text())
    assert info["Name"] == "bex-primary-proof" and info["Version"] == "1.2.3"
    run(executable, "setup.py", "build_ext", "--inplace", cwd=root)
    check = "import primary_math,primary_data; assert [primary_math.weighted(a,b) for a,b in [(17,-4),(-9,7),(0,0)]] == [107,-42,0]; assert primary_math.dessert() == primary_data.LABEL == 'Crème brûlée & 李'"
    run(executable, "-c", check, cwd=root)
    extension = next(root.glob("primary_math*" + sysconfig.get_config_var("EXT_SUFFIX")))
    compiled_sha = hashlib.sha256(extension.read_bytes()).hexdigest()
    run(executable, "-m", "pip", "--isolated", "wheel", "--no-index", "--no-deps", "--no-build-isolation", "--wheel-dir", "wheels", ".", cwd=root)
    wheel = next((root / "wheels").glob("*.whl"))
    target = root / "installed"
    run(executable, "-m", "pip", "--isolated", "install", "--no-index", "--no-deps", "--target", str(target), str(wheel), cwd=root)
    empty = root / "empty"
    empty.mkdir()
    run(executable, "-c", "import sys; sys.path.insert(0," + repr(str(target)) + "); " + check, cwd=empty)
    assert hashlib.sha256(next(target.glob("primary_math*" + sysconfig.get_config_var("EXT_SUFFIX"))).read_bytes()).hexdigest() == compiled_sha

    (root / "broken.c").write_text("#include <Python.h>\nthis is an invalid C extension;\n")
    (root / "bad_setup.py").write_text("from setuptools import Extension,setup\nsetup(name='bad', ext_modules=[Extension('broken', ['broken.c'])])\n")
    run(executable, "bad_setup.py", "build_ext", "--build-lib", "bad-lib", "--build-temp", "bad-temp", cwd=root, fails=True)
    assert not list((root / "bad-lib").glob("*.so"))
    invalid = root / "invalid-1.0-py3-none-any.whl"
    invalid.write_bytes(b"not a wheel")
    run(executable, "-m", "pip", "--isolated", "install", "--no-index", "--no-deps", "--target", "bad-target", str(invalid), cwd=root, fails=True)
    assert not (root / "bad-target/invalid.py").exists()

print("native primary Python " + version + " metadata, pkg_resources/distutils, wheel and C-extension checks passed without network")
