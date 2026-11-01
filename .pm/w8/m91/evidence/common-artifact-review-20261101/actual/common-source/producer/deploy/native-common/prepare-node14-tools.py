"""Give legacy node-gyp a private compatible default; retain explicit choices."""
import json
from pathlib import Path
import re
import subprocess
import sys


PREFIX = "/opt/bex/native-node-python"
PYTHON = PREFIX + "/bin/python3.9"
FINDER = Path("/usr/local/lib/node_modules/npm/node_modules/node-gyp/lib/find-python.js")
ANCHOR = """        {
          before: () => { this.addLog('checking if "python" can be used') },"""
ANCHOR3 = ANCHOR.replace('"python"', '"python3"')
PRIVATE_CHECK = """        {
          before: () => { this.addLog('checking the Bex private legacy Node build interpreter') },
          check: this.checkCommand,
          arg: '/opt/bex/native-node-python/bin/python3.9'
        },
"""
PREVIOUS_PRIVATE_CHECK = PRIVATE_CHECK.replace("private legacy Node", "private Node14")


def patch_finder(source):
    # This is an insertion in the bundled finder, not a GYP API backport.
    # The upstream FORCE early return and config/PYTHON checks retain priority.
    clean = source.replace(PRIVATE_CHECK, "").replace(PREVIOUS_PRIVATE_CHECK, "")
    if any(clean.count(anchor) != 1 for anchor in (ANCHOR, ANCHOR3)) or source.count(PRIVATE_CHECK) + source.count(PREVIOUS_PRIVATE_CHECK) > 1:
        raise ValueError("legacy node-gyp Python finder default anchor changed")
    # node-gyp5 checks python first; node-gyp9 checks python3 first.
    anchor = min((ANCHOR, ANCHOR3), key=clean.index)
    prefix = clean.split(anchor)[0]
    markers = ("if (this.env.NODE_GYP_FORCE_PYTHON)", "arg: this.configPython", "arg: this.env.PYTHON")
    positions = [prefix.find(marker) for marker in markers]
    if -1 in positions or positions != sorted(positions) or PREFIX in clean:
        raise ValueError("legacy node-gyp explicit Python selection changed")
    return clean.replace(anchor, PRIVATE_CHECK + anchor)


def prepare(version):
    if not re.fullmatch(r"3\.9\.[0-9]+", version):
        raise ValueError("private legacy Node interpreter must be the reviewed Python3.9 release")
    # Official Python3.9 is linked with $ORIGIN/../lib. Verify the relocated
    # interpreter and stdlib/extension closure instead of changing app loader
    # settings, PATH, PYTHONHOME, or PYTHONPATH.
    result = subprocess.run([PYTHON, "-I", "-c",
        "import ctypes,distutils.version,json,ssl,sys; print(json.dumps([sys.version.split()[0],sys.prefix,sys.executable]))"],
        check=True, capture_output=True, text=True)
    if json.loads(result.stdout) != [version, PREFIX, PYTHON]:
        raise ValueError("private legacy Node Python relocation/version mismatch")
    source = FINDER.read_text()
    patched = patch_finder(source)
    if source != patched:
        FINDER.write_text(patched)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: prepare-node14-tools.py REVIEWED_PYTHON_VERSION")
    prepare(sys.argv[1])
