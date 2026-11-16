import { describe, expect, it } from "vitest";

import { gaInitScript, gaMeasurementId } from "../google-analytics";

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

describe("gaInitScript", () => {
  it("configures the stream", () => {
    expect(gaInitScript("G-7X5XTVZ55N")).toContain(
      "gtag('config','G-7X5XTVZ55N');",
    );
  });
});
