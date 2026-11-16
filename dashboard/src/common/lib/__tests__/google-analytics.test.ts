import { describe, expect, it } from "vitest";

import {
  GA_READY_EVENT,
  GA_INIT_SCRIPT,
  gaMeasurementId,
} from "../google-analytics";

describe("gaMeasurementId", () => {
  it("accepts a GA4 measurement ID", () => {
    expect(gaMeasurementId("G-7X5XTVZ55N")).toBe("G-7X5XTVZ55N");
    expect(gaMeasurementId(" G-7X5XTVZ55N\n")).toBe("G-7X5XTVZ55N");
  });

  it("loads nothing when unset or malformed", () => {
    expect(gaMeasurementId(undefined)).toBeNull();
    expect(gaMeasurementId("")).toBeNull();
    expect(gaMeasurementId("UA-123-1")).toBeNull();
    expect(gaMeasurementId("G-X');alert(1);//")).toBeNull();
  });
});

describe("GA_INIT_SCRIPT", () => {
  const run = (webdriver: boolean) => {
    const pushed: unknown[] = [];
    // In a document `dataLayer` and `location` are globals.
    new Function(
      "window",
      "navigator",
      "location",
      "dataLayer",
      GA_INIT_SCRIPT,
    )(
      { dataLayer: pushed },
      { webdriver },
      {
        origin: "https://dashboard.bex.co",
        href: "https://dashboard.bex.co/invite?token=secret",
      },
      pushed,
    );
    return pushed.map((entry) =>
      "event" in (entry as object)
        ? entry
        : Array.from(entry as ArrayLike<unknown>),
    );
  };

  it("fires the tag only after an empty page context is set and the automatic page view is off", () => {
    const [js, context, ready, ...rest] = run(false);
    expect((js as unknown[])[0]).toBe("js");
    expect(context).toEqual([
      "set",
      {
        send_page_view: false,
        page_location: "https://dashboard.bex.co/",
        page_title: "(loading)",
      },
    ]);
    expect(ready).toEqual({ event: GA_READY_EVENT });
    expect(rest).toEqual([]);
  });

  it("marks an automated browser's traffic internal before the tag fires", () => {
    const pushed = run(true);
    expect(pushed.at(-2)).toEqual(["set", { traffic_type: "internal" }]);
    expect(pushed.at(-1)).toEqual({ event: GA_READY_EVENT });
  });

  it("never issues an on-page config, which the container ignores", () => {
    expect(GA_INIT_SCRIPT).not.toContain("config");
  });
});
