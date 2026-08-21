import { KRATOS_PUBLIC_URL } from "@/common/lib/ory/config";
import { safeNext } from "@/common/lib/safe-next";

/**
 * A Kratos self-service link resolved onto the app route that already serves
 * that flow. `search` spells out every key of the target route's
 * `validateSearch` because TanStack's typed navigation requires the full shape
 * (the same reason `EMPTY_LOGIN_SEARCH` exists).
 */
export type KratosLinkTarget =
  | {
      to: "/auth/login";
      search: {
        next: string | undefined;
        flow: undefined;
        login_challenge: string | undefined;
        aal: undefined;
      };
    }
  | {
      to: "/auth/sign-up";
      search: {
        next: string | undefined;
        flow: undefined;
        login_challenge: string | undefined;
      };
    }
  | { to: "/auth/forgot-password"; search: { flow: undefined } }
  | {
      to: "/auth/verification";
      search: { flow: undefined; next: string | undefined };
    };

/** Kratos flow name → the route that renders it (mirrors `oryConfig.project.*_ui_url`). */
const FLOW_ROUTE = {
  login: "/auth/login",
  registration: "/auth/sign-up",
  recovery: "/auth/forgot-password",
  verification: "/auth/verification",
} as const;

type KratosFlow = keyof typeof FLOW_ROUTE;

const SELF_SERVICE =
  /^\/self-service\/(login|registration|recovery|verification)\/browser\/?$/;

/**
 * Reduce a Kratos `return_to` to the app's own `?next=`.
 *
 * `return_to` is always absolute, and `safeNext` rejects absolute values
 * outright (anything not starting with a single "/" collapses to "/"), so the
 * URL must be reduced to `pathname+search+hash` FIRST and only then re-validated
 * through `safeNext`. Handing it the raw value would silently flatten every deep
 * link to "/".
 */
function nextFromReturnTo(
  returnTo: string | null,
  origin: string,
): string | undefined {
  if (!returnTo) return undefined;
  let url: URL;
  try {
    // Parsed WITHOUT a base on purpose. `return_to` is always absolute here
    // (Kratos builds it; so does use-ory-flow), and resolving against a base
    // would launder values that are not: `https:%2f%2fevil.com` throws standalone
    // but base-resolves to a same-origin `/%2f%2fevil.com`, and `//evil.com` /
    // `/\evil.com` base-resolve to evil.com outright. One rule rejects all three.
    url = new URL(returnTo);
  } catch {
    return undefined;
  }
  // Kratos allowlists other origins too (oauth.bex.co) — those are not ours to
  // route, and a client-side navigation could never reach them anyway.
  if (url.origin !== origin) return undefined;
  const relative = safeNext(`${url.pathname}${url.search}${url.hash}`, origin);
  return relative === "/" ? undefined : relative; // "/" is the default — no ?next= noise
}

/**
 * Resolve one of Ory Elements' hardcoded cross-origin self-service links onto
 * the in-app route that already serves the same flow, or `null` when the href
 * is anything else (in which case the caller must leave the click alone).
 *
 * Ory Elements v1.2.0 builds these as `${sdk.url}/self-service/{flow}/browser`
 * (`initFlowUrl`) and ignores `project.*_ui_url`, so following one costs a
 * cross-origin round trip plus a 303 back into a second full page load. Every
 * target route mints its own flow over AJAX (`use-ory-flow.ts`), so navigating
 * straight there is equivalent — minus both document loads.
 *
 * Matched on Kratos's URL shape rather than on Ory's component internals: the
 * endpoint layout is a stable public API, so a library bump can at worst stop
 * matching (falling back to today's full-page behavior), never mis-route.
 */
export function kratosLinkTarget(
  href: string,
  origin: string,
  kratosUrl: string = KRATOS_PUBLIC_URL,
): KratosLinkTarget | null {
  let url: URL;
  let base: URL;
  try {
    url = new URL(href, origin);
    base = new URL(kratosUrl, origin);
  } catch {
    return null;
  }

  if (url.origin !== base.origin) return null;
  // Kratos may be mounted under a path prefix on the app's own origin — the
  // documented local-dev proxy is VITE_KRATOS_PUBLIC_URL=…:5173/kratos. Without
  // the prefix check that setup would match every in-app link.
  const basePath = base.pathname.replace(/\/+$/, "");
  if (basePath && !url.pathname.startsWith(`${basePath}/`)) return null;

  const match = SELF_SERVICE.exec(url.pathname.slice(basePath.length));
  if (!match) return null; // logout, settings, errors, /.well-known/… stay full-page

  const flow = match[1] as KratosFlow;
  const params = url.searchParams;
  const next = nextFromReturnTo(params.get("return_to"), origin);
  // Advisory: binds the flow to a Hydra authorization already in progress.
  // Forwarded only to the two routes that model it; `use-ory-flow` threads it
  // into createBrowser*Flow exactly as the Kratos endpoint would have.
  const loginChallenge = params.get("login_challenge") ?? undefined;
  // `identity_schema` is deliberately dropped: bex runs a single schema
  // (kratos.values.yaml `default_schema_id: default`) and no route models it.
  // A second schema means adding a route param before forwarding it here.

  switch (FLOW_ROUTE[flow]) {
    case "/auth/login":
      return {
        to: "/auth/login",
        search: {
          next,
          // Never carried: the target page mints its own flow and keeps the id
          // out of the URL, which is the whole point of going client-side.
          flow: undefined,
          login_challenge: loginChallenge,
          // Never synthesized: a cross-link is a fresh first-factor intent.
          // Only requireAuth mints `aal`, off a real session_aal2_required.
          aal: undefined,
        },
      };
    case "/auth/sign-up":
      return {
        to: "/auth/sign-up",
        search: { next, flow: undefined, login_challenge: loginChallenge },
      };
    case "/auth/forgot-password":
      // No `next` in this route's search schema yet, so `return_to` is dropped
      // rather than silently lost in a param the route would reject.
      return { to: "/auth/forgot-password", search: { flow: undefined } };
    case "/auth/verification":
      return { to: "/auth/verification", search: { flow: undefined, next } };
  }
}
