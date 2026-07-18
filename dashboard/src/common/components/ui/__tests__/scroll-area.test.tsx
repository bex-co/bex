import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { ScrollArea } from "@/common/components/ui/scroll-area";

describe("ScrollArea containment (w1/m49/t007)", () => {
  it("viewportClassName lands on the viewport — the element that actually scrolls", () => {
    const { container } = render(
      <ScrollArea className="pr-3" viewportClassName="max-h-72">
        <div>content</div>
      </ScrollArea>,
    );
    const root = container.querySelector('[data-slot="scroll-area"]');
    const viewport = container.querySelector(
      '[data-slot="scroll-area-viewport"]',
    );
    expect(viewport?.className).toContain("max-h-72");
    expect(root?.className).not.toContain("max-h-72");
  });

  // The live bug this pins (found 2026-07-17 on dashboard.bex.co/webhooks):
  // `max-h-*` on the ScrollArea ROOT constrains nothing — the viewport is
  // `size-full` inside an auto-height root, so the list renders full height,
  // overflows the dialog, and content hit-test-intercepts the footer buttons
  // (clicking Create toggled a checkbox instead of submitting). jsdom can't
  // do layout, so the regression guard is a source sweep: no call site may
  // pass a height constraint through `className`.
  it("no call site passes h-*/max-h-* to ScrollArea via className", () => {
    const srcRoot = join(__dirname, "..", "..", "..", "..");
    const offenders: string[] = [];
    const scrollAreaRe =
      /<ScrollArea[^>]*\bclassName="[^"]*\b(?:max-h-|h-\d|h-\[)[^"]*"/g;

    function walk(dir: string) {
      for (const entry of readdirSync(dir)) {
        const path = join(dir, entry);
        if (statSync(path).isDirectory()) {
          if (entry === "node_modules") continue;
          walk(path);
        } else if (path.endsWith(".tsx") && !path.includes("__tests__")) {
          const source = readFileSync(path, "utf8");
          if (scrollAreaRe.test(source)) offenders.push(path);
          scrollAreaRe.lastIndex = 0;
        }
      }
    }
    walk(srcRoot);
    expect(offenders).toEqual([]);
  });
});
