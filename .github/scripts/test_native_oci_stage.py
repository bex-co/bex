"""Hermetic transport/layout controls; not image runtime or security acceptance."""
import hashlib
import http.client
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
import urllib.error
import urllib.request

SPEC = importlib.util.spec_from_file_location("native_oci_stage", Path(__file__).with_name("native-oci-stage.py"))
stage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(stage)
OCI_INDEX = "application/vnd.oci.image.index.v1+json"
OCI_MANIFEST = "application/vnd.oci.image.manifest.v1+json"


def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()


def encoded(value):
    # Deliberate whitespace proves raw manifest/config bytes are not reserialized.
    return json.dumps(value, indent=3).encode() + b"\n"


def descriptor(data, media_type):
    return {"mediaType": media_type, "digest": digest(data), "size": len(data)}


def fixture(architecture="amd64", duplicate=False, index_architecture="amd64", variant=None):
    layer = b"synthetic-opaque-compressed-layer-bytes\x00" * 40
    config_document = {"architecture": architecture, "os": "linux", "config": {"Env": ["NODE_VERSION=20.20.2"]}, "rootfs": {"type": "layers", "diff_ids": []}}
    if variant:
        config_document["variant"] = variant
    config = encoded(config_document)
    manifest = encoded({"schemaVersion": 2, "mediaType": OCI_MANIFEST, "config": descriptor(config, "application/vnd.oci.image.config.v1+json"), "layers": [descriptor(layer, "application/vnd.oci.image.layer.v1.tar+gzip")]})
    child = dict(descriptor(manifest, OCI_MANIFEST), platform={"architecture": index_architecture, "os": "linux"})
    if variant:
        child["platform"]["variant"] = variant
    root = encoded({"schemaVersion": 2, "mediaType": OCI_INDEX, "manifests": [child, child] if duplicate else [child]})
    return {"root": root, "manifest": manifest, "config": config, "layer": layer}


class Response(io.BytesIO):
    def __init__(self, data, status=200, headers=None):
        super().__init__(data)
        self.status = status
        self.headers = {"Content-Length": str(len(data))} if headers is None else headers


class FailingStream(Response):
    def read(self, size=-1):
        raise http.client.IncompleteRead(super().read(size), 100)


class Registry:
    def __init__(self, data, behaviors=None):
        self.data = {digest(body): body for body in data.values()}
        self.behaviors = behaviors or {}
        self.calls = []

    def open(self, kind, key, offset):
        number = sum(call[1] == key for call in self.calls)
        self.calls.append((kind, key, offset))
        choices = self.behaviors.get(key, [])
        if number < len(choices):
            return choices[number](offset)
        data = self.data[key]
        if offset:
            return Response(data[offset:], 206, {"Content-Length": str(len(data) - offset), "Content-Range": f"bytes {offset}-{len(data)-1}/{len(data)}"})
        return Response(data)


class StageControls(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="native-oci-controls-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.data = fixture()
        self.ref = stage.REPOSITORY + "@" + digest(self.data["root"])
        self.output = self.root / "layout"
        self.proof = self.root / "proof.json"

    def stager(self, registry=None, platform="linux/amd64", data=None):
        data = data or self.data
        return stage.Stager(stage.REPOSITORY + "@" + digest(data["root"]), platform, self.output, self.proof, registry or Registry(data), sleep=lambda _: None)

    def test_genuine_layout_preserves_exact_root_child_and_config(self):
        result = self.stager().run()
        self.assertTrue(result["complete"])
        layout = json.loads((self.output / "index.json").read_text())
        self.assertEqual(len(layout["manifests"]), 1)
        self.assertEqual(layout["manifests"][0]["digest"], digest(self.data["manifest"]))
        for body in self.data.values():
            self.assertEqual((self.output / "blobs/sha256" / digest(body)[7:]).read_bytes(), body)
        self.assertEqual(json.loads((self.output / "oci-layout").read_text()), {"imageLayoutVersion": "1.0.0"})
        self.assertEqual(json.loads(self.proof.read_text())["config"]["digest"], digest(self.data["config"]))

    def test_arm64_selected_raw_child_and_config_are_preserved(self):
        data = fixture("arm64", index_architecture="arm64")
        result = self.stager(platform="linux/arm64", data=data).run()
        self.assertEqual(result["selected"]["platform"], {"os": "linux", "architecture": "arm64"})
        self.assertEqual(result["selected"]["digest"], digest(data["manifest"]))
        self.assertEqual((self.output / "blobs/sha256" / digest(data["config"])[7:]).read_bytes(), data["config"])

    def test_proof_can_live_inside_new_owned_output(self):
        proof = self.output / "stage-proof.json"
        result = stage.Stager(self.ref, "linux/amd64", self.output, proof, Registry(self.data), sleep=lambda _: None).run()
        self.assertEqual(json.loads(proof.read_text()), result)

    def test_arm64_variant_is_preserved_without_manifest_rewriting(self):
        data = fixture("arm64", index_architecture="arm64", variant="v8")
        result = self.stager(platform="linux/arm64", data=data).run()
        self.assertEqual(result["selected"]["platform"]["variant"], "v8")
        self.assertEqual((self.output / "blobs/sha256" / digest(data["manifest"])[7:]).read_bytes(), data["manifest"])

    def test_eof_retains_partial_then_validated_range_resumes(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer[:29], headers={"Content-Length": str(len(layer))})]})
        result = self.stager(registry).run()
        calls = [call for call in registry.calls if call[1] == digest(layer)]
        self.assertEqual([call[2] for call in calls], [0, 29])
        self.assertTrue(result["complete"])
        self.assertEqual((self.output / "blobs/sha256" / digest(layer)[7:]).read_bytes(), layer)

    def test_incomplete_read_exception_partial_is_retained(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: FailingStream(layer[:17], headers={"Content-Length": str(len(layer))})]})
        self.stager(registry).run()
        self.assertEqual([x[2] for x in registry.calls if x[1] == digest(layer)], [0, 17])

    def test_ignored_range_200_restarts_instead_of_appending(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer[:13], headers={"Content-Length": str(len(layer))}), lambda _: Response(layer)]})
        result = self.stager(registry).run()
        self.assertTrue(result["blobs"][-1]["attempts"][1]["range_ignored_restart"])
        self.assertEqual((self.output / "blobs/sha256" / digest(layer)[7:]).read_bytes(), layer)

    def test_wrong_range_resets_before_next_complete_attempt(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer[:11], headers={"Content-Length": str(len(layer))}), lambda _: Response(layer[11:], 206, {"Content-Range": f"bytes 12-{len(layer)-1}/{len(layer)}", "Content-Length": str(len(layer)-11)})]})
        self.stager(registry).run()
        self.assertEqual([x[2] for x in registry.calls if x[1] == digest(layer)], [0, 11, 0])

    def test_digest_mismatch_exhausts_three_and_never_publishes(self):
        layer = self.data["layer"]
        corrupt = b"X" + layer[1:]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(corrupt)] * 3})
        with self.assertRaisesRegex(stage.StageError, "three attempts"):
            self.stager(registry).run()
        self.assertEqual(len([x for x in registry.calls if x[1] == digest(layer)]), 3)
        self.assertFalse((self.output / "blobs/sha256" / digest(layer)[7:]).exists())
        self.assertFalse((self.output / "parts" / (digest(layer)[7:] + ".part")).exists())
        self.assertFalse((self.output / "index.json").exists())
        self.assertFalse(json.loads(self.proof.read_text())["complete"])

    def test_wrong_size_fails_closed_before_verified_publication(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer + b"extra")] * 3})
        with self.assertRaises(stage.StageError):
            self.stager(registry).run()
        self.assertFalse((self.output / "index.json").exists())
        self.assertFalse((self.output / "blobs/sha256" / digest(layer)[7:]).exists())

    def test_unknown_http_length_still_enforces_descriptor_size(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer + b"extra", headers={})] * 3})
        with self.assertRaises(stage.StageError):
            self.stager(registry).run()
        self.assertFalse((self.output / "blobs/sha256" / digest(layer)[7:]).exists())

    def test_root_digest_failure_cannot_select_other_metadata(self):
        root_digest = digest(self.data["root"])
        corrupt = self.data["root"].replace(b"amd64", b"arm64")
        registry = Registry(self.data, {root_digest: [lambda _: Response(corrupt)] * 3})
        with self.assertRaises(stage.StageError):
            self.stager(registry).run()
        self.assertEqual(len(registry.calls), 3)
        self.assertTrue(all(call[1] == root_digest for call in registry.calls))
        self.assertFalse((self.output / "index.json").exists())

    def test_transport_error_receipt_never_contains_signed_url(self):
        class BrokenRegistry(Registry):
            def open(self, kind, key, offset):
                raise urllib.error.URLError("https://cdn.invalid/blob?signed=PRIVATE token=SECRET")
        with self.assertRaises(stage.StageError):
            self.stager(BrokenRegistry(self.data)).run()
        text = self.proof.read_text()
        self.assertNotIn("PRIVATE", text)
        self.assertNotIn("SECRET", text)
        self.assertNotIn("cdn.invalid", text)

    def test_partial_stays_unverified_and_complete_verified_cache_reused(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer[:7], headers={"Content-Length": str(len(layer))}), lambda offset: Response(b"", 206, {"Content-Length": str(len(layer)-offset), "Content-Range": f"bytes {offset}-{len(layer)-1}/{len(layer)}"}), lambda offset: Response(b"", 206, {"Content-Length": str(len(layer)-offset), "Content-Range": f"bytes {offset}-{len(layer)-1}/{len(layer)}"})]})
        with self.assertRaises(stage.StageError):
            self.stager(registry).run()
        self.assertEqual((self.output / "parts" / (digest(layer)[7:] + ".part")).read_bytes(), layer[:7])
        self.assertFalse((self.output / "index.json").exists())
        next_registry = Registry(self.data)
        self.stager(next_registry).run()
        self.assertEqual(next_registry.calls, [("blobs", digest(layer), 7)])
        final_registry = Registry(self.data)
        self.assertTrue(self.stager(final_registry).run()["complete"])
        self.assertEqual(final_registry.calls, [])

    def test_complete_partial_is_verified_before_atomic_recovery(self):
        instance = self.stager()
        partial = self.output / "parts" / (digest(self.data["layer"])[7:] + ".part")
        partial.write_bytes(self.data["layer"])
        result = instance.run()
        self.assertTrue(result["blobs"][-1]["recovered_complete_partial"])
        self.assertFalse(any(call[1] == digest(self.data["layer"]) for call in instance.registry.calls))
        self.assertFalse(partial.exists())

    def test_full_incomplete_read_payload_is_recovered_before_next_request(self):
        layer = self.data["layer"]
        registry = Registry(self.data, {digest(layer): [lambda _: FailingStream(layer, headers={"Content-Length": str(len(layer))})]})
        result = self.stager(registry).run()
        calls = [call for call in registry.calls if call[1] == digest(layer)]
        self.assertEqual(calls, [("blobs", digest(layer), 0)])
        self.assertTrue(result["blobs"][-1]["recovered_complete_partial"])
        self.assertEqual((self.output / "blobs/sha256" / digest(layer)[7:]).read_bytes(), layer)

    def test_last_attempt_complete_payload_is_verified_without_fourth_request(self):
        layer = self.data["layer"]
        def headers(offset):
            return {"Content-Length": str(len(layer) - offset), "Content-Range": f"bytes {offset}-{len(layer)-1}/{len(layer)}"}
        registry = Registry(self.data, {digest(layer): [lambda _: Response(layer[:7], headers={"Content-Length": str(len(layer))}), lambda offset: Response(layer[offset:13], 206, headers(offset)), lambda offset: FailingStream(layer[offset:], 206, headers(offset))]})
        result = self.stager(registry).run()
        self.assertEqual([call[2] for call in registry.calls if call[1] == digest(layer)], [0, 7, 13])
        self.assertTrue(result["complete"])
        self.assertEqual(result["blobs"][-1]["attempts"][-1]["outcome"], "verified_complete_partial")

    def test_complete_wrong_hash_partial_is_deleted_before_fresh_request(self):
        layer = self.data["layer"]
        instance = self.stager()
        partial = self.output / "parts" / (digest(layer)[7:] + ".part")
        partial.write_bytes(b"X" + layer[1:])
        result = instance.run()
        self.assertTrue(result["complete"])
        self.assertEqual([call[2] for call in instance.registry.calls if call[1] == digest(layer)], [0])
        self.assertEqual((self.output / "blobs/sha256" / digest(layer)[7:]).read_bytes(), layer)

    def test_platform_missing_ambiguous_and_raw_config_mismatch(self):
        for name, data, platform in [("missing", fixture(), "linux/arm64"), ("ambiguous", fixture(duplicate=True), "linux/amd64"), ("config", fixture("arm64"), "linux/amd64")]:
            with self.subTest(name=name):
                with tempfile.TemporaryDirectory(dir=self.root) as temporary:
                    output, proof = Path(temporary) / "layout", Path(temporary) / "proof.json"
                    registry = Registry(data)
                    with self.assertRaises(stage.StageError):
                        stage.Stager(stage.REPOSITORY + "@" + digest(data["root"]), platform, output, proof, registry, sleep=lambda _: None).run()
                    self.assertFalse((output / "index.json").exists())
                    self.assertFalse(any(call[1] == digest(data["layer"]) for call in registry.calls))

    def test_image_cap_ref_and_unowned_or_symlink_outputs_rejected(self):
        for ref in ["https://" + self.ref, self.ref + "?secret=value", self.ref.replace("ghcr.io", "evil.invalid"), stage.REPOSITORY + ":latest"]:
            with self.subTest(ref=ref), self.assertRaises(stage.StageError):
                stage.Stager(ref, "linux/amd64", self.output, self.proof)
        self.output.mkdir(mode=0o700)
        sentinel = self.output / "foreign"
        sentinel.write_bytes(b"untouched")
        with self.assertRaises(stage.StageError):
            self.stager()
        self.assertEqual(sentinel.read_bytes(), b"untouched")
        target = self.root / "link"
        target.symlink_to(self.output, target_is_directory=True)
        with self.assertRaises(stage.StageError):
            stage.Stager(self.ref, "linux/amd64", target, self.proof)

    def test_image_and_layer_caps_reject_before_layer_download(self):
        for sizes in [[stage.MAX_IMAGE + 1], [stage.MAX_IMAGE // 2 + 1] * 2]:
            with self.subTest(sizes=sizes), tempfile.TemporaryDirectory(dir=self.root) as temporary:
                data = fixture()
                manifest = json.loads(data["manifest"])
                manifest["layers"] = [{"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": digest(data["layer"]), "size": size} for size in sizes]
                data["manifest"] = encoded(manifest)
                data["root"] = encoded({"schemaVersion": 2, "mediaType": OCI_INDEX, "manifests": [dict(descriptor(data["manifest"], OCI_MANIFEST), platform={"os": "linux", "architecture": "amd64"})]})
                registry = Registry(data)
                with self.assertRaises(stage.StageError):
                    stage.Stager(stage.REPOSITORY + "@" + digest(data["root"]), "linux/amd64", Path(temporary) / "layout", Path(temporary) / "proof", registry, sleep=lambda _: None).run()
                self.assertFalse(any(call[1] == digest(data["layer"]) for call in registry.calls))

    def test_cross_origin_redirect_strips_auth_and_denies_downgrade(self):
        request = urllib.request.Request("https://ghcr.io/v2/blob", headers={"Authorization": "Bearer SECRET", "Range": "bytes=11-"})
        handler = stage.SafeRedirect()
        redirected = handler.redirect_request(request, None, 302, "Found", {}, "https://cdn.invalid/blob?signed=PRIVATE")
        self.assertIsNone(redirected.get_header("Authorization"))
        self.assertEqual(redirected.get_header("Range"), "bytes=11-")
        same = handler.redirect_request(request, None, 302, "Found", {}, "https://ghcr.io/v2/another")
        self.assertEqual(same.get_header("Authorization"), "Bearer SECRET")
        for url in ["http://cdn.invalid/blob", "https://user:pass@cdn.invalid/blob"]:
            with self.subTest(url=url), self.assertRaises(stage.StageError):
                handler.redirect_request(request, None, 302, "Found", {}, url)

    def test_anonymous_token_scope_and_https_blob_request(self):
        calls = []
        class Opener:
            def open(self, request, timeout):
                calls.append(request)
                if isinstance(request, str):
                    return Response(b'{"token":"ANONYMOUS","expires_in":300}')
                return Response(b"body")
        registry = stage.Registry()
        registry.opener = Opener()
        with registry.open("blobs", digest(b"body"), 11) as response:
            self.assertEqual(response.read(), b"body")
        self.assertEqual(calls[0], "https://ghcr.io/token?service=ghcr.io&scope=repository%3Abex-co%2Fbex-cnb-run%3Apull")
        self.assertEqual(calls[1].full_url, "https://ghcr.io/v2/bex-co/bex-cnb-run/blobs/" + digest(b"body"))
        self.assertEqual(calls[1].get_header("Authorization"), "Bearer ANONYMOUS")
        self.assertEqual(calls[1].get_header("Range"), "bytes=11-")


if __name__ == "__main__":
    unittest.main(verbosity=2)
