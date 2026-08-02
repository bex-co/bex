import { en } from "../en";
import { zh } from "../zh";

function keys(value: unknown, prefix = ""): string[] {
  if (!value || typeof value !== "object") return [prefix];
  return Object.entries(value).flatMap(([key, child]) =>
    keys(child, prefix ? `${prefix}.${key}` : key),
  );
}

describe("translations", () => {
  it("keeps English and Chinese key-complete", () => {
    expect(keys(zh).sort()).toEqual(keys(en).sort());
  });
});
