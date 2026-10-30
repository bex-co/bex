"""Pure protocol regressions for CSI digest selection and all-node readiness."""

import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("hcloud_csi", Path(__file__).with_name("hcloud_csi.py"))
csi = importlib.util.module_from_spec(spec)
spec.loader.exec_module(csi)
IMAGE = "ghcr.io/bex-co/bex-operator@sha256:" + "a" * 64


def pin_values(image=IMAGE):
    return {
        role: {"image": {"hcloudCSIDriver": {"name": image, "tag": ""}}}
        for role in ("controller", "node")
    }


def ready_documents(image=IMAGE):
    workloads = {"items": []}
    pods = {"items": []}
    for role, count in (("controller", 1), ("node", 2)):
        labels = {"app.kubernetes.io/component": role, "app.kubernetes.io/instance": "hcloud-csi"}
        containers = [{"name": "hcloud-csi-driver", "image": image}]
        status = {"observedGeneration": 2}
        workload = {
            "kind": "Deployment" if role == "controller" else "DaemonSet",
            "metadata": {"name": f"custom-csi-{role}", "generation": 2, "labels": labels},
            "spec": {"template": {"spec": {"containers": containers}}},
            "status": status,
        }
        if role == "controller":
            workload["spec"]["replicas"] = count
            status.update(updatedReplicas=count, readyReplicas=count, availableReplicas=count)
        else:
            status.update(desiredNumberScheduled=count, updatedNumberScheduled=count, numberReady=count)
        workloads["items"].append(workload)
        for index in range(count):
            pods["items"].append({
                "metadata": {"name": f"{role}-{index}", "labels": labels},
                "spec": {"containers": containers},
                "status": {
                    "phase": "Running",
                    "conditions": [{"type": "Ready", "status": "True"}],
                    "containerStatuses": [{"name": "hcloud-csi-driver", "ready": True}],
                },
            })
    return workloads, pods


class PinTests(unittest.TestCase):
    def test_exact_reviewed_digest_for_both_roles(self):
        self.assertEqual(csi.validate_pin(pin_values(), "reviewed"), IMAGE)

    def test_rejects_tags_other_repositories_split_images_and_extra_settings(self):
        bad_values = [
            pin_values(IMAGE.split("@")[0] + ":candidate"),
            pin_values("example.test/driver@sha256:" + "a" * 64),
            pin_values("ghcr.io/bex-co/bex-operator@sha256:short"),
            {"controller": []},
        ]
        split = pin_values()
        split["node"]["image"]["hcloudCSIDriver"]["name"] = IMAGE[:-1] + "b"
        bad_values.append(split)
        tagged = pin_values()
        tagged["node"]["image"]["hcloudCSIDriver"]["tag"] = "latest"
        bad_values.append(tagged)
        extra = pin_values()
        extra["node"]["replicas"] = 0
        bad_values.append(extra)
        for values in bad_values:
            with self.subTest(values=values), self.assertRaises(ValueError):
                csi.validate_pin(values, "reviewed")


class ReadyTests(unittest.TestCase):
    def test_all_controller_and_node_pods_ready_on_reviewed_image(self):
        workloads, pods = ready_documents()
        self.assertEqual(csi.verify_ready(workloads, pods, IMAGE), {
            "image": IMAGE, "readyControllerPods": 1, "readyNodePods": 2,
        })
        self.assertEqual(len(csi.driver_workloads(workloads)), 2)

    def test_rejects_incomplete_rollout_or_wrong_generation(self):
        for role, field, value in ((0, "observedGeneration", 1), (0, "updatedReplicas", 0), (1, "updatedNumberScheduled", 1), (1, "numberUnavailable", 1), (1, "desiredNumberScheduled", 0)):
            workloads, pods = ready_documents()
            workloads["items"][role]["status"][field] = value
            with self.subTest(role=role, field=field), self.assertRaises(ValueError):
                csi.verify_ready(workloads, pods, IMAGE)

    def test_rejects_one_old_node_image_even_when_workloads_say_ready(self):
        workloads, pods = ready_documents()
        pods = copy.deepcopy(pods)
        pods["items"][-1]["spec"]["containers"][0]["image"] = "old:tag"
        with self.assertRaises(ValueError):
            csi.verify_ready(workloads, pods, IMAGE)

    def test_rejects_missing_pods_and_unready_driver(self):
        workloads, pods = ready_documents()
        with self.assertRaises(ValueError):
            csi.verify_ready(workloads, {"items": pods["items"][:-1]}, IMAGE)
        pods["items"][-1]["status"]["containerStatuses"][0]["ready"] = False
        with self.assertRaises(ValueError):
            csi.verify_ready(workloads, pods, IMAGE)

    def test_terminating_old_driver_prevents_class_activation(self):
        workloads, pods = ready_documents()
        old = copy.deepcopy(pods["items"][-1])
        old["metadata"]["deletionTimestamp"] = "2026-10-29T00:00:00Z"
        old["spec"]["containers"][0]["image"] = "old:tag"
        pods["items"].append(old)
        with self.assertRaises(ValueError):
            csi.verify_ready(workloads, pods, IMAGE)

    def test_requires_both_driver_workloads(self):
        workloads, _ = ready_documents()
        with self.assertRaises(ValueError):
            csi.driver_workloads({"items": workloads["items"][:1]})


if __name__ == "__main__":
    unittest.main()
