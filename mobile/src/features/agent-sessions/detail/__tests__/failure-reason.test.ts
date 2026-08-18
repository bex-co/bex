import { failureReasonText, failureStatusText } from "../failure-reason";

describe("agent-session failure reason", () => {
  it("accepts strings and older structured error payloads", () => {
    expect(failureReasonText("agent exited")).toBe("agent exited");
    expect(
      failureReasonText({ message: "model unavailable", code: "MODEL" }),
    ).toBe("model unavailable");
    expect(failureReasonText({ code: "AGENT_FAILED" })).toBe("AGENT_FAILED");
  });

  it("never stringifies an unknown object into user-visible noise", () => {
    expect(failureReasonText({ nested: { error: true } })).toBe(null);
    expect(failureReasonText([])).toBe(null);
    expect(failureReasonText("  ")).toBe(null);
    expect(failureReasonText("[object Object]")).toBe(null);
    expect(failureReasonText('{"message":"sandbox failed"}')).toBe(
      "sandbox failed",
    );
  });

  it("surfaces a descriptive lifecycle status when no reason was recorded", () => {
    expect(failureStatusText("sandbox create failed")).toBe(
      "Sandbox create failed",
    );
    // Generic phase words add nothing over the fallback copy.
    for (const generic of ["failed", "error", "canceled", "unknown", ""]) {
      expect(failureStatusText(generic)).toBe(null);
    }
    expect(failureStatusText(null)).toBe(null);
    expect(failureStatusText(42)).toBe(null);
  });
});
