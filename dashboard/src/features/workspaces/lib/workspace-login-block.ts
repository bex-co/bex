import type {
  LoginRefusal,
  LoginRefusalOrganization,
} from "@/common/apollo/auth-error-link";

/**
 * What bex-api has refused the caller for want of a login requirement's coded
 * 403: `WORKSPACE_2FA_REQUIRED` (w22/m402, docs/ADR012-auth.md 9a) or
 * `WORKSPACE_GOOGLE_LOGIN_REQUIRED` (w7/m465, 9b), per workspace and per
 * organization. The Apollo auth error link reports every such refusal here,
 * once however many queries failed, and `WorkspaceLoginGate` (or an org page's
 * shell) shows ONE blocked page instead of a toast or error card per query.
 *
 * A refusal an org's own setting decided names the org (w7/m466, 9c). It is
 * recorded against its workspace, when it has one, with the org kept so the
 * blocked page can say which org requires it; and against the org, since the
 * same setting holds the org's own pages. A refusal on the org's own objects
 * (its members, invites, settings) names no workspace, so it is recorded
 * against the org alone.
 *
 * Client memory only: it is a hint about what the server last said, never the
 * decision. A block is dropped when the user may have fixed it (leaving the
 * account pages where they enroll, pressing "check again", or the full-page
 * round trip of signing in with Google), and the next query re-reports it if
 * the server still refuses. The latest report wins: once the member meets one
 * requirement, the server names the other. The state is written only from the
 * browser's Apollo client, so SSR renders see it empty.
 */

/** One recorded refusal: what is required, and by which org if not the
 *  workspace's own setting. */
export type LoginBlock = Omit<LoginRefusal, "workspaceId">;

/** A block recorded against an org: always the org's own setting's. */
export type OrganizationLoginBlock = LoginBlock & {
  organization: LoginRefusalOrganization;
};

type Listener = () => void;

const EMPTY: ReadonlyMap<string, LoginBlock> = new Map();

/** Keyed `workspace:<id>` or `org:<id>`, so the two id spaces never meet. */
let blocked: ReadonlyMap<string, LoginBlock> = EMPTY;
const listeners = new Set<Listener>();

function publish(next: ReadonlyMap<string, LoginBlock>): void {
  blocked = next;
  for (const listener of listeners) listener();
}

const workspaceKey = (id: string) => `workspace:${id}`;
const organizationKey = (id: string) => `org:${id}`;

function sameBlock(a: LoginBlock | undefined, b: LoginBlock): boolean {
  return (
    a !== undefined &&
    a.requirement === b.requirement &&
    a.organization?.id === b.organization?.id &&
    a.organization?.name === b.organization?.name
  );
}

/** Record one refusal against its workspace and, when the org's setting
 *  decided it, against the org. */
export function reportLoginRefusal({
  workspaceId,
  ...block
}: LoginRefusal): void {
  const entries: [string, LoginBlock][] = [];
  if (workspaceId) entries.push([workspaceKey(workspaceId), block]);
  if (block.organization) {
    entries.push([organizationKey(block.organization.id), block]);
  }
  const changed = entries.filter(
    ([key]) => !sameBlock(blocked.get(key), block),
  );
  if (changed.length === 0) return;
  const next = new Map(blocked);
  for (const [key, value] of changed) next.set(key, value);
  publish(next);
}

/**
 * Forget every recorded block. Enrollment and sign-in are per account, not per
 * workspace or org, so one account-level change can lift all of them; the next
 * query re-reports whichever the server still refuses.
 */
export function clearLoginBlocks(): void {
  if (blocked.size === 0) return;
  publish(EMPTY);
}

export function subscribeLoginBlocks(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** What `workspaceId` is currently recorded as refusing, if anything. */
export function workspaceLoginBlock(workspaceId: string): LoginBlock | null {
  return blocked.get(workspaceKey(workspaceId)) ?? null;
}

/** What the org's own setting is currently recorded as refusing, if
 *  anything. Only a refusal naming the org is recorded under its key
 *  (`reportLoginRefusal`), so its `organization` is always set. */
export function organizationLoginBlock(
  organizationId: string,
): OrganizationLoginBlock | null {
  const block = blocked.get(organizationKey(organizationId));
  return (block as OrganizationLoginBlock | undefined) ?? null;
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
