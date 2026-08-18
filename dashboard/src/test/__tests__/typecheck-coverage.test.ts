import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

// w9/m85 regression tripwire. Test files were excluded from `tsc` (tsconfig.app
// excludes **/*.test.*) and vitest does not typecheck, so a fixture could drift
// from the type it claims to build and NOTHING failed. tsconfig.test.json now
// type-checks the test tree and `yarn typecheck`/`lint` run it. These two guards
// make a silent regression of that wiring a hard failure.

const dashboardRoot = resolve(import.meta.dirname, "../../..");
const readJSONC = (rel: string) =>
  JSON.parse(
    readFileSync(resolve(dashboardRoot, rel), "utf8")
      // Strip // line comments (tsconfig.test.json is JSONC) before JSON.parse.
      .replace(/^\s*\/\/.*$/gm, ""),
  );

describe("test-file typecheck coverage (w9/m85)", () => {
  it("keeps tsconfig.test.json type-checking the whole tree, tests included", () => {
    const cfg = readJSONC("tsconfig.test.json");
    // An empty (or absent) exclude is the whole point — re-adding a `*.test.*`
    // exclude here is exactly the silent regression this guards against.
    expect(cfg.exclude ?? []).toEqual([]);
    expect(cfg.include).toContain("src");
  });

  it("keeps the test typecheck wired into `yarn typecheck` (and thus lint + CI)", () => {
    const pkg = readJSONC("package.json");
    // CI runs `yarn typecheck` / `yarn lint`; both must reach the test project.
    expect(pkg.scripts.typecheck).toContain("tsc -p tsconfig.test.json");
    expect(pkg.scripts.lint).toContain("yarn typecheck");
  });

  it("actively type-checks this file (the directive below proves tsc reaches it)", () => {
    // If test files were re-excluded, tsc would never process this file, so the
    // wiring guards above are the real tripwire. While the typecheck IS live,
    // this deliberate error must be caught — delete the @ts-expect-error and
    // `yarn typecheck` turns red, which is the demonstration that fixture drift
    // (a wrong type in a test) is once again a hard failure.
    // @ts-expect-error TRIPWIRE: a string is not assignable to number.
    const deliberatelyWrong: number = "not a number";
    expect(typeof deliberatelyWrong).toBe("string");
  });
});
