import fs from "node:fs";
import path from "node:path";

const source = fs.readFileSync(
  path.resolve(
    process.cwd(),
    "src/features/agent-sessions/composer/repository-picker.tsx",
  ),
  "utf8",
);

describe("repository picker keyboard layout", () => {
  it("shrinks the sheet into the keyboard-safe viewport", () => {
    expect(
      source.includes(
        'behavior={Platform.OS === "ios" ? "height" : undefined}',
      ),
    ).toBe(true);
    expect(source.includes("flexShrink: 1")).toBe(true);
    expect(source.includes("paddingTop: insets.top + space.sm")).toBe(true);
  });

  it("keeps the close affordance at the native minimum target size", () => {
    expect(source.includes("minWidth: rowMinHeight")).toBe(true);
    expect(source.includes("minHeight: rowMinHeight")).toBe(true);
  });
});
