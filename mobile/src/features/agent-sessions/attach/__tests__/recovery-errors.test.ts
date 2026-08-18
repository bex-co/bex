import { isTerminalAttachFailure } from "../recovery-errors";

describe("agent attach recovery errors", () => {
  it("never blindly retries authorization or conflict failures", () => {
    for (const error of [
      { status: 401 },
      { statusCode: 403 },
      new Error("agent stream returned 409"),
      { networkError: { statusCode: "403" } },
    ]) {
      expect(isTerminalAttachFailure(error)).toBe(true);
    }
  });

  it("allows bounded recovery for transient failures", () => {
    expect(isTerminalAttachFailure({ statusCode: 503 })).toBe(false);
    expect(isTerminalAttachFailure(new Error("network unavailable"))).toBe(
      false,
    );
  });
});
