import type { UiNode } from "@ory/client-fetch";

/**
 * Kratos's two-step (identifier-first) registration handler emits DUPLICATE ui
 * nodes in some flow responses — observed against prod (auth.bex.co):
 *
 * - `POST /self-service/registration?flow=…` with `screen=previous` and no
 *   `method` (exactly what Ory Elements' email/back button submits) answers
 *   400 with `profile:method=profile` present TWICE.
 * - A profile submit that Kratos rejects (e.g. account-exists mitigation)
 *   answers 400 with `profile:screen=previous` present TWICE.
 *
 * Ory Elements renders one button per node, so each duplicated node becomes a
 * second visible "Sign up" (or email/back) button in the card. Re-fetching
 * the same flow (`GET /self-service/registration/flows`) returns the nodes
 * deduplicated, so this only ever shows up on inline-rendered submit
 * responses. Sanitizing at the fetch layer covers both Ory Elements' internal
 * submits (`oryConfig.sdk.options.fetchApi`) and this app's own flow fetches
 * (`createFrontendApi`), regardless of which response path carries the dupes.
 */

function nodeKey(node: UiNode): string {
  const attrs = node.attributes as
    | { name?: string; type?: string; value?: unknown }
    | undefined;
  return [node.group, attrs?.name, attrs?.type, JSON.stringify(attrs?.value)]
    .map((part) => String(part))
    .join("|");
}

/** Drops exact duplicate ui nodes (same group/name/type/value), keeping the first. */
export function dedupeUiNodes(nodes: UiNode[]): UiNode[] {
  const seen = new Set<string>();
  return nodes.filter((node) => {
    const key = nodeKey(node);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

type JsonObject = Record<string, unknown>;

/**
 * Wraps a fetch implementation so any Kratos flow response with duplicated
 * `ui.nodes` is rewritten deduplicated. Responses that aren't JSON, don't
 * carry `ui.nodes`, or have no duplicates are returned untouched.
 */
export function createFlowSanitizingFetch(fetchFn: typeof fetch): typeof fetch {
  return async (input, init) => {
    const response = await fetchFn(input, init);
    if (!(response.headers.get("content-type") ?? "").includes("json")) {
      return response;
    }
    let body: JsonObject;
    try {
      body = (await response.clone().json()) as JsonObject;
    } catch {
      return response; // not a parseable JSON body — leave it alone
    }
    const ui = body?.ui as { nodes?: unknown } | undefined;
    if (!ui || !Array.isArray(ui.nodes)) return response;
    const nodes = ui.nodes as UiNode[];
    const deduped = dedupeUiNodes(nodes);
    if (deduped.length === nodes.length) return response;
    // Rebuilt body is already decoded — don't re-advertise the wire encoding
    // or its old length.
    const headers = new Headers(response.headers);
    headers.delete("content-encoding");
    headers.delete("content-length");
    return new Response(
      JSON.stringify({ ...body, ui: { ...ui, nodes: deduped } }),
      { status: response.status, statusText: response.statusText, headers },
    );
  };
}

/**
 * Shared instance for both Kratos clients (`createFrontendApi` and Ory
 * Elements' `sdk.options`). Defers to the current global `fetch` per call so
 * test mocks and SSR runtimes keep working.
 */
export const sanitizedFlowFetch: typeof fetch = createFlowSanitizingFetch(
  (input, init) => fetch(input, init),
);
