import { useRootContext } from "@/common/hooks/use-root-context";
import { hasVerifiedSession } from "@/features/onboarding/lib/email-verification";

/**
 * Whether bex-api serves this session (`hasVerifiedSession`). An unverified
 * session is non-null, but bex-api refuses its every request with
 * EMAIL_VERIFICATION_REQUIRED, so workspace data is not fetched or polled for
 * it (w6/079). `useIsAuthenticated` stays plain "has a session" for the auth
 * wrapper.
 */
export function useHasVerifiedSession(): boolean {
  const { session } = useRootContext();
  return hasVerifiedSession(session);
}
