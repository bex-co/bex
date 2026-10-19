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
  ])("%o reads %s", (service, key) => {
    expect(undeployedChangesHintKey(service)).toBe(key);
  });
});
