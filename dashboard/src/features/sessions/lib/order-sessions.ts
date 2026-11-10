import type { SessionView } from "@/features/sessions/types";

/**
 * The current session first, then the others by sign-in time, newest first
 * (Kratos lists them in no useful order). A session with no readable sign-in
 * time sorts last. Does not mutate the input (w21/032).
 */
export function orderSessions(sessions: SessionView[]): SessionView[] {
  const signedInAt = (s: SessionView) => {
    const ms = s.authenticatedAt ? Date.parse(s.authenticatedAt) : NaN;
    return Number.isNaN(ms) ? -Infinity : ms;
  };
  return [...sessions].sort((a, b) => {
    if (a.current !== b.current) return a.current ? -1 : 1;
    return signedInAt(b) - signedInAt(a);
  });
}
