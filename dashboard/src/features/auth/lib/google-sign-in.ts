import {
  isUiNodeInputAttributes,
  type LoginFlow,
  type SettingsFlow,
} from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { oryErrorInfo } from "@/common/lib/ory/errors";

/** The Kratos oidc provider id of Google (scripts/lib/kratos-oidc-fragment.sh). */
const GOOGLE_PROVIDER = "google";

/**
 * Mints a browser flow for `returnTo`, dropping a `return_to` Kratos does not
 * allowlist (a local dashboard against a remote Kratos) rather than failing
 * the sign-in, as `useOryFlow` does.
 */
async function withReturnTo<T>(
  create: (returnTo?: string) => Promise<T>,
  returnTo: string,
): Promise<T> {
  try {
    return await create(returnTo);
  } catch (err) {
    const { id } = await oryErrorInfo(err);
    if (id !== "self_service_flow_return_to_forbidden") throw err;
    return create();
  }
}

/** The login flow's anti-CSRF token, which its oidc submit carries. */
function csrfToken(flow: LoginFlow): string | undefined {
  for (const { attributes } of flow.ui.nodes) {
    if (
      isUiNodeInputAttributes(attributes) &&
      attributes.name === "csrf_token"
    ) {
      return typeof attributes.value === "string"
        ? attributes.value
        : undefined;
    }
  }
  return undefined;
}

/** Whether the settings flow offers to link Google: the identity has no
 *  Google credential yet (a linked one offers "unlink google" instead). */
function offersGoogleLink(flow: SettingsFlow): boolean {
  return flow.ui.nodes.some(
    ({ group, attributes }) =>
      group === "oidc" &&
      isUiNodeInputAttributes(attributes) &&
      attributes.name === "link" &&
      attributes.value === GOOGLE_PROVIDER,
  );
}

/**
 * Submits a flow and follows where Kratos sends the browser. An oidc submit
 * never completes inline: Kratos answers `browser_location_change_required`
 * with the provider's URL. Any other answer that names a destination is
 * followed too (a settings change from a session too old to be privileged
 * answers 403 `session_refresh_required`, pointing at the re-authentication).
 * An answer with no destination rejects.
 */
async function submitAndFollow(submit: () => Promise<unknown>): Promise<true> {
  try {
    await submit();
  } catch (err) {
    const { redirectBrowserTo } = await oryErrorInfo(err);
    if (!redirectBrowserTo) throw err;
    window.location.assign(redirectBrowserTo);
    return true;
  }
  throw new Error("Kratos did not send the browser to Google");
}

/**
 * Signs the current browser session in with Google (w7/m465): what a member
 * of a workspace that requires Google login does to get back in
 * (docs/ADR012-auth.md 9b). bex-api reads the session's
 * `authentication_methods`, so the session itself must carry
 * `{method: oidc, provider: google}`; Kratos adds it in one of two ways,
 * depending on whether the identity has a Google credential yet. Both come
 * back to `returnTo` once Google has answered.
 *
 * - **Google not linked yet.** A browser settings flow offers "link google":
 *   submitting it (`method=oidc&link=google`) links the Google account and
 *   adds the Google sign-in to the current session at once. A refresh login
 *   with Google does not work here: Kratos (v26.2.0, probed live) offers no
 *   oidc option on that flow, and its callback leaves the session as it was.
 *   A session too old for a settings change is sent to re-authenticate
 *   first, as Kratos asks.
 * - **Google already linked.** The settings flow offers "unlink google"
 *   instead, so the session re-authenticates with Google through a browser
 *   login flow with `refresh=true` (the one kind Kratos mints for a browser
 *   that holds a session), submitted with `method=oidc&provider=google`, as
 *   Ory Elements' own provider button submits a login flow.
 *
 * Resolves `true` once the browser is on its way. Anything else Kratos says
 * (Google not configured on this installation, an expired flow) rejects, so
 * the caller can say the sign-in did not start.
 */
export async function startGoogleSignIn(returnTo: string): Promise<true> {
  const api = createFrontendApi();
  const settings = await withReturnTo(
    (to) => api.createBrowserSettingsFlow({ returnTo: to }),
    returnTo,
  );
  if (offersGoogleLink(settings)) {
    // No csrf_token: the SDK's oidc settings body has no such field (its
    // serializer drops it), and Kratos's oidc settings strategy checks none.
    // This is the very body Ory Elements' own "Link" button sends.
    return submitAndFollow(() =>
      api.updateSettingsFlow({
        flow: settings.id,
        updateSettingsFlowBody: { method: "oidc", link: GOOGLE_PROVIDER },
      }),
    );
  }
  const login = await withReturnTo(
    (to) => api.createBrowserLoginFlow({ refresh: true, returnTo: to }),
    returnTo,
  );
  return submitAndFollow(() =>
    api.updateLoginFlow({
      flow: login.id,
      updateLoginFlowBody: {
        method: "oidc",
        provider: GOOGLE_PROVIDER,
        csrf_token: csrfToken(login),
      },
    }),
  );
}
