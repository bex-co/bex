import type { Session } from "@ory/client-fetch";
import {
  pageLocation,
  pageReferrer,
  telemetryLabel,
  trackEvent,
  trackPageView,
  type TelemetryEvent,
} from "@/common/lib/telemetry";

/** The deepest match's route template (`/services/$serviceId/logs`), never the
 * path the visitor is on. An index route's trailing slash is dropped. */
export function pageTemplate(
  matches: ReadonlyArray<{ fullPath?: unknown }>,
): string {
  const fullPath = matches.at(-1)?.fullPath;
  if (typeof fullPath !== "string" || fullPath.length <= 1) return "/";
  return fullPath.replace(/\/+$/, "");
}

/** Structural, not `AnyRouter`: with this module typed against the router's
 * own types, tsc stopped typing the `useRouterState` selects in
 * route-skeletons.tsx (TS2367, TS2352). */
interface RenderedRouter {
  subscribe(
    eventType: "onRendered",
    fn: (event: { toLocation: { pathname: string } }) => void,
  ): unknown;
  state: {
    matches: ReadonlyArray<{ fullPath?: unknown; context?: unknown }>;
  };
}

// A session first seen longer than this after it authenticated was signed in
// before this browser reported it (a returning tab, or the rollout of this
// code): not a login to count.
const FRESH_AUTH_MS = 10 * 60_000;
// Registration mints the identity and its first session together.
const SIGN_UP_WINDOW_MS = 5 * 60_000;

function time(value: unknown): number {
  if (value instanceof Date) return value.getTime();
  return typeof value === "string" ? Date.parse(value) : NaN;
}

/** Which auth event a newly seen Kratos session is: `sign_up` when the identity
 * was created with it, `login` otherwise, nothing when it is not fresh. */
export function sessionEvent(
  session: Session,
  now: number,
): { event: TelemetryEvent; method: string | undefined } | null {
  const authenticatedAt = time(session.authenticated_at);
  if (Number.isNaN(authenticatedAt)) return null;
  if (now - authenticatedAt >= FRESH_AUTH_MS) return null;
  const first = session.authentication_methods?.[0];
  const createdAt = time(session.identity?.created_at);
  const signedUp =
    !Number.isNaN(createdAt) && authenticatedAt - createdAt < SIGN_UP_WINDOW_MS;
  return {
    event: signedUp ? "sign_up" : "login",
    // GitHub or Google rather than "oidc".
    method: telemetryLabel(first?.provider) ?? telemetryLabel(first?.method),
  };
}

// The last session this browser reported, so a session counts once across
// tabs and reloads. The id is not the session token.
export const SESSION_KEY = "bex.telemetry.session";
// Set when Stripe Checkout opens, so its return is counted once and a reload
// of the return URL is not.
export const CHECKOUT_KEY = "bex.telemetry.checkout";

type StorageName = "localStorage" | "sessionStorage";

// `window[name]` is read inside the try: with storage blocked the getter
// itself throws.
function read(name: StorageName, key: string): string | null {
  try {
    return window[name].getItem(key);
  } catch {
    return null;
  }
}

function write(name: StorageName, key: string, value: string | null) {
  try {
    if (value === null) window[name].removeItem(key);
    else window[name].setItem(key, value);
  } catch {
    // Without storage an event may repeat; nothing else depends on it.
  }
}

export function rememberCheckout(): void {
  write("sessionStorage", CHECKOUT_KEY, "1");
}

function reportCheckoutReturn(search: string): void {
  const billing = new URLSearchParams(search).get("billing");
  if (billing !== "success" && billing !== "cancelled") return;
  if (!read("sessionStorage", CHECKOUT_KEY)) return;
  write("sessionStorage", CHECKOUT_KEY, null);
  trackEvent(billing === "success" ? "add_payment_info" : "checkout_cancelled");
}

function reportSession(session: Session | null | undefined): void {
  if (!session?.id) return;
  if (read("localStorage", SESSION_KEY) === session.id) return;
  write("localStorage", SESSION_KEY, session.id);
  const auth = sessionEvent(session, Date.now());
  if (auth) trackEvent(auth.event, { method: auth.method });
}

/**
 * Report page views and the events only a navigation can see. This is the only
 * source of page views: the tag's automatic one is off and so is its
 * history-based detection (dashboard/AGENTS.md § Analytics). `onRendered` also
 * fires for the hydrated document, which `onResolved` does not.
 */
export function installRouterTelemetry(router: RenderedRouter): void {
  if (typeof window === "undefined" || !window.gtag) return;
  let lastPath: string | undefined;
  let lastPageLocation: string | undefined;
  router.subscribe("onRendered", ({ toLocation }) => {
    const matches = router.state.matches;
    if (toLocation.pathname !== lastPath) {
      lastPath = toLocation.pathname;
      const { origin, search } = window.location;
      const template = pageTemplate(matches);
      const location = pageLocation(origin, template, search);
      trackPageView({
        location,
        template,
        referrer: lastPageLocation ?? pageReferrer(document.referrer, origin),
      });
      lastPageLocation = location;
      reportCheckoutReturn(search);
    }
    const context = matches[0]?.context as
      { session?: Session | null } | undefined;
    reportSession(context?.session);
  });
}
