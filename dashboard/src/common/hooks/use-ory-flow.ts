import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import type {
  LoginFlow,
  RegistrationFlow,
  RecoveryFlow,
  VerificationFlow,
  SettingsFlow,
  FrontendApi,
} from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { oryErrorInfo } from "@/common/lib/ory/errors";
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
import { safeNext } from "@/common/lib/safe-next";
import { EMPTY_LOGIN_SEARCH } from "@/common/lib/auth/auth";

type OryFlowMap = {
  login: LoginFlow;
  registration: RegistrationFlow;
  recovery: RecoveryFlow;
  verification: VerificationFlow;
  settings: SettingsFlow;
};

/** Kratos UI-node groups that represent a second factor (docs/ADR012-auth.md § MFA). */
const SECOND_FACTOR_GROUPS = new Set(["totp", "webauthn", "lookup_secret"]);

/**
 * A flow Kratos minted for aal2 that has nothing to challenge with: an identity
 * with no second factor yields a "complete the second authentication challenge"
 * flow whose only node is the CSRF token. That card would be empty, so the caller
 * navigates on rather than render it. (Only a hand-typed `?aal=aal2` reaches this
 * — the auth guard asks for a step-up solely when whoami says one is owed.)
 *
 * Keyed on the flow's own `requested_aal`, not on the `aal` we asked for: a
 * step-up with no session behind it falls back to a first-factor flow, and that
 * flow must not be judged by this rule.
 */
function isEmptyStepUp(flow: LoginFlow): boolean {
  return (
    flow.requested_aal === "aal2" &&
    !(flow.ui?.nodes ?? []).some((node) => SECOND_FACTOR_GROUPS.has(node.group))
  );
}

/**
 * Per-tab persistence of the active flow id, so a mid-form reload (or the
 * two-step recovery flow's email → code hop) resumes the same flow instead
 * of starting over. sessionStorage (not localStorage): flows are per-tab,
 * short-lived state — sharing them across tabs would cross CSRF contexts.
 */
const storageKey = (kind: keyof OryFlowMap) => `bex.ory-flow.${kind}`;

function readStoredFlowId(kind: keyof OryFlowMap): string | null {
  try {
    return window.sessionStorage.getItem(storageKey(kind));
  } catch {
    return null; // storage disabled (private mode etc.) — degrade to in-memory only
  }
}

function writeStoredFlowId(kind: keyof OryFlowMap, id: string | null) {
  try {
    if (id === null) window.sessionStorage.removeItem(storageKey(kind));
    else window.sessionStorage.setItem(storageKey(kind), id);
  } catch {
    // storage disabled — flow still lives in React state for this mount
  }
}

/** Call from a flow's onSuccess so the next visit doesn't retry a used-up flow id. */
export function clearStoredOryFlow(kind: keyof OryFlowMap) {
  writeStoredFlowId(kind, null);
}

function getFlow<K extends keyof OryFlowMap>(
  api: FrontendApi,
  kind: K,
  id: string,
): Promise<OryFlowMap[K]> {
  const req =
    kind === "login"
      ? api.getLoginFlow({ id })
      : kind === "registration"
        ? api.getRegistrationFlow({ id })
        : kind === "recovery"
          ? api.getRecoveryFlow({ id })
          : kind === "verification"
            ? api.getVerificationFlow({ id })
            : api.getSettingsFlow({ id });
  return req as Promise<OryFlowMap[K]>;
}

/**
 * How a flow can be asked for. `loginChallenge` (login/registration) links it to
 * a Hydra OAuth2 authorization request, which Kratos's native `oauth2_provider`
 * integration then accepts itself (docs/ADR012-auth.md, w4/m9); `aal: "aal2"`
 * (login) asks for the second-factor challenge against the live session instead
 * of a password form (§ MFA, w4/m11) — the caller passes it because the session
 * fetch said a factor is owed, not on a hunch (w4/m17).
 *
 * A named bag, not positional args: the retry cascade below re-mints a flow with
 * whichever one of these Kratos objected to dropped, and `{ ...ask, aal: undefined }`
 * says which far better than counting `undefined`s into an argument list.
 */
type FlowAsk = { returnTo?: string; loginChallenge?: string; aal?: string };

/**
 * Creates a fresh flow via AJAX. Kratos's `/self-service/{kind}/browser`
 * endpoints return the flow as JSON (no redirect) when called via fetch —
 * the same call opened as a link would 303 back to us with `?flow=` in the
 * URL, which is exactly what this hook exists to avoid. Anti-CSRF cookies
 * still get set: auth and dashboard share a site (bex.co / localhost), so
 * the browser sends them on Ory Elements' subsequent form-submit fetches.
 *
 * Resolves to `null` when Kratos has no flow to give (see `bootstrapViaKratos`).
 */
function createFlow<K extends keyof OryFlowMap>(
  api: FrontendApi,
  kind: K,
  { returnTo, loginChallenge, aal }: FlowAsk,
): Promise<OryFlowMap[K] | null> {
  const req =
    kind === "login"
      ? api.createBrowserLoginFlow({ returnTo, loginChallenge, aal })
      : kind === "registration"
        ? api.createBrowserRegistrationFlow({ returnTo, loginChallenge })
        : kind === "recovery"
          ? api.createBrowserRecoveryFlow({ returnTo })
          : kind === "verification"
            ? api.createBrowserVerificationFlow({ returnTo })
            : api.createBrowserSettingsFlow({ returnTo });
  return req as Promise<OryFlowMap[K] | null>;
}

/**
 * Asks Kratos for the same flow again, but as a plain browser navigation — no
 * `Accept: application/json` — so it may answer with a redirect instead of a flow
 * body. Two things arrive here:
 *
 * - **The OAuth2 short-circuit (w4/m17).** A browser that already holds a session
 *   Kratos can accept the login challenge against gets no flow at all over AJAX:
 *   Kratos v1.3.1 answers `200` with a body of literally `null` — nothing to
 *   render, and an AJAX call cannot be redirected. Asked as a browser, the very
 *   same URL 303s straight to Hydra's continue URL and the authorization goes
 *   through with no login UI. Rendering the `null` instead is what used to strand
 *   an already-signed-in user on the login page's skeleton, forever.
 * - **The escape hatch.** Any AJAX bootstrap we couldn't make sense of. Kratos
 *   303s back here with `?flow=` appended, which the hook adopts and scrubs.
 *
 * Every param the AJAX call carried rides along: dropping the challenge here
 * would silently abandon the authorization the user is in the middle of.
 */
function bootstrapViaKratos(
  kind: keyof OryFlowMap,
  { returnTo, loginChallenge, aal }: FlowAsk,
) {
  const params = new URLSearchParams(returnTo ? { return_to: returnTo } : {});
  if (loginChallenge) params.set("login_challenge", loginChallenge);
  if (aal) params.set("aal", aal);
  window.location.href = `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser?${params}`;
}

/**
 * Provides (client-side) the Ory Kratos self-service flow this page renders,
 * keeping the flow reference OUT of the address bar:
 *
 * - No known flow → mint one via AJAX (`createBrowser*Flow`), hold it in
 *   React state + sessionStorage. The URL never changes.
 * - `?flow=` present in the URL (arrivals we don't control: recovery /
 *   verification email links, OIDC round trips, Ory Elements' own
 *   expired-mid-submit restart, legacy bookmarks) → persist the id to
 *   sessionStorage, then strip the param with a replace-navigation; the
 *   effect re-runs and loads the flow from storage. One-shot, leaves a
 *   clean URL and an intact back-button.
 * - Stored/inbound id turns out expired, used, or foreign → silently fall
 *   through to minting a fresh flow. A days-old bookmark just works.
 * - Kratos declines to hand back a flow at all — because the browser's existing
 *   session already answers the question being asked (an OAuth2 login challenge
 *   it can accept outright) → `bootstrapViaKratos` re-asks as a browser, and
 *   Kratos redirects the flow onward instead of describing it.
 *
 * `returnTo` must NOT be this page's own URL: Kratos short-circuits flow
 * creation when a valid session exists (`session_already_available`) and
 * we navigate to `returnTo` — pointing it back at an auth page would loop.
 */
export interface UseOryFlowOptions {
  /** Where to land after the flow succeeds (must not be an auth page). */
  returnTo?: string;
  /** Hydra OAuth2 challenge linking this flow to an authorization request (w4/m9). */
  loginChallenge?: string;
  /**
   * Assurance level to request (login only). `"aal2"` renders the second-factor
   * challenge against the live session: the caller passes it because the session
   * fetch reported `session_aal2_required` — this hook does not guess (w4/m17).
   */
  aal?: "aal2";
}

export function useOryFlow<K extends keyof OryFlowMap>(
  kind: K,
  flowIdFromUrl: string | undefined,
  { returnTo = "/", loginChallenge, aal }: UseOryFlowOptions = {},
): OryFlowMap[K] | null {
  const [flow, setFlow] = useState<OryFlowMap[K] | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;

    // Inbound ?flow= — adopt it, then scrub the URL. The search-param change
    // re-runs this effect, which then loads the id from sessionStorage.
    if (flowIdFromUrl) {
      writeStoredFlowId(kind, flowIdFromUrl);
      void navigate({
        to: ".",
        search: (prev: Record<string, unknown>) => ({
          ...prev,
          flow: undefined,
        }),
        replace: true,
      });
      return;
    }

    const api = createFrontendApi();
    // `returnTo` reaches here straight from the URL (`?next=`), so normalize it
    // to a same-origin relative path before it becomes either the Kratos
    // return_to or a navigate href — an external value is an open redirect
    // (safe-next.ts). The absolute form handed to Kratos is thus always
    // same-origin too.
    const safeReturnTo = safeNext(returnTo, window.location.origin);
    const ask: FlowAsk = {
      returnTo: new URL(safeReturnTo, window.location.origin).toString(),
      loginChallenge,
      aal,
    };

    // Already signed in with nothing more to prove — go where the user was
    // headed. It goes in `href`, not `to`: returnTo is an arbitrary href and may
    // carry a query string (`/auth/consent?consent_challenge=…`), which `to`
    // would swallow into the pathname.
    const goToReturnTo = () =>
      void navigate({ to: "/", href: safeReturnTo, replace: true });
    const handBackToKratos = () => bootstrapViaKratos(kind, ask);

    async function load() {
      // Both a `login_challenge` (an OAuth2 authorization in progress,
      // docs/ADR012-auth.md, w4/m9) and an `aal` step-up bind the flow to *this*
      // visit — to Hydra's request, or to the live session. Neither may resume a
      // stored flow, and neither is stored: a later ordinary visit must start
      // its own first-factor flow.
      const bound = Boolean(loginChallenge || aal);
      const knownId = bound ? null : readStoredFlowId(kind);
      if (knownId) {
        try {
          const existing = await getFlow(api, kind, knownId);
          if (!cancelled) setFlow(existing);
          return;
        } catch {
          writeStoredFlowId(kind, null); // expired/used/foreign — mint fresh
        }
      }

      try {
        let fresh: OryFlowMap[K] | null;
        try {
          fresh = await createFlow(api, kind, ask);
        } catch (err) {
          const { id, redirectBrowserTo } = await oryErrorInfo(err);
          if (cancelled) return;
          if (id === "browser_location_change_required" && redirectBrowserTo) {
            // Kratos wants the browser elsewhere. Ory Elements' own submits are
            // what usually provoke this; flow *creation* no longer does (v1.3.1
            // answers the OAuth2 short-circuit with `200 null` instead — see
            // bootstrapViaKratos). Honored anyway: if Kratos names a destination,
            // going there is always right, and older versions do name one here.
            window.location.href = redirectBrowserTo;
            return;
          }
          if (id === "session_already_available") {
            // A live session refuses a login flow. With an authorization riding
            // on this visit, hand the challenge back to Kratos — it accepts it
            // against that very session when asked as a browser (navigating to
            // returnTo would abandon the authorization). Otherwise the user is
            // simply signed in with nothing left to prove: any second factor owed
            // would already have been asked for by `aal`, because the session
            // fetch knows — this hook no longer probes for it (w4/m17).
            if (loginChallenge) handBackToKratos();
            else goToReturnTo();
            return;
          }
          if (id === "session_aal1_required") {
            // An `aal=aal2` step-up with no session behind it — a stale link, or
            // a session that expired between the guard's whoami and this call.
            // Nothing to step up: start an ordinary login.
            fresh = await createFlow(api, kind, { ...ask, aal: undefined });
          } else if (id === "self_service_flow_return_to_forbidden") {
            // Kratos rejects return_to values missing from its
            // allowed_return_urls (e.g. localhost dev against an environment
            // that doesn't allowlist it) — the flow itself still works
            // without one, so retry rather than fail the page.
            fresh = await createFlow(api, kind, {
              ...ask,
              returnTo: undefined,
            });
          } else if (loginChallenge) {
            // Stale/invalid challenge (single-use, short TTL) — the challenge
            // is advisory, never load-bearing: degrade to the ordinary page.
            fresh = await createFlow(api, kind, {
              ...ask,
              loginChallenge: undefined,
            });
          } else {
            throw err;
          }
        }
        if (cancelled) return;
        if (!fresh) {
          handBackToKratos(); // `200 null` — see bootstrapViaKratos (w4/m17)
          return;
        }
        if (isEmptyStepUp(fresh as LoginFlow)) {
          goToReturnTo();
          return;
        }
        if (!bound) writeStoredFlowId(kind, fresh.id);
        setFlow(fresh);
      } catch (err) {
        // A *retry* failed (the first attempt's failures are all handled inline).
        if (cancelled) return;
        const { id } = await oryErrorInfo(err);
        if (id === "session_already_available") {
          // The stale-challenge retry, dropped back to an ordinary login flow,
          // met the live session this browser holds: signed in, nothing to prove.
          goToReturnTo();
          return;
        }
        if (id === "session_inactive") {
          // Settings without a session — sign in first.
          void navigate({
            to: "/auth/login",
            search: EMPTY_LOGIN_SEARCH,
            replace: true,
          });
          return;
        }
        // Unknown failure — fall back to Kratos's redirect-based bootstrap
        // (readds ?flow= for this one attempt, but keeps the page usable).
        handBackToKratos();
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [kind, flowIdFromUrl, returnTo, navigate, loginChallenge, aal]);

  return flow;
}
