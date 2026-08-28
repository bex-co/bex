import { Configuration, OAuth2Api } from "@ory/client-fetch";
import type { OAuth2ConsentSession } from "@ory/client-fetch";
import { fetchSession } from "@/common/server-fn/session";
import { isSameOrigin } from "@/common/server-fn/same-origin";
import { config } from "@/config/config";
import {
  BodyTooLargeError,
  readBoundedJson,
} from "@/common/server-fn/bounded-body";
import { safeHttpHref } from "@/common/lib/external-url";

// The revoke body is a tiny { clientId } object — bound it before buffering
// (codex-security #11): request.json() has no ceiling and Content-Length can be
// omitted on a chunked/HTTP-2 body.
const AGENTS_BODY_MAX = 1 << 14; // 16 KiB

// Settings → Security & Compliance "Connected agents" card (docs/ADR012-auth.md
// §7, w4/m18): everything that can currently act as the signed-in human via a
// Hydra-issued OAuth2 access token — the flip side of `hydra-consent.ts`'s
// accept call. Hydra tracks one consent session per (subject, client, login
// session); `listConnectedAgents` merges those into one row per client so a
// user sees "ChatGPT" once, not once per time they approved it.
//
// SERVER-ONLY, same discipline as `hydra-consent.ts`: reached exclusively via
// the `/api/connected-agents` route's server handlers (dynamic import), so
// neither the admin client nor `HYDRA_ADMIN_URL` reach the client bundle.

export type ConnectedAgentView = {
  clientId: string;
  clientName: string;
  clientUri?: string;
  scopes: string[];
  /** RFC 3339 of the most recent consent grant for this client, or null if Hydra didn't say. */
  grantedAt: string | null;
};

function hydraAdmin(): OAuth2Api | null {
  const admin = process.env.HYDRA_ADMIN_URL?.replace(/\/$/, "");
  if (!admin) return null;
  return new OAuth2Api(new Configuration({ basePath: admin }));
}

async function subjectFor(request: Request): Promise<string | null> {
  const cookie = request.headers.get("cookie");
  if (!cookie) return null;
  const { session } = await fetchSession(cookie);
  return session?.identity?.id ?? null;
}

/** The later of two RFC 3339 grant dates, treating null as "unknown, not later." */
function laterGrant(a: string | null, b: string | null): string | null {
  if (!a) return b;
  if (!b) return a;
  return a > b ? a : b;
}

type Accumulator = {
  clientName: string;
  clientUri?: string;
  scopes: Set<string>;
  grantedAt: string | null;
};

/** One row per client_id: union the granted scopes, keep the latest grant date. */
function mergeByClient(sessions: OAuth2ConsentSession[]): ConnectedAgentView[] {
  const byClient = new Map<string, Accumulator>();
  for (const s of sessions) {
    const client = s.consent_request?.client;
    const clientId = client?.client_id;
    if (!clientId) continue;
    const grantedAt = s.handled_at ? s.handled_at.toISOString() : null;

    const existing = byClient.get(clientId);
    if (existing) {
      for (const scope of s.grant_scope ?? []) existing.scopes.add(scope);
      existing.grantedAt = laterGrant(existing.grantedAt, grantedAt);
    } else {
      byClient.set(clientId, {
        clientName: client.client_name || clientId,
        // client_uri is attacker-chosen (self-registered DCR client) and is
        // rendered as a live anchor href on the connected-agents settings row.
        // Drop any non-http(s) scheme so the row renders the client name as
        // plain text rather than a hostile link (round-21 finding 5).
        clientUri: safeHttpHref(client.client_uri),
        scopes: new Set(s.grant_scope ?? []),
        grantedAt,
      });
    }
  }
  return [...byClient.entries()]
    .map(([clientId, acc]) => ({
      clientId,
      clientName: acc.clientName,
      clientUri: acc.clientUri,
      scopes: [...acc.scopes],
      grantedAt: acc.grantedAt,
    }))
    .sort((a, b) => (b.grantedAt ?? "").localeCompare(a.grantedAt ?? ""));
}

/** Every OAuth2 client the signed-in human has granted consent to, newest first. */
export async function listConnectedAgents(request: Request): Promise<Response> {
  const subject = await subjectFor(request);
  if (!subject) return Response.json([]);

  const hydra = hydraAdmin();
  if (!hydra) {
    return new Response("consent provider not configured", { status: 503 });
  }

  try {
    const sessions = await hydra.listOAuth2ConsentSessions({ subject });
    return Response.json(mergeByClient(sessions));
  } catch {
    return new Response("failed to list connected agents", { status: 502 });
  }
}

/**
 * Revokes every consent session the subject granted a given client —
 * invalidating all of that client's outstanding access tokens (Hydra's own
 * semantics for `revokeOAuth2ConsentSessions` with `subject` + `client`).
 */
export async function revokeConnectedAgent(
  request: Request,
): Promise<Response> {
  const url = new URL(request.url);
  if (!isSameOrigin(request, url)) {
    return new Response("revoke refused: cross-site request", { status: 403 });
  }

  const subject = await subjectFor(request);
  if (!subject) return new Response("no session", { status: 401 });

  let clientId: string;
  try {
    const body = await readBoundedJson<{ clientId?: string }>(
      request,
      AGENTS_BODY_MAX,
    );
    clientId = body.clientId ?? "";
  } catch (err) {
    if (err instanceof BodyTooLargeError) {
      return new Response("request too large", { status: 413 });
    }
    return new Response("malformed request", { status: 400 });
  }
  if (!clientId) return new Response("missing clientId", { status: 400 });

  const hydra = hydraAdmin();
  if (!hydra) {
    return new Response("consent provider not configured", { status: 503 });
  }

  try {
    await hydra.revokeOAuth2ConsentSessions({ subject, client: clientId });
    const apiBase = (config.ssrApiUrl ?? "").replace(/\/graphql\/?$/, "");
    const cookie = request.headers.get("cookie");
    const marker = await fetch(`${apiBase}/v1/oauth/revocations`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        ...(cookie ? { cookie } : {}),
      },
      body: JSON.stringify({ clientId }),
    });
    if (!marker.ok) {
      return new Response("revoke propagation failed", { status: 502 });
    }
    return new Response(null, { status: 204 });
  } catch {
    return new Response("revoke failed", { status: 502 });
  }
}
