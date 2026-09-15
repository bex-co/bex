import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { MockLink } from "@apollo/client/testing";
import { Observable } from "rxjs";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import { GitConnectionsDocument, ReposDocument } from "@/graphql/definitions";
import {
  ServiceSourcePicker,
  type SourceTab,
} from "@/features/services/components/service-source-picker";
import type { RepoView } from "@/features/services/hooks/use-repos";

// w1/m153 t002: a background poll of the git connections query over cached
// data must not swap the GitHub tab for its skeleton. Before the fix it did, so
// the repo search unmounted mid-typing and lost focus every 30 s.
//
// The real hooks (useGitConnection, useGitConnections, useRepos) run against a
// real client built with the dashboard's apolloDefaultOptions, so the test pins
// the hooks' loading contract rather than a stub of it.

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const repos = [
  {
    __typename: "Repo",
    id: 1,
    fullName: "acme/api",
    private: true,
    defaultBranch: "main",
    htmlUrl: "https://github.com/acme/api",
    cloneUrl: "https://github.com/acme/api.git",
    accountLogin: "acme",
    installationId: 42,
  },
  {
    __typename: "Repo",
    id: 2,
    fullName: "puncsky/site",
    private: false,
    defaultBranch: "trunk",
    htmlUrl: "https://github.com/puncsky/site",
    cloneUrl: "https://github.com/puncsky/site.git",
    accountLogin: "puncsky",
    installationId: 43,
  },
];

const gitConnections = [
  {
    __typename: "GitConnection",
    connected: true,
    accountLogin: "acme",
    installationId: 42,
    createdAt: "2026-09-01T00:00:00Z",
    installUrl: "https://github.com/settings/installations/42",
  },
];

/**
 * A MockLink behind a gate: while held, requests are recorded but not answered,
 * so a refetch stays in flight for as long as the test needs to inspect it.
 */
function gatedClient() {
  const requests: string[] = [];
  let gate: Promise<void> | null = null;
  let open = () => {};
  const hold = new ApolloLink(
    (operation, forward) =>
      new Observable<ApolloLink.Result>((observer) => {
        requests.push(operation.operationName ?? "");
        let closed = false;
        let subscription: { unsubscribe(): void } | undefined;
        void (gate ?? Promise.resolve()).then(() => {
          if (!closed) subscription = forward(operation).subscribe(observer);
        });
        return () => {
          closed = true;
          subscription?.unsubscribe();
        };
      }),
  );
  const mocks = new MockLink(
    [
      {
        request: {
          query: GitConnectionsDocument,
          variables: { ownerId: "tea-1" },
        },
        result: { data: { gitConnections } },
        maxUsageCount: Number.POSITIVE_INFINITY,
      },
      {
        request: { query: ReposDocument, variables: { ownerId: "tea-1" } },
        result: { data: { repos } },
        maxUsageCount: Number.POSITIVE_INFINITY,
      },
    ],
    { defaultOptions: { delay: 0 } },
  );
  const client = new ApolloClient({
    cache: new InMemoryCache(),
    link: ApolloLink.from([hold, mocks]),
    defaultOptions: apolloDefaultOptions,
  });
  return {
    client,
    requests,
    holdResponses() {
      gate = new Promise((resolve) => {
        open = resolve;
      });
    },
    releaseResponses() {
      gate = null;
      open();
    },
  };
}

function Picker() {
  const [tab, setTab] = useState<SourceTab>("github");
  const [selectedRepo, setSelectedRepo] = useState<RepoView | null>(null);
  const [gitUrl, setGitUrl] = useState("");
  return (
    <ServiceSourcePicker
      tab={tab}
      onTabChange={setTab}
      selectedRepo={selectedRepo}
      onSelectRepo={setSelectedRepo}
      gitUrl={gitUrl}
      onGitUrlChange={setGitUrl}
    />
  );
}

const SEARCH = { name: "Search repositories…" } as const;

function skeletonCount() {
  return document.querySelectorAll('[data-slot="skeleton"]').length;
}

describe("ServiceSourcePicker across a git connections poll (w1/m153 t002)", () => {
  it("keeps the repo search mounted, focused and typed, and the repo list rendered, across a refetch over cached data", async () => {
    const apollo = gatedClient();
    const user = userEvent.setup();
    render(
      <ApolloProvider client={apollo.client}>
        <Picker />
      </ApolloProvider>,
    );

    const search = await screen.findByRole("textbox", SEARCH);
    expect(
      await screen.findByRole("button", { name: /acme\/api/ }),
    ).toBeInTheDocument();
    await user.type(search, "acme");
    expect(search).toHaveFocus();
    expect(
      screen.queryByRole("button", { name: /puncsky\/site/ }),
    ).not.toBeInTheDocument();
    const skeletonsBefore = skeletonCount();
    const connectionRequestsBefore = apollo.requests.filter(
      (name) => name === "GitConnections",
    ).length;

    // A poll tick: refetch the connections query while its data is cached,
    // and hold the response so the in-flight state can be inspected.
    apollo.holdResponses();
    let refresh: Promise<unknown> = Promise.resolve();
    await act(async () => {
      refresh = apollo.client.refetchQueries({
        include: [GitConnectionsDocument],
      });
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    const expectSearchIntact = () => {
      expect(screen.getByRole("textbox", SEARCH)).toBe(search);
      expect(search.isConnected).toBe(true);
      expect(search).toHaveFocus();
      expect(search).toHaveValue("acme");
      expect(
        screen.getByRole("button", { name: /acme\/api/ }),
      ).toBeInTheDocument();
      expect(skeletonCount()).toBe(skeletonsBefore);
    };

    // In flight: the refetch really went out, and nothing was swapped out.
    expect(
      apollo.requests.filter((name) => name === "GitConnections").length,
    ).toBeGreaterThan(connectionRequestsBefore);
    expectSearchIntact();

    await act(async () => {
      apollo.releaseResponses();
      await refresh;
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    // Settled: still the same focused input with the typed filter applied.
    expectSearchIntact();
    expect(
      screen.queryByRole("button", { name: /puncsky\/site/ }),
    ).not.toBeInTheDocument();
  });
});
