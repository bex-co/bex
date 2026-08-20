import { describe, expect, it, beforeEach } from "vitest";
import { stashAuthNext, takeAuthNext, clearAuthNext } from "../auth-next";

// The same-tab relay that carries a guarded `?next=` across the D8 auth
// restructure (ADR075 D3/D8, w6/m42): sign-up → verification → login. The
// security property under test: nothing that leaves this module can be an
// open redirect — unsafe values die at write AND at read.
describe("auth-next relay", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("round-trips a safe relative target and clears on take", () => {
    stashAuthNext("/services/srv-1?tab=env#top");
    expect(takeAuthNext()).toBe("/services/srv-1?tab=env#top");
    expect(takeAuthNext()).toBeUndefined(); // consumed
  });

  it("stashes nothing for absent or default targets", () => {
    stashAuthNext(undefined);
    expect(takeAuthNext()).toBeUndefined();
    stashAuthNext("/");
    expect(takeAuthNext()).toBeUndefined();
  });

  it("normalizes unsafe targets away at write", () => {
    stashAuthNext("https://evil.example/phish");
    expect(takeAuthNext()).toBeUndefined();
    stashAuthNext("//evil.example/phish");
    expect(takeAuthNext()).toBeUndefined();
  });

  it("re-validates at read so a tampered stash cannot redirect", () => {
    sessionStorage.setItem("bex.auth.next", "https://evil.example/phish");
    expect(takeAuthNext()).toBeUndefined();
    sessionStorage.setItem("bex.auth.next", "/\\evil.example");
    expect(takeAuthNext()).toBeUndefined();
  });

  it("clearAuthNext drops a pending relay", () => {
    stashAuthNext("/usage");
    clearAuthNext();
    expect(takeAuthNext()).toBeUndefined();
  });
});
