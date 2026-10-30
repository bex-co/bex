"""Read-only projection and ownership regressions for persistent-disk inventory."""

import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("disk_inventory", Path(__file__).with_name("service_disk_inventory.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def row(kind, **values):
    return {name: values.get(name, "") for name, _ in module.FIELDS[kind]}


class InventoryTests(unittest.TestCase):
    def test_joins_exact_app_lifetime_and_preserves_requested_vs_physical_capacity(self):
        app = row("apps", namespace="tea-one", name="long-app", uid="app-current", serviceId="srv-owned", requestedSizeGB="1", allocatedSizeGB="1", observedSizeGB="1", capacityGB="10")
        claim = row("claims", namespace="tea-one", name="disk-truncated-hash", uid="claim-one", controller="App/long-app/app-current", storageClass="hcloud-volumes-luks", requestedStorage="1Gi", capacityStorage="10Gi", phase="Bound", volumeName="pvc-one")
        old = row("claims", namespace="tea-one", name="old-claim", uid="claim-old", controller="App/long-app/app-previous", volumeName="pvc-old")
        database = row("claims", namespace="tea-one", name="pg-data", controller="StatefulSet/pg/pg-uid", volumeName="pvc-database")
        volume = row("volumes", name="pvc-one", storageClass="hcloud-volumes-luks", capacityStorage="10Gi", phase="Bound", reclaimPolicy="Delete", claimNamespace="tea-one", claimName="disk-truncated-hash", claimUid="claim-one", csiDriver="csi.hetzner.cloud", volumeHandle="123456")
        result = module.inventory([app], [claim, old, database], [volume])
        item = result["appsWithDisks"][0]
        self.assertEqual(item["disk"]["requestedSizeGB"], 1)
        self.assertEqual(item["disk"]["capacityGB"], 10)
        self.assertEqual(len(item["claims"]), 1)
        self.assertTrue(item["claims"][0]["bindingMatches"])
        self.assertEqual(item["claims"][0]["volume"]["volumeHandle"], "123456")
        self.assertEqual([claim["name"] for claim in result["claimsWhoseOwnerIsNotInDiskInventory"]], ["old-claim"])
        self.assertEqual(module.volume_names([claim, old, database]), ["pvc-old", "pvc-one"])

    def test_binding_mismatch_and_missing_volume_are_not_reported_as_safe(self):
        app = row("apps", namespace="tea-one", name="app", uid="app-id", requestedSizeGB="1")
        claim = row("claims", namespace="tea-one", name="disk-app", uid="claim-current", controller="App/app/app-id", volumeName="pvc-one")
        wrong = row("volumes", name="pvc-one", claimNamespace="tea-one", claimName="disk-app", claimUid="claim-old")
        self.assertFalse(module.inventory([app], [claim], [wrong])["appsWithDisks"][0]["claims"][0]["bindingMatches"])
        missing = module.inventory([app], [claim], [])["appsWithDisks"][0]["claims"][0]
        self.assertIsNone(missing["bindingMatches"])
        self.assertIsNone(missing["volume"])

    def test_volume_names_cannot_inject_kubectl_options(self):
        claim = row("claims", controller="App/app/app-id", volumeName="--server=unexpected")
        with self.assertRaises(ValueError):
            module.volume_names([claim])

    def test_platform_adoption_reports_image_ids_and_ready_replicas_only(self):
        deployment = row("deployments", namespace="bex-system", name="bex-controller-manager", uid="deployment", generation="4", observedGeneration="4", replicas="2", updatedReplicas="2", readyReplicas="1", availableReplicas="1", containerImages="manager=ghcr.io/bex-co/bex-operator@sha256:abc;")
        pod = row("pods", namespace="bex-system", name="manager-one", uid="pod", phase="Running", containerImages="manager=ghcr.io/bex-co/bex-operator@sha256:abc;", containerImageIds="manager=containerd://sha256:child;", containerReady="manager=false;")
        result = module.platform_images([deployment], [pod])
        self.assertEqual(result["deployments"][0]["updatedReplicas"], 2)
        self.assertEqual(result["deployments"][0]["readyReplicas"], 1)
        self.assertEqual(result["pods"][0]["containerImageIds"]["manager"], "containerd://sha256:child")
        self.assertFalse(result["pods"][0]["containerReady"]["manager"])

    def test_projection_width_is_checked_before_reporting(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "metadata.tsv"
            path.write_text("unexpected\tcolumns\n")
            with self.assertRaises(ValueError):
                module.read_rows("apps", path)


if __name__ == "__main__":
    unittest.main()
