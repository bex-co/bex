import { Configuration, OAuth2Api } from "@ory/client-fetch";

// Headless OAuth2 consent acceptor (docs/auth.md, w4/m9). Hydra redirects the
// browser here with a consent_challenge after login (which Kratos's native
// oauth2_provider integration accepted). Hydra never skips this redirect by
// design — the canonical pattern for first-party/trusted clients is to accept
// immediately (<100ms, no UI). We auto-accept when Hydra says the grant is
// remembered/skippable (`skip`, incl. clients with `skip_consent: true`) or the
// client_id is in the operator-blessed allowlist; anything else is denied — a
// human consent UI for true third parties is future work, and silently granting
// would be worse than failing.
//
// SERVER-ONLY: talks to Hydra's admin API via @ory/client-fetch's OAuth2Api
// (the same SDK the browser side uses for Kratos's FrontendApi). Reached
// exclusively from the /auth/consent server route handler (dynamic import
// inside the server block), so none of this — or the env it reads — reaches
// the client bundle. Env is deliberately NOT VITE_-prefixed: `HYDRA_ADMIN_URL`
// (e.g. in-cluster http://hydra-admin.auth.svc:4445) and
// `OAUTH_TRUSTED_CLIENTS` (comma-separated client_ids).

/** How long Hydra remembers an accepted consent, so a returning user's client
 * isn't re-challenged within the window. */
const REMEMBER_FOR_SECONDS = 3600;

function trustedClients(): Set<string> {
  return new Set(
    (process.env.OAUTH_TRUSTED_CLIENTS ?? "")
      .split(",")
      .map((c) => c.trim())
      .filter(Boolean),
  );
}

/**
 * Handle Hydra's consent redirect: look the challenge up at the admin API,
 * auto-accept for remembered/trusted clients, deny unknown ones. A missing or
 * stale challenge degrades to the dashboard home — never an error page a human
 * has to parse (the challenge is single-use and short-TTL; a bookmarked URL
 * must not strand anyone).
 */
export async function handleConsent(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const home = () => Response.redirect(new URL("/", url.origin).toString(), 302);

  const consentChallenge = url.searchParams.get("consent_challenge");
  if (!consentChallenge) return home();

  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  if (!admin) {
    return new Response("consent provider not configured", { status: 503 });
  }
  const hydra = new OAuth2Api(new Configuration({ basePath: admin }));

  let consent;
  try {
    consent = await hydra.getOAuth2ConsentRequest({ consentChallenge });
  } catch {
    // Stale/unknown challenge — degrade to home; a fresh authorize will mint a new one.
    return home();
  }

  const clientID = consent.client?.client_id ?? "";
  const trusted =
    consent.skip === true ||
    consent.client?.skip_consent === true ||
    trustedClients().has(clientID);
  if (!trusted) {
    return new Response(
      `consent required: client ${JSON.stringify(clientID)} is not a trusted first-party client`,
      { status: 403 },
    );
  }

  try {
    const { redirect_to } = await hydra.acceptOAuth2ConsentRequest({
      consentChallenge,
      acceptOAuth2ConsentRequest: {
        grant_scope: consent.requested_scope ?? [],
        grant_access_token_audience:
          consent.requested_access_token_audience ?? [],
        remember: true,
        remember_for: REMEMBER_FOR_SECONDS,
      },
    });
    return Response.redirect(redirect_to, 302);
  } catch {
    return new Response("consent accept failed", { status: 502 });
  }
}
