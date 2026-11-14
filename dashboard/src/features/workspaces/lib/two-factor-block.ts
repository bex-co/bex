/**
 * Which workspaces bex-api has refused the caller with the coded 403
 * `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a). The Apollo auth
 * error link reports every such refusal here, once per workspace however many
 * queries failed, and `WorkspaceTwoFactorGate` shows ONE blocked page for the
 * selected workspace instead of a toast or error card per query.
 *
 * Client memory only: it is a hint about what the server last said, never the
 * decision. A block is dropped when the user may have fixed it (leaving the
 * account pages where they enroll, or pressing "check again"), and the next
 * workspace query re-reports it if the server still refuses. The state is
 * written only from the browser's Apollo client, so SSR renders see it empty.
 */

type Listener = () => void;

const EMPTY: ReadonlySet<string> = new Set();

let blocked: ReadonlySet<string> = EMPTY;
const listeners = new Set<Listener>();

function publish(next: ReadonlySet<string>): void {
  blocked = next;
  for (const listener of listeners) listener();
}

/** Record that `workspaceId` refused the caller for want of a second factor. */
export function reportWorkspaceTwoFactorRequired(workspaceId: string): void {
  if (blocked.has(workspaceId)) return;
  publish(new Set(blocked).add(workspaceId));
}

/**
 * Forget every recorded block. Enrollment is per account, not per workspace,
 * so one account-level change can lift all of them; the next query re-reports
 * whichever the server still refuses.
 */
export function clearWorkspaceTwoFactorBlocks(): void {
  if (blocked.size === 0) return;
  publish(EMPTY);
}

export function subscribeWorkspaceTwoFactorBlocks(
  listener: Listener,
): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Whether `workspaceId` is currently recorded as refused. */
export function isWorkspaceTwoFactorBlocked(workspaceId: string): boolean {
  return blocked.has(workspaceId);
}

/**
 * Routes a blocked member must still reach, so the page that says "enroll" can
 * lead somewhere and the block cannot loop: account settings (Kratos's
 * enrollment flow, ADR012 9) and creating another workspace.
 */
const EXEMPT_PATHS = ["/settings", "/new/workspace"];

export function isTwoFactorExemptPath(pathname: string): boolean {
  return EXEMPT_PATHS.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`),
  );
}
