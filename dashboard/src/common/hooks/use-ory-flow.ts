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
import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
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
 * True when an aal2 step-up login flow actually presents a second-factor
 * challenge. Guards the step-up path: a session that is already aal2 (or an
 * identity with no second factor) yields a flow with no such node, and we must
 * not render an empty challenge card — the caller falls back to navigating on.
 */
function offersSecondFactor(flow: LoginFlow): boolean {
  return (flow.ui?.nodes ?? []).some((node) =>
    SECOND_FACTOR_GROUPS.has(node.group),
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
 * Creates a fresh flow via AJAX. Kratos's `/self-service/{kind}/browser`
 * endpoints return the flow as JSON (no redirect) when called via fetch —
 * the same call opened as a link would 303 back to us with `?flow=` in the
 * URL, which is exactly what this hook exists to avoid. Anti-CSRF cookies
 * still get set: auth and dashboard share a site (bex.co / localhost), so
 * the browser sends them on Ory Elements' subsequent form-submit fetches.
 *
 * `loginChallenge` (login/registration only) links the flow to a Hydra OAuth2
 * authorization request — Kratos's native `oauth2_provider` integration then
 * accepts the challenge itself on success (docs/ADR012-auth.md, w4/m9).
 *
 * `aal` (login only) requests a higher assurance level: `"aal2"` makes Kratos
 * return the second-factor step against the existing session (docs/ADR012-auth.md
 * § MFA, w4/m11) instead of a first-factor password form.
 */
function createFlow<K extends keyof OryFlowMap>(
  api: FrontendApi,
  kind: K,
  returnTo: string | undefined,
  loginChallenge?: string,
  aal?: string,
): Promise<OryFlowMap[K]> {
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
  return req as Promise<OryFlowMap[K]>;
}

/**
 * Extracts Kratos's machine-readable error id (e.g. `session_already_available`)
 * and, for `browser_location_change_required`, where to send the browser (how
 * Kratos answers an AJAX call whose flow must continue elsewhere — e.g. an
 * OAuth2 login challenge short-circuited by an existing session).
 */
async function oryErrorInfo(
  err: unknown,
): Promise<{ id?: string; redirectBrowserTo?: string }> {
  const response = (err as { response?: Response })?.response;
  if (!response) return {};
  try {
    const body = (await response.clone().json()) as {
      error?: { id?: string };
      redirect_browser_to?: string;
    };
    return { id: body.error?.id, redirectBrowserTo: body.redirect_browser_to };
  } catch {
    return {};
  }
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
}

export function useOryFlow<K extends keyof OryFlowMap>(
  kind: K,
  flowIdFromUrl: string | undefined,
  { returnTo = "/", loginChallenge }: UseOryFlowOptions = {},
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
    const returnUrl = new URL(returnTo, window.location.origin).toString();

    async function load() {
      // A login_challenge visit is an OAuth2 authorization in progress
      // (docs/ADR012-auth.md, w4/m9): always mint a fresh flow bound to the challenge —
      // a stored flow isn't linked to the Hydra request — and don't persist it
      // (a later ordinary visit must not resume an OAuth-linked flow).
      const knownId = loginChallenge ? null : readStoredFlowId(kind);
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
        let fresh: OryFlowMap[K];
        try {
          fresh = await createFlow(api, kind, returnUrl, loginChallenge);
        } catch (err) {
          const { id, redirectBrowserTo } = await oryErrorInfo(err);
          if (id === "browser_location_change_required" && redirectBrowserTo) {
            // OAuth2 short-circuit: an existing session satisfied the login
            // challenge (Kratos accepted it) — continue the authorize flow.
            window.location.href = redirectBrowserTo;
            return;
          }
          if (id === "self_service_flow_return_to_forbidden") {
            // Kratos rejects return_to values missing from its
            // allowed_return_urls (e.g. localhost dev against an environment
            // that doesn't allowlist it) — the flow itself still works
            // without one, so retry rather than fail the page.
            fresh = await createFlow(api, kind, undefined, loginChallenge);
          } else if (loginChallenge) {
            // Stale/invalid challenge (single-use, short TTL) — the challenge
            // is advisory, never load-bearing: degrade to the ordinary page.
            fresh = await createFlow(api, kind, returnUrl);
          } else {
            throw err;
          }
        }
        if (cancelled) return;
        if (!loginChallenge) writeStoredFlowId(kind, fresh.id);
        setFlow(fresh);
      } catch (err) {
        if (cancelled) return;
        const { id: errorId } = await oryErrorInfo(err);
        if (errorId === "session_already_available") {
          // A session exists, so Kratos refuses a first-factor login flow.
          // Under the `highest_available` AAL policy (docs/ADR012-auth.md § MFA) this
          // is exactly the "authenticated with a password (aal1) but a second
          // factor is still owed" case — the whoami that gates protected pages
          // 403s until aal2, so the auth guard bounced the user here. Mint an
          // aal2 step-up flow: Kratos returns the second-factor challenge
          // against the live session and Ory Elements renders it. If that flow
          // carries no second-factor node (session already aal2, or an
          // identity with no second factor manually visiting /auth/login),
          // there's nothing to challenge — go where the user was headed.
          if (kind === "login") {
            try {
              const stepUp = await createFlow(
                api,
                kind,
                returnUrl,
                loginChallenge,
                "aal2",
              );
              if (cancelled) return;
              if (offersSecondFactor(stepUp as LoginFlow)) {
                // Step-up flows are bound to the live session; never persist —
                // a later fresh visit must start its own first-factor flow.
                setFlow(stepUp);
                return;
              }
            } catch {
              // Kratos rejects the aal2 request (no second factor to satisfy
              // it) — fall through to navigate on.
            }
          }
          // Already signed in with nothing more to prove — go where the user
          // was headed instead.
          void navigate({ to: returnTo, replace: true });
          return;
        }
        if (errorId === "session_inactive") {
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
        window.location.href = `${KRATOS_PUBLIC_URL}/self-service/${kind}/browser?return_to=${encodeURIComponent(returnUrl)}`;
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [kind, flowIdFromUrl, returnTo, navigate, loginChallenge]);

  return flow;
}
