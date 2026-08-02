import { beforeEach, describe, expect, it } from "vitest";
import {
  INVITE_TOKEN_STORAGE_KEY,
  stashInviteTokenFromURL,
  validInviteToken,
} from "@/common/lib/invite-token";

const TOKEN = "0123456789abcdef0123456789abcdef";

describe("invite token capture", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    window.history.replaceState(null, "", "/");
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
});
