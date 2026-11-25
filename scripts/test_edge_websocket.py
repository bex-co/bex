"""A WebSocket through the tenant edge outlives the edge's read deadline (w6/m639).

Both tenant entrypoints set Traefik's respondingTimeouts.readTimeout (6000 s,
w8/207) to bound how long a client may take to send a request. Render enforces
no maximum duration for a WebSocket (render.com/docs/websocket), and the edge
keeps that bound off an upgraded connection only because of how the pinned
Traefik behaves: Go's HTTP server clears a connection's deadlines when the
reverse proxy hijacks it after the backend's 101, and the edge advertises no
HTTP/2 WebSockets (RFC 8441), so every WebSocket takes that path. A Traefik
upgrade that changed either would cut every tenant WebSocket at 100 minutes,
so this runs the image base/values/traefik.values.yaml pins.

The egress meter plugin is loaded as a tenant route runs it, interpreted and
wrapping the hijacked connection, and both entrypoints get a short read
deadline. A pinged WebSocket on each must outlive it four times over, while a
slow request body on the same entrypoint is still cut at it: the control that
proves the deadline is in force. The edge must also not offer HTTP/2
WebSockets, which would bypass the hijack (ADR022 § Bounded request reading at
the tenant edge).

Run: python3 -m unittest scripts/test_edge_websocket.py -v
Requires Docker and yq v4; no cluster or credentials. BEX_TRAEFIK_IMAGE tries a
candidate image before its pin.
"""

import base64
import concurrent.futures
import hashlib
import http.server
import os
from pathlib import Path
import shutil
import socket
import ssl
import struct
import subprocess
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request
import uuid


ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT / "deploy/gitops/charts/traefik-plugins/websocketegress"
# The ConfigMap the edge mounts carries exactly these (charts/traefik-plugins).
PLUGIN_FILES = (".traefik.yml", "go.mod", "websocketegress.go")
READ_TIMEOUT = 3
HOLD = 4 * READ_TIMEOUT
APP_ID = "srv-edge-websocket-test"
GUID = b"258EAFA5-E914-47DA-95CA-C5AB0DC11B85"

def dynamic_config(port):
    """The file provider's routes: both entrypoints, through the meter, to the app."""
    return f"""
http:
  middlewares:
    meter:
      plugin:
        bexWebsocketEgress:
          appId: {APP_ID}
          metricsAddr: ":9101"
  routers:
    web:
      rule: PathPrefix(`/`)
      entryPoints: [web]
      middlewares: [meter]
      service: backend
    websecure:
      rule: PathPrefix(`/`)
      entryPoints: [websecure]
      middlewares: [meter]
      service: backend
      tls: {{}}
  services:
    backend:
      loadBalancer:
        servers:
          - url: http://host.docker.internal:{port}
"""


def pinned_image():
    tag = subprocess.check_output(
        ["yq", "-r", ".image.tag", "deploy/gitops/base/values/traefik.values.yaml"], cwd=ROOT, text=True).strip()
    return os.environ.get("BEX_TRAEFIK_IMAGE", "docker.io/traefik:" + tag)


def frame(op, data, mask):
    """One final frame; a client masks, the echo server does not."""
    n = len(data)
    head = bytes([0x80 | op])
    bit = 0x80 if mask else 0
    head += bytes([bit | n]) if n < 126 else bytes([bit | 126]) + struct.pack(">H", n)
    if not mask:
        return head + data
    key = os.urandom(4)
    return head + key + bytes(b ^ key[i % 4] for i, b in enumerate(data))


def read_frame(read):
    head = read(2)
    op, n = head[0] & 0x0F, head[1] & 0x7F
    if n == 126:
        n = struct.unpack(">H", read(2))[0]
    elif n == 127:
        n = struct.unpack(">Q", read(8))[0]
    key = read(4) if head[1] & 0x80 else b"\0\0\0\0"
    return op, bytes(b ^ key[i % 4] for i, b in enumerate(read(n)))


class Backend(http.server.BaseHTTPRequestHandler):
    """The app: a WebSocket echo (pong for ping), and a POST that counts its body."""

    protocol_version = "HTTP/1.1"

    def log_message(self, *args):
        pass

    def do_GET(self):
        if self.headers.get("Upgrade", "").lower() != "websocket":
            self.reply(200, b"ok")
            return
        accept = base64.b64encode(hashlib.sha1(self.headers["Sec-WebSocket-Key"].encode() + GUID).digest())
        self.send_response(101)
        self.send_header("Upgrade", "websocket")
        self.send_header("Connection", "Upgrade")
        self.send_header("Sec-WebSocket-Accept", accept.decode())
        self.end_headers()
        self.close_connection = True

        def read(n):
            data = self.rfile.read(n)
            if len(data) < n:
                raise EOFError
            return data

        try:
            while True:
                op, data = read_frame(read)
                if op == 8:
                    self.wfile.write(frame(8, data[:2], mask=False))
                    return
                self.wfile.write(frame(10 if op == 9 else op, data, mask=False))
        except (EOFError, OSError):
            return

    def do_POST(self):
        want, got = int(self.headers.get("Content-Length", "0")), 0
        try:
            while got < want:
                chunk = self.rfile.read1(want - got)
                if not chunk:
                    break
                got += len(chunk)
        except OSError:
            pass
        self.server.bodies.append((want, got))
        self.reply(200, b"got=%d" % got)

    def reply(self, status, body):
        self.send_response(status)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def connect(port, tls, alpn=None):
    sock = socket.create_connection(("127.0.0.1", port), timeout=10)
    if tls:
        context = ssl.create_default_context()
        # Traefik's own default certificate: the TLS layer is under test, not trust.
        context.check_hostname, context.verify_mode = False, ssl.CERT_NONE
        if alpn:
            context.set_alpn_protocols(alpn)
        sock = context.wrap_socket(sock, server_hostname="edge.test")
    return sock


class Reader:
    """Exact-length reads from a socket, keeping what arrives early."""

    def __init__(self, sock, buf=b""):
        self.sock, self.buf = sock, buf

    def read(self, n):
        while len(self.buf) < n:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise EOFError("the edge closed the connection")
            self.buf += chunk
        data, self.buf = self.buf[:n], self.buf[n:]
        return data


class WebSocket:
    def __init__(self, port, tls):
        self.sock = connect(port, tls)
        key = base64.b64encode(os.urandom(16)).decode()
        self.sock.sendall(
            f"GET /ws HTTP/1.1\r\nHost: edge.test\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n".encode())
        buf = b""
        while b"\r\n\r\n" not in buf:
            chunk = self.sock.recv(4096)
            if not chunk:
                raise AssertionError(f"the edge closed the handshake: {buf!r}")
            buf += chunk
        head, rest = buf.split(b"\r\n\r\n", 1)
        self.status = head.split(b"\r\n", 1)[0].decode()
        self.reader = Reader(self.sock, rest)

    def call(self, op, data):
        self.sock.sendall(frame(op, data, mask=True))
        return read_frame(self.reader.read)


def hold(port, tls):
    """Ping every 0.5 s for HOLD seconds; return (seconds held, pongs, echo)."""
    ws = WebSocket(port, tls)
    assert " 101 " in ws.status, ws.status
    start, pongs = time.monotonic(), 0
    try:
        while time.monotonic() - start < HOLD:
            payload = b"ping %d" % pongs
            op, data = ws.call(9, payload)
            assert (op, data) == (10, payload), (op, data)
            pongs += 1
            time.sleep(0.5)
        held = time.monotonic() - start
        echo = ws.call(1, b"still here")
        ws.sock.sendall(frame(8, struct.pack(">H", 1000), mask=True))
        return held, pongs, echo
    except (EOFError, OSError) as error:
        raise AssertionError(f"the WebSocket ended after {time.monotonic() - start:.1f}s "
                             f"(read deadline {READ_TIMEOUT}s), {pongs} pongs: {error!r}") from error
    finally:
        ws.sock.close()


def http2_settings(port):
    """The SETTINGS the edge opens an HTTP/2 connection with: (ALPN, {id: value})."""
    with connect(port, tls=True, alpn=["h2"]) as sock:
        # Client preface, then an empty SETTINGS frame.
        sock.sendall(b"PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n" + b"\0\0\0\x04\0\0\0\0\0")
        reader = Reader(sock)
        head = reader.read(9)
        assert head[3] == 4, f"the edge's first HTTP/2 frame is type {head[3]}, not SETTINGS"
        payload = reader.read(int.from_bytes(head[:3], "big"))
        return sock.selected_alpn_protocol(), dict(struct.unpack(">HI", payload[i:i + 6]) for i in range(0, len(payload), 6))


def slow_body(port, tls):
    """Send a 1000-byte body at 20 B/s; return (seconds until the edge answered or closed, its answer)."""
    sock = connect(port, tls)
    start = time.monotonic()
    sock.sendall(b"POST /upload HTTP/1.1\r\nHost: edge.test\r\nContent-Length: 1000\r\n\r\n")
    sock.settimeout(0.5)
    answer = b""
    try:
        while time.monotonic() - start < HOLD:
            try:
                answer = sock.recv(4096)
                break
            except socket.timeout:
                sock.sendall(b"x" * 10)
    except OSError:
        pass
    finally:
        sock.close()
    return time.monotonic() - start, answer


class EdgeWebSocketTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.backend = http.server.ThreadingHTTPServer(("0.0.0.0", 0), Backend)
        cls.backend.daemon_threads = True
        cls.backend.bodies = []
        threading.Thread(target=cls.backend.serve_forever, daemon=True).start()
        cls.tmp = Path(tempfile.mkdtemp(prefix="bex-edge-websocket-"))
        cls.container = "bex-edge-websocket-" + uuid.uuid4().hex[:8]
        try:
            cls.start_edge()
        except BaseException:
            cls.tearDownClass()
            raise

    @classmethod
    def start_edge(cls):
        image = pinned_image()
        if subprocess.run(["docker", "image", "inspect", image], capture_output=True).returncode:
            for attempt in range(1, 6):
                if subprocess.run(["docker", "pull", "-q", image]).returncode == 0:
                    break
                if attempt == 5:
                    raise RuntimeError(f"docker pull {image} failed 5 times")
                time.sleep(attempt * 10)
        plugin = cls.tmp / "plugin"
        plugin.mkdir()
        for name in PLUGIN_FILES:
            shutil.copy(PLUGIN / name, plugin / name)
        dynamic = cls.tmp / "dynamic.yml"
        dynamic.write_text(dynamic_config(cls.backend.server_address[1]))
        timeout = f"{READ_TIMEOUT}s"
        subprocess.check_call([
            "docker", "run", "-d", "--rm", "--name", cls.container,
            "--add-host", "host.docker.internal:host-gateway",
            "-p", "127.0.0.1::8000", "-p", "127.0.0.1::8443", "-p", "127.0.0.1::9101",
            "-v", f"{plugin}:/plugins-local/src/github.com/bex-co/bex-websocket-egress:ro",
            "-v", f"{dynamic}:/etc/traefik/dynamic.yml:ro",
            image,
            # The tenant entrypoints' shape (base/values/traefik.values.yaml),
            # with a deadline short enough to outlive in seconds.
            "--entryPoints.web.address=:8000/tcp",
            "--entryPoints.websecure.address=:8443/tcp",
            "--entryPoints.websecure.http.tls=true",
            f"--entryPoints.web.transport.respondingTimeouts.readTimeout={timeout}",
            f"--entryPoints.websecure.transport.respondingTimeouts.readTimeout={timeout}",
            "--experimental.abortOnPluginFailure=true",
            "--experimental.localPlugins.bexWebsocketEgress.moduleName=github.com/bex-co/bex-websocket-egress",
            "--providers.file.filename=/etc/traefik/dynamic.yml",
        ], stdout=subprocess.DEVNULL)
        cls.ports = {}
        for port in ("8000", "8443", "9101"):
            mapped = subprocess.check_output(["docker", "port", cls.container, port + "/tcp"], text=True)
            cls.ports[port] = int(mapped.splitlines()[0].rsplit(":", 1)[1])
        deadline = time.monotonic() + 90
        while True:
            try:
                with urllib.request.urlopen(f"http://127.0.0.1:{cls.ports['8000']}/", timeout=5) as response:
                    if response.read() == b"ok":
                        return
            except urllib.error.HTTPError as error:
                error.close()  # 404 until the file provider's routers load
            except OSError:
                pass
            if time.monotonic() > deadline:
                logs = subprocess.run(["docker", "logs", cls.container], capture_output=True, text=True)
                raise RuntimeError(f"the edge never routed to the backend:\n{logs.stdout}{logs.stderr}")
            time.sleep(1)

    @classmethod
    def tearDownClass(cls):
        subprocess.run(["docker", "rm", "-f", cls.container], capture_output=True)
        cls.backend.shutdown()
        cls.backend.server_close()
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def entrypoints(self):
        return {"web": (self.ports["8000"], False), "websecure": (self.ports["8443"], True)}

    def test_a_pinged_websocket_outlives_the_read_deadline(self):
        with concurrent.futures.ThreadPoolExecutor() as pool:
            held = {name: pool.submit(hold, *target) for name, target in self.entrypoints().items()}
        for name, result in held.items():
            with self.subTest(entrypoint=name):
                seconds, pongs, echo = result.result()
                self.assertGreaterEqual(seconds, HOLD)
                self.assertGreaterEqual(pongs, 2 * HOLD - 2)
                self.assertEqual(echo, (1, b"still here"))
        # The meter counted the frames, so they crossed the plugin's hijack
        # wrapper as a tenant route's do, not a route without it.
        with urllib.request.urlopen(f"http://127.0.0.1:{self.ports['9101']}/metrics", timeout=5) as response:
            metrics = response.read().decode()
        line = next(l for l in metrics.splitlines() if l.startswith(f'bex_websocket_egress_bytes_total{{app_id="{APP_ID}"}}'))
        self.assertGreater(int(line.split()[-1]), 0, line)

    def test_the_edge_offers_no_http2_websockets(self):
        # A browser opens a WebSocket over HTTP/2 only where the server sets
        # SETTINGS_ENABLE_CONNECT_PROTOCOL (0x8, RFC 8441). There it is a
        # stream, never hijacked, so the hold above would not cover it.
        alpn, settings = http2_settings(self.ports["8443"])
        self.assertEqual(alpn, "h2")
        self.assertNotEqual(settings.get(0x8), 1, settings)

    def test_a_slow_request_body_is_still_cut_at_the_read_deadline(self):
        with concurrent.futures.ThreadPoolExecutor() as pool:
            cut = {name: pool.submit(slow_body, *target) for name, target in self.entrypoints().items()}
        for name, result in cut.items():
            with self.subTest(entrypoint=name):
                seconds, answer = result.result()
                self.assertGreaterEqual(seconds, READ_TIMEOUT - 0.5, answer)
                self.assertLess(seconds, READ_TIMEOUT + 3, answer)
                self.assertNotIn(b" 200 ", answer.split(b"\r\n", 1)[0])
        self.assertTrue(self.backend.bodies, "the backend saw no upload")
        for want, got in self.backend.bodies:
            self.assertLess(got, want)


if __name__ == "__main__":
    unittest.main()
