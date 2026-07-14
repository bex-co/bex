import type { Session } from "@ory/client-fetch";
import { createFrontendApi } from "@/common/lib/ory/frontend";
import { fetchSession } from "@/common/server-fn/session";
import { isSameOrigin } from "@/common/server-fn/same-origin";

// Settings → Security & Compliance "Active sessions" card (docs/ADR012-auth.md,
// w4/006 folded into w4/m18): every browser session Kratos currently
// recognizes as this human, plus "sign out other sessions" — the self-service
// FrontendApi (cookie-authenticated), not the admin API, since a user is only
// ever allowed to see/revoke their *own* sessions here.
//
// SERVER-ONLY, same discipline as `hydra-consent.ts`: reached exclusively via
// the `/api/sessions` route's server handlers (dynamic import).

export type SessionView = {
  id: string;
  /** True for the session behind this very request — never revocable from here. */
  current: boolean;
  ipAddress?: string;
  location?: string;
  userAgent?: string;
  authenticatedAt: string | null;
};

function toView(session: Session, currentId: string | undefined): SessionView {
  const device = session.devices?.[0];
  return {
    id: session.id,
    current: !!currentId && session.id === currentId,
    ipAddress: device?.ip_address,
    location: device?.location,
    userAgent: device?.user_agent,
    authenticatedAt: session.authenticated_at
      ? session.authenticated_at.toISOString()
      : null,
  };
}

/** The current session first, then every other session Kratos still recognizes. */
export async function listSessions(request: Request): Promise<Response> {
  const cookie = request.headers.get("cookie");
  if (!cookie) return Response.json([]);

  const { session: current } = await fetchSession(cookie);
  if (!current) return Response.json([]);

  try {
    const others = await createFrontendApi(cookie).listMySessions();
    return Response.json([
      toView(current, current.id),
      ...others.map((s) => toView(s, current.id)),
    ]);
  } catch {
    return new Response("failed to list sessions", { status: 502 });
  }
}

/**
 * `{ action: "sign-out-others" }` invalidates every session but this one;
 * `{ action: "revoke", id }` invalidates one specific other session (Kratos
 * itself refuses to let the current session revoke itself this way).
 */
export async function revokeSession(request: Request): Promise<Response> {
  const url = new URL(request.url);
  if (!isSameOrigin(request, url)) {
    return new Response("revoke refused: cross-site request", { status: 403 });
  }

  const cookie = request.headers.get("cookie");
  if (!cookie) return new Response("no session", { status: 401 });
  const { session } = await fetchSession(cookie);
  if (!session) return new Response("no session", { status: 401 });

  let body: { action?: string; id?: string };
  try {
    body = (await request.json()) as { action?: string; id?: string };
  } catch {
    return new Response("malformed request", { status: 400 });
  }

  const frontend = createFrontendApi(cookie);
  try {
    if (body.action === "sign-out-others") {
      const result = await frontend.disableMyOtherSessions();
      return Response.json(result);
    }
    if (body.action === "revoke" && body.id) {
      await frontend.disableMySession({ id: body.id });
      return new Response(null, { status: 204 });
    }
    return new Response("malformed request", { status: 400 });
  } catch {
    return new Response("revoke failed", { status: 502 });
  }
}
