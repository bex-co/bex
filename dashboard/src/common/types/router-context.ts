import type { getClient } from "@/common/apollo/client";
import type { Session } from "@ory/client-fetch";

export type RouterContext = {
  client: ReturnType<typeof getClient>;
  session?: Session | null;
  /** The root's session fetch found a live aal1 session owing a second factor
   * (`session_aal2_required`) — `requireAuth` sends those users to the login
   * page's step-up rather than to a sign-in form (docs/ADR012-auth.md § MFA). */
  aal2Required?: boolean;
};
