import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";

import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";

// This badge serves TWO backend vocabularies and only the first was ever mapped
// (w10/m11/t005):
//
//   Blueprint.Status     — created | paused | in_sync | syncing | error
//   BlueprintSync.State  — created | running | success | error
//
// An unmapped value fell through to a label whose message is literally
// "{status}", so Sync History rendered plain lowercase "success" beside a
// styled red "Error" in the same column. These assertions fail the moment a
// real backend value stops being mapped — which is the regression that
// actually happened.
const BLUEPRINT_STATUSES = ["created", "paused", "in_sync", "syncing", "error"];
const SYNC_STATES = ["created", "running", "success", "error"];

describe("BlueprintStatusBadge", () => {
  it.each([...new Set([...BLUEPRINT_STATUSES, ...SYNC_STATES])])(
    "renders %s with a real label, not the raw backend value",
    (status) => {
      render(<BlueprintStatusBadge status={status} />);
      const badge = screen.getByText((_, el) => el?.textContent?.trim() === el?.textContent && true, {
        selector: '[data-slot="badge"]',
      });
      const label = badge.textContent?.trim() ?? "";

      expect(label).not.toBe("");
      // The fallback renders the raw value verbatim — lowercase and
      // underscored. A real label never looks like that.
      expect(label).not.toBe(status);
      expect(label).not.toContain("_");
    },
  );

  it("gives failures a destructive variant and successes a non-destructive one", () => {
    // Assert on the variant's own background, not any mention of the word:
    // shadcn's base classes reference destructive in `aria-invalid:` rules for
    // every variant, so a substring match would pass for anything.
    const { unmount } = render(<BlueprintStatusBadge status="error" />);
    expect(screen.getByText("Error").className).toContain("bg-destructive");
    unmount();

    render(<BlueprintStatusBadge status="success" />);
    // Success must not borrow the failure styling — the two sit in the same
    // column and are read at a glance.
    expect(screen.getByText("Success").className).not.toContain("bg-destructive");
  });

  it("still renders something for an unknown value rather than blanking", () => {
    // Defensive: a value neither vocabulary defines should degrade visibly, not
    // vanish. This is the fallback's legitimate job.
    render(<BlueprintStatusBadge status="some_future_state" />);
    expect(screen.getByText(/some_future_state/)).toBeInTheDocument();
  });
});
