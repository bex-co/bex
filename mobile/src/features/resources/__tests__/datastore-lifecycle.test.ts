import {
  datastoreLifecycleTransition,
  datastoreSuspensionConverged,
} from "../datastore-lifecycle";

describe("shared datastore lifecycle state", () => {
  it("derives only the common suspend and resume transitions", () => {
    for (const status of ["available", "unavailable", "AVAILABLE"]) {
      expect(
        datastoreLifecycleTransition({ status, suspended: "not_suspended" }),
      ).toBe("suspend");
    }
    expect(
      datastoreLifecycleTransition({
        status: "available",
        suspended: "suspended",
      }),
    ).toBe("resume");
    expect(
      datastoreLifecycleTransition({ status: "available", suspended: true }),
    ).toBe("resume");
    expect(
      datastoreLifecycleTransition({ status: "deleting", suspended: true }),
    ).toBe(null);
    expect(
      datastoreLifecycleTransition({ status: "creating", suspended: false }),
    ).toBe(null);
  });

  it("proves convergence only from the requested suspension state", () => {
    expect(datastoreSuspensionConverged("suspend", "suspended")).toBe(true);
    expect(datastoreSuspensionConverged("suspend", false)).toBe(false);
    expect(datastoreSuspensionConverged("resume", "suspended")).toBe(false);
    expect(datastoreSuspensionConverged("resume", "not_suspended")).toBe(true);
  });
});
