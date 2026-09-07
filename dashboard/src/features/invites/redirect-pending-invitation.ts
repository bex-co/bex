import {
  peekPendingInviteToken,
  stashInviteTokenFromURL,
} from "@/common/lib/invite-token";
import { rememberInviteReturn } from "./invite-return";

/** Run at the authentication boundary before dashboard/billing route loaders.
 * The bearer is tab-scoped, so SSR cannot inspect it. Ordinary SSR keeps its
 * route-shaped shell; initial browser loading checks intent before proceeding.
 * Preloading a link must never capture intent or change the current URL. */
export function pendingInvitationDestination({
  authenticated,
  eligible,
  preload,
}: {
  authenticated: boolean;
  eligible: boolean;
  preload: boolean;
}) {
  if (typeof window === "undefined" || !authenticated || !eligible || preload)
    return;
  stashInviteTokenFromURL();
  if (!peekPendingInviteToken()) return;
  rememberInviteReturn(window.location.pathname + window.location.search);
  return "/invite" as const;
}
