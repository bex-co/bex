import { describe, it, expect } from "vitest";
import { blueprintResourceHref } from "@/features/blueprints/lib/resource-href";

// A blueprint's Managed Resources table was the one resource table in the
// product whose names were plain text, so a reader had to leave the page and
// search the resource by name (w4/101). The backend's `type` is a closed set
// produced in exactly four places in blueprint.go; this pins each arm of it.
describe("blueprintResourceHref", () => {
  it("sends every backend resource type to its own detail page", () => {
    expect(blueprintResourceHref("web_service", "srv-1")).toBe(
      "/services/srv-1",
    );
    expect(blueprintResourceHref("private_service", "srv-2")).toBe(
      "/services/srv-2",
    );
    expect(blueprintResourceHref("background_worker", "srv-3")).toBe(
      "/services/srv-3",
    );
    expect(blueprintResourceHref("cron_job", "srv-4")).toBe("/services/srv-4");
    expect(blueprintResourceHref("postgres", "dpg-1")).toBe("/databases/dpg-1");
    expect(blueprintResourceHref("key_value", "red-1")).toBe("/keyvalue/red-1");
    expect(blueprintResourceHref("environment_group", "evg-1")).toBe(
      "/env-groups/evg-1",
    );
  });

  // A static site is canonical under /static/<id>, not /services/<id>
  // (Render parity, w5/m57). The helper defers to serviceBaseForType rather
  // than carrying a second copy of that rule, so this arm cannot drift from
  // the rest of the product's service links.
  it("keeps a static site on its canonical base", () => {
    expect(blueprintResourceHref("static_site", "srv-5")).toBe("/static/srv-5");
  });

  // A resource kind this build has no page for must render as plain text — the
  // pre-fix behavior for everything — never as a broken link.
  it("returns no href rather than a broken one", () => {
    expect(blueprintResourceHref("future_resource_kind", "x-1")).toBeNull();
    expect(blueprintResourceHref("", "x-1")).toBeNull();
    expect(blueprintResourceHref(null, "x-1")).toBeNull();
    expect(blueprintResourceHref(undefined, "x-1")).toBeNull();
  });

  // A resource whose backing object was deleted out of band has no id to link.
  it("returns no href when the resource has no id", () => {
    expect(blueprintResourceHref("web_service", "")).toBeNull();
    expect(blueprintResourceHref("web_service", null)).toBeNull();
    expect(blueprintResourceHref("web_service", undefined)).toBeNull();
  });
});
