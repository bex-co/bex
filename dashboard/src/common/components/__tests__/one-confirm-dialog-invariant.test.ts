import { describe, it, expect } from "vitest";

// The dashboard has exactly ONE confirm-dialog implementation (w1/m89).
//
// Before this guard, 25 files had each re-spelled the same header/description/
// cancel/confirm JSX. That is not a style problem: it meant any improvement to
// the shape — a pending spinner, a consistent destructive variant, an aria fix,
// disabling cancel mid-flight — had to land 25 times or land inconsistently,
// and it landed inconsistently. Two sites had even hand-rolled their own
// typed-confirmation gate because no shared one existed.
//
// So this fails the build when a component reaches for the raw kit again,
// rather than letting the pattern quietly regrow. Same shape as
// routes/__tests__/one-rail-invariant.test.ts, which guards the sidebar the
// same way and for the same reason.
const modules = import.meta.glob("../../../**/*.{ts,tsx}", {
  eager: true,
  query: "?raw",
  import: "default",
});

/** The kit itself, and the one primitive allowed to build on it. */
const ALLOWED = ["/ui/alert-dialog.tsx", "/confirm-dialog.tsx"];

describe("one confirm dialog", () => {
  it("discovers source files (the glob is not silently empty)", () => {
    // A glob that stops matching would turn this guard into a no-op that still
    // reports green.
    expect(Object.keys(modules).length).toBeGreaterThan(300);
  });

  it("has no hand-rolled AlertDialog outside the primitive", () => {
    const offenders: string[] = [];
    for (const [path, source] of Object.entries(modules)) {
      if (typeof source !== "string") continue;
      if (ALLOWED.some((allowed) => path.endsWith(allowed))) continue;
      if (path.includes("__tests__")) continue;
      // Importing the kit's pieces directly is the tell: a component building
      // its own dialog needs AlertDialogCancel/Action/Content.
      if (/\bAlertDialog(Cancel|Action|Content|Footer|Header)\b/.test(source)) {
        offenders.push(path.replace("../../../", "src/"));
      }
    }

    expect(
      offenders,
      `these build their own confirm dialog instead of using ConfirmDialog:\n  ${offenders.join("\n  ")}`,
    ).toEqual([]);
  });
});
