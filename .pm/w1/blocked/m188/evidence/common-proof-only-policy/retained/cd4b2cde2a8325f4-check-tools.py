"""Real offline transformations; all files are confined to one temporary tree."""
import ctypes
import ctypes.util
import gzip
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import sysconfig
import tempfile


def run(*args, data=None, fails=False):
    result = subprocess.run(args, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if (result.returncode != 0) != fails:
        raise RuntimeError((args, result.returncode, result.stdout.decode(errors="replace"), result.stderr.decode(errors="replace")))
    return result.stdout


with tempfile.TemporaryDirectory(prefix="bex-native-check-") as directory:
    os.chdir(directory)
    html = run("pandoc", "-f", "markdown", "-t", "html", data="# naïve\n\n**three** rows.\n".encode()).decode()
    assert '<h1 id="naïve">naïve</h1>' in html and "<strong>three</strong>" in html
    run("pandoc", "-f", "bex-unsupported-format", data=b"text", fails=True)

    assert run("sqlite3", "rows.db", "CREATE TABLE t(v); INSERT INTO t VALUES ('naïve'),('quote''s'),('comma,value'); SELECT count(*) FROM t;") == b"3\n"
    assert run("sqlite3", "rows.db", "SELECT v FROM t ORDER BY rowid;").decode() == "naïve\nquote's\ncomma,value\n"
    run("sqlite3", "rows.db", "SELECT missing FROM t;", fails=True)

    run("vips", "black", "black.v", "2", "3", "--bands", "3")
    run("vips", "invert", "black.v", "white.png")
    assert run("vipsheader", "-f", "width", "white.png").strip() == b"2"
    assert run("vipsheader", "-f", "height", "white.png").strip() == b"3"
    assert float(run("vips", "avg", "white.png")) == 255
    assert ctypes.CDLL(ctypes.util.find_library("vips")).vips_version(0) >= 8

    Path("messages.po").write_text('msgid ""\nmsgstr "Content-Type: text/plain; charset=UTF-8\\n"\n\nmsgid "hello"\nmsgstr "naïve"\n')
    run("msgfmt", "messages.po", "-o", "messages.mo")
    assert 'msgstr "naïve"' in run("msgunfmt", "messages.mo").decode()

    Path("red.ps").write_text("%!PS\n1 0 0 setrgbcolor 0 0 8 8 rectfill showpage\n")
    run("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-g8x8", "-sOutputFile=red.png", "red.ps")
    png = Path("red.png").read_bytes()
    assert png[:8] == b"\x89PNG\r\n\x1a\n" and struct.unpack(">II", png[16:24]) == (8, 8)
    assert b"IDAT" in png and b"IEND" in png

    Path("assets").mkdir()
    Path("assets/one.txt").write_text("naïve\n")
    manifest = run("jq", "-c", ".files[]", data=b'{"files":["one.txt"]}')
    assert json.loads(manifest) == "one.txt"
    run("rsync", "-a", "assets/", "copied/")
    assert Path("copied/one.txt").read_bytes() == Path("assets/one.txt").read_bytes()

    node = "/opt/bex/native-node-tools/node"
    Path("typed.cts").write_text("export function answer(): number { return 42; }\n")
    run("tsc", "--strict", "--module", "commonjs", "--target", "ES2020", "--outDir", "compiled", "typed.cts")
    run(node, "-e", "if(require('./compiled/typed.cjs').answer()!==42)process.exit(1)")
    Path("invalid.cts").write_text('const number: number = "bad";\n')
    run("tsc", "--strict", "--noEmit", "invalid.cts", fails=True)

    Path("answer.i").write_text("%module answer\n%inline %{\nint value(void) { return 42; }\n%}\n")
    run("swig", "-python", "answer.i")
    run("gcc", "-shared", "-fPIC", "-I" + sysconfig.get_path("include"), "answer_wrap.c", "-o", "_answer" + sysconfig.get_config_var("EXT_SUFFIX"))
    run(sys.executable, "-c", "import answer; assert answer.value() == 42")

    Path("value.mjs").write_text("export const answer = 42;\n")
    Path("entry.mjs").write_text("export {answer} from './value.mjs';\n")
    run("webpack", "--mode", "production", "--target", "node", "--entry", "./entry.mjs", "--output-path", str(Path("bundled").resolve()), "--output-filename", "bundle.cjs", "--output-library-type", "commonjs2")
    run(node, "-e", "if(require('./bundled/bundle.cjs').answer!==42)process.exit(1)")

    original = "naïve,quote's\n".encode() * 100
    compressed = run("pigz", "-c", data=original)
    assert gzip.decompress(compressed) == original
    assert run("pigz", "-d", "-c", data=compressed) == original
    run("zip", "-q", "assets.zip", "assets/one.txt")
    assert run("unzip", "-p", "assets.zip", "assets/one.txt") == Path("assets/one.txt").read_bytes()

    Path("setup.py").write_text("from setuptools import setup\nsetup(name='bex-native-proof', version='1.2.3', py_modules=['proof'])\n")
    Path("proof.py").write_text("def answer(): return 42\n")
    run(sys.executable, "setup.py", "egg_info")
    assert "Version: 1.2.3" in Path("bex_native_proof.egg-info/PKG-INFO").read_text()
    run(sys.executable, "-m", "pip", "wheel", "--no-index", "--no-deps", "--no-build-isolation", "--wheel-dir", "wheels", ".")
    wheel = next(Path("wheels").glob("*.whl"))
    run(sys.executable, "-m", "pip", "install", "--no-index", "--no-deps", "--target", "installed", str(wheel))
    run(sys.executable, "-c", "import sys; sys.path.insert(0,'installed'); import proof; assert proof.answer()==42")

print("native tool transformations passed without network")
