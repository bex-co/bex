import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { UIMessageChunk } from "ai";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import { makeFixtureFetch } from "@/features/agent-sessions/lib/mock-stream";

// The stored attachments the AgentAttachment query answers with, by id.
const stored = new Map<string, Record<string, unknown>>();
vi.mock("@apollo/client/react", () => ({
  useQuery: (_doc: unknown, options: { variables: { id: string } }) => {
    const row = stored.get(options.variables.id);
    return {
      data: row ? { agentAttachment: row } : undefined,
      loading: false,
      refetch: vi.fn(),
    };
  },
  useApolloClient: () => ({ query: vi.fn() }),
}));

beforeEach(() => stored.clear());

const mintTicket = async () => ({ ticket: "fixture-ticket" });

function tool(
  id: string,
  name: string,
  input: unknown,
  output: unknown,
): UIMessageChunk[] {
  return [
    { type: "tool-input-start", toolCallId: id, toolName: name, dynamic: true },
    {
      type: "tool-input-available",
      toolCallId: id,
      toolName: name,
      input,
      dynamic: true,
    },
    { type: "tool-output-available", toolCallId: id, output, dynamic: true },
  ];
}

// A share_file call as an ACP adapter relays it: MCP content blocks with the
// structured result as JSON text inside — here claiming a name of its own.
function shareCall(id: string, attachmentId: string): UIMessageChunk[] {
  return tool("s-" + id, "mcp__bex__share_file", { path: "/tmp/x" }, [
    {
      type: "text",
      text: JSON.stringify({
        attachmentId,
        name: "totally-safe.pdf",
        mediaType: "application/pdf",
        size: 1,
      }),
    },
  ]);
}

function storedOutput(id: string, overrides: Record<string, unknown> = {}) {
  stored.set(id, {
    id,
    sessionId: "as-share",
    direction: "output",
    name: "report.csv",
    mediaType: "text/csv",
    size: 2048,
    downloadUrl: "https://storage.example.test/x",
    ...overrides,
  });
}

async function renderStream(chunks: UIMessageChunk[]) {
  const transport = createAgentSessionTransport({
    sessionId: "as-share",
    mintTicket,
    fetch: makeFixtureFetch(
      [
        { type: "start", messageId: "asm-share" },
        ...chunks,
        { type: "finish" },
      ],
      { terminal: true },
    ),
  });
  const view = render(
    <SessionConversationImpl
      sessionId="as-share"
      isTerminal
      transport={transport}
    />,
  );
  await screen.findByText("Session ended.");
  return view;
}

describe("files the agent shares", () => {
  it("draws the card from the stored attachment, not the tool's words", async () => {
    storedOutput("att-abc123");
    await renderStream(shareCall("1", "att-abc123"));
    expect(screen.getByText("Shared a file")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Download report.csv" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/totally-safe/)).toBeNull();
  });

  it("shows nothing for an id that is not this session's output", async () => {
    storedOutput("att-input1", { direction: "input" });
    storedOutput("att-other1", { sessionId: "as-other" });
    await renderStream([
      ...shareCall("1", "att-input1"),
      ...shareCall("2", "att-other1"),
      ...shareCall("3", "att-unknown1"),
      ...tool(
        "e",
        "mcp__bex__share_file",
        {},
        { error: { code: "too_large", message: "x" } },
      ),
    ]);
    expect(screen.queryByText("Shared a file")).toBeNull();
  });

  it("keeps one activity group around a share and shows the file after it", async () => {
    storedOutput("att-abc123");
    const { container } = await renderStream([
      ...tool("a", "Run", { command: "make report" }, { ok: true }),
      ...shareCall("1", "att-abc123"),
      ...tool("b", "Run", { command: "ls" }, { ok: true }),
    ]);
    const groups = container.querySelectorAll('[data-block="activity"]');
    expect(groups).toHaveLength(1);
    const card = container.querySelector('[data-block="shared-files"]');
    expect(card).not.toBeNull();
    expect(
      groups[0].compareDocumentPosition(card!) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});
