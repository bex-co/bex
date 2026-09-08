import { datastoreSuspensionConverged } from "../datastore-lifecycle";

describe("shared datastore lifecycle state", () => {
  it("proves convergence only from the requested suspension state", () => {
    expect(datastoreSuspensionConverged("suspend", "suspended")).toBe(true);
    expect(datastoreSuspensionConverged("suspend", false)).toBe(false);
    expect(datastoreSuspensionConverged("resume", "suspended")).toBe(false);
    expect(datastoreSuspensionConverged("resume", "not_suspended")).toBe(true);
  });
});
