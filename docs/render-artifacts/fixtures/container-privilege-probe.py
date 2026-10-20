import os, socket, json, stat, http.server
CAPS = ("chown dac_override dac_read_search fowner fsetid kill setgid setuid setpcap linux_immutable "
        "net_bind_service net_broadcast net_admin net_raw ipc_lock ipc_owner sys_module sys_rawio sys_chroot "
        "sys_ptrace sys_pacct sys_admin sys_boot sys_nice sys_resource sys_time sys_tty_config mknod lease "
        "audit_write audit_control setfcap mac_override mac_admin syslog wake_alarm block_suspend audit_read "
        "perfmon bpf checkpoint_restore").split()
def caps(h):
    v = int(h, 16)
    return [CAPS[i] if i < len(CAPS) else str(i) for i in range(64) if v >> i & 1]
def rd(p):
    try:
        return open(p).read().strip()
    except Exception as e:
        return "ERR " + type(e).__name__ + ": " + str(e)
def try_(f):
    try:
        f(); return "ok"
    except Exception as e:
        return "ERR " + type(e).__name__ + ": " + str(e)
def child(f):
    pid = os.fork()
    if pid == 0:
        try:
            f(); os._exit(0)
        except PermissionError: os._exit(13)
        except Exception: os._exit(1)
    return {0: "ok", 13: "EPERM"}.get(os.waitstatus_to_exitcode(os.waitpid(pid, 0)[1]), "ERR")
r = {"uid": os.getuid(), "gid": os.getgid(), "uname": " ".join(os.uname()), "proc_version": rd("/proc/version")}
for l in rd("/proc/self/status").splitlines():
    k, _, v = l.partition(":")
    if k in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"):
        r[k] = caps(v.strip())
    elif k in ("NoNewPrivs", "Seccomp", "Seccomp_filters"):
        r[k] = v.strip()
for k, p in {"uid_map": "/proc/self/uid_map", "gid_map": "/proc/self/gid_map",
             "ip_unprivileged_port_start": "/proc/sys/net/ipv4/ip_unprivileged_port_start",
             "ping_group_range": "/proc/sys/net/ipv4/ping_group_range", "apparmor": "/proc/self/attr/current",
             "cgroup": "/proc/self/cgroup"}.items():
    r[k] = rd(p)
r["root_mount"] = [l for l in rd("/proc/self/mountinfo").splitlines() if " / / " in l][:1]
f = "/tmp/probe-file"
open(f, "w").close()
r["t_chown_to_101"] = try_(lambda: os.chown(f, 101, 101))
r["t_chmod_0600_after_chown(fowner)"] = try_(lambda: os.chmod(f, 0o600))
r["t_root_write_other_owner_0600(dac_override)"] = try_(lambda: open(f, "a").close())
r["t_fowner_chmod_other_owner"] = try_(lambda: os.chmod(f, 0o644))
r["t_setgroups_setgid_setuid_101"] = child(lambda: (os.setgroups([]), os.setgid(101), os.setuid(101)))
def bind(port):
    s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1); s.bind(("0.0.0.0", port)); s.close()
r["t_root_bind_80"] = try_(lambda: bind(80))
r["t_uid101_bind_81"] = child(lambda: (os.setgroups([]), os.setgid(101), os.setuid(101), bind(81)))
r["t_raw_socket(net_raw)"] = try_(lambda: socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.IPPROTO_ICMP).close())
r["t_ping_socket"] = try_(lambda: socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_ICMP).close())
r["t_mknod_char(mknod)"] = try_(lambda: os.mknod("/tmp/probe-null", stat.S_IFCHR | 0o600, os.makedev(1, 3)))
r["t_chroot(sys_chroot)"] = child(lambda: os.chroot("/tmp"))

import ctypes, signal, time
libc = ctypes.CDLL(None, use_errno=True)
def sysret(n, *a):
    rv = libc.syscall(n, *a); e = ctypes.get_errno()
    return "ok" if rv >= 0 else "ERR errno=%d %s" % (e, os.strerror(e))
r["protected_regular"] = rd("/proc/sys/fs/protected_regular")
r["protected_fifos"] = rd("/proc/sys/fs/protected_fifos")
d = "/var/tmp/probe-dir"; os.makedirs(d, exist_ok=True); os.chmod(d, 0o755)
f2 = d + "/f"; open(f2, "w").close()
try:
    os.chmod(f2, 0o600); os.chown(f2, 101, 101)
    r["t2_root_rw_other_owner_0600_nonsticky(dac_override)"] = try_(lambda: open(f2, "r+").close())
except Exception as e:
    r["t2_root_rw_other_owner_0600_nonsticky(dac_override)"] = "setup " + str(e)
r["t2_root_read_other_owner_0600_tmp_no_creat"] = try_(lambda: open(f, "r").close())
params = ctypes.create_string_buffer(120)
r["t2_io_uring_setup(seccomp)"] = sysret(425, 1, params)
r["t2_unshare_newuser"] = child(lambda: (_ for _ in ()).throw(PermissionError()) if libc.unshare(0x10000000) != 0 else None)
r["t2_unshare_newns(sys_admin)"] = child(lambda: (_ for _ in ()).throw(PermissionError()) if libc.unshare(0x00020000) != 0 else None)
pid = os.fork()
if pid == 0:
    os.setgroups([]); os.setgid(101); os.setuid(101); time.sleep(30); os._exit(0)
time.sleep(0.5)
r["t2_root_kill_uid101_proc(kill)"] = try_(lambda: os.kill(pid, 0))
try:
    os.kill(pid, signal.SIGKILL)
except Exception:
    pass
r["t2_setpcap_drop_bounding"] = child(lambda: (_ for _ in ()).throw(PermissionError()) if libc.prctl(24, 21, 0, 0, 0) != 0 else None)
r["t2_fsetid_note"] = "CapEff lists fsetid=%s" % ("fsetid" in r.get("CapEff", []))
body = json.dumps(r, indent=1, sort_keys=True).encode()
print("BEXPROBE " + json.dumps(r, sort_keys=True), flush=True)
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.send_header("Content-Type", "application/json"); self.end_headers(); self.wfile.write(body)
    do_HEAD = do_GET
http.server.HTTPServer(("0.0.0.0", int(os.environ.get("PORT", "10000"))), H).serve_forever()
