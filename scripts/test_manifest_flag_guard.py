"""Tests for scripts/manifest_flag_guard.py against throwaway git repositories (w8/331)."""

import contextlib
import io
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

try:
    import yaml  # noqa: F401
except ImportError:  # pragma: no cover - the guard itself skips without PyYAML
    yaml = None

import manifest_flag_guard as guard

MAIN_GO = "lego/operator/cmd/manager/main.go"
MANAGER = "lego/operator/config/manager/manager.yaml"
KUSTOMIZATION = "lego/operator/config/prod/kustomization.yaml"


def go_source(*flags):
    lines = [f'\tflag.StringVar(&cfg.f{i}, "{name}", "", "help")' for i, name in enumerate(flags)]
    return "package main\n\nfunc parse() {\n" + "\n".join(lines) + "\n\tflag.Parse()\n}\n"


def manifest(args, image="controller:latest", sidecar_args=()):
    containers = [{"name": "manager", "image": image, "args": list(args)}]
    if sidecar_args:
        containers.append({"name": "proxy", "image": "quay.io/brancz/kube-rbac-proxy:v0.22.1", "args": list(sidecar_args)})
    body = {
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": {"name": "controller-manager"},
        "spec": {"template": {"spec": {"containers": containers}}},
    }
    return yaml.safe_dump(body)


@unittest.skipIf(yaml is None, "PyYAML unavailable")
class ManifestFlagGuardTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.repo = Path(self.tmp.name)
        self.cwd = os.getcwd()
        os.chdir(self.repo)
        self.addCleanup(os.chdir, self.cwd)
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "user.name", "test")
        self.git("config", "commit.gpgsign", "false")
        # The released binary: defines --leader-elect only; production is pinned to it.
        self.write(MAIN_GO, go_source("leader-elect"))
        self.write(MANAGER, manifest(["--leader-elect"]))
        released = self.commit("operator")
        self.commit(f"chore(deploy): pin platform images to {released[:12]} [skip ci]", allow_empty=True)
        self.base = self.rev("HEAD")

    def git(self, *args):
        return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout

    def write(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def commit(self, message, allow_empty=False):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", message, *(["--allow-empty"] if allow_empty else []))
        return self.rev("HEAD")

    def rev(self, ref):
        return self.git("rev-parse", ref).strip()

    def undefined(self, base, head):
        return guard.check(base, head)[1]

    def test_flag_added_with_its_definition_in_the_same_push_is_refused(self):
        # f114fdd9f's shape: definition and manifest argument in one commit.
        self.write(MAIN_GO, go_source("leader-elect", "pprof-bind-address"))
        self.write(MANAGER, manifest(["--leader-elect", "--pprof-bind-address=127.0.0.1:6060"]))
        head = self.commit("add pprof")
        self.assertEqual(self.undefined(self.base, head), [(MANAGER, "pprof-bind-address")])

    def test_flag_the_pinned_image_already_defines_passes(self):
        # Code first, pinned, then the manifest argument.
        self.write(MAIN_GO, go_source("leader-elect", "pprof-bind-address"))
        code = self.commit("define pprof")
        self.commit(f"chore(deploy): pin platform images to {code[:12]} [skip ci]", allow_empty=True)
        base = self.rev("HEAD")
        self.write(MANAGER, manifest(["--leader-elect", "--pprof-bind-address=127.0.0.1:6060"]))
        self.assertEqual(self.undefined(base, self.commit("pass pprof")), [])

    def test_third_party_container_flags_are_ignored(self):
        self.write(MANAGER, manifest(["--leader-elect"], sidecar_args=["--upstream=http://127.0.0.1:8080/"]))
        self.assertEqual(self.undefined(self.base, self.commit("sidecar")), [])

    def test_bex_registry_images_are_checked(self):
        self.write(MANAGER, manifest(["--leader-elect", "--new-thing"], image="ghcr.io/bex-co/bex-api@sha256:00"))
        self.assertEqual(self.undefined(self.base, self.commit("api flag")), [(MANAGER, "new-thing")])

    def test_kustomize_patch_adding_an_arg_is_checked(self):
        self.write(
            KUSTOMIZATION,
            textwrap.dedent(
                """\
                resources:
                  - ../default
                patches:
                  - target:
                      kind: Deployment
                      name: controller-manager
                    patch: |-
                      - op: add
                        path: /spec/template/spec/containers/0/args/-
                        value: --zone=eu
                      - op: add
                        path: /spec/template/spec/containers/0/env/-
                        value:
                          name: BEX_ZONE
                          value: eu
                """
            ),
        )
        self.assertEqual(self.undefined(self.base, self.commit("patch")), [(KUSTOMIZATION, "zone")])

    def test_removed_flags_and_env_vars_pass(self):
        self.write(MANAGER, manifest([]))
        self.assertEqual(self.undefined(self.base, self.commit("drop flag")), [])

    def test_no_pin_commit_skips_with_a_warning(self):
        self.git("checkout", "-q", "--orphan", "fresh")
        self.write(MAIN_GO, go_source("leader-elect"))
        self.write(MANAGER, manifest(["--leader-elect"]))
        base = self.commit("fresh base")
        self.write(MANAGER, manifest(["--leader-elect", "--brand-new"]))
        head = self.commit("brand new")
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            self.assertEqual(self.undefined(base, head), [])
        self.assertIn("no pin commit", stderr.getvalue())

    def test_main_names_the_flag_and_fails(self):
        self.write(MANAGER, manifest(["--leader-elect", "--pprof-bind-address=127.0.0.1:6060"]))
        head = self.commit("arg only")
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr):
            self.assertEqual(guard.main(["guard", self.base, head]), 1)
        self.assertIn("adds --pprof-bind-address", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
