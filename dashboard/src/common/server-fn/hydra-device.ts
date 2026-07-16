const RENDER_CLI_CLIENT_ID = "429024F5E608930E2A65EF92591A25CC";

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

/**
 * Bridge Hydra's RFC 8628 browser verification sequence. The first visit (the
 * verification_uri_complete opened by the CLI) sends the user_code to Hydra's
 * public verifier so Hydra can set its CSRF cookie and mint a device_challenge.
 * Hydra redirects back here with that challenge; the server accepts the
 * user-code pairing through the cluster-private admin API and follows Hydra's
 * redirect into the existing Kratos login + Hydra consent flow.
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

export { RENDER_CLI_CLIENT_ID };
