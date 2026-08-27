import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useReauthDraft } from "../use-reauth-draft";

interface Draft {
  vars: Array<{ key: string; value: string }>;
}

const draft: Draft = { vars: [{ key: "API_KEY", value: "typed-secret" }] };

describe("useReauthDraft (w3/m80 t005)", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });
  afterEach(() => {
    vi.useRealTimers();
    window.sessionStorage.clear();
  });

  it("round-trips a saved draft, then deletes it on consume", () => {
    const { result } = renderHook(() => useReauthDraft<Draft>("env:srv-1"));

    result.current.save(draft);
    expect(result.current.consumeRestored()).toEqual(draft);
    // consume is single-use: a second read finds nothing (the redirect already
    // rehydrated it once).
    expect(result.current.consumeRestored()).toBeNull();
  });

  it("returns null when nothing was saved", () => {
    const { result } = renderHook(() => useReauthDraft<Draft>("env:srv-2"));
    expect(result.current.consumeRestored()).toBeNull();
  });

  it("clear() removes a saved draft so no stale restore survives a real save", () => {
    const { result } = renderHook(() => useReauthDraft<Draft>("env:srv-3"));

    result.current.save(draft);
    result.current.clear();
    expect(result.current.consumeRestored()).toBeNull();
  });

  it("drops a draft older than the TTL", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:00Z"));
    const { result } = renderHook(() =>
      useReauthDraft<Draft>("env:srv-4", 60_000),
    );

    result.current.save(draft);
    vi.advanceTimersByTime(61_000);
    expect(result.current.consumeRestored()).toBeNull();
  });

  it("keeps two resources' drafts separate", () => {
    const a = renderHook(() => useReauthDraft<Draft>("env:srv-a")).result;
    const b = renderHook(() => useReauthDraft<Draft>("env:srv-b")).result;

    a.current.save(draft);
    expect(b.current.consumeRestored()).toBeNull();
    expect(a.current.consumeRestored()).toEqual(draft);
  });
});
