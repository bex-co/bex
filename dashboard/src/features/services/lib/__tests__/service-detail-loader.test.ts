import { describe, it, expect, vi } from "vitest";
import { isRedirect, type ParsedLocation } from "@tanstack/react-router";
import { loadServiceDetail } from "../service-detail-loader";
import type { RouterContext } from "@/common/types/router-context";

function clientReturning(server: unknown): RouterContext["client"] {
  return {
    query: vi.fn(async () => ({ data: { server }, error: undefined })),
  } as unknown as RouterContext["client"];
}
function locationOf(pathname: string): ParsedLocation {
  return { pathname, href: pathname } as ParsedLocation;
}
function hrefOf(err: unknown): string | undefined {
  return (err as { options?: { href?: string } }).options?.href;
}

describe("loadServiceDetail base canonicalization (w5/m57)", () => {
  it("bounces a static_site under /services to its /static twin, preserving the sub-path", async () => {
    const client = clientReturning({
      id: "srv-1",
      name: "site",
      type: "static_site",
    });
    const err = await loadServiceDetail(
      client,
      "srv-1",
      "/services",
      locationOf("/services/srv-1/settings"),
    ).catch((e) => e);
    expect(isRedirect(err)).toBe(true);
    expect(hrefOf(err)).toBe("/static/srv-1/settings");
  });

  it("bounces a compute service under /static back to /services", async () => {
    const client = clientReturning({
      id: "srv-2",
      name: "api",
      type: "web_service",
    });
    const err = await loadServiceDetail(
      client,
      "srv-2",
      "/static",
      locationOf("/static/srv-2/env"),
    ).catch((e) => e);
    expect(isRedirect(err)).toBe(true);
    expect(hrefOf(err)).toBe("/services/srv-2/env");
  });

  it("renders in place when already on the canonical base", async () => {
    const client = clientReturning({
      id: "srv-1",
      name: "site",
      type: "static_site",
    });
    const result = await loadServiceDetail(
      client,
      "srv-1",
      "/static",
      locationOf("/static/srv-1/events"),
    );
    expect(result).toMatchObject({
      state: "ready",
      resource: { type: "static_site" },
    });
  });

  it("does not redirect a not-found service (keeps the shell's not-found state)", async () => {
    const client = clientReturning(null);
    const result = await loadServiceDetail(
      client,
      "srv-x",
      "/static",
      locationOf("/static/srv-x/events"),
    );
    expect(result).toEqual({ state: "not-found" });
  });
});
