import { describe, expect, it } from "vitest";
import { disabledReasonText } from "../disabled-reason";

const t = () => "translated";

describe("disabledReasonText (w21/022)", () => {
  it("translates the auto-disable sentence", () => {
    const keys: string[] = [];
    const translate = (key: string) => {
      keys.push(key);
      return "translated";
    };
    expect(
      disabledReasonText(
        "disabled automatically after repeated delivery failures",
        translate,
      ),
    ).toBe("translated");
    expect(keys).toEqual(["webhooks.disabledReasonAuto"]);
  });
  it("shows nothing for a manual disable or no reason", () => {
    expect(disabledReasonText("disabled manually", t)).toBeNull();
    expect(disabledReasonText("", t)).toBeNull();
    expect(disabledReasonText(undefined, t)).toBeNull();
  });
  it("keeps an unknown future reason raw, never hidden", () => {
    expect(disabledReasonText("quota exceeded", t)).toBe("quota exceeded");
  });
});
