import { resolve } from "node:path";
import { ESLint } from "eslint";
import { describe, expect, it } from "vitest";

// `yarn lint` runs the jsx-a11y rules on the app's JSX: this asks ESLint what
// config it resolves for real files, so removing the plugin (or the rules, or
// widening the scope to the tests) fails here, not at the next review.
const cwd = resolve(import.meta.dirname, "../../..");
const eslint = new ESLint({ cwd });
const severity = (value: unknown) => (Array.isArray(value) ? value[0] : value);

describe("the accessibility lint", () => {
  it("is an error on the app's components", async () => {
    const config = await eslint.calculateConfigForFile(
      "src/common/components/ui/button.tsx",
    );
    for (const rule of [
      "jsx-a11y-x/alt-text",
      "jsx-a11y-x/click-events-have-key-events",
      "jsx-a11y-x/no-static-element-interactions",
      "jsx-a11y-x/label-has-associated-control",
      "jsx-a11y-x/anchor-is-valid",
    ]) {
      expect(severity(config?.rules?.[rule]), rule).toBe(2);
    }
  });

  it("checks only DOM elements for autoFocus", async () => {
    const config = await eslint.calculateConfigForFile(
      "src/common/components/ui/button.tsx",
    );
    expect(config?.rules?.["jsx-a11y-x/no-autofocus"]).toEqual([
      2,
      { ignoreNonDOM: true },
    ]);
  });

  it("does not run on a test's fixtures", async () => {
    const config = await eslint.calculateConfigForFile(
      "src/test/__tests__/axe.test.tsx",
    );
    expect(config?.rules?.["jsx-a11y-x/alt-text"]).toBeUndefined();
  });
});
