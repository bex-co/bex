"""Bounded own host resources. No process, watcher or filesystem action on import."""
import datetime
import json
import math
import os
from pathlib import Path
import selectors
import signal
import subprocess
import threading
import time

MIB = 1024 * 1024
FLOOR = 1024 ** 3
MARGIN = 128 * MIB
CAP = 32 * MIB
SOFT = 24 * MIB
START_FREE = FLOOR + MARGIN + CAP
CAPS = {'source': 3*MIB, 'private': 3*MIB, 'scratch': 3*MIB,
        'entities': 4*MIB, 'outputs': 11*MIB, 'meta': 8*MIB}


class Stop(RuntimeError):
    pass


def encoded(value):
    return (json.dumps(value, ensure_ascii=False, separators=(',', ':')) + '\n').encode()


def free_bytes(root):
    stat = os.statvfs(root)
    return stat.f_bavail * stat.f_frsize


def component(path):
    parts = Path(path).parts
    if parts[0] in ('source', 'private', 'scratch'):
        return parts[0]
    if parts[:2] == ('evidence', 'entities'):
        return 'entities'
    if parts[:2] == ('evidence', 'outputs'):
        return 'outputs'
    return 'meta'


def measured(root):
    totals = {name: 0 for name in CAPS}
    for parent, dirs, files in os.walk(root, followlinks=False):
        for name in dirs + files:
            path = Path(parent) / name
            try:
                stat = path.lstat()
            except FileNotFoundError:
                continue  # An exact own temporary file was atomically renamed/deleted.
            totals[component(path.relative_to(root))] += max(stat.st_size, stat.st_blocks * 512)
    return totals


class Budget:
    def __init__(self, root, free=free_bytes, clock=time.monotonic):
        self.root, self.free, self.clock = Path(root), free, clock
        self.started = clock()
        self.lock = threading.RLock()
        self.done = threading.Event()
        self.error = None
        self.thread = None
        self.external_jars = set()

    def register_jar(self, path, expected_parent):
        import re
        path, parent = Path(path), Path(expected_parent).resolve(strict=True)
        if path.parent.resolve(strict=True) != parent or not re.fullmatch(r'qa-session-[a-f0-9]{12}\.jar', path.name):
            raise Stop('not an exact own QA logout handle')
        if path.is_symlink() or path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077:
            raise Stop('own QA logout handle permissions differ')
        with self.lock:
            self.external_jars.add(path)

    def observation(self):
        counts = measured(self.root)
        for path in self.external_jars:
            if path.exists():
                try:
                    stat = path.lstat()
                except FileNotFoundError:
                    continue  # Exact own logout removed it between observation calls.
                if path.is_symlink():
                    raise Stop('own QA handle became a symlink')
                counts['private'] += max(stat.st_size, stat.st_blocks * 512)
        return counts, self.free(self.root)

    def check(self, cleanup=False):
        if not cleanup and self.error:
            raise Stop(self.error)
        counts, available = self.observation()
        if not cleanup and (any(counts[name] > limit for name, limit in CAPS.items()) or sum(counts.values()) >= CAP):
            raise Stop('owned hard component/32MiB bound reached')
        if not cleanup and (available <= FLOOR + MARGIN or sum(counts.values()) >= SOFT):
            raise Stop('physical1GiB+128MiB or own24MiB stop margin reached')
        if not cleanup and self.clock() - self.started > 5400:
            raise Stop('whole transport5400s stop deadline')
        return counts, available

    def write(self, relative, raw, cleanup=False):
        relative = Path(relative)
        if relative.is_absolute() or '..' in relative.parts:
            raise Stop('write outside owned root')
        with self.lock:
            counts, available = self.check(cleanup)
            extra = max(4096, math.ceil(len(raw)/4096)*4096) + 8192
            ceiling = CAP if cleanup else SOFT
            if counts[component(relative)] + extra > CAPS[component(relative)] or sum(counts.values()) + extra >= ceiling:
                raise Stop('synchronous bounded write refused')
            if available - extra < FLOOR:
                raise Stop('physical1GiB floor forbids artifact write; continue remote cleanup in memory')
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            if path.is_symlink() or any(parent.is_symlink() for parent in path.parents if parent != self.root.parent):
                raise Stop('owned output path is symlinked')
            temporary = path.with_name(path.name + '.tmp')
            created = False
            try:
                with temporary.open('xb') as stream:
                    created = True; stream.write(raw)
                temporary.chmod(0o600)
                temporary.replace(path)
            except BaseException:
                if created:
                    temporary.unlink(missing_ok=True)
                raise

    def json(self, relative, value, cleanup=False):
        self.write(relative, encoded(value), cleanup)

    def permit_external_write(self, name, maximum, cleanup=False):
        """Preflight known subprocess file peaks; watcher is still observational."""
        with self.lock:
            counts,available=self.check(cleanup)
            charge=max(4096,math.ceil(maximum/4096)*4096)+8192
            if name not in CAPS or counts[name]+charge>CAPS[name] or sum(counts.values())+charge>=(CAP if cleanup else SOFT):
                raise Stop('known external write peak does not fit')
            if available-charge<FLOOR:
                raise Stop('known external write peak reaches physical1GiB floor')

    def start(self):
        def watch():
            deadline = self.clock()
            while not self.done.is_set():
                try:
                    with self.lock:
                        counts, available = self.check()
                        row = {'elapsed': round(self.clock()-self.started, 3), 'physical_free': available,
                               'owned_bytes': counts, 'not_a_kernel_quota': True}
                        path = self.root / 'evidence/capacity.jsonl'
                        raw = encoded(row)
                        if path.exists() and path.stat().st_size + len(raw) > MIB:
                            raise Stop('capacity receipt1MiB bound')
                        if sum(counts.values()) + 8192 >= SOFT or counts['meta'] + 8192 > CAPS['meta']:
                            raise Stop('capacity receipt would cross own stop bound')
                        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                        with path.open('ab') as output:
                            output.write(raw)
                except BaseException as error:
                    self.error = type(error).__name__ + ': ' + str(error)
                    return
                deadline += 1
                self.done.wait(max(0, deadline-self.clock()))
        self.thread = threading.Thread(target=watch, name='m83-owned-capacity-1s', daemon=True)
        self.thread.start()

    def close(self):
        self.done.set()
        if self.thread:
            self.thread.join(timeout=2)
        if self.thread and self.thread.is_alive():
            self.error = (self.error + '; ' if self.error else '') + 'capacity monitor did not stop'
        return self.error


# Same exited-leader/pipe-holder cleanup rule independently reviewed for pnpmV4.
def group_exists(pgid):
    try:os.killpg(pgid,0);return True
    except ProcessLookupError:return False
    except PermissionError:return True  # Unknown/pending is never absence.


def terminate_owned_group(process):
    pgid=process.pid
    if pgid<=1 or pgid==os.getpgrp():raise Stop('invalid owned process group')
    leader_exited=process.poll() is not None
    present=group_exists(pgid)
    signal_error=None
    try:os.killpg(pgid,signal.SIGKILL)
    except ProcessLookupError:pass
    except PermissionError:signal_error='EPERM; absence must still be proven'
    try:process.wait(timeout=2)
    except subprocess.TimeoutExpired:raise Stop('direct child could not be reaped; owned PGID='+str(pgid))
    deadline=time.monotonic()+2
    while group_exists(pgid) and time.monotonic()<deadline:time.sleep(.01)
    if group_exists(pgid):raise Stop('owned process group absence unproved; PGID='+str(pgid))
    return {'pgid':pgid,'direct_child_reaped':True,'group_absent':True,'leader_exit':process.returncode,'background_descendants':leader_exited and present,'signal_error':signal_error}


class Runner:
    def __init__(self, budget):
        self.budget = budget
        self.active = {}
        self.lock = threading.RLock()

    def capture(self, argv, timeout, env=None, input_bytes=None, limit=2*MIB, cleanup=False):
        self.budget.check(cleanup)
        if input_bytes is not None and len(input_bytes) > 16384:
            raise Stop('private stdin16KiB bound')
        started, problem = time.monotonic(), None
        process = subprocess.Popen(argv, env=env, stdin=subprocess.PIPE if input_bytes is not None else subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        with self.lock:
            self.active[process.pid] = process
        selector = selectors.DefaultSelector()
        output = {'stdout': bytearray(), 'stderr': bytearray()}
        cleanup_result = None
        try:
            if input_bytes is not None:
                try:
                    process.stdin.write(input_bytes); process.stdin.close()
                except BrokenPipeError:
                    process.stdin.close()
            for name, pipe in [('stdout', process.stdout), ('stderr', process.stderr)]:
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            while selector.get_map() or process.poll() is None:
                try:
                    self.budget.check(cleanup)
                except Stop:
                    problem = 'resource'; break
                if time.monotonic() - started >= timeout:
                    problem = 'timeout'; break
                if not selector.get_map():
                    time.sleep(.01); continue
                for key, _ in selector.select(.02):
                    raw = os.read(key.fileobj.fileno(), 16384)
                    if not raw:
                        selector.unregister(key.fileobj); continue
                    room = limit - sum(len(value) for value in output.values())
                    output[key.data].extend(raw[:max(0, room)])
                    if len(raw) > room:
                        problem = 'output'; break
                if problem:
                    break
        finally:
            try:
                cleanup_result = terminate_owned_group(process)
            finally:
                selector.close(); process.stdout.close(); process.stderr.close()
                with self.lock:
                    if cleanup_result and cleanup_result['group_absent']:
                        self.active.pop(process.pid, None)
        if cleanup_result['background_descendants'] and problem is None:
            problem = 'unexpected-background-descendant'
        return {'exit_code': 124 if problem else process.returncode, 'complete': problem is None,
                'bound': problem, 'signal': -process.returncode if process.returncode < 0 else None,
                'elapsed_seconds': round(time.monotonic()-started, 3), 'group_cleanup': cleanup_result,
                **{name: bytes(value) for name, value in output.items()}}

    def stop_all(self):
        results = []
        with self.lock:
            processes = list(self.active.values())
        for process in processes:
            result = terminate_owned_group(process); results.append(result)
            with self.lock:
                self.active.pop(process.pid, None)
        return results
