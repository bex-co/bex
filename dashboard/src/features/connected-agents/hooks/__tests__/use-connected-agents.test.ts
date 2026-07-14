import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useConnectedAgents } from "@/features/connected-agents/hooks/use-connected-agents";
import type { ConnectedAgentView } from "@/features/connected-agents/types";

const AGENT: ConnectedAgentView = {
  clientId: "agent-1",
  clientName: "Claude Code",
  scopes: ["openid"],
  grantedAt: "2026-01-01T00:00:00.000Z",
};

function stubFetch(impl: (url: string, init?: RequestInit) => Response) {
  vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => impl(url, init)));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useConnectedAgents", () => {
  it("loads the list on mount", async () => {
    stubFetch(() => Response.json([AGENT]));
    const { result } = renderHook(() => useConnectedAgents());

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.agents).toEqual([AGENT]);
    expect(result.current.error).toBe(false);
  });

  it("sets error on a failed list fetch", async () => {
    stubFetch(() => new Response("boom", { status: 500 }));
    const { result } = renderHook(() => useConnectedAgents());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe(true);
    expect(result.current.agents).toEqual([]);
  });

  it("revoke removes the row on success and posts the clientId", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    stubFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method === "POST") return new Response(null, { status: 204 });
      return Response.json([AGENT]);
    });
    const { result } = renderHook(() => useConnectedAgents());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let ok = false;
    await act(async () => {
      ok = await result.current.revoke("agent-1", "Claude Code");
    });

    expect(ok).toBe(true);
    const post = calls.find((c) => c.init?.method === "POST");
    expect(post?.url).toContain("/api/connected-agents");
    expect(JSON.parse(post!.init!.body as string)).toEqual({
      clientId: "agent-1",
    });
  });

  it("revoke resolves false on failure and keeps the row", async () => {
    stubFetch((_, init) => {
      if (init?.method === "POST") return new Response("boom", { status: 500 });
      return Response.json([AGENT]);
    });
    const { result } = renderHook(() => useConnectedAgents());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let ok = true;
    await act(async () => {
      ok = await result.current.revoke("agent-1", "Claude Code");
    });
    expect(ok).toBe(false);
  });
});
