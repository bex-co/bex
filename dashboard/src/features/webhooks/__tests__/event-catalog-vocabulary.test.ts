import { describe, expect, it } from "vitest";
import {
  catalogEntries,
  editablePickerEventTypes,
  eventLabelKey,
  pickerEventTypes,
} from "../event-catalog";
import deWebhooks from "../locales/de";
import enWebhooks from "../locales/en";
import esWebhooks from "../locales/es";
import jaWebhooks from "../locales/ja";
import koWebhooks from "../locales/ko";
import ptWebhooks from "../locales/pt";
import ruWebhooks from "../locales/ru";
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
) as {
  bexSupported: string[];
  renderDashboard: string[];
  renderOpenAPI: string[];
  antiGoalUnsupported: string[];
  sourceBoundUnsupported: string[];
};

describe("webhook event vocabulary", () => {
  it("labels every served type in the picker catalog across every locale", () => {
    const locales = {
      de: deWebhooks,
      en: enWebhooks,
      es: esWebhooks,
      ja: jaWebhooks,
      ko: koWebhooks,
      pt: ptWebhooks,
      ru: ruWebhooks,
      zh: zhWebhooks,
    };
    const missing: string[] = [];
    for (const type of fixture.bexSupported) {
      const key = eventLabelKey(type);
      if (!key) {
        missing.push(`${type}:no-key`);
        continue;
      }
      for (const [locale, messages] of Object.entries(locales)) {
        if (!(key in messages)) {
          missing.push(`${type}:${locale}`);
        }
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

  // w4/709: unemitted Render OpenAPI enum values are never advertised in the
  // create picker, but are labeled across all locales and surfaced in the
  // edit picker when already stored on a webhook.
  it("keeps unemitted Render enum values out of the create picker and labels them when stored", () => {
    const locales = {
      de: deWebhooks,
      en: enWebhooks,
      es: esWebhooks,
      ja: jaWebhooks,
      ko: koWebhooks,
      pt: ptWebhooks,
      ru: ruWebhooks,
      zh: zhWebhooks,
    };
    const unemitted = [
      ...fixture.antiGoalUnsupported,
      ...fixture.sourceBoundUnsupported,
    ];
    expect(unemitted.length).toBeGreaterThan(0);

    const picker = pickerEventTypes(fixture.bexSupported);
    for (const type of unemitted) {
      expect(fixture.renderOpenAPI).toContain(type);
      expect(fixture.bexSupported).not.toContain(type);
      expect(picker).not.toContain(type);

      const key = eventLabelKey(type);
      expect(key).toBe(`webhooks.event.${type}`);
      for (const messages of Object.values(locales)) {
        expect(key! in messages).toBe(true);
      }
    }

    const edited = editablePickerEventTypes(picker, [
      "deploy_ended",
      "maintenance_started",
      "edge_cache_enabled",
    ]);
    expect(edited).toContain("maintenance_started");
    expect(edited).not.toContain("maintenance_ended");
    expect(edited).not.toContain("edge_cache_enabled");

    const editedEntries = catalogEntries(edited);
    expect(
      editedEntries.find((entry) => entry.groupKey === "other"),
    ).toBeUndefined();
    const maintenance = editedEntries.find(
      (entry) => entry.groupKey === "maintenance",
    );
    expect(maintenance?.events.map((event) => event.type)).toEqual([
      "maintenance_started",
    ]);

    const allStoredEntries = catalogEntries(
      editablePickerEventTypes(picker, unemitted),
    );
    expect(
      allStoredEntries.find((entry) => entry.groupKey === "other"),
    ).toBeUndefined();
  });
});
