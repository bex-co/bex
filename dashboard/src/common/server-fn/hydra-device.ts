import { fetchSession } from "@/common/server-fn/session";
import { isSameOrigin } from "@/common/server-fn/same-origin";
import {
  BodyTooLargeError,
  readBoundedFormData,
} from "@/common/server-fn/bounded-body";

const RENDER_CLI_CLIENT_ID = "429024F5E608930E2A65EF92591A25CC";
const DESKTOP_CLIENT_ID = "bex-desktop";

/** The device grant is confined to bex's first-party device clients (the CLI
 * and the desktop/editor), plus any operator-registered platform clients
 * (OAUTH_PLATFORM_CLIENTS, comma-separated). A self-registered third-party
 * client that somehow reached the accept redirect is refused as unexpected. */
function allowedDeviceClients(): Set<string> {
  const allowed = new Set<string>([RENDER_CLI_CLIENT_ID, DESKTOP_CLIENT_ID]);
  for (const id of (process.env.OAUTH_PLATFORM_CLIENTS ?? "").split(",")) {
    const trimmed = id.trim();
    if (trimmed) allowed.add(trimmed);
  }
  return allowed;
}

// The device-confirm form carries only a user_code + device_challenge, so
// anything past a few KiB is abuse — bound it before buffering (codex-security
// #11): request.formData() has no ceiling and Content-Length can be omitted.
const DEVICE_BODY_MAX = 1 << 14; // 16 KiB

function hydraURLs(): { admin: string; public: string } | null {
  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  const publicURL = process.env.HYDRA_PUBLIC_URL?.replace(/\/$/, "");
  return admin && publicURL ? { admin, public: publicURL } : null;
}

function error(message: string, status: number): Response {
  return new Response(message, {
    status,
    headers: { "content-type": "text/plain; charset=utf-8" },
  });
}

/** The only device-confirm state that reaches the browser: what a signed-in
 * human needs to see to confirm the code matches their CLI (mirrors
 * ConsentView in hydra-consent.ts). */
export type DeviceView = {
  userCode: string;
  challenge: string;
};

/** Recoverable device-verification failures (w10/m8 t001): each renders inside
 * AuthPageShell — like the confirm page itself — instead of the bare text
 * body `error()` used to return on a document GET/POST navigation. Adversarial
 * or purely operational failures (cross-site, no session, malformed body, a
 * down Hydra) stay as plain-text `error()` responses; they are not a state a
 * legitimate user recovers from by reading a nicer page. */
export type DeviceErrorCode =
  | "missing_code"
  | "invalid_code"
  | "unconfigured"
  | "unexpected_client";

export type DeviceErrorResult = { errorCode: DeviceErrorCode };

const DEVICE_ERROR_CODES = new Set<DeviceErrorCode>([
  "missing_code",
  "invalid_code",
  "unconfigured",
  "unexpected_client",
]);

function isDeviceErrorCode(value: string): value is DeviceErrorCode {
  return DEVICE_ERROR_CODES.has(value as DeviceErrorCode);
}

/** Redirects a POST failure back to the GET endpoint so it renders inside
 * AuthPageShell — the same self-redirect shape hydra-consent.ts's retry
 * mechanism already uses for its own failure path. */
function redirectToDeviceError(origin: string, code: DeviceErrorCode): Response {
  const target = new URL("/auth/device", origin);
  target.searchParams.set("device_error", code);
  return Response.redirect(target.toString(), 303);
}

/**
 * Bridge Hydra's RFC 8628 browser verification sequence. The first visit (the
 * verification_uri_complete opened by the CLI) sends the user_code to Hydra's
 * public verifier so Hydra can set its CSRF cookie and mint a device_challenge.
 * Hydra redirects back here with that challenge.
 *
 * codex-security #9 + the device-code phish fix: the grant is NEVER paired on a
 * GET, signed in or not. A signed-in caller gets an explicit "authorize this
 * device?" confirmation first; a signed-out visitor is bounced through login
 * with the user code + challenge preserved in the login page's same-origin
 * `next` param (loginFirst below — the consent route's pattern), and lands on
 * the SAME confirmation page after authenticating. Only a same-origin,
 * session-bound POST (handleDeviceConfirm) calls accept — so the trusted CLI
 * client's skip_consent consent auto-accept is unreachable until a human has
 * explicitly confirmed, and an attacker who tricks a signed-out victim into
 * opening the verification link and logging in still polls nothing.
 *
 * A signed-in caller's confirmation state is returned as a `DeviceView`
 * rather than rendered here — the `/auth/device` route hands it to the React
 * component (AuthPageShell chrome, same as every other auth page) as deferred
 * loader context, mirroring the consent route's `ConsentView` handoff.
 */
export async function handleDeviceVerification(
  request: Request,
): Promise<Response | DeviceView | DeviceErrorResult> {
  const requestURL = new URL(request.url);

  // A POST failure (acceptDevicePairing, handleDeviceConfirm) redirects here
  // with this param so the failure renders inside AuthPageShell instead of a
  // bare POST-response body. Takes priority over every other check below.
  const errorParam = requestURL.searchParams.get("device_error");
  if (errorParam && isDeviceErrorCode(errorParam)) {
    return { errorCode: errorParam };
  }

  const userCode = requestURL.searchParams.get("user_code")?.trim() ?? "";
  const challenge =
    requestURL.searchParams.get("device_challenge")?.trim() ?? "";
  const hydra = hydraURLs();

  if (!hydra) return { errorCode: "unconfigured" };
  if (!userCode) return { errorCode: "missing_code" };

  if (!challenge) {
    const verify = new URL("/oauth2/device/verify", hydra.public);
    verify.searchParams.set("user_code", userCode);
    return Response.redirect(verify.toString(), 302);
  }

  const { session, aal2Required } = await fetchSession(
    request.headers.get("cookie") ?? "",
  );
  if (session) {
    return { userCode, challenge };
  }
  return loginFirst(requestURL, userCode, challenge, aal2Required);
}

/**
 * Where a device verification with no usable session goes: log in, then come
 * back. The user code + device challenge ride the login page's `next` param —
 * the same mechanism the consent route uses for its own challenge
 * (hydra-consent.ts `loginFirst`). No extra signed state is needed: the
 * device_challenge is Hydra-minted, single-use, short-TTL opaque state, and the
 * login page normalizes `next` to a same-origin relative path (safe-next.ts)
 * before it becomes Kratos's return_to. After authenticating, the browser
 * re-enters handleDeviceVerification above — now with a session — and gets the
 * confirmation page; pairing still happens only via handleDeviceConfirm.
 *
 * A session owing a second factor is a step-up, not a sign-in, so the login
 * page is told so outright (`aal=aal2`), exactly as the consent route does —
 * otherwise the login page would bounce straight back here and loop.
 */
function loginFirst(
  url: URL,
  userCode: string,
  challenge: string,
  aal2Required: boolean,
): Response {
  const back = `/auth/device?user_code=${encodeURIComponent(userCode)}&device_challenge=${encodeURIComponent(challenge)}`;
  const login = new URL("/auth/login", url.origin);
  login.searchParams.set("next", back);
  if (aal2Required) login.searchParams.set("aal", "aal2");
  return Response.redirect(login.toString(), 302);
}

/**
 * The confirmation POST (same-origin, session-bound). A signed-in user has
 * clicked "Authorize"; pair the grant and continue into the consent flow.
 */
export async function handleDeviceConfirm(request: Request): Promise<Response> {
  const url = new URL(request.url);
  if (!isSameOrigin(request, url)) {
    return error("device confirmation refused: cross-site", 403);
  }
  const { session } = await fetchSession(request.headers.get("cookie") ?? "");
  if (!session) return error("device confirmation refused: no session", 403);

  let form: FormData;
  try {
    form = await readBoundedFormData(request, DEVICE_BODY_MAX);
  } catch (err) {
    if (err instanceof BodyTooLargeError) {
      return error("device confirmation too large", 413);
    }
    return error("malformed device confirmation", 400);
  }
  const userCode = String(form.get("user_code") ?? "").trim();
  const challenge = String(form.get("device_challenge") ?? "").trim();
  if (!userCode || !challenge) return error("missing device code", 400);

  const hydra = hydraURLs();
  if (!hydra) return redirectToDeviceError(url.origin, "unconfigured");
  return acceptDevicePairing(hydra, url.origin, userCode, challenge);
}

/** acceptDevicePairing PUTs the user_code/challenge to Hydra's admin API and
 * follows its redirect into the login + consent flow, refusing an unexpected
 * client. Reached only from the confirmed, session-bound POST
 * (handleDeviceConfirm) — never from a GET. */
async function acceptDevicePairing(
  hydra: { admin: string; public: string },
  origin: string,
  userCode: string,
  challenge: string,
): Promise<Response> {
  let response: Response;
  try {
    const accept = new URL(
      "/admin/oauth2/auth/requests/device/accept",
      hydra.admin,
    );
    accept.searchParams.set("device_challenge", challenge);
    response = await fetch(accept, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ user_code: userCode }),
    });
  } catch {
    return error("device authorization unavailable", 502);
  }

  if (!response.ok) {
    return redirectToDeviceError(origin, "invalid_code");
  }

  let redirectTo: string;
  try {
    redirectTo = String(
      ((await response.json()) as { redirect_to?: unknown }).redirect_to ?? "",
    );
  } catch {
    return error("device authorization returned an invalid response", 502);
  }

  let redirect: URL;
  try {
    redirect = new URL(redirectTo);
  } catch {
    return error("device authorization returned an invalid redirect", 502);
  }
  const publicOrigin = new URL(hydra.public).origin;
  if (
    redirect.origin !== publicOrigin ||
    !allowedDeviceClients().has(redirect.searchParams.get("client_id") ?? "")
  ) {
    return redirectToDeviceError(origin, "unexpected_client");
  }

  return Response.redirect(redirect.toString(), 302);
}

export { RENDER_CLI_CLIENT_ID, DESKTOP_CLIENT_ID };
