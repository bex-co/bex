import { describe, expect, it } from "vitest";
import {
  MAXMEMORY_POLICIES,
  PERSISTENCE_MODES,
  maxmemoryPolicyToUi,
  persistenceModeToUi,
} from "@/features/keyvalue/lib/labels";

// w4/046: bex-api reads the eviction policy back with underscores while the UI
// options (and the CRD/save path) use hyphens; without this mapping the detail
// selector matched no option and rendered blank.
describe("maxmemoryPolicyToUi", () => {
  it("maps every underscored API policy onto its hyphenated UI option", () => {
    for (const ui of MAXMEMORY_POLICIES) {
      const wire = ui.replace(/-/g, "_"); // the shape bex-api actually returns
      expect(maxmemoryPolicyToUi(wire)).toBe(ui);
      // Every mapped value is a real option, so the Select can display it.
      expect(MAXMEMORY_POLICIES).toContain(maxmemoryPolicyToUi(wire));
    }
  });

  it("leaves noeviction (no separator) unchanged", () => {
    expect(maxmemoryPolicyToUi("noeviction")).toBe("noeviction");
  });

  it("passes an empty read through, never fabricating a policy", () => {
    expect(maxmemoryPolicyToUi("")).toBe("");
  });

  it("passes an unrecognized value through untouched", () => {
    expect(maxmemoryPolicyToUi("something_else")).toBe("something_else");
  });
});

describe("persistenceModeToUi", () => {
  it("maps journal_snapshot onto the hyphen UI option", () => {
    expect(persistenceModeToUi("journal_snapshot")).toBe("journal-snapshot");
    expect(PERSISTENCE_MODES).toContain(
      persistenceModeToUi("journal_snapshot"),
    );
  });

  it("leaves snapshot and off unchanged", () => {
    expect(persistenceModeToUi("snapshot")).toBe("snapshot");
    expect(persistenceModeToUi("off")).toBe("off");
  });

  it("passes an empty read through, never fabricating a mode", () => {
    expect(persistenceModeToUi("")).toBe("");
  });
});
