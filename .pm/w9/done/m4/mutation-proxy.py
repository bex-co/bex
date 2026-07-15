#!/usr/bin/env python3
# A response-mutating reverse proxy in front of a live bex-api, used by
# mutation-check.sh (w9/m4/t005) to prove the cli-compat verify legs fail loudly
# against a broken wire shape. It forwards every request verbatim to the real
# bex-api and, when the request path matches the family named by $MUTATION,
# rewrites that ONE response into its pre-fix (regressed) shape — reintroducing
# exactly the bug the matching RC fixed. If a verify leg still passes against
# this, the leg is vacuous.
#
#   MUTATION       target path            regression reintroduced
#   svc_autodeploy /v1/services*          RC2: autoDeploy string enum -> JSON bool
#   pg_flatten     /v1/postgres (list)    RC3: {postgres,cursor} envelope -> bare array
#   deploy_image   /v1/services/*/deploys RC2: Deploy.image object -> bare string
#   logs_blank     /v1/logs               RC8: required nextStartTime/EndTime -> ""
#   kv_flatten     /v1/key-value*         RC14: nested owner/options -> flat, dropped fields
#   env_flatten    /v1/environments       RC15: {environment,cursor} envelope -> bare array
import json, os, sys, urllib.request, urllib.error
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

UPSTREAM = os.environ["UPSTREAM"].rstrip("/")  # e.g. http://localhost:54090
MUTATION = os.environ.get("MUTATION", "")
PORT = int(os.environ.get("PROXY_PORT", "0"))


# walk visits every dict node in a decoded JSON document and calls fn on it, so
# each mutation only has to describe its per-object edit, not the recursion.
def walk(o, fn):
    if isinstance(o, dict):
        fn(o)
        for v in o.values():
            walk(v, fn)
    elif isinstance(o, list):
        for v in o:
            walk(v, fn)


def mutate(path, body):
    try:
        doc = json.loads(body)
    except Exception:
        return body
    p = path.split("?", 1)[0]

    if MUTATION == "svc_autodeploy" and p.startswith("/v1/services"):
        # RC2: the CLI's AutoDeploy is a "yes"/"no" string enum; emit a bool.
        def bust(o):
            if "autoDeploy" in o:
                o["autoDeploy"] = False
        walk(doc, bust)

    elif MUTATION == "pg_flatten" and p == "/v1/postgres" and isinstance(doc, list):
        # RC3: unwrap the cursor envelope back to a bare array of postgres objects.
        doc = [item.get("postgres", item) if isinstance(item, dict) else item for item in doc]

    elif MUTATION == "deploy_image" and p.endswith("/deploys") and isinstance(doc, list):
        # RC2: collapse Deploy.image from {ref,...} back to a bare string.
        for item in doc:
            d = item.get("deploy", item) if isinstance(item, dict) else item
            if isinstance(d, dict) and isinstance(d.get("image"), dict):
                d["image"] = d["image"].get("ref", "")

    elif MUTATION == "logs_blank" and p == "/v1/logs" and isinstance(doc, dict):
        # RC8: blank the required cursor timestamps (the empty-result crash).
        doc["nextStartTime"] = ""
        doc["nextEndTime"] = ""

    elif MUTATION == "kv_flatten" and p.startswith("/v1/key-value"):
        # RC14: flatten owner/options, drop maxmemoryPolicy/persistenceMode.
        def bust(o):
            if "ownerId" in o or "owner" in o:
                o["ownerId"] = ""
                o.pop("owner", None)
                o.pop("maxmemoryPolicy", None)
                o.pop("persistenceMode", None)
                o.pop("options", None)
        walk(doc, bust)

    elif MUTATION == "env_flatten" and p == "/v1/environments" and isinstance(doc, list):
        # RC15: unwrap the cursor envelope back to a bare array.
        doc = [item.get("environment", item) if isinstance(item, dict) else item for item in doc]

    return json.dumps(doc).encode()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *a):
        pass

    def _proxy(self):
        length = int(self.headers.get("Content-Length", 0))
        req_body = self.rfile.read(length) if length else None
        url = UPSTREAM + self.path
        req = urllib.request.Request(url, data=req_body, method=self.command)
        for k, v in self.headers.items():
            if k.lower() not in ("host", "content-length", "connection"):
                req.add_header(k, v)
        try:
            resp = urllib.request.urlopen(req)
            status, body, ctype = resp.status, resp.read(), resp.headers.get("Content-Type", "")
        except urllib.error.HTTPError as e:
            status, body, ctype = e.code, e.read(), e.headers.get("Content-Type", "")
        except Exception as e:
            self.send_response(502)
            self.end_headers()
            self.wfile.write(str(e).encode())
            return
        if "application/json" in ctype and body:
            body = mutate(self.path, body)
        self.send_response(status)
        self.send_header("Content-Type", ctype or "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    do_GET = do_POST = do_PATCH = do_PUT = do_DELETE = _proxy


srv = ThreadingHTTPServer(("127.0.0.1", PORT), Handler)
print(srv.server_address[1], flush=True)  # actual bound port for the driver
srv.serve_forever()
