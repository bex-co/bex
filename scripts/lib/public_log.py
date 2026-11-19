#!/usr/bin/env python3
"""Keep tenant data out of this public repository's Actions logs (w5/m501).

The repository is public, so every run log is readable by anyone. A job that
holds production credentials prints platform-level facts only
(docs/ADR019-infra-credentials.md §Decision 5); whatever it reads about
tenants passes through this module first:

  some-command 2>&1 | python3 scripts/lib/public_log.py
      redact stdin to stdout, line by line
  python3 scripts/lib/public_log.py mask-inputs service_id namespace
      print `::add-mask::` for those workflow_dispatch inputs, so a tenant
      identifier typed into a dispatch shows as *** in every later step

Redaction replaces each of these with a short keyed hash, so one object keeps
one name throughout a log while the name itself stays private:

  - bex ids (tea-<xid>, srv-<xid>, ...) and every name built on one
    (tea-<xid>-<service>-<pod suffix>, <workspace>-sandbox, bld-tea-<xid>-...)
  - the cluster's node and Machine names
  - IPv4 and IPv6 addresses
  - hostnames under the tenant domain (onbex.co)
  - Hetzner provider ids (hcloud://<n>)

The key is the cluster's kube-system namespace UID (PUBLIC_LOG_KEY overrides
it). Only a kubeconfig holder can read it, so the public cannot confirm a
guessed service name by hashing it, and an operator with production access
gets the same hashes from a local run. Without a cluster the key is random:
the hashes then hold within one invocation only.
"""
from __future__ import annotations

import hashlib
import hmac
import ipaddress
import json
import os
import re
import secrets
import subprocess
import sys

# bex ids are "<3-letter prefix>-<20-char xid>" (lego/backend/internal/id).
# A name built on one keeps going in hyphenated segments: the rest of the
# token is the tenant's service name and a pod or job suffix.
ID_RE = re.compile(r"(?<![A-Za-z0-9])([a-z]{3})-([0-9a-v]{20})((?:-[a-z0-9]+)*)(?![A-Za-z0-9])")
ID_LABELS = {"tea": "ws", "srv": "srv", "dpg": "pg", "red": "kv"}
# Suffixes the platform appends to a workspace id; kept so the namespace's
# role stays readable.
PLATFORM_SUFFIXES = ("-sandbox",)
# What redact() leaves of a tenant name: an optional short platform prefix
# (bld-, disk-, ...) and a tag, nothing of the original.
TAGGED_RE = re.compile(
    r"(?:[a-z]{2,8}-)?[a-z]+#[0-9a-f]{6}(?:~[0-9a-f]{6}|" + "|".join(map(re.escape, PLATFORM_SUFFIXES)) + r")?"
)

# Namespaces whose pods are platform components: their pod and workload names
# carry no tenant data. Mirrors the platform set of
# TenantRegistryImagePullFailing (deploy/gitops/base/rules/alerting_rules.yml)
# plus the Cluster API and runtime namespaces. scripts/lib/public_log_check.py
# lets a production job print a literal read of one of these.
COMPONENT_NAMESPACES = frozenset({
    "argocd", "auth", "bex-capi", "bex-registry", "bex-system", "caph-system",
    "capi-kubeadm-bootstrap-system", "capi-kubeadm-control-plane-system",
    "capi-system", "cert-manager", "cilium-secrets", "cnpg-system", "dashboard",
    "kpack", "kube-node-lease", "kube-public", "kube-system", "monitoring",
    "opensandbox-system", "secrets", "traefik",
})
# Platform namespaces that hold tenant work (builds, sandboxes, hand-applied
# Apps): the namespace name is the platform's, the pod names inside are not.
WORKLOAD_NAMESPACES = frozenset({"bex-build", "bex-sandbox", "default", "local", "opensandbox-snapshot"})

# Cluster API node names: <cluster>-control-plane-<5> and
# <cluster>-<pool>-<machineset 5>-<5>. The live node list is matched too;
# this catches names that left the cluster before the log was written.
NODE_RE = re.compile(
    r"(?<![A-Za-z0-9])bex-(?:control-plane-[a-z0-9]{5}"
    r"|(?:platform|sandbox|tenant(?:-[a-z0-9]+)?)-[a-z0-9]{5}-[a-z0-9]{5})(?![A-Za-z0-9-])"
)
HOST_RE = re.compile(r"(?<![A-Za-z0-9.-])((?:[A-Za-z0-9-]+\.)+)onbex\.co(?![A-Za-z0-9-])")
HCLOUD_RE = re.compile(r"hcloud://(\d+)")
IPV4_RE = re.compile(r"(?<![0-9.])(\d{1,3}(?:\.\d{1,3}){3})(?!\.?\d)")
IPV6_RE = re.compile(r"(?<![0-9A-Za-z:])((?:[0-9A-Fa-f]{0,4}:){2,7}[0-9A-Fa-f]{0,4})(?![0-9A-Za-z:])")


def _kubectl(*args: str) -> str:
    # --request-timeout does not bound kubectl's discovery retries against an
    # API server that never answers, so the process gets its own deadline.
    try:
        out = subprocess.run(
            ["kubectl", *args, "--request-timeout=10s"],
            capture_output=True, text=True, timeout=25, check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    return out.stdout if out.returncode == 0 else ""


def cluster_key() -> tuple[bytes, bool]:
    """(key, whether the cluster answered). The key is PUBLIC_LOG_KEY, else the
    kube-system namespace UID, else random bytes."""
    uid = _kubectl("get", "namespace", "kube-system", "-o", "jsonpath={.metadata.uid}").strip()
    override = os.environ.get("PUBLIC_LOG_KEY", "")
    if override:
        return override.encode(), bool(uid)
    return (uid.encode(), True) if uid else (secrets.token_bytes(16), False)


def cluster_redactor() -> "Redactor":
    """A Redactor with the cluster's key that also matches its node and Machine
    names; a cluster that did not answer is not asked for them."""
    key, reachable = cluster_key()
    names: list[str] = []
    if reachable:
        names = _kubectl("get", "nodes", "-o", "jsonpath={.items[*].metadata.name}").split()
        names += _kubectl("get", "machines.cluster.x-k8s.io", "-A", "-o", "jsonpath={.items[*].metadata.name}").split()
    return Redactor(key, names)


class Redactor:
    def __init__(self, key: bytes, node_names: list[str] | tuple[str, ...] = ()):
        self.key = key
        self.node_re = None
        names = sorted({n for n in node_names if n}, key=len, reverse=True)
        if names:
            self.node_re = re.compile(
                r"(?<![A-Za-z0-9])(?:" + "|".join(map(re.escape, names)) + r")(?![A-Za-z0-9])"
            )

    def digest(self, kind: str, value: str) -> str:
        return hmac.new(self.key, f"{kind}:{value}".encode(), hashlib.sha256).hexdigest()[:6]

    def tag(self, kind: str, value: str) -> str:
        """`<kind>#<hash>`: the public name of one private value."""
        return f"{kind}#{self.digest(kind, value)}"

    def _id(self, m: re.Match) -> str:
        prefix, xid, rest = m.group(1), m.group(2), m.group(3)
        label = ID_LABELS.get(prefix, prefix)
        head = self.tag(label, f"{prefix}-{xid}")
        if not rest:
            return head
        if rest in PLATFORM_SUFFIXES:
            return head + rest
        return f"{head}~{self.digest('name', m.group(0))}"

    def _ip(self, m: re.Match) -> str:
        try:
            ip = ipaddress.ip_address(m.group(1))
        except ValueError:
            return m.group(0)
        if ip.is_loopback or ip.is_unspecified:
            return m.group(0)
        return self.tag("ip", str(ip))

    def redact(self, text: str) -> str:
        text = HOST_RE.sub(lambda m: self.tag("host", m.group(1)) + ".onbex.co", text)
        text = ID_RE.sub(self._id, text)
        if self.node_re is not None:
            text = self.node_re.sub(lambda m: self.tag("node", m.group(0)), text)
        text = NODE_RE.sub(lambda m: self.tag("node", m.group(0)), text)
        text = HCLOUD_RE.sub(lambda m: "hcloud://" + self.tag("server", m.group(1)), text)
        text = IPV4_RE.sub(self._ip, text)
        return IPV6_RE.sub(self._ip, text)


def mask_inputs(names: list[str]) -> int:
    """Print ::add-mask:: for each named workflow_dispatch input that is set."""
    path = os.environ.get("GITHUB_EVENT_PATH", "")
    if not path:
        print("mask-inputs: GITHUB_EVENT_PATH is not set (run it inside GitHub Actions)", file=sys.stderr)
        return 2
    with open(path, encoding="utf-8") as fh:
        inputs = (json.load(fh) or {}).get("inputs") or {}
    for name in names:
        value = str(inputs.get(name) or "").strip()
        if value:
            print(f"::add-mask::{value}")
    return 0


def main(argv: list[str]) -> int:
    if argv[:1] == ["mask-inputs"]:
        return mask_inputs(argv[1:])
    if argv:
        print(__doc__, file=sys.stderr)
        return 2
    redactor = cluster_redactor()
    out = sys.stdout
    for raw in iter(sys.stdin.buffer.readline, b""):
        line = raw.decode("utf-8", "replace")
        # A mask a piped command registers must reach the runner as written:
        # rewriting it would leave the real value unmasked. The runner never
        # prints the line itself.
        out.write(line if line.lstrip().startswith("::add-mask::") else redactor.redact(line))
        out.flush()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
