import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

/**
 * The Ory theme-CSS scoping guard.
 *
 * `@ory/elements-react/theme/styles.css` is ~66 KB raw and used to be injected
 * into EVERY page's SSR document from the root route's head(). Only the routes
 * that render Ory flow components need it; everywhere else it's dead weight.
 * This guard pins both halves of that: the root never imports the sheet, and
 * the set of routes injecting `oryThemeStyle` is exactly the Ory-flow set (so
 * a new Ory page that forgets the style — or the style creeping back into a
 * shared spot — fails here).
 */
const ROUTES_DIR = join(import.meta.dirname, "..");

const ORY_FLOW_ROUTES = [
  "auth.forgot-password.tsx",
  "auth.login.tsx",
  "auth.reset-password.tsx",
  "auth.sign-up.tsx",
  "auth.verification.tsx",
  "settings.tsx",
];

function routeFiles(): string[] {
  return readdirSync(ROUTES_DIR)
    .filter((f) => f.endsWith(".tsx") && !f.startsWith("api."))
    .sort();
}

describe("Ory theme style scoping", () => {
  it("the root route never imports the Ory theme sheet", () => {
    const root = readFileSync(join(ROUTES_DIR, "__root.tsx"), "utf8");
    expect(root).not.toMatch(/@ory\/elements-react\/theme\/styles\.css/);
  });

  it("exactly the Ory-flow routes inject oryThemeStyle", () => {
    const injecting = routeFiles().filter((file) =>
      /^import\s+\{\s*oryThemeStyle\s*\}/m.test(
        readFileSync(join(ROUTES_DIR, file), "utf8"),
      ),
    );
    expect(injecting).toEqual(ORY_FLOW_ROUTES);
  });
});
