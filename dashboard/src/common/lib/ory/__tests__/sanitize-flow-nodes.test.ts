import { describe, expect, it, vi } from "vitest";
import type { UiNode } from "@ory/client-fetch";
import {
  createFlowSanitizingFetch,
  dedupeUiNodes,
} from "../sanitize-flow-nodes";

function inputNode(
  group: string,
  name: string,
  type: string,
  value?: unknown,
): UiNode {
  return {
    group,
    type: "input",
    attributes: {
      node_type: "input",
      name,
      type,
      value,
      required: false,
      autocomplete: "",
      disabled: false,
    },
    messages: [],
    meta: {},
  } as unknown as UiNode;
}

describe("dedupeUiNodes", () => {
  it("drops exact duplicate nodes, keeping the first occurrence", () => {
    const nodes = [
      inputNode("default", "traits.email", "email", "a@b.c"),
      inputNode("profile", "method", "submit", "profile"),
      inputNode("profile", "method", "submit", "profile"),
    ];
    const deduped = dedupeUiNodes(nodes);
    expect(deduped).toHaveLength(2);
    expect(deduped[0]).toBe(nodes[0]);
    expect(deduped[1]).toBe(nodes[1]);
  });

  it("keeps nodes that differ only in value", () => {
    const nodes = [
      inputNode("oidc", "provider", "submit", "github"),
      inputNode("oidc", "provider", "submit", "google"),
    ];
    expect(dedupeUiNodes(nodes)).toHaveLength(2);
  });

  it("keeps nodes that differ only in group", () => {
    const nodes = [
      inputNode("default", "method", "submit", "profile"),
      inputNode("profile", "method", "submit", "profile"),
    ];
    expect(dedupeUiNodes(nodes)).toHaveLength(2);
  });

  it("keeps nodes that differ only in type", () => {
    const nodes = [
      inputNode("default", "traits.email", "hidden", "a@b.c"),
      inputNode("default", "traits.email", "email", "a@b.c"),
    ];
    expect(dedupeUiNodes(nodes)).toHaveLength(2);
  });

  it("returns the input untouched when there are no duplicates", () => {
    const nodes = [
      inputNode("default", "csrf_token", "hidden", "tok"),
      inputNode("default", "traits.name", "text", "bex"),
      inputNode("profile", "method", "submit", "profile"),
    ];
    expect(dedupeUiNodes(nodes)).toEqual(nodes);
  });
});

describe("createFlowSanitizingFetch", () => {
  function jsonResponse(body: unknown, init?: ResponseInit): Response {
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "content-type": "application/json" },
      ...init,
    });
  }

  it("rewrites a flow response with duplicated ui.nodes", async () => {
    const duplicated = inputNode("profile", "method", "submit", "profile");
    const flow = {
      id: "flow-id",
      ui: {
        action: "https://auth.bex.co/self-service/registration?flow=flow-id",
        nodes: [
          inputNode("default", "traits.email", "email", "a@b.c"),
          duplicated,
          duplicated,
        ],
      },
    };
    const fetchFn = vi.fn(async () => jsonResponse(flow));
    const res = await createFlowSanitizingFetch(fetchFn)("https://x", {});
    const parsed = (await res.json()) as {
      ui: { nodes: UiNode[]; action: string };
    };
    expect(parsed.ui.nodes).toHaveLength(2);
    expect(parsed.ui.action).toBe(flow.ui.action);
  });

  it("returns the original response when there are no duplicates", async () => {
    const original = jsonResponse({
      ui: { nodes: [inputNode("profile", "method", "submit", "profile")] },
    });
    const fetchFn = vi.fn(async () => original);
    const res = await createFlowSanitizingFetch(fetchFn)("https://x", {});
    expect(res).toBe(original);
  });

  it("passes non-JSON responses through untouched", async () => {
    const original = new Response("<html></html>", {
      headers: { "content-type": "text/html" },
    });
    const fetchFn = vi.fn(async () => original);
    const res = await createFlowSanitizingFetch(fetchFn)("https://x", {});
    expect(res).toBe(original);
  });

  it("passes JSON responses without ui.nodes through untouched", async () => {
    const original = jsonResponse({ error: { id: "session_inactive" } });
    const fetchFn = vi.fn(async () => original);
    const res = await createFlowSanitizingFetch(fetchFn)("https://x", {});
    expect(res).toBe(original);
  });

  it("preserves status and strips stale content-length on rewrite", async () => {
    const flow = {
      ui: {
        nodes: [
          inputNode("profile", "screen", "submit", "previous"),
          inputNode("profile", "screen", "submit", "previous"),
        ],
      },
    };
    const fetchFn = vi.fn(async () =>
      jsonResponse(flow, { status: 400, statusText: "Bad Request" }),
    );
    const res = await createFlowSanitizingFetch(fetchFn)("https://x", {});
    expect(res.status).toBe(400);
    expect(res.headers.get("content-length")).toBeNull();
    const parsed = (await res.json()) as { ui: { nodes: UiNode[] } };
    expect(parsed.ui.nodes).toHaveLength(1);
  });
});
