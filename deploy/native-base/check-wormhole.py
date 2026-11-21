"""A real magic-wormhole round trip through a local mailbox and transit relay.

Render pre-installs magic-wormhole on every native runtime so a disk file moves
through Shell (w6/m549). The client under test is the candidate's own packaged
/usr/bin/wormhole. The relay pair is the suite's own Debian packages, unpacked
into the directory named by the only argument and never installed, so the
client keeps exactly the files that were published. Runs without network.
"""
import hashlib
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import time

WORMHOLE = "/usr/bin/wormhole"
SYSTEM_PYTHON = "/usr/bin/python3"
MAILBOX_PORT, TRANSIT_PORT = 4000, 4001
# A space and non-ASCII characters, as a tenant's disk file may carry.
NAME = "disk backup naïve ☃.bin"


def run(*args):
    result = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    if result.returncode:
        raise RuntimeError((args, result.returncode, result.stdout.decode(errors="replace"), result.stderr.decode(errors="replace")))
    return result.stdout.decode()


def upstream_version(package):
    version = run("dpkg-query", "-W", "-f=${Version}", package).strip()
    return version.split(":", 1)[-1].rsplit("-", 1)[0]


def wait_listening(port, process, log):
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("relay server exited %s: %s" % (process.returncode, log.read_text(errors="replace")))
        try:
            socket.create_connection(("127.0.0.1", port), timeout=1).close()
            return
        except OSError:
            time.sleep(0.2)
    raise RuntimeError("relay server never listened on %d: %s" % (port, log.read_text(errors="replace")))


def client(*args):
    # Only the relay URL and helper differ from a tenant's Shell; --no-listen
    # on both ends sends every byte through the transit relay, the path a
    # service behind NAT and a workstation behind NAT take.
    return (WORMHOLE, "--relay-url", "ws://127.0.0.1:%d/v1" % MAILBOX_PORT,
            "--transit-helper", "tcp:127.0.0.1:%d" % TRANSIT_PORT) + args + ("--hide-progress", "--no-listen")


def transfer(root, env, send_code, receive_code, payload):
    outbox, inbox = root / ("out-" + send_code), root / ("in-" + send_code)
    outbox.mkdir()
    inbox.mkdir()
    (outbox / NAME).write_bytes(payload)
    sender = subprocess.Popen(client("send", "--code", send_code, NAME), cwd=outbox, env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    try:
        received = subprocess.run(client("receive", "--accept-file", receive_code), cwd=inbox, env=env,
                                  stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=180)
        sent = sender.communicate(timeout=60)[0]
    finally:
        if sender.poll() is None:
            sender.kill()
            sender.wait()
    return sender.returncode, sent.decode(errors="replace"), received.returncode, received.stdout.decode(errors="replace"), inbox


def main():
    servers = Path(sys.argv[1]).resolve() / "usr/lib/python3/dist-packages"
    # A primary runtime's PATH and interpreter never replace the packaged client.
    if shutil.which("wormhole") != WORMHOLE:
        raise RuntimeError("wormhole on PATH is %r, not the packaged %s" % (shutil.which("wormhole"), WORMHOLE))
    shebang = Path(WORMHOLE).read_bytes().split(b"\n", 1)[0]
    if not shebang.startswith(b"#!") or shebang[2:].strip() != SYSTEM_PYTHON.encode():
        raise RuntimeError("wormhole does not run on the system interpreter: %r" % shebang)
    expected = "magic-wormhole " + upstream_version("magic-wormhole")
    if run("wormhole", "--version").strip() != expected:
        raise RuntimeError("wormhole --version is not the packaged " + expected)

    # The client keeps the image's own environment; only the servers see the
    # unpacked packages.
    client_env = dict(os.environ)
    server_env = dict(client_env, PYTHONPATH=str(servers))
    with tempfile.TemporaryDirectory(prefix="bex-native-wormhole-") as directory:
        root = Path(directory)
        processes = []
        try:
            for name, port, options in (("wormhole-mailbox", MAILBOX_PORT, ("--channel-db", str(root / "channel.sqlite"))),
                                        ("transitrelay", TRANSIT_PORT, ())):
                log = root / (name + ".log")
                with log.open("wb") as output:
                    process = subprocess.Popen((SYSTEM_PYTHON, "-m", "twisted", name, "--port", "tcp:%d:interface=127.0.0.1" % port) + options,
                                               cwd=root, env=server_env, stdout=output, stderr=subprocess.STDOUT)
                processes.append(process)
                wait_listening(port, process, log)

            payload = os.urandom(8 << 20)
            sent, sent_log, received, received_log, inbox = transfer(root, client_env, "7-bex-disk", "7-bex-disk", payload)
            files = sorted(p.name for p in inbox.iterdir())
            if sent or received or files != [NAME]:
                raise RuntimeError(("round trip failed", sent, sent_log, received, received_log, files))
            if hashlib.sha256((inbox / NAME).read_bytes()).hexdigest() != hashlib.sha256(payload).hexdigest():
                raise RuntimeError("received bytes differ from the sent file")

            # A mistyped code must fail key confirmation on both ends and write nothing.
            sent, sent_log, received, received_log, inbox = transfer(root, client_env, "8-bex-disk", "8-bex-wrong", b"secret\n")
            if not sent or not received or any(inbox.iterdir()):
                raise RuntimeError(("a wrong code was not refused", sent, sent_log, received, received_log))
        finally:
            for process in processes:
                process.terminate()
            for process in processes:
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
    print("%s sent %d bytes through a local transit relay and refused a wrong code without network" % (expected, len(payload)))


if __name__ == "__main__":
    main()
