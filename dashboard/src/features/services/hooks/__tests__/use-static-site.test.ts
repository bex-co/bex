import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { toServiceView } from "@/features/services/lib/status";
import type { ServiceView } from "@/features/services/types";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { useStaticSiteMutations } from "@/features/services/hooks/use-static-site";

// w4/136: bex-api trims accepted route/header paths, so the edge-rule setters
// hand the editor what the read-back says was stored — projected through the
// same read mapping the page renders, never the raw wire rows.
const storedNode = {
  __typename: "Server",
  id: "srv-1",
  routes: [
    null,
    {
      __typename: "StaticRoute",
      type: "rewrite",
      source: "/qa-alias/*",
      destination: "/:splat",
    },
  ],
  headers: [
    null,
    {
      __typename: "StaticHeader",
      path: "/qa/*",
      name: "X-QA-Overlap",
      value: " scoped ",
    },
  ],
} as unknown as Parameters<typeof toServiceView>[0];

function renderMutations(views: ServiceView[]) {
  const refetch = vi.fn(async () => views);
  return renderHook(() => useStaticSiteMutations("srv-1", refetch));
}

beforeEach(() => {
  mutate.mockReset();
  mutate.mockResolvedValue({ data: {} });
});

describe("useStaticSiteMutations", () => {
  it("resolves route and header saves to the stored rules of the written service", async () => {
    const other = { ...toServiceView(storedNode), id: "srv-other" };
    const { result } = renderMutations([other, toServiceView(storedNode)]);

    let routes: unknown;
    let headers: unknown;
    await act(async () => {
      routes = await result.current.setRoutes([
        { type: "rewrite", source: " /qa-alias/* ", destination: " /:splat " },
      ]);
      headers = await result.current.setHeaders([
        { path: " /qa/* ", name: "X-QA-Overlap", value: " scoped " },
      ]);
    });

    expect(routes).toStrictEqual({
      ok: true,
      saved: [
        { type: "rewrite", source: "/qa-alias/*", destination: "/:splat" },
      ],
    });
    // Header value bytes are the server's, untouched by the dashboard.
    expect(headers).toStrictEqual({
      ok: true,
      saved: [{ path: "/qa/*", name: "X-QA-Overlap", value: " scoped " }],
    });
  });

  it("reports an absent read-back as saved-but-unread, not an empty list", async () => {
    const { result } = renderMutations([]);

    let routes: unknown = "unset";
    let headers: unknown = "unset";
    await act(async () => {
      routes = await result.current.setRoutes([]);
      headers = await result.current.setHeaders([]);
    });

    expect(routes).toStrictEqual({ ok: true });
    expect(headers).toStrictEqual({ ok: true });
  });

  it("reports a refused write as not ok and keeps publish path a boolean", async () => {
    mutate.mockRejectedValue(new Error("invalid route"));
    const { result } = renderMutations([toServiceView(storedNode)]);

    let routes: unknown = "unset";
    let publish: unknown = "unset";
    await act(async () => {
      routes = await result.current.setRoutes([]);
      publish = await result.current.setPublishPath("dist");
    });

    expect(routes).toStrictEqual({ ok: false });
    expect(publish).toBe(false);

    mutate.mockResolvedValue({ data: {} });
    await act(async () => {
      publish = await result.current.setPublishPath("dist");
    });
    expect(publish).toBe(true);
  });
});
