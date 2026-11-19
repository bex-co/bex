import { describe, expect, it } from "vitest";
import { shortModel, shownModel } from "@/features/agent-sessions/lib/models";

describe("shortModel", () => {
  it("drops the provider prefix and OpenRouter's latest marker", () => {
    expect(shortModel("anthropic/claude-opus-5.5")).toBe("claude-opus-5.5");
    expect(shortModel("~deepseek/deepseek-flash-latest")).toBe(
      "deepseek-flash-latest",
    );
    expect(shortModel("claude-opus-4-1")).toBe("claude-opus-4-1");
  });
});

describe("shownModel", () => {
  const offer = { model: "b", models: ["a", "b", "c"] };
  it("shows the pick, else the running model, else the offer's default", () => {
    expect(shownModel(offer, "c", "a")).toBe("c");
    expect(shownModel(offer, "", "a")).toBe("a");
    expect(shownModel(offer, "", null)).toBe("b");
    expect(shownModel({ model: null, models: ["x"] }, "")).toBe("x");
  });
});
