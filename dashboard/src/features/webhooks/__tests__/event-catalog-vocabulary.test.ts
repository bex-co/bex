import { describe, expect, it } from "vitest";
import {
  catalogEntries,
  eventLabelKey,
  pickerEventTypes,
} from "../event-catalog";
import enWebhooks from "../locales/en";
import zhWebhooks from "../locales/zh";
import { readFileSync } from "node:fs";

// The Go side asserts `webhooks.EventTypes` equals this fixture's
// `bexSupported` (vocabulary_test.go), so the fixture is the served vocabulary.
// A served type with no picker label leaked as raw snake_case under "Other"
// (w4/039 service_moved, w21/021 disk_snapshot_restored); this is the guard.
const fixture = JSON.parse(
  readFileSync(
    `${process.cwd()}/../docs/render-artifacts/fixtures/render-webhook-vocabulary-2026-11-02.json`,
    "utf8",
  ),
) as { bexSupported: string[]; renderDashboard: string[] };

describe("webhook event vocabulary", () => {
  it("labels every served type in the picker catalog, en and zh", () => {
    const missing: string[] = [];
    for (const type of fixture.bexSupported) {
      const key = eventLabelKey(type);
      if (!key || !(key in enWebhooks) || !(key in zhWebhooks)) {
        missing.push(type);
      }
    }
    expect(missing).toEqual([]);
  });

  it("leaves no served type under Other", () => {
    const entries = catalogEntries(fixture.bexSupported);
    expect(entries.find((entry) => entry.groupKey === "other")).toBeUndefined();
  });

  // w9/m345: Render's API accepts the three edge-cache types and its dashboard
  // picker does not list them; pipeline_minutes_exhausted is in both. The
  // served vocabulary carries all four, so the picker is what drops them.
  it("offers pipeline_minutes_exhausted and keeps the edge-cache types API-only", () => {
    const picker = pickerEventTypes(fixture.bexSupported);
    const edgeCache = [
      "edge_cache_enabled",
      "edge_cache_disabled",
      "edge_cache_purged",
    ];
    for (const type of edgeCache) {
      expect(fixture.bexSupported).toContain(type);
      expect(fixture.renderDashboard).not.toContain(type);
      expect(picker).not.toContain(type);
    }
    expect(fixture.renderDashboard).toContain("pipeline_minutes_exhausted");
    const deploy = catalogEntries(picker).find(
      (entry) => entry.groupKey === "deploy",
    );
    expect(deploy?.events.map((event) => event.type)).toContain(
      "pipeline_minutes_exhausted",
    );
    const listed = catalogEntries(picker).flatMap((entry) =>
      entry.events.map((event) => event.type),
    );
    expect(listed.filter((type) => edgeCache.includes(type))).toEqual([]);
  });
});
