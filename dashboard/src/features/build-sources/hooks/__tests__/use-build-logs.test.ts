import { describe, expect, it } from "vitest";
import { subscribeUrl } from "@/features/logs/hooks/use-live-logs";

const EMPTY_FILTERS = {
  level: "",
  method: "",
  statusCode: "",
  path: "",
  host: "",
};

describe("subscribeUrl build-source scoping (w2/m614/t004)", () => {
  it("appends build=bld-… alongside resource=bds-… and type=build when provided", () => {
    const url = subscribeUrl({
      resource: "bds-abc123",
      build: "bld-xyz789",
      type: "build",
      text: "",
      textMode: undefined,
      instance: "",
      startTime: "2026-11-24T00:00:00.000Z",
      requestFilters: EMPTY_FILTERS,
    });
    const parsed = new URL(url, "https://api.bex.co");
    expect(parsed.pathname).toBe("/v1/logs/subscribe");
    expect(parsed.searchParams.get("resource")).toBe("bds-abc123");
    expect(parsed.searchParams.get("build")).toBe("bld-xyz789");
    expect(parsed.searchParams.get("type")).toBe("build");
  });

  it("omits build parameter when not provided", () => {
    const url = subscribeUrl({
      resource: "srv-abc123",
      type: "app",
      text: "",
      textMode: undefined,
      instance: "",
      startTime: "2026-11-24T00:00:00.000Z",
      requestFilters: EMPTY_FILTERS,
    });
    const parsed = new URL(url, "https://api.bex.co");
    expect(parsed.searchParams.has("build")).toBe(false);
  });
});
