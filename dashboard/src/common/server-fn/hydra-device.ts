import { fetchSession } from "@/common/server-fn/session";
import { isSameOrigin } from "@/common/server-fn/same-origin";
import {
  BodyTooLargeError,
  readBoundedFormData,
} from "@/common/server-fn/bounded-body";

const RENDER_CLI_CLIENT_ID = "429024F5E608930E2A65EF92591A25CC";

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

function htmlEscape(s: string): string {
  return s.replace(/[&<>"']/g, (c) => {
    switch (c) {
      case "&":
        return "&amp;";
      case "<":
        return "&lt;";
      case ">":
        return "&gt;";
      case '"':
        return "&quot;";
      default:
        return "&#39;";
    }
  });
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
 */
export async function handleDeviceVerification(
  request: Request,
): Promise<Response> {
  const requestURL = new URL(request.url);
  const userCode = requestURL.searchParams.get("user_code")?.trim() ?? "";
  const challenge =
    requestURL.searchParams.get("device_challenge")?.trim() ?? "";
  const hydra = hydraURLs();

  if (!hydra) return error("device authorization not configured", 503);
  if (!userCode) return error("missing or expired device code", 400);

  if (!challenge) {
    const verify = new URL("/oauth2/device/verify", hydra.public);
    verify.searchParams.set("user_code", userCode);
    return Response.redirect(verify.toString(), 302);
  }

  const { session, aal2Required } = await fetchSession(
    request.headers.get("cookie") ?? "",
  );
  if (session) {
    return deviceConfirmationPage(requestURL.origin, userCode, challenge);
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
  if (!hydra) return error("device authorization not configured", 503);
  return acceptDevicePairing(hydra, userCode, challenge);
}

/** acceptDevicePairing PUTs the user_code/challenge to Hydra's admin API and
 * follows its redirect into the login + consent flow, refusing an unexpected
 * client. Reached only from the confirmed, session-bound POST
 * (handleDeviceConfirm) — never from a GET. */
async function acceptDevicePairing(
  hydra: { admin: string; public: string },
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
    return error("device code is invalid, expired, or already used", 400);
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
    redirect.searchParams.get("client_id") !== RENDER_CLI_CLIENT_ID
  ) {
    return error("device authorization refused an unexpected client", 403);
  }

  return Response.redirect(redirect.toString(), 302);
}

/** deviceConfirmationPage renders a same-origin confirmation interstitial. The
 * hidden form POSTs the user_code + device_challenge back to /auth/device; the
 * POST handler verifies same-origin + an authenticated session before pairing
 * (codex-security #9). */
function deviceConfirmationPage(
  origin: string,
  userCode: string,
  challenge: string,
): Response {
  const safeCode = htmlEscape(userCode);
  const safeChallenge = htmlEscape(challenge);
  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Authorize device — bex</title>
  <style>
    body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;
      background:#111827;color:#f9fafb}
    main{max-width:28rem;padding:2rem;text-align:center}
    h1{font-size:1.5rem;margin-bottom:.5rem}p{color:#d1d5db;line-height:1.6}
    .code{font-family:ui-monospace,monospace;background:#1f2937;padding:.25rem .5rem;border-radius:.375rem;
      letter-spacing:.1em;margin:.25rem 0}
    form{margin-top:1.5rem;display:flex;gap:.75rem;justify-content:center}
    button{background:#6366f1;color:#fff;border:0;border-radius:.5rem;padding:.6rem 1.25rem;font-weight:600;cursor:pointer}
    a{color:#9ca3af;text-decoration:underline}
  </style>
</head>
<body><main>
  <h1>Authorize the bex CLI?</h1>
  <p>A device is requesting access to your bex account. Confirm the code matches what your CLI displayed:</p>
  <p class="code">${safeCode}</p>
  <p><small>Only authorize this if you started the request.</small></p>
  <form method="POST" action="${htmlEscape(origin)}/auth/device">
    <input type="hidden" name="user_code" value="${safeCode}">
    <input type="hidden" name="device_challenge" value="${safeChallenge}">
    <button type="submit">Authorize device</button>
    <a href="/">Cancel</a>
  </form>
</main></body>
</html>`;
  return new Response(html, {
    headers: { "content-type": "text/html; charset=utf-8" },
  });
}

export { RENDER_CLI_CLIENT_ID };
