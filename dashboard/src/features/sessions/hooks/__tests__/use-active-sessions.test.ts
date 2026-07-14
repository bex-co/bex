import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useActiveSessions } from "@/features/sessions/hooks/use-active-sessions";
import type { SessionView } from "@/features/sessions/types";

const CURRENT: SessionView = {
  id: "session-current",
  current: true,
  userAgent: "Chrome",
  authenticatedAt: null,
};
const OTHER: SessionView = {
  id: "session-other",
  current: false,
  userAgent: "Safari",
  authenticatedAt: null,
};

function stubFetch(impl: (url: string, init?: RequestInit) => Response) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => impl(url, init)),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useActiveSessions", () => {
  it("loads the list on mount", async () => {
    stubFetch(() => Response.json([CURRENT, OTHER]));
    const { result } = renderHook(() => useActiveSessions());

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.sessions).toEqual([CURRENT, OTHER]);
  });

  it("sets error on a failed list fetch", async () => {
    stubFetch(() => new Response("boom", { status: 500 }));
    const { result } = renderHook(() => useActiveSessions());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe(true);
  });

  it("revoke removes only that session and posts { action: revoke, id }", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    stubFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method === "POST") return new Response(null, { status: 204 });
      return Response.json([CURRENT, OTHER]);
    });
    const { result } = renderHook(() => useActiveSessions());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let ok = false;
    await act(async () => {
      ok = await result.current.revoke("session-other");
    });

    expect(ok).toBe(true);
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(post!.init!.body as string)).toEqual({
      action: "revoke",
      id: "session-other",
    });
  });

  it("signOutOthers keeps only the current session and posts { action: sign-out-others }", async () => {
    const calls: { url: string; init?: RequestInit }[] = [];
    stubFetch((url, init) => {
      calls.push({ url, init });
      if (init?.method === "POST")
        return Response.json({ count: 1 }, { status: 200 });
      return Response.json([CURRENT, OTHER]);
    });
    const { result } = renderHook(() => useActiveSessions());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let ok = false;
    await act(async () => {
      ok = await result.current.signOutOthers();
    });

    expect(ok).toBe(true);
    expect(result.current.sessions).toEqual([CURRENT]);
    const post = calls.find((c) => c.init?.method === "POST");
    expect(JSON.parse(post!.init!.body as string)).toEqual({
      action: "sign-out-others",
    });
  });
});
