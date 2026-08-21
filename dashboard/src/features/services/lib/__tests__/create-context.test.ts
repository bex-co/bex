import { describe, expect, it } from "vitest";
import {
  parseNewServiceSearch,
  serviceTypeCreateCopy,
  DEFAULT_SERVICE_TYPE,
  SERVICE_TYPES,
  SERVICE_TYPE_CREATE_ITEMS,
  SERVICE_TYPE_CREATE_COPY,
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
    expect(parseNewServiceSearch({ projectId: "", environmentId: 0 })).toEqual(
      {},
    );
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

describe("serviceTypeCreateCopy", () => {
  it("covers every service type with a distinct title", () => {
    const titles = SERVICE_TYPES.map(
      (t) => SERVICE_TYPE_CREATE_COPY[t].titleKey,
    );
    expect(titles).toHaveLength(SERVICE_TYPES.length);
    expect(new Set(titles).size).toBe(SERVICE_TYPES.length);
  });

  it("never labels a non-web type with the web copy", () => {
    const web = SERVICE_TYPE_CREATE_COPY.web_service;
    for (const type of SERVICE_TYPES) {
      if (type === "web_service") continue;
      expect(SERVICE_TYPE_CREATE_COPY[type]).not.toEqual(web);
    }
  });

  // The route's document title reads `?type=` while the form falls back to its
  // own default, so an absent type has to resolve the same way on both paths —
  // otherwise a bare /services/new shows one name in the tab and another in the
  // heading, which is what it did before w6/m43.
  it("resolves an absent type to the form's own default", () => {
    expect(serviceTypeCreateCopy(undefined)).toEqual(
      serviceTypeCreateCopy(DEFAULT_SERVICE_TYPE),
    );
  });
});
