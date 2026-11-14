import { describe, expect, it } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import i18n from "@/i18n/init";
import {
  AgentSessionDocument,
  type AgentSessionQuery,
} from "@/graphql/definitions";
import { TurnOrigins } from "@/features/agent-sessions/components/turn-origins";
import {
  useAgentSessionTurnOrigins,
  type TurnOrigin,
} from "@/features/agent-sessions/hooks/use-agent-session-turn-origins";

const REVIEW_URL =
  "https://github.com/acme/widgets/pull/7#pullrequestreview-11";
const COMMENT_URL = "https://github.com/acme/widgets/pull/7#issuecomment-22";
const RUN_URL = "https://github.com/acme/widgets/actions/runs/33";

function lines(name = "What steered this turn") {
  const list = screen.getByRole("list", { name });
  return within(list).getAllByRole("listitem");
}

describe("TurnOrigins (w9/m344)", () => {
  it("names the reviewer and links the review in a new tab", () => {
    render(
      <TurnOrigins
        origins={[{ kind: "pr_review", author: "alice", url: REVIEW_URL }]}
      />,
    );
    const link = screen.getByRole("link", {
      name: "Steered by a review from @alice",
    });
    expect(link).toHaveAttribute("href", REVIEW_URL);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("names the commenter of a PR comment", () => {
    render(
      <TurnOrigins
        origins={[{ kind: "pr_comment", author: "bob", url: COMMENT_URL }]}
      />,
    );
    expect(
      screen.getByRole("link", { name: "Steered by a PR comment from @bob" }),
    ).toHaveAttribute("href", COMMENT_URL);
  });

  it("links the failed CI run", () => {
    render(
      <TurnOrigins
        origins={[{ kind: "ci_failure", author: null, url: RUN_URL }]}
      />,
    );
    expect(
      screen.getByRole("link", { name: "Steered by a failed CI run" }),
    ).toHaveAttribute("href", RUN_URL);
  });

  it("says when a failed check also fails on the base branch (w6/m399)", () => {
    render(
      <TurnOrigins
        origins={[
          {
            kind: "ci_failure",
            author: null,
            url: RUN_URL,
            baseFailingRef: "main",
          },
        ]}
      />,
    );
    expect(
      screen.getByRole("link", {
        name: "Steered by a failed CI run; some checks also fail on main, not because of this change",
      }),
    ).toHaveAttribute("href", RUN_URL);
  });

  it("drops the @ when GitHub sent no author", () => {
    render(
      <TurnOrigins
        origins={[
          { kind: "pr_review", author: null, url: REVIEW_URL },
          { kind: "pr_comment", author: "", url: COMMENT_URL },
        ]}
      />,
    );
    expect(lines().map((line) => line.textContent)).toEqual([
      "Steered by a PR review",
      "Steered by a PR comment",
    ]);
  });

  it("lists every origin a single steer delivered, one line each, in order", () => {
    render(
      <TurnOrigins
        origins={[
          { kind: "pr_comment", author: "bob", url: COMMENT_URL },
          { kind: "pr_review", author: "alice", url: REVIEW_URL },
          { kind: "ci_failure", author: null, url: RUN_URL },
        ]}
      />,
    );
    expect(lines().map((line) => line.textContent)).toEqual([
      "Steered by a PR comment from @bob",
      "Steered by a review from @alice",
      "Steered by a failed CI run",
    ]);
    expect(
      screen.getAllByRole("link").map((link) => link.getAttribute("href")),
    ).toEqual([COMMENT_URL, REVIEW_URL, RUN_URL]);
  });

  it("shows a non-http(s) or missing URL as plain text, never a link", () => {
    render(
      <TurnOrigins
        origins={[
          { kind: "pr_comment", author: "mallory", url: "javascript:alert(1)" },
          { kind: "pr_review", author: "eve", url: "//evil.example/x" },
          { kind: "ci_failure", author: null, url: null },
        ]}
      />,
    );
    expect(lines().map((line) => line.textContent)).toEqual([
      "Steered by a PR comment from @mallory",
      "Steered by a review from @eve",
      "Steered by a failed CI run",
    ]);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("renders nothing for a turn without origins or with only unknown kinds", () => {
    const { container, rerender } = render(<TurnOrigins origins={[]} />);
    expect(container).toBeEmptyDOMElement();
    rerender(
      <TurnOrigins
        origins={[{ kind: "deploy_hook", author: "x", url: RUN_URL }]}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("names the /bex comment that started the session on turn 1 (w9/m348)", () => {
    const { rerender } = render(
      <TurnOrigins
        origins={[
          { turn: 1, kind: "pr_comment", author: "bob", url: COMMENT_URL },
        ]}
      />,
    );
    const list = screen.getByRole("list", {
      name: "What started this session",
    });
    expect(
      within(list).getByRole("link", { name: "Started from @bob's comment" }),
    ).toHaveAttribute("href", COMMENT_URL);
    rerender(
      <TurnOrigins
        origins={[{ turn: 1, kind: "pr_comment", author: "", url: null }]}
      />,
    );
    expect(
      lines("What started this session").map((line) => line.textContent),
    ).toEqual(["Started from a PR comment"]);
    // A comment on a later turn is still a steer.
    rerender(
      <TurnOrigins
        origins={[
          { turn: 2, kind: "pr_comment", author: "bob", url: COMMENT_URL },
        ]}
      />,
    );
    expect(lines().map((line) => line.textContent)).toEqual([
      "Steered by a PR comment from @bob",
    ]);
  });

  it("words the starting comment in zh", async () => {
    await i18n.changeLanguage("zh");
    try {
      render(
        <TurnOrigins
          origins={[
            { turn: 1, kind: "pr_comment", author: "bob", url: COMMENT_URL },
          ]}
        />,
      );
      const list = screen.getByRole("list", { name: "发起本会话的来源" });
      expect(
        within(list)
          .getAllByRole("listitem")
          .map((line) => line.textContent),
      ).toEqual(["由 @bob 的评论发起"]);
    } finally {
      await i18n.changeLanguage("en");
    }
  });

  it("words every kind in zh", async () => {
    await i18n.changeLanguage("zh");
    try {
      render(
        <TurnOrigins
          origins={[
            { kind: "pr_review", author: "alice", url: REVIEW_URL },
            { kind: "pr_comment", author: "bob", url: COMMENT_URL },
            { kind: "ci_failure", author: null, url: RUN_URL },
          ]}
        />,
      );
      const list = screen.getByRole("list", { name: "引导本轮的来源" });
      expect(
        within(list)
          .getAllByRole("listitem")
          .map((line) => line.textContent),
      ).toEqual([
        "由 @alice 的评审引导",
        "由 @bob 的 PR 评论引导",
        "由一次失败的 CI 运行引导",
      ]);
    } finally {
      await i18n.changeLanguage("en");
    }
  });
});

// The detail page's own read, as the page holds it in Apollo's cache.
function detail(
  turnOrigins: NonNullable<AgentSessionQuery["agentSession"]>["turnOrigins"],
): AgentSessionQuery {
  return {
    agentSession: {
      __typename: "AgentSession",
      id: "ags-1",
      ownerId: "tea-1",
      repo: "acme/widgets",
      branch: "bex-agent/fix",
      title: null,
      tags: null,
      maxSpendUSD: null,
      automationId: null,
      environmentBuildId: null,
      createdBy: null,
      parentSessionId: null,
      rootSessionId: null,
      depth: 0,
      secretNames: null,
      sandboxId: null,
      sshAddress: null,
      phase: "running",
      state: "running",
      steerable: false,
      status: "working",
      headSha: null,
      prUrl: "https://github.com/acme/widgets/pull/7",
      prNumber: 7,
      prState: "open",
      turns: 3,
      deliveryMode: null,
      failureReason: null,
      failureReasonCode: null,
      deliveryNote: null,
      createdAt: "2026-11-12T00:00:00Z",
      updatedAt: "2026-11-12T00:10:00Z",
      canceledAt: null,
      archivedAt: null,
      turnOrigins,
      appliedSecrets: [],
      agentConfig: {
        __typename: "AgentSessionConfig",
        agent: "claude",
        model: null,
        modelEndpoint: null,
        task: "fix it",
        template: null,
      },
      outcome: null,
    },
  };
}

function origin(turn: number, kind: string, author: string | null) {
  return {
    __typename: "AgentSessionTurnOrigin" as const,
    turn,
    kind,
    author,
    url: RUN_URL,
    baseFailingRef: null,
  };
}

function Probe({ turn }: { turn: number | null }) {
  const origins: TurnOrigin[] = useAgentSessionTurnOrigins("ags-1", turn);
  return (
    <span data-testid="probe">
      {origins.map((o) => `${o.kind}:${o.author ?? ""}`).join(",")}
    </span>
  );
}

function renderProbe(turn: number | null, data: AgentSessionQuery) {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    // No mocks: a turn reads the page's cached session, never the network.
    link: new MockLink([]),
  });
  client.writeQuery({
    query: AgentSessionDocument,
    variables: { id: "ags-1" },
    data,
  });
  render(
    <ApolloProvider client={client}>
      <Probe turn={turn} />
    </ApolloProvider>,
  );
  return client;
}

describe("useAgentSessionTurnOrigins", () => {
  it("reads the origins of the asked turn only", () => {
    renderProbe(
      2,
      detail([
        origin(2, "pr_comment", "bob"),
        origin(2, "ci_failure", null),
        origin(3, "pr_review", "alice"),
      ]),
    );
    expect(screen.getByTestId("probe")).toHaveTextContent(
      "pr_comment:bob,ci_failure:",
    );
  });

  it("returns nothing for a turn PR feedback did not start", () => {
    renderProbe(1, detail([origin(2, "pr_comment", "bob")]));
    expect(screen.getByTestId("probe")).toBeEmptyDOMElement();
  });

  it("returns nothing for a prompt without a turn number", () => {
    renderProbe(null, detail([origin(2, "pr_comment", "bob")]));
    expect(screen.getByTestId("probe")).toBeEmptyDOMElement();
  });

  it("picks up a just-delivered steer's origin when the page's read refreshes", async () => {
    const client = renderProbe(3, detail([origin(2, "pr_comment", "bob")]));
    expect(screen.getByTestId("probe")).toBeEmptyDOMElement();
    await act(async () => {
      client.writeQuery({
        query: AgentSessionDocument,
        variables: { id: "ags-1" },
        data: detail([
          origin(2, "pr_comment", "bob"),
          origin(3, "pr_review", "alice"),
        ]),
      });
    });
    expect(screen.getByTestId("probe")).toHaveTextContent("pr_review:alice");
  });
});
