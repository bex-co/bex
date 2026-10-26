import { describe, expect, it } from "vitest";
import { undeployedChangesHintKey } from "@/features/services/lib/status";

// The undeployed-changes notice names the next step that can actually run:
// nothing while a rolling deploy already carries the saves (w4/m184), and
// Resume before any deploy on a suspended service (w4/m234).
describe("undeployedChangesHintKey", () => {
  it.each([
    [
      { undeployedChangesApplying: false, suspended: false },
      "services.undeployedChangesHint",
    ],
    [
      { undeployedChangesApplying: false, suspended: true },
      "services.undeployedChangesSuspendedHint",
    ],
    [
      { undeployedChangesApplying: true, suspended: false },
      "services.undeployedChangesApplyingHint",
    ],
    // w4/414: an image override saved nothing, so it gets its own notice;
    // a genuine saved change alongside it still reads as saved.
    [
      {
        undeployedChangesApplying: false,
        suspended: false,
        undeployedChangesReason: "image_override",
      },
      "services.undeployedChangesOverrideHint",
    ],
    [
      {
        undeployedChangesApplying: false,
        suspended: false,
        undeployedChangesReason: "saved",
      },
      "services.undeployedChangesHint",
    ],
    [
      {
        undeployedChangesApplying: false,
        suspended: false,
        undeployedChangesReason: "historical_release",
      },
      "services.undeployedChangesHint",
    ],
  ])("%o reads %s", (service, key) => {
    expect(undeployedChangesHintKey(service)).toBe(key);
  });
});
