import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ApolloClient, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import type { ReactNode } from "react";
import { AgentSessionTurnSkillsDocument } from "@/graphql/definitions";
import { TurnSkills } from "@/features/agent-sessions/components/turn-skills";
import { useAgentSessionTurnSkills } from "@/features/agent-sessions/hooks/use-agent-session-turn-skills";

describe("TurnSkills (ADR099 D4)", () => {
  it("renders nothing for a turn that invoked no skills", () => {
    const { container } = render(<TurnSkills skills={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows each invoked skill with its arguments", () => {
    render(
      <TurnSkills
        skills={[
          { name: "release", arguments: "v2.0" },
          { name: "run-tests", arguments: null },
        ]}
      />,
    );
    const list = screen.getByRole("list", { name: "Skills used in this turn" });
    expect(list).toHaveTextContent("release v2.0");
    expect(list).toHaveTextContent("run-tests");
  });
});

function Probe({ turn }: { turn: number | null }) {
  const skills = useAgentSessionTurnSkills("ags-1", turn);
  return <span data-testid="probe">{skills.map((s) => s.name).join(",")}</span>;
}

const mocks = [
  {
    request: {
      query: AgentSessionTurnSkillsDocument,
      variables: { id: "ags-1" },
    },
    result: {
      data: {
        agentSessionTranscript: {
          __typename: "AgentSessionTranscriptPage",
          turns: [
            {
              __typename: "AgentSessionTranscriptTurn",
              turn: 1,
              skills: [],
            },
            {
              __typename: "AgentSessionTranscriptTurn",
              turn: 2,
              skills: [
                {
                  __typename: "AgentSessionSkill",
                  name: "release",
                  arguments: "v2.0",
                },
              ],
            },
          ],
        },
      },
    },
  },
];

function withClient(children: ReactNode, withMocks = true) {
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: new MockLink(withMocks ? mocks : []),
  });
  return <ApolloProvider client={client}>{children}</ApolloProvider>;
}

describe("useAgentSessionTurnSkills", () => {
  it("reads the skills of the asked turn only", async () => {
    render(withClient(<Probe turn={2} />));
    await waitFor(() =>
      expect(screen.getByTestId("probe")).toHaveTextContent("release"),
    );
  });

  it("returns nothing for a prompt without a turn number", () => {
    render(withClient(<Probe turn={null} />, false));
    expect(screen.getByTestId("probe")).toBeEmptyDOMElement();
  });
});
