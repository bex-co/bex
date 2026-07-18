import {
  getCookie,
  setCookie,
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
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(WORKSPACE_SELECTION_KEY, id);
  }
}
