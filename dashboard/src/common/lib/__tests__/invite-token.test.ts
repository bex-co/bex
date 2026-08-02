import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  INVITE_TOKEN_STORAGE_KEY,
  retainPendingInviteToken,
  stashInviteTokenFromURL,
  takePendingInviteToken,
  validInviteToken,
} from "@/common/lib/invite-token";

const TOKEN = "0123456789abcdef0123456789abcdef";
const OLD_TOKEN = "abcdef0123456789abcdef0123456789";

describe("invite token capture", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("accepts only the exact generated token shape", () => {
    expect(validInviteToken(TOKEN)).toBe(true);
    expect(validInviteToken(TOKEN.toUpperCase())).toBe(false);
    expect(validInviteToken(`${TOKEN}0`)).toBe(false);
    expect(validInviteToken("short")).toBe(false);
    expect(validInviteToken([TOKEN])).toBe(false);
  });

  it("stashes only the token and scrubs the dedicated handoff URL", () => {
    window.history.replaceState(
      null,
      "",
      `/invite?invite=${TOKEN}&utm=x#fragment`,
    );

    expect(stashInviteTokenFromURL({ scrubAll: true })).toBe("stored");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
    expect(window.location.pathname).toBe("/invite");
    expect(window.location.search).toBe("");
    expect(window.location.hash).toBe("");
    expect(window.sessionStorage.length).toBe(1);
  });

  it.each([
    `invite=${TOKEN}&invite=${TOKEN}`,
    `invite=${TOKEN.toUpperCase()}`,
    `invite=${TOKEN}0`,
    "invite=not-hex",
  ])("rejects malformed or duplicate values: %s", (query) => {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
    window.history.replaceState(null, "", `/invite?${query}`);

    expect(stashInviteTokenFromURL({ scrubAll: true })).toBe("invalid");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(window.location.href).not.toContain("invite=");
  });

  it("preserves unrelated auth-flow parameters while removing the bearer", () => {
    window.history.replaceState(
      null,
      "",
      `/auth/sign-up?flow=ory-flow&invite=${TOKEN}#safe`,
    );

    expect(stashInviteTokenFromURL()).toBe("stored");
    expect(window.location.search).toBe("?flow=ory-flow");
    expect(window.location.hash).toBe("#safe");
  });

  it("clears an old token and scrubs the URL before a replacement write fails", () => {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, OLD_TOKEN);
    window.history.replaceState(null, "", `/invite?invite=${TOKEN}`);
    vi.spyOn(Storage.prototype, "setItem").mockImplementationOnce(() => {
      expect(window.location.search).toBe("");
      throw new DOMException("storage denied", "QuotaExceededError");
    });

    expect(stashInviteTokenFromURL({ scrubAll: true })).toBe("unavailable");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
    expect(window.location.href).not.toContain(TOKEN);
  });
});

describe("invite token consumption", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    window.history.replaceState(null, "", "/");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("strictly consumes and immediately scrubs one valid URL token", () => {
    window.history.replaceState(
      null,
      "",
      `/services?tab=events&invite=${TOKEN}#latest`,
    );

    expect(takePendingInviteToken()).toBe(TOKEN);
    expect(window.location.search).toBe("?tab=events");
    expect(window.location.hash).toBe("#latest");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });

  it.each([
    `invite=${TOKEN}&invite=${TOKEN}`,
    `invite=${TOKEN.toUpperCase()}`,
    "invite=not-hex",
  ])("rejects %s without falling back to a stale stored token", (query) => {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, OLD_TOKEN);
    window.history.replaceState(null, "", `/services?${query}`);

    expect(takePendingInviteToken()).toBeNull();
    expect(window.location.href).not.toContain("invite=");
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });

  it("consumes an exact stored token only once", () => {
    window.sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);

    expect(takePendingInviteToken()).toBe(TOKEN);
    expect(takePendingInviteToken()).toBeNull();
  });

  it("retains only exact tokens for an explicit retry", () => {
    expect(retainPendingInviteToken(TOKEN)).toBe(true);
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
    expect(retainPendingInviteToken(`${TOKEN}0`)).toBe(false);
    expect(window.sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
  });
});
