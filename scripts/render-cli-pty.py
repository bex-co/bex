#!/usr/bin/env python3
"""Run an interactive command in a bounded, transcript-free pseudo-terminal."""

from __future__ import annotations

import argparse
import errno
import fcntl
import os
import pty
import re
import select
import signal
import struct
import sys
import tempfile
import time
import termios
from dataclasses import dataclass


NAME_RE = re.compile(r"^[a-zA-Z][a-zA-Z0-9_-]*$")


@dataclass(frozen=True)
class Expectation:
    name: str
    marker: bytes
    response: bytes | None


def named_values(values: list[str], flag: str) -> dict[str, str]:
    parsed: dict[str, str] = {}
    for value in values:
        name, separator, content = value.partition("=")
        if not separator or not NAME_RE.fullmatch(name) or not content:
            raise ValueError(f"{flag} must use NAME=VALUE with a non-empty value")
        if name in parsed:
            raise ValueError(f"duplicate {flag} name: {name}")
        parsed[name] = content
    return parsed


def parse_args() -> tuple[argparse.Namespace, list[Expectation]]:
    parser = argparse.ArgumentParser(
        description=(
            "Run a command with TTY-backed standard streams, bounded marker waits, "
            "and no raw terminal transcript."
        )
    )
    parser.add_argument("--timeout", type=float, default=30.0, help="overall deadline in seconds")
    parser.add_argument(
        "--exit-timeout",
        type=float,
        default=None,
        help="shorter child-exit deadline after every marker has matched",
    )
    parser.add_argument("--rows", type=int, default=40, help="PTY row count")
    parser.add_argument("--cols", type=int, default=120, help="PTY column count")
    parser.add_argument(
        "--expect",
        action="append",
        default=[],
        metavar="NAME=MARKER",
        help="ordered marker to require; repeat for a sequence",
    )
    parser.add_argument(
        "--send-after",
        action="append",
        default=[],
        metavar="NAME=TEXT",
        help="text to send after the expectation with NAME is observed",
    )
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()

    if args.command[:1] == ["--"]:
        args.command = args.command[1:]
    if not args.command:
        parser.error("a command is required after --")
    if args.timeout <= 0:
        parser.error("--timeout must be positive")
    if args.exit_timeout is not None and args.exit_timeout <= 0:
        parser.error("--exit-timeout must be positive")
    if not 1 <= args.rows <= 999 or not 1 <= args.cols <= 999:
        parser.error("--rows and --cols must be between 1 and 999")

    try:
        expected = named_values(args.expect, "--expect")
        responses = named_values(args.send_after, "--send-after")
    except ValueError as error:
        parser.error(str(error))
    unknown_responses = responses.keys() - expected.keys()
    if unknown_responses:
        parser.error(f"--send-after has no matching --expect: {sorted(unknown_responses)[0]}")

    expectations = [
        Expectation(
            name=name,
            marker=marker.encode(),
            response=responses.get(name, "").encode() if name in responses else None,
        )
        for name, marker in expected.items()
    ]
    return args, expectations


def set_window_size(fd: int, rows: int, cols: int) -> None:
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))


def terminate_group(pid: int) -> None:
    for sig, grace in ((signal.SIGTERM, 0.5), (signal.SIGKILL, 0.0)):
        try:
            os.killpg(pid, sig)
        except ProcessLookupError:
            return
        deadline = time.monotonic() + grace
        while grace and time.monotonic() < deadline:
            try:
                waited, _ = os.waitpid(pid, os.WNOHANG)
            except ChildProcessError:
                # The main loop may already have reaped the group leader while
                # a descendant still holds the PTY open.
                waited = pid
            if waited == pid:
                break
            time.sleep(0.02)


def normalized_exit_status(wait_status: int) -> int:
    if os.WIFEXITED(wait_status):
        return os.WEXITSTATUS(wait_status)
    if os.WIFSIGNALED(wait_status):
        return 128 + os.WTERMSIG(wait_status)
    return 1


def run(args: argparse.Namespace, expectations: list[Expectation]) -> int:
    child_environment = os.environ.copy()
    child_environment["TERM"] = "xterm-256color"
    child_environment.pop("CI", None)
    child_environment.pop("RENDER_OUTPUT", None)

    with tempfile.TemporaryDirectory(prefix="bex-render-cli-") as config_dir:
        child_environment["RENDER_CLI_CONFIG_PATH"] = os.path.join(config_dir, "cli.yaml")
        pid, master_fd = pty.fork()
        if pid == 0:
            try:
                set_window_size(0, args.rows, args.cols)
                os.execvpe(args.command[0], args.command, child_environment)
            except FileNotFoundError:
                os._exit(127)
            except PermissionError:
                os._exit(126)
            except BaseException:
                os._exit(125)

        set_window_size(master_fd, args.rows, args.cols)
        deadline = time.monotonic() + args.timeout
        exit_deadline_set = False
        pending = 0
        retained = bytearray()
        child_status: int | None = None
        eof = False

        try:
            while child_status is None or not eof:
                now = time.monotonic()
                if now >= deadline:
                    waiting_for = expectations[pending].name if pending < len(expectations) else "child-exit"
                    print(f"FAIL pty timeout waiting-for={waiting_for}", file=sys.stderr)
                    terminate_group(pid)
                    try:
                        os.waitpid(pid, 0)
                    except ChildProcessError:
                        pass
                    return 124

                if child_status is None:
                    waited, status = os.waitpid(pid, os.WNOHANG)
                    if waited == pid:
                        child_status = status

                readable, _, _ = select.select([master_fd] if not eof else [], [], [], min(0.05, deadline - now))
                if readable:
                    try:
                        chunk = os.read(master_fd, 65536)
                    except OSError as error:
                        if error.errno != errno.EIO:
                            raise
                        chunk = b""
                    if not chunk:
                        eof = True
                    elif pending < len(expectations):
                        retained.extend(chunk)
                        while pending < len(expectations):
                            expectation = expectations[pending]
                            marker_at = retained.find(expectation.marker)
                            if marker_at < 0:
                                keep = max(len(expectation.marker) - 1, 4096)
                                if len(retained) > keep:
                                    del retained[:-keep]
                                break
                            del retained[: marker_at + len(expectation.marker)]
                            print(f"PASS pty marker {expectation.name}")
                            if expectation.response is not None:
                                os.write(master_fd, expectation.response)
                            pending += 1
                            if (
                                pending == len(expectations)
                                and args.exit_timeout is not None
                                and not exit_deadline_set
                            ):
                                deadline = min(deadline, time.monotonic() + args.exit_timeout)
                                exit_deadline_set = True

                if child_status is not None and eof:
                    break
        except KeyboardInterrupt:
            print("FAIL pty interrupted", file=sys.stderr)
            terminate_group(pid)
            try:
                os.waitpid(pid, 0)
            except ChildProcessError:
                pass
            return 130
        except Exception:
            print("FAIL pty internal-io", file=sys.stderr)
            terminate_group(pid)
            try:
                os.waitpid(pid, 0)
            except ChildProcessError:
                pass
            return 125
        finally:
            os.close(master_fd)

    if child_status is None:
        _, child_status = os.waitpid(pid, 0)
    exit_status = normalized_exit_status(child_status)
    if pending < len(expectations):
        print(f"FAIL pty marker {expectations[pending].name} missing", file=sys.stderr)
        return exit_status or 1
    if exit_status:
        print(f"FAIL pty child-exit status={exit_status}", file=sys.stderr)
    else:
        print("PASS pty child-exit status=0")
    return exit_status


def main() -> int:
    args, expectations = parse_args()
    return run(args, expectations)


if __name__ == "__main__":
    raise SystemExit(main())
