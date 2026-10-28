import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import type { UIMessageChunk } from "ai";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { AgentSessionToolOutputDocument } from "@/graphql/definitions";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { ActivityGroup } from "@/features/agent-sessions/components/activity-group";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import { makeFixtureFetch } from "@/features/agent-sessions/lib/mock-stream";

// w10/m18: a replayed tool output the server clipped carries a flag when the
// complete text was stored out of line; "Show full output" loads it from
// bex-api on demand. Without the flag (clipped before full copies were kept)
// the step says the full output was not retained.

const CLIPPED = "head\n\n[bex: 9000 chars clipped]\n\ntail";
const FULL = "head" + "m".repeat(9000) + "tail";

function apollo(mocks: object[]) {
  const client = new ApolloClient({
    link: new MockLink(mocks as never) as ApolloLink,
    cache: new InMemoryCache(apolloCacheConfig),
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <ApolloProvider client={client}>{children}</ApolloProvider>;
  };
}

function toolOutputMock(
  turn: number,
  toolCallId: string,
  outputs: { field: string; text: string }[],
) {
  return {
    request: {
      query: AgentSessionToolOutputDocument,
      variables: { id: "ags-full", turn, toolCallId },
    },
    result: {
      data: {
        agentSessionToolOutput: {
          __typename: "AgentSessionToolOutput",
          sessionId: "ags-full",
          turn,
          toolCallId,
          outputs: outputs.map((o) => ({
            __typename: "AgentSessionToolOutputField",
            ...o,
          })),
        },
      },
    },
  };
}

const REPLAY: UIMessageChunk[] = [
  { type: "start", messageId: "durable-ags-full" },
  {
    type: "data-user-prompt",
    data: { turn: 2, text: "run it" },
  } as UIMessageChunk,
  {
    type: "tool-input-available",
    toolCallId: "t-big",
    toolName: "bash",
    input: { command: "make test" },
    dynamic: true,
  } as UIMessageChunk,
  {
    type: "tool-output-available",
    toolCallId: "t-big",
    output: CLIPPED,
    dynamic: true,
    // agenttranscript.Expand's flag for an item whose full copy is stored.
    providerMetadata: { "bex-transcript": { fullOutput: true } },
  } as UIMessageChunk,
  {
    type: "tool-input-available",
    toolCallId: "t-old",
    toolName: "bash",
    input: { command: "make lint" },
    dynamic: true,
  } as UIMessageChunk,
  {
    type: "tool-output-available",
    toolCallId: "t-old",
    output: CLIPPED,
    dynamic: true,
  } as UIMessageChunk,
];

describe("full tool output on demand", () => {
  it("loads a flagged clipped output for its turn and tool call; an unflagged one is not retained", async () => {
    const Wrapper = apollo([
      toolOutputMock(2, "t-big", [{ field: "output", text: FULL }]),
    ]);
    render(
      <Wrapper>
        <SessionConversationImpl
          sessionId="ags-full"
          isTerminal
          phase="completed"
          transport={createAgentSessionTransport({
            sessionId: "ags-full",
            mintTicket: async () => ({ ticket: "fixture-ticket" }),
            fetch: makeFixtureFetch([...REPLAY, { type: "finish" }], {
              terminal: true,
            }),
          })}
        />
      </Wrapper>,
    );
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );
    const group = document.querySelector<HTMLElement>(
      '[data-block="activity"]',
    )!;
    await userEvent.click(within(group).getAllByRole("button")[0]);

    // Only the flagged step offers the full output; the other one says so.
    const show = within(group).getAllByRole("button", {
      name: "Show full output",
    });
    expect(show).toHaveLength(1);
    expect(
      within(group).getByText("Full output not retained"),
    ).toBeInTheDocument();

    await userEvent.click(show[0]);
    await waitFor(() =>
      expect(
        group.querySelector('[data-full-output="output"]')?.textContent,
      ).toBe(FULL),
    );
    expect(
      within(group).getByRole("button", { name: "Copy full output" }),
    ).toBeInTheDocument();
    await userEvent.click(
      within(group).getByRole("button", { name: "Hide full output" }),
    );
    expect(group.querySelector("[data-full-output]")).toBeNull();
  });

  it("says not retained when the server has no full copy", async () => {
    const Wrapper = apollo([
      {
        request: {
          query: AgentSessionToolOutputDocument,
          variables: { id: "ags-full", turn: 1, toolCallId: "t-gone" },
        },
        result: {
          errors: [
            {
              message: "no full output is stored for this tool call",
              extensions: { code: "AGENT_SESSION_TOOL_OUTPUT_NOT_FOUND" },
            },
          ],
        },
      },
    ]);
    render(
      <Wrapper>
        <ActivityGroup
          sessionId="ags-full"
          steps={[
            {
              kind: "tool",
              toolCallId: "t-gone",
              name: "bash",
              state: "output-available",
              input: { command: "make" },
              output: CLIPPED,
              turn: 1,
              fullOutput: true,
            },
          ]}
        />
      </Wrapper>,
    );
    await userEvent.click(screen.getByRole("button"));
    await userEvent.click(
      screen.getByRole("button", { name: "Show full output" }),
    );
    await waitFor(() =>
      expect(screen.getByText("Full output not retained")).toBeInTheDocument(),
    );
  });

  it("loads a terminal snapshot's full output by its terminal field", async () => {
    const Wrapper = apollo([
      toolOutputMock(3, "t-term", [
        { field: "output", text: "not this one" },
        { field: "terminal:term-1", text: FULL },
      ]),
    ]);
    render(
      <Wrapper>
        <ActivityGroup
          sessionId="ags-full"
          steps={[
            {
              kind: "terminal",
              toolCallId: "t-term",
              terminalId: "term-1",
              output: CLIPPED,
              turn: 3,
              fullOutput: true,
            },
          ]}
        />
      </Wrapper>,
    );
    await userEvent.click(screen.getByRole("button"));
    await userEvent.click(
      screen.getByRole("button", { name: "Show full output" }),
    );
    await waitFor(() =>
      expect(
        document.querySelector('[data-full-output="terminal:term-1"]')
          ?.textContent,
      ).toBe(FULL),
    );
    expect(document.querySelector('[data-full-output="output"]')).toBeNull();
  });
});
