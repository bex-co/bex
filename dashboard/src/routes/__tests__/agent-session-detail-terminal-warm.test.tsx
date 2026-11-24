import { describe, expect, it, vi } from "vitest";
import { renderToString } from "react-dom/server";
import {
  ApolloClient,
  ApolloLink,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import {
  AgentSessionCapabilitiesDocument,
  AgentSessionDocument,
} from "@/graphql/definitions";
import { useSessionTerminalOffered } from "@/features/agent-sessions/hooks/use-agent-session-terminal";

// w10/059: on a cold load of a session's page the header painted Files,
// Connect and "…", then inserted Terminal between Files and Connect once a
// client capabilities query answered. The loader now warms that read, so the
// SSR pass and hydration render the Terminal with the other actions.

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-selected" }),
}));

import { Route } from "../agents_.$agentSessionId";

type Loader = (args: unknown) => Promise<{ state: string }>;
const loader = Route.options.loader as unknown as Loader;

function capabilitiesAnswer(terminalEnabled: boolean) {
  return {
    agentSessionCapabilities: {
      __typename: "AgentSessionCapabilities",
      enabled: true,
      sshEnabled: true,
      terminalEnabled,
      defaultTemplate: "agent",
      agents: [],
      templates: [],
      github: { __typename: "AgentSessionGitHubReadiness", connected: true },
      attachments: {
        __typename: "AgentAttachmentLimits",
        enabled: false,
        maxBytes: 0,
        maxPerTurn: 0,
      },
    },
  };
}

/**
 * A real Apollo cache behind the loader. The capabilities read goes through a
 * link that records its owner; the session read (a large document this test
 * does not need in the cache) answers from `session`, which the test settles.
 */
function setup({ refuseCapabilities = false } = {}) {
  const capabilityOwners: string[] = [];
  const cache = new ApolloClient({
    cache: new InMemoryCache(),
    link: new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          capabilityOwners.push(String(operation.variables.ownerId));
          observer.next(
            refuseCapabilities
              ? {
                  data: null,
                  errors: [
                    { message: "forbidden", extensions: { code: "FORBIDDEN" } },
                  ],
                }
              : { data: capabilitiesAnswer(true) },
          );
          observer.complete();
        }),
    ),
  });
  let answerSession: (ownerId: string) => void = () => {};
  const session = new Promise<unknown>((resolve) => {
    answerSession = (ownerId) =>
      resolve({
        data: {
          agentSession: { id: "ags-1", ownerId, prompt: "fix the flaky test" },
        },
      });
  });
  const client = {
    query: (options: { query: unknown }) =>
      options.query === AgentSessionDocument
        ? session
        : cache.query(options as Parameters<typeof cache.query>[0]),
  };
  return { cache, client, capabilityOwners, answerSession };
}

function Probe({ ownerId }: { ownerId: string }) {
  return useSessionTerminalOffered({ ownerId }) ? (
    <button type="button">Terminal</button>
  ) : (
    <span>no terminal</span>
  );
}

/** The header's Terminal gate, rendered as the SSR pass renders it. */
function serverRender(cache: ApolloClient, ownerId: string): string {
  return renderToString(
    <ApolloProvider client={cache}>
      <Probe ownerId={ownerId} />
    </ApolloProvider>,
  );
}

const load = (client: unknown, workspaceId: string | null) =>
  loader({
    context: { client, workspaceId },
    params: { agentSessionId: "ags-1" },
    cause: "enter",
  });

describe("agent session detail loader warms the Terminal's capabilities", () => {
  it("reads the selected workspace's beside the session, so the SSR header has the Terminal", async () => {
    const { cache, client, capabilityOwners, answerSession } = setup();
    // Unwarmed, the server's pass has no Terminal: the button the client
    // query used to insert after first paint.
    expect(serverRender(cache, "tea-selected")).toContain("no terminal");

    const loading = load(client, "tea-selected");
    // Started before the session answers, not after it.
    await vi.waitFor(() => expect(capabilityOwners).toEqual(["tea-selected"]));
    answerSession("tea-selected");

    expect(await loading).toMatchObject({ state: "ready" });
    expect(capabilityOwners).toEqual(["tea-selected"]);
    expect(serverRender(cache, "tea-selected")).toContain("Terminal</button>");
  });

  it("reads the session owner's when the session is another workspace's", async () => {
    const { cache, client, capabilityOwners, answerSession } = setup();
    const loading = load(client, "tea-selected");
    answerSession("tea-other");

    expect(await loading).toMatchObject({ state: "ready" });
    expect(capabilityOwners).toEqual(["tea-selected", "tea-other"]);
    expect(serverRender(cache, "tea-other")).toContain("Terminal</button>");
  });

  it("reads the session owner's when no workspace is selected", async () => {
    const { cache, client, capabilityOwners, answerSession } = setup();
    const loading = load(client, null);
    answerSession("tea-other");

    expect(await loading).toMatchObject({ state: "ready" });
    expect(capabilityOwners).toEqual(["tea-other"]);
    expect(serverRender(cache, "tea-other")).toContain("Terminal</button>");
  });

  it("keeps the session's title when the capabilities read is refused", async () => {
    const { cache, client, answerSession } = setup({
      refuseCapabilities: true,
    });
    const loading = load(client, "tea-selected");
    answerSession("tea-selected");

    expect(await loading).toMatchObject({ state: "ready" });
    // Nothing cached: the header's own query asks again on the client.
    expect(
      cache.cache.readQuery({
        query: AgentSessionCapabilitiesDocument,
        variables: { ownerId: "tea-selected" },
      }),
    ).toBeNull();
  });
});
