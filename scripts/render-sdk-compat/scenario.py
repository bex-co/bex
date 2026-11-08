"""Drive bex sandboxes with the unmodified Render Python SDK (w32/m284).

Each step prints `PASS <step>` or `FAIL <step>: <error>`; the exit status is
the number of failed steps. Every sandbox it creates is terminated at the end.
Reads RENDER_API_KEY (a bex bearer token), RENDER_WORKSPACE_ID and
BEX_API_URL (the bare origin; the SDK appends /v1 itself).
"""

import os
import sys
import tempfile
import time
import traceback

from render import Render
from render.experimental.sandbox import SandboxFileNotFoundError
from render.experimental.sandbox.types import SandboxExecExit, SandboxExecOutput

failures = 0
created = []


def step(name, fn):
    global failures
    try:
        detail = fn()
        print(f"PASS {name}" + (f" ({detail})" if detail else ""), flush=True)
    except Exception as exc:  # noqa: BLE001 - every failure is a grade
        failures += 1
        print(f"FAIL {name}: {type(exc).__name__}: {exc}", flush=True)
        if os.environ.get("SDK_COMPAT_TRACE"):
            traceback.print_exc()


render = Render(base_url=os.environ["BEX_API_URL"])
sandboxes = render.sandboxes
state = {}


def create():
    box = sandboxes.create(plan="1c-2g", timeout_seconds=900, network_policy="deny-all")
    created.append(box.id)
    state["id"] = box.id
    return box.id


def wait():
    box = sandboxes.wait_until_running(state["id"])
    assert box.status == "running", box.status
    return box.status


def get():
    box = sandboxes.from_id(state["id"])
    assert box.id == state["id"]
    return f"region={box.region}"


def listed():
    page = sandboxes.list(limit=100)
    ids = [item.id for item in page]
    assert state["id"] in ids, ids
    return f"{len(ids)} listed"


def run(command):
    out, code = [], None
    for event in sandboxes.exec(state["id"], command):
        if isinstance(event, SandboxExecOutput):
            out.append(event.data)
        elif isinstance(event, SandboxExecExit):
            code = event.exit_code
    return "".join(out), code


def exec_output():
    out, code = run("printf sdk-ok; echo err >&2; exit 3")
    assert "sdk-ok" in out and code == 3, (out, code)
    return "exit 3"


def exec_quiet():
    # Longer than the SDK's 45 s read timeout: it lives only on keep-alives.
    started = time.monotonic()
    out, code = run("sleep 50; printf done")
    assert out == "done" and code == 0, (out, code)
    return f"{time.monotonic() - started:.0f}s"


def files():
    with tempfile.TemporaryDirectory() as tmp:
        src = os.path.join(tmp, "in.txt")
        with open(src, "w") as f:
            f.write("payload-1\n")
        sandboxes.copy_to(state["id"], src, "/tmp/sdk-compat.txt")
        back = os.path.join(tmp, "out.txt")
        sandboxes.copy_from(state["id"], "/tmp/sdk-compat.txt", back)
        with open(back) as f:
            got = f.read()
        assert got == "payload-1\n", got
    return "round trip"


def missing_file():
    with tempfile.TemporaryDirectory() as tmp:
        try:
            sandboxes.copy_from(state["id"], "/tmp/does-not-exist-sdk", os.path.join(tmp, "x"))
        except SandboxFileNotFoundError:
            return "SandboxFileNotFoundError"
    raise AssertionError("a missing remote file did not raise SandboxFileNotFoundError")


def terminate():
    sandboxes.terminate(state["id"])
    created.remove(state["id"])
    return None


step("create", create)
if "id" in state:
    step("wait_until_running", wait)
    step("from_id", get)
    step("list", listed)
    step("exec output and exit code", exec_output)
    step("exec quiet for 50s", exec_quiet)
    step("copy_to / copy_from", files)
    step("copy_from missing file", missing_file)
    step("terminate", terminate)
for leftover in created:
    try:
        sandboxes.terminate(leftover)
    except Exception as exc:  # noqa: BLE001
        print(f"cleanup: could not terminate {leftover}: {exc}", file=sys.stderr)
sys.exit(failures)
