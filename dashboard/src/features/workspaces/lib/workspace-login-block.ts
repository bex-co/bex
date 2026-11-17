import type { WorkspaceLoginRequirement } from "@/common/apollo/auth-error-link";

/**
 * Which workspaces bex-api has refused the caller with a login requirement's
 * coded 403, and which requirement: `WORKSPACE_2FA_REQUIRED` (w22/m402,
 * docs/ADR012-auth.md 9a) or `WORKSPACE_GOOGLE_LOGIN_REQUIRED` (w7/m465, 9b).
 * The Apollo auth error link reports every such refusal here, once per
 * workspace however many queries failed, and `WorkspaceLoginGate` shows ONE
 * blocked page for the selected workspace instead of a toast or error card
 * per query.
 *
 * Client memory only: it is a hint about what the server last said, never the
 * decision. A block is dropped when the user may have fixed it (leaving the
 * account pages where they enroll, pressing "check again", or the full-page
 * round trip of signing in with Google), and the next workspace query
 * re-reports it if the server still refuses. The latest report for a
 * workspace wins: once the member meets one requirement, the server names the
 * other. The state is written only from the browser's Apollo client, so SSR
 * renders see it empty.
 */

type Listener = () => void;

const EMPTY: ReadonlyMap<string, WorkspaceLoginRequirement> = new Map();

let blocked: ReadonlyMap<string, WorkspaceLoginRequirement> = EMPTY;
const listeners = new Set<Listener>();

function publish(next: ReadonlyMap<string, WorkspaceLoginRequirement>): void {
  blocked = next;
  for (const listener of listeners) listener();
}

/** Record that `workspaceId` refused the caller for want of `requirement`. */
export function reportWorkspaceLoginRequired(
  workspaceId: string,
  requirement: WorkspaceLoginRequirement,
): void {
  if (blocked.get(workspaceId) === requirement) return;
  publish(new Map(blocked).set(workspaceId, requirement));
}

/**
 * Forget every recorded block. Enrollment and sign-in are per account, not per
 * workspace, so one account-level change can lift all of them; the next query
 * re-reports whichever the server still refuses.
 */
export function clearWorkspaceLoginBlocks(): void {
  if (blocked.size === 0) return;
  publish(EMPTY);
}

export function subscribeWorkspaceLoginBlocks(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The requirement `workspaceId` is currently recorded as refusing, if any. */
export function workspaceLoginBlock(
  workspaceId: string,
): WorkspaceLoginRequirement | null {
  return blocked.get(workspaceId) ?? null;
}

/**
 * Routes a blocked member must still reach, so the page that says "enroll" can
 * lead somewhere and the block cannot loop: account settings (Kratos's
 * enrollment and connected-accounts flows, ADR012 9) and creating another
 * workspace. Signing in with Google needs no route here: it leaves the app for
 * Kratos and Google and comes back with a fresh page.
 */
const EXEMPT_PATHS = ["/settings", "/new/workspace"];

export function isLoginRequirementExemptPath(pathname: string): boolean {
  return EXEMPT_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`),
  );
}
