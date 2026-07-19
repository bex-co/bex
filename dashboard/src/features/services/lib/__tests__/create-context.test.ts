import { describe, expect, it } from "vitest";
import {
  parseNewServiceSearch,
  SERVICE_TYPES,
  SERVICE_TYPE_CREATE_ITEMS,
} from "../create-context";

describe("parseNewServiceSearch", () => {
  it("accepts each known service type", () => {
    for (const type of SERVICE_TYPES) {
      expect(parseNewServiceSearch({ type })).toEqual({ type });
    }
  });

  it("drops an unknown or non-string type", () => {
    expect(parseNewServiceSearch({ type: "bogus" })).toEqual({});
    expect(parseNewServiceSearch({ type: 42 })).toEqual({});
    expect(parseNewServiceSearch({})).toEqual({});
  });

  it("keeps nonempty projectId / environmentId and drops empties", () => {
    expect(
      parseNewServiceSearch({
        type: "cron_job",
        projectId: "prj-1",
        environmentId: "env-1",
      }),
    ).toEqual({ type: "cron_job", projectId: "prj-1", environmentId: "env-1" });
    expect(
      parseNewServiceSearch({ projectId: "", environmentId: 0 }),
    ).toEqual({});
  });
});

describe("SERVICE_TYPE_CREATE_ITEMS", () => {
  it("covers every service type exactly once, in menu order", () => {
    expect(SERVICE_TYPE_CREATE_ITEMS.map((i) => i.type)).toEqual([
      "web_service",
      "private_service",
      "background_worker",
      "cron_job",
      "static_site",
    ]);
    // every type is offered as a first-class create entry
    expect(new Set(SERVICE_TYPE_CREATE_ITEMS.map((i) => i.type))).toEqual(
      new Set(SERVICE_TYPES),
    );
  });

  it("gives each item a services.type* label key", () => {
    for (const item of SERVICE_TYPE_CREATE_ITEMS) {
      expect(item.labelKey).toMatch(/^services\.type/);
    }
  });
});
