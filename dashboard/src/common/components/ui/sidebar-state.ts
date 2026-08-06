import {
  getCookie,
  setCookie,
} from "@/common/hooks/use-cookie-storage-state/cookie";

/**
 * Persisted sidebar state — collapse state + expanded width in a single cookie,
 * so the server can render the correct first paint with no expand→collapse (or
 * width) flash on hydration. Modeled on Devin's sidebar (app.devin.ai): the
 * cookie stores a *signed width* — a positive integer px means "expanded at
 * this width", a negative one means "collapsed, but remember this width for
 * when re-expanded" (w2/m63).
 *
 * Cookie (not localStorage) is deliberate: only a cookie is readable during
 * SSR, and the state must be known before the first byte is rendered.
 */
export const SIDEBAR_COOKIE_NAME = "sidebar_state";
const SIDEBAR_COOKIE_MAX_AGE_DAYS = 7;

/** Icon-rail width when collapsed (matches the shadcn primitive default). */
export const SIDEBAR_WIDTH_ICON = "3rem";
/** Bounds + default for the resizable expanded width, in CSS px. */
export const SIDEBAR_MIN_WIDTH_PX = 192; // 12rem
export const SIDEBAR_MAX_WIDTH_PX = 384; // 24rem
export const SIDEBAR_DEFAULT_WIDTH_PX = 256; // 16rem — the prior fixed width
/**
 * Drag position (px from the sidebar's leading edge) below which a resize
 * snaps to the collapsed icon rail. Kept comfortably under the min width so a
 * drag *within* the allowed band never accidentally collapses.
 */
export const SIDEBAR_COLLAPSE_AT_PX = 140;

export interface SidebarState {
  open: boolean;
  /** Expanded width in px; retained even while collapsed. */
  width: number;
}

export function clampSidebarWidth(px: number): number {
  return Math.min(
    Math.max(Math.round(px), SIDEBAR_MIN_WIDTH_PX),
    SIDEBAR_MAX_WIDTH_PX,
  );
}

/**
 * Encode state as the signed-width cookie value: `256` (expanded, 256px) or
 * `-256` (collapsed, remembering 256px).
 */
export function encodeSidebarState({ open, width }: SidebarState): string {
  const w = clampSidebarWidth(width);
  return String(open ? w : -w);
}

/**
 * Decode a cookie value into state, or `null` when there is no usable value so
 * the caller can fall back to its own default. Tolerates the legacy boolean
 * cookie (`true`/`false`) that earlier builds wrote.
 */
export function decodeSidebarState(
  raw: string | undefined | null,
): SidebarState | null {
  if (raw == null) return null;
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  if (trimmed === "true")
    return { open: true, width: SIDEBAR_DEFAULT_WIDTH_PX };
  if (trimmed === "false")
    return { open: false, width: SIDEBAR_DEFAULT_WIDTH_PX };

  const n = Number(trimmed);
  if (!Number.isFinite(n) || n === 0) return null;
  return { open: n > 0, width: clampSidebarWidth(Math.abs(n)) };
}

/**
 * Read the persisted state isomorphically (server: request cookie; client:
 * document.cookie). Returns `null` when nothing is stored yet.
 */
export function getPersistedSidebarState(): SidebarState | null {
  return decodeSidebarState(getCookie(SIDEBAR_COOKIE_NAME));
}

/** Persist state (client-only write; the server no-ops the setter). */
export function persistSidebarState(state: SidebarState): void {
  setCookie(SIDEBAR_COOKIE_NAME, encodeSidebarState(state), {
    expires: SIDEBAR_COOKIE_MAX_AGE_DAYS,
    sameSite: "lax",
    path: "/",
  });
}
