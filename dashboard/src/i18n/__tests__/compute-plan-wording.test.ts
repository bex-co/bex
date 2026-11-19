import { describe, expect, it } from "vitest";
import { en } from "@/i18n";

// Render renamed instance types to compute plans on 2026-08-26: "'Compute
// plan' replaces 'instance type' across the dashboard, API, CLI, and docs"
// (docs/render-artifacts/compute-page.md, w6/m483). No English message may
// call a plan an instance type again, except a name Render itself still
// publishes: its dashboard calls the plan_changed event "Instance Type
// Changed" (render.com/docs/webhooks).
const RENDER_STILL_PUBLISHES = new Set([
  "services.eventsTypePlanChanged",
  "metrics.eventPlanChanged",
]);

describe("compute plan wording (w6/m483)", () => {
  it("never calls a plan an instance type", () => {
    const offenders = Object.entries(en)
      .filter(
        ([key, message]) =>
          /\binstance[ -]types?\b/i.test(message) &&
          !RENDER_STILL_PUBLISHES.has(key),
      )
      .map(([key, message]) => `${key}: ${message}`);
    expect(offenders).toEqual([]);
  });

  it("keeps the event name Render's dashboard still shows", () => {
    for (const key of RENDER_STILL_PUBLISHES) {
      expect(en[key]).toMatch(/^Instance type changed$/i);
    }
  });
});
