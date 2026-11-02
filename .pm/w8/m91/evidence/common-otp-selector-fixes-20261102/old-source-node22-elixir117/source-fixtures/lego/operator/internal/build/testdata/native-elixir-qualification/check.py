"""Actual offline OTP/NIF/rebar consumer; never an image or hosted approval."""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys


TEXT = "bex café λ 雪"


def run(*args):
    Path("qa-output").mkdir(exist_ok=True)
    try:
        result = subprocess.run(args, check=False, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, text=True, timeout=180)
    except subprocess.TimeoutExpired as error:
        record = {"command": args, "timeout_seconds": 180,
                  "stdout": (error.stdout or b"").decode(errors="replace"),
                  "stderr": (error.stderr or b"").decode(errors="replace")}
        with Path("qa-output/commands.jsonl").open("a") as log:
            log.write(json.dumps(record) + "\n")
        raise
    record = {"command": args, "exit": result.returncode,
              "stdout": result.stdout, "stderr": result.stderr}
    with Path("qa-output/commands.jsonl").open("a") as log:
        log.write(json.dumps(record) + "\n")
    if result.returncode:
        raise RuntimeError(json.dumps(record))
    return result.stdout.strip()


def check_result(value):
    # The three languages compute this result independently: Erlang UTF-8 and
    # crypto, the C NIF's OpenSSL/libffi calls, and Python's Unicode/hashlib.
    encoded, total, digest = value.split("|")
    assert base64.b64decode(encoded, validate=True) == TEXT.encode("utf-8")
    assert total == "42"
    assert digest.lower() == hashlib.sha256(TEXT.encode("utf-8")).hexdigest()


def identity(expected):
    assert "VERSION_CODENAME=bookworm" in Path("/etc/os-release").read_text().splitlines()
    for key, value in expected["env"].items():
        assert os.environ.get(key) == value, (key, os.environ.get(key), value)
    assert run("elixir", "-e", "IO.puts(System.version())") == expected["elixir"]
    root = Path(run("erl", "-noshell", "-eval", 'io:format("~s",[code:root_dir()]),halt().'))
    major = expected["otp"].split(".")[0]
    assert (root / "releases" / major / "OTP_VERSION").read_text().strip() == expected["otp"]
    assert run("node", "-p", "process.versions.node") == expected["node"]
    versions = {}
    for command, key in (("rebar", "REBAR_VERSION"), ("rebar3", "REBAR3_VERSION")):
        output = run(command, "--version")
        assert re.search(r"(?<![\d.])" + re.escape(expected["env"][key]) + r"(?![\d.])", output), output
        versions[command] = output
    return root, versions


def main(phase):
    assert phase in ("build", "runtime")
    expected = json.loads(Path("expected.json").read_text())
    root, versions = identity(expected)
    if phase == "build":
        os.environ.update(HEX_OFFLINE="1", REBAR_OFFLINE="1")
        Path("priv").mkdir(exist_ok=True)
        flags = shlex.split(run("pkg-config", "--cflags", "--libs", "libffi", "openssl"))
        run("cc", "-shared", "-fPIC", "-Wall", "-Wextra", "-Werror",
            "-I" + str(root / "usr" / "include"), "bex_nif.c", "-o", "priv/bex_nif.so", *flags)
        # Both tools consume this dependency-free project with network disabled.
        run("rebar", "compile")
        run("rebar3", "as", "prod", "compile")
        # mix.exs declares the local rebar-built app in extra_applications;
        # preserve normal dependency validation, pruning and warning failures.
        run("elixir", "-pa", "_build/prod/lib/bex_probe/ebin", "-S", "mix", "compile", "--warnings-as-errors")
    ebin = "_build/prod/lib/bex_probe/ebin"
    erlang = run("erl", "-noshell", "-pa", ebin, "-eval",
                 'io:format("~s",[bex_probe:check()]),halt().')
    elixir = run("elixir", "-pa", ebin, "-pa", "_build/prod/lib/bex_mix/ebin",
                 "-e", 'IO.puts(BexMix.answer())')
    check_result(erlang)
    check_result(elixir)
    output = {"pass": True, "phase": phase, "expected": expected,
              "rebar_versions": versions, "erlang_result": erlang,
              "elixir_result": elixir, "otp_root": str(root),
              "scope": "network-off generated-image fixture; no hosted acceptance"}
    Path("qa-output").mkdir(exist_ok=True)
    data = json.dumps(output, indent=2, ensure_ascii=False) + "\n"
    Path("qa-output", phase + ".json").write_text(data)
    print(data, end="")


if __name__ == "__main__":
    main(sys.argv[1])
