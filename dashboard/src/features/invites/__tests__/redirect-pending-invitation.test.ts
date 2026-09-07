import { beforeEach, expect, it } from "vitest";
import { INVITE_TOKEN_STORAGE_KEY } from "@/common/lib/invite-token";
import { takeInviteReturn } from "../invite-return";
import { pendingInvitationDestination } from "../redirect-pending-invitation";
const TOKEN = "0123456789abcdef0123456789abcdef";
const entry = { authenticated: true, eligible: true, preload: false };
beforeEach(() => {
  sessionStorage.clear();
  window.history.replaceState(null, "", "/workspace/settings");
});
it("resolves pending intent without consuming the bearer and remembers the prior page", () => {
  sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
  expect(pendingInvitationDestination(entry)).toBe("/invite");
  expect(sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBe(TOKEN);
  expect(takeInviteReturn()).toBe("/workspace/settings");
});
it("does not interrupt ordinary dashboard loading", () => {
  expect(pendingInvitationDestination(entry)).toBeUndefined();
});
it.each([
  { ...entry, authenticated: false },
  { ...entry, eligible: false },
  { ...entry, preload: true },
])("leaves authentication, review and preloading alone: %j", (context) => {
  window.history.replaceState(null, "", `/?invite=${TOKEN}`);
  expect(pendingInvitationDestination(context)).toBeUndefined();
  expect(window.location.search).toContain(TOKEN);
});
it("scrubs a legacy URL before redirecting without storing its bearer in the return path", () => {
  window.history.replaceState(null, "", `/workspace/settings?invite=${TOKEN}`);
  expect(pendingInvitationDestination(entry)).toBe("/invite");
  expect(window.location.search).toBe("");
  expect(takeInviteReturn()).toBe("/workspace/settings");
});
it("does not restore a stale invite after a malformed replacement", () => {
  sessionStorage.setItem(INVITE_TOKEN_STORAGE_KEY, TOKEN);
  window.history.replaceState(null, "", "/?invite=invalid");
  expect(pendingInvitationDestination(entry)).toBeUndefined();
  expect(sessionStorage.getItem(INVITE_TOKEN_STORAGE_KEY)).toBeNull();
});
