import {
  getCookie,
  setCookie,
  removeCookie,
} from "@/common/hooks/use-cookie-storage-state/cookie";

export const WORKSPACE_SELECTION_KEY = "bex.selectedWorkspaceId";

export function getPersistedWorkspaceId(): string | null {
  const value = getCookie(WORKSPACE_SELECTION_KEY)?.trim();
  return value ? value : null;
}

export function persistWorkspaceId(id: string): void {
  setCookie(WORKSPACE_SELECTION_KEY, id, {
    expires: 365,
    sameSite: "lax",
    path: "/",
  });
}

/** Clears the cookie after confirmed last-workspace removal (w6/m144). */
export function clearPersistedWorkspaceId(): void {
  removeCookie(WORKSPACE_SELECTION_KEY, { path: "/" });
}
