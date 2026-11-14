import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import {
  AgentSessionQueueDocument,
  DeleteQueuedAgentSessionMessageDocument,
  EditQueuedAgentSessionMessageDocument,
  InterruptAgentSessionDocument,
  MoveQueuedAgentSessionMessageDocument,
  SteerAgentSessionDocument,
  type AgentSessionQueuedMessageFieldsFragment,
} from "@/graphql/definitions";
import { QueuedMessageList } from "@/features/agent-sessions/components/queued-messages";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import { useAgentSessionQueue } from "@/features/agent-sessions/hooks/use-agent-session-queue";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

// The queue wired end to end against a fake bex-api: the real hooks, the real
// documents and a real Apollo cache, answering the verbs of ADR047 D12 from one
// in-memory queue. What it proves is the order of the requests and what the
// list shows after each answer.

vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => ({
      capabilities: {
        enabled: true,
        modelKeyReady: true,
        github: { connected: true },
        attachments: { enabled: false, maxBytes: 0, maxPerTurn: 0 },
      },
      loading: false,
      error: undefined,
    }),
  }),
);
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("@/features/agent-environment/hooks/use-agent-skills", () => ({
  useAgentSkills: () => ({
    skills: [],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

type Wire = AgentSessionQueuedMessageFieldsFragment;
const wire = (id: string, prompt: string): Wire => ({
  __typename: "AgentSessionQueuedMessage",
  id,
  sessionId: "ags-1",
  position: 0,
  prompt,
  author: "alice",
  attachmentIds: null,
  blockedReason: null,
  createdAt: "2026-11-14T00:00:00Z",
});

/** What the fake server holds and the requests it has answered, in order. */
const server = {
  queue: [] as Wire[],
  log: [] as string[],
  nextId: 4,
};

const wireSession = (phase: string) => ({
  __typename: "AgentSession" as const,
  id: "ags-1",
  ownerId: "tea-1",
  repo: "acme/widgets",
  branch: "bex-agent/fix",
  title: null,
  tags: null,
  maxSpendUSD: null,
  automationId: null,
  createdBy: null,
  parentSessionId: null,
  rootSessionId: null,
  depth: 0,
  secretNames: null,
  sandboxId: "sbx-1",
  sshAddress: null,
  phase,
  state: "running",
  steerable: false,
  status: "working",
  headSha: null,
  prUrl: null,
  prNumber: null,
  prState: null,
  turns: 2,
  deliveryMode: null,
  failureReason: null,
  failureReasonCode: null,
  deliveryNote: null,
  createdAt: "2026-11-14T00:00:00Z",
  updatedAt: "2026-11-14T00:00:00Z",
  canceledAt: null,
  archivedAt: null,
  agentConfig: {
    __typename: "AgentSessionConfig" as const,
    agent: "claude",
    model: null,
    modelEndpoint: null,
    task: "fix it",
    template: null,
  },
  outcome: null,
  ticket: null,
  url: null,
  streamUrl: null,
  expiresAt: null,
});

const numbered = () => server.queue.map((m, i) => ({ ...m, position: i + 1 }));

function fakeServer() {
  const forever = { maxUsageCount: Number.POSITIVE_INFINITY };
  const any = () => true;
  return new MockLink([
    {
      ...forever,
      request: { query: AgentSessionQueueDocument, variables: any },
      result: () => {
        server.log.push("read");
        return {
          data: {
            agentSession: {
              __typename: "AgentSession",
              id: "ags-1",
              pendingMessages: server.queue.length,
              queuedMessages: numbered(),
            },
          },
        };
      },
    },
    {
      ...forever,
      request: { query: InterruptAgentSessionDocument, variables: any },
      result: () => {
        server.log.push("interrupt");
        return { data: { interruptAgentSession: wireSession("running") } };
      },
    },
    {
      ...forever,
      request: { query: MoveQueuedAgentSessionMessageDocument, variables: any },
      result: (v: { messageId: string; position: number }) => {
        server.log.push(`move ${v.messageId} to ${v.position}`);
        const at = server.queue.findIndex((m) => m.id === v.messageId);
        const [moved] = server.queue.splice(at, 1);
        server.queue.splice(v.position - 1, 0, moved);
        return { data: { moveQueuedAgentSessionMessage: numbered() } };
      },
    },
    {
      ...forever,
      request: { query: EditQueuedAgentSessionMessageDocument, variables: any },
      result: (v: { messageId: string; prompt: string }) => {
        server.log.push(`edit ${v.messageId}`);
        const m = server.queue.find((q) => q.id === v.messageId)!;
        m.prompt = v.prompt;
        return { data: { editQueuedAgentSessionMessage: { ...m } } };
      },
    },
    {
      ...forever,
      request: {
        query: DeleteQueuedAgentSessionMessageDocument,
        variables: any,
      },
      result: (v: { messageId: string }) => {
        server.log.push(`delete ${v.messageId}`);
        server.queue = server.queue.filter((m) => m.id !== v.messageId);
        return { data: { deleteQueuedAgentSessionMessage: true } };
      },
    },
    {
      ...forever,
      request: { query: SteerAgentSessionDocument, variables: any },
      result: (v: { prompt: string; idempotencyKey?: string }) => {
        server.log.push(
          `steer ${v.prompt} key=${v.idempotencyKey ? "set" : "none"}`,
        );
        const queued = wire(`aqm-${server.nextId++}`, v.prompt);
        server.queue.push(queued);
        return {
          data: {
            steerAgentSession: {
              ...wireSession("running"),
              queuedMessage: { ...queued, position: server.queue.length },
            },
          },
        };
      },
    },
  ]);
}

function Page({ session }: { session: AgentSessionView }) {
  const messages = useAgentSessionQueue(session);
  return (
    <>
      <QueuedMessageList session={session} messages={messages} />
      <SteeringComposer
        session={session}
        chat={null}
        queuedCount={messages.length}
      />
    </>
  );
}

function renderPage() {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: fakeServer(),
  });
  return render(
    <ApolloProvider client={client}>
      <Page session={agentSessionView({ id: "ags-1", phase: "running" })} />
    </ApolloProvider>,
  );
}

const prompts = () =>
  screen
    .queryAllByTestId("queued-message")
    .map((li) => li.querySelector("p")?.textContent);

function row(id: string) {
  const el = document.querySelector(`[data-message-id="${id}"]`);
  if (!(el instanceof HTMLElement)) throw new Error(`no row ${id}`);
  return within(el);
}

beforeEach(() => {
  server.queue = [
    wire("aqm-1", "run the linter"),
    wire("aqm-2", "then the tests"),
    wire("aqm-3", "then open a PR"),
  ];
  server.log = [];
  server.nextId = 4;
});

describe("the queue against bex-api", () => {
  it("lists what the server holds, next first", async () => {
    renderPage();
    await waitFor(() =>
      expect(prompts()).toEqual([
        "run the linter",
        "then the tests",
        "then open a PR",
      ]),
    );
  });

  it("send now: stops the turn, then moves the message to the front, and the list follows", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("then open a PR");
    server.log = [];

    await user.click(row("aqm-3").getByRole("button", { name: "Send now" }));

    await waitFor(() =>
      expect(prompts()).toEqual([
        "then open a PR",
        "run the linter",
        "then the tests",
      ]),
    );
    // Stop turn first, then the move to place 1, then the re-read.
    const verbs = server.log.filter((entry) => entry !== "read");
    expect(verbs).toEqual(["interrupt", "move aqm-3 to 1"]);
    await waitFor(() => expect(server.log.at(-1)).toBe("read"));
  });

  it("send now on the first message only stops the turn", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("run the linter");
    server.log = [];
    await user.click(row("aqm-1").getByRole("button", { name: "Send now" }));
    await waitFor(() => expect(server.log).toContain("interrupt"));
    expect(server.log.some((entry) => entry.startsWith("move"))).toBe(false);
  });

  it("moves, edits and removes, each as its own request, the list following each", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("then the tests");

    await user.click(row("aqm-2").getByRole("button", { name: "Move up" }));
    await waitFor(() =>
      expect(prompts()).toEqual([
        "then the tests",
        "run the linter",
        "then open a PR",
      ]),
    );
    expect(server.log).toContain("move aqm-2 to 1");

    await user.click(row("aqm-1").getByRole("button", { name: "Move down" }));
    await waitFor(() =>
      expect(prompts()).toEqual([
        "then the tests",
        "then open a PR",
        "run the linter",
      ]),
    );
    expect(server.log).toContain("move aqm-1 to 3");

    await user.click(row("aqm-3").getByRole("button", { name: "Edit" }));
    await user.clear(
      screen.getByRole("textbox", { name: "Edit queued message" }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "Edit queued message" }),
      "then a PR with a summary{Enter}",
    );
    await waitFor(() =>
      expect(prompts()).toEqual([
        "then the tests",
        "then a PR with a summary",
        "run the linter",
      ]),
    );

    await user.click(row("aqm-2").getByRole("button", { name: "Remove" }));
    await waitFor(() =>
      expect(prompts()).toEqual(["then a PR with a summary", "run the linter"]),
    );
    expect(server.log).toEqual(
      expect.arrayContaining(["edit aqm-3", "delete aqm-2"]),
    );
  });

  it("a send while the turn runs becomes a chip at once, with a key for its retry", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByText("then open a PR");

    await user.type(
      screen.getByRole("textbox", { name: "" }),
      "and update the changelog",
    );
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(prompts()).toEqual([
        "run the linter",
        "then the tests",
        "then open a PR",
        "and update the changelog",
      ]),
    );
    expect(server.log).toContain("steer and update the changelog key=set");
    expect(
      screen.getByRole("heading", { name: "4 queued messages" }),
    ).toBeInTheDocument();
  });
});
