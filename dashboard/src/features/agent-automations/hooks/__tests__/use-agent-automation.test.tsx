import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { AgentAutomationDocument } from "@/graphql/definitions";
import { automation } from "@/features/agent-automations/components/__tests__/fixtures";
import { useAgentAutomation } from "../use-agent-automation";

const workspace = vi.hoisted(() => ({ current: "tea-a" as string | null }));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspace.current }),
}));

const id = "aut-dbpvn25hi5vc738gj8e0";

function setup(key: string, mocks: MockLink.MockedResponse[]) {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new MockLink(mocks),
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return renderHook(() => useAgentAutomation(key), { wrapper });
}

describe("useAgentAutomation", () => {
  it("reads an id without an owner, so it resolves in its own workspace", async () => {
    workspace.current = "tea-a";
    const { result } = setup(id, [
      {
        request: { query: AgentAutomationDocument, variables: { id } },
        result: {
          data: { agentAutomation: automation({ id, ownerId: "tea-b" }) },
        },
      },
    ]);
    await waitFor(() =>
      expect(result.current.automation?.ownerId).toBe("tea-b"),
    );
  });

  it("reads an id even before a workspace is selected", async () => {
    workspace.current = null;
    const { result } = setup(id, [
      {
        request: { query: AgentAutomationDocument, variables: { id } },
        result: { data: { agentAutomation: automation({ id }) } },
      },
    ]);
    await waitFor(() => expect(result.current.automation?.id).toBe(id));
  });

  it("holds a name until a workspace is selected", async () => {
    workspace.current = null;
    const { result } = setup("nightly-deps", []);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeUndefined();
  });

  it("reads a name within the selected workspace", async () => {
    workspace.current = "tea-a";
    const { result } = setup("nightly-deps", [
      {
        request: {
          query: AgentAutomationDocument,
          variables: { ownerId: "tea-a", id: "nightly-deps" },
        },
        result: { data: { agentAutomation: automation({ ownerId: "tea-a" }) } },
      },
    ]);
    await waitFor(() =>
      expect(result.current.automation?.name).toBe("nightly-deps"),
    );
  });
});
