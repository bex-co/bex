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
function gatedClient(connections: unknown[] = gitConnections) {
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
        result: { data: { gitConnections: connections } },
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

function Picker({ currentRepo = null }: { currentRepo?: string | null }) {
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
      currentRepo={currentRepo}
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

// w1/m526 (DoD 3): an account connected in another tab — the connect flow's
// new tab — must reach the picker's repo list without a reload. Repos used to
// refetch only on window focus, which a new-tab return does not always raise;
// now they follow the polled connection set.
describe("ServiceSourcePicker repos follow the connection set (w1/m526)", () => {
  it("refetches the repos when a poll finds a new installation, with no focus event", async () => {
    let connections = gitConnections;
    const requests: string[] = [];
    const link = new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          const name = operation.operationName ?? "";
          requests.push(name);
          observer.next({
            data:
              name === "GitConnections"
                ? { gitConnections: connections }
                : { repos },
          });
          observer.complete();
        }),
    );
    const client = new ApolloClient({
      cache: new InMemoryCache(),
      link,
      defaultOptions: apolloDefaultOptions,
    });
    render(
      <ApolloProvider client={client}>
        <Picker />
      </ApolloProvider>,
    );
    await screen.findByRole("button", { name: /acme\/api/ });
    const reposRequests = () =>
      requests.filter((name) => name === "Repos").length;
    const poll = async () => {
      await act(async () => {
        await client.refetchQueries({ include: [GitConnectionsDocument] });
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
    };

    // A poll that finds the same set leaves the repos alone.
    const before = reposRequests();
    await poll();
    expect(reposRequests()).toBe(before);

    // The other tab connected puncsky: the next poll brings its installation.
    connections = [
      ...gitConnections,
      {
        __typename: "GitConnection",
        connected: true,
        accountLogin: "puncsky",
        installationId: 43,
        createdAt: "2026-11-19T00:00:00Z",
        installUrl: "https://github.com/settings/installations/43",
      },
    ];
    await poll();
    expect(reposRequests()).toBe(before + 1);
  });
});

// w4/434: while the connections read is first in flight, the GitHub tab
// previews the connect prompt a workspace without GitHub then sees, rather
// than a block ~95 px shorter that made the whole form jump.
describe("ServiceSourcePicker first connections read (w4/434)", () => {
  it("previews the connect prompt's anatomy until the read settles", async () => {
    const apollo = gatedClient([]);
    apollo.holdResponses();
    render(
      <ApolloProvider client={apollo.client}>
        <Picker />
      </ApolloProvider>,
    );

    const preview = document.querySelector('[data-preview="git-connect"]');
    expect(preview).not.toBeNull();
    expect(preview).toHaveAttribute("inert");
    expect(preview).toHaveAttribute("aria-busy", "true");
    const [icon, text, button] = [...preview!.children];
    expect(preview!.children).toHaveLength(3);
    expect(icon).toHaveClass("size-8");
    expect(text.children).toHaveLength(3);
    expect(text.children[2]).toHaveClass("sm:hidden");
    expect(button).toHaveClass("h-11", "sm:h-9");

    await act(async () => {
      apollo.releaseResponses();
    });
    const connect = await screen.findByRole("button", {
      name: /Connect GitHub/,
    });
    const prompt = connect.parentElement!;
    expect(prompt.className).toBe(preview!.className);
    expect(prompt.children).toHaveLength(3);
    expect(document.querySelector('[data-preview="git-connect"]')).toBeNull();
  });
});

// The GitHub tab's repository list. Each state says what it is: a list that
// failed used to read "No repositories found.", the same words as a workspace
// with none, and neither said what to do next.
describe("ServiceSourcePicker repository list states", () => {
  /** A client whose Repos answer the test scripts, call by call. */
  function scriptedClient(
    reposAnswer: () => ApolloLink.Result,
    connections: unknown[] = gitConnections,
  ) {
    const link = new ApolloLink(
      (operation) =>
        new Observable<ApolloLink.Result>((observer) => {
          observer.next(
            operation.operationName === "GitConnections"
              ? { data: { gitConnections: connections } }
              : reposAnswer(),
          );
          observer.complete();
        }),
    );
    return new ApolloClient({
      cache: new InMemoryCache(),
      link,
      defaultOptions: apolloDefaultOptions,
    });
  }
  const renderPicker = (client: ApolloClient, currentRepo?: string) =>
    render(
      <ApolloProvider client={client}>
        <Picker currentRepo={currentRepo} />
      </ApolloProvider>,
    );

  it("says the list failed and offers another try, not that there are none", async () => {
    let failing = true;
    const user = userEvent.setup();
    renderPicker(
      scriptedClient(() =>
        failing
          ? { errors: [{ message: "upstream unavailable" }] }
          : { data: { repos } },
      ),
    );

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load your repositories.");
    expect(screen.queryByText("No repositories found.")).toBeNull();
    // The search and the accounts menu stay in reach.
    expect(screen.getByRole("textbox", SEARCH)).toBeInTheDocument();

    failing = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("button", { name: /acme\/api/ }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("tells a workspace whose accounts grant no repository what to do, and opens the way there", async () => {
    const user = userEvent.setup();
    renderPicker(scriptedClient(() => ({ data: { repos: [] } })));
    expect(
      await screen.findByText("No repositories found."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Give bex access to a repository on GitHub/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();

    // The accounts menu beside the search field, opened from here.
    expect(screen.queryByRole("dialog", { name: "Credentials" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Open Credentials" }));
    expect(
      await screen.findByRole("dialog", { name: "Credentials" }),
    ).toBeInTheDocument();
  });

  it("offers to clear a search that matches nothing", async () => {
    const user = userEvent.setup();
    renderPicker(scriptedClient(() => ({ data: { repos } })));
    await screen.findByRole("button", { name: /acme\/api/ });
    const search = screen.getByRole("textbox", SEARCH);
    await user.type(search, "zzzz");
    expect(
      screen.getByText("No repositories match your search."),
    ).toBeInTheDocument();
    // The usual cause is a repository the accounts do not grant: the way to
    // grant it is offered beside the way back.
    expect(
      screen.getByText(/Not listed\? Give bex access/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Open Credentials" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear search" }));
    expect(search).toHaveValue("");
    expect(
      screen.getByRole("button", { name: /puncsky\/site/ }),
    ).toBeInTheDocument();
  });

  // A phone shows a lock for a private repository where wider screens show
  // the word, and the row's parts sit side by side with no space between
  // them in the markup. Its name says each part, set apart.
  it("names a row by its repository, Private when it is, and its default branch", async () => {
    renderPicker(scriptedClient(() => ({ data: { repos } })));
    const row = await screen.findByRole("button", { name: /acme\/api/ });
    expect(row).toHaveAccessibleName("acme/api Private main");
    expect(
      screen.getByRole("button", { name: /puncsky\/site/ }),
    ).toHaveAccessibleName("puncsky/site trunk");
  });

  // Update Source marks the service's repository as chosen. Once another is
  // chosen that mark moves, and nothing said which one the service uses now.
  it("tags the repository in use once another is chosen", async () => {
    const user = userEvent.setup();
    renderPicker(
      scriptedClient(() => ({ data: { repos } })),
      "https://github.com/acme/api",
    );
    const inUse = await screen.findByRole("button", { name: /acme\/api/ });
    const other = screen.getByRole("button", { name: /puncsky\/site/ });
    expect(inUse).toHaveAttribute("aria-pressed", "true");
    expect(inUse).toHaveAccessibleName("acme/api Private main");

    await user.click(other);
    expect(other).toHaveAttribute("aria-pressed", "true");
    expect(inUse).toHaveAttribute("aria-pressed", "false");
    expect(inUse).toHaveAccessibleName("acme/api Private Current main");
    expect(other).toHaveAccessibleName("puncsky/site trunk");
  });

  it("tags no repository where no service uses one yet", async () => {
    const user = userEvent.setup();
    renderPicker(scriptedClient(() => ({ data: { repos } })));
    await user.click(
      await screen.findByRole("button", { name: /puncsky\/site/ }),
    );
    expect(screen.queryByText("Current")).not.toBeInTheDocument();
  });

  // The repository in use can sit far down a long list. The dialog opens
  // scaled, so a row's bounding box read while it opens is short of where the
  // row will be; the list then rested between two rows. Layout offsets do
  // not scale.
  it("rests the list one row above the repository in use, read from layout offsets", async () => {
    const many = Array.from({ length: 30 }, (_, i) => ({
      ...repos[0],
      id: 100 + i,
      fullName: `acme/r${i}`,
      htmlUrl: `https://github.com/acme/r${i}`,
      cloneUrl: `https://github.com/acme/r${i}.git`,
    }));
    const rowIndex = (row: Element) =>
      [...document.querySelectorAll("[data-repo-row]")].indexOf(row);
    const spies = [
      vi
        .spyOn(HTMLElement.prototype, "offsetTop", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]") ? rowIndex(this) * 44 : 0;
        }),
      vi
        .spyOn(HTMLElement.prototype, "offsetHeight", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]") ? 44 : 0;
        }),
      vi
        .spyOn(Element.prototype, "clientHeight", "get")
        .mockImplementation(function (this: Element) {
          return this.matches("[data-repo-list]") ? 220 : 0;
        }),
      // What a box read mid-animation looks like: every row at the top.
      vi
        .spyOn(Element.prototype, "getBoundingClientRect")
        .mockReturnValue(new DOMRect(0, 0, 0, 0)),
    ];
    try {
      renderPicker(
        scriptedClient(() => ({ data: { repos: many } })),
        "https://github.com/acme/r12",
      );
      const inUse = await screen.findByRole("button", { name: /acme\/r12 / });
      const list = inUse.closest<HTMLElement>("[data-repo-list]")!;
      // Row 12 starts at 528 px; one 44 px row stays above it.
      expect(list.scrollTop).toBe(484);
    } finally {
      for (const spy of spies) spy.mockRestore();
    }
  });

  // A row already in view is left where it is: choosing a repository must
  // not move the list under the pointer.
  it("leaves the list where it is when the chosen row is in view", async () => {
    const spies = [
      vi
        .spyOn(HTMLElement.prototype, "offsetTop", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]")
            ? [...document.querySelectorAll("[data-repo-row]")].indexOf(this) *
                44
            : 0;
        }),
      vi
        .spyOn(HTMLElement.prototype, "offsetHeight", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]") ? 44 : 0;
        }),
      vi
        .spyOn(Element.prototype, "clientHeight", "get")
        .mockImplementation(function (this: Element) {
          return this.matches("[data-repo-list]") ? 220 : 0;
        }),
    ];
    try {
      const user = userEvent.setup();
      renderPicker(scriptedClient(() => ({ data: { repos } })));
      const second = await screen.findByRole("button", {
        name: /puncsky\/site/,
      });
      const list = second.closest<HTMLElement>("[data-repo-list]")!;
      await user.click(second);
      expect(second).toHaveAttribute("aria-pressed", "true");
      expect(list.scrollTop).toBe(0);
    } finally {
      for (const spy of spies) spy.mockRestore();
    }
  });

  // From `sm` the list ends on half a row to say it goes on. Choosing that
  // half row placed it as a mounting list places the repository in use, one
  // row from the top: the list moved four rows under the pointer.
  it("brings a chosen row that is partly out of view just into view", async () => {
    const many = Array.from({ length: 30 }, (_, i) => ({
      ...repos[0],
      id: 100 + i,
      fullName: `acme/r${i}`,
      htmlUrl: `https://github.com/acme/r${i}`,
      cloneUrl: `https://github.com/acme/r${i}.git`,
    }));
    const rowIndex = (row: Element) =>
      [...document.querySelectorAll("[data-repo-row]")].indexOf(row);
    const spies = [
      vi
        .spyOn(HTMLElement.prototype, "offsetTop", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]") ? rowIndex(this) * 44 : 0;
        }),
      vi
        .spyOn(HTMLElement.prototype, "offsetHeight", "get")
        .mockImplementation(function (this: HTMLElement) {
          return this.matches("[data-repo-row]") ? 44 : 0;
        }),
      vi
        .spyOn(Element.prototype, "clientHeight", "get")
        .mockImplementation(function (this: Element) {
          return this.matches("[data-repo-list]") ? 240 : 0;
        }),
    ];
    try {
      const user = userEvent.setup();
      renderPicker(scriptedClient(() => ({ data: { repos: many } })));
      // Row 5 spans 220 to 264 px of a 240 px list: 20 px of it shows.
      const cut = await screen.findByRole("button", { name: /acme\/r5 / });
      const list = cut.closest<HTMLElement>("[data-repo-list]")!;
      await user.click(cut);
      expect(cut).toHaveAttribute("aria-pressed", "true");
      expect(list.scrollTop).toBe(24);

      // And one cut by the top edge comes down to it.
      list.scrollTop = 100;
      const above = screen.getByRole("button", { name: /acme\/r2 / });
      await user.click(above);
      expect(list.scrollTop).toBe(88);
    } finally {
      for (const spy of spies) spy.mockRestore();
    }
  });

  // Beside the accounts button a phone's search field is narrow: the full
  // placeholder was cut mid-word. The field's name stays the full one.
  it("shortens the search placeholder on a phone and keeps the field's name", async () => {
    const width = window.innerWidth;
    window.innerWidth = 390;
    try {
      renderPicker(scriptedClient(() => ({ data: { repos } })));
      const search = await screen.findByRole("textbox", SEARCH);
      expect(search).toHaveAttribute("placeholder", "Search…");
    } finally {
      window.innerWidth = width;
    }
  });

  // A workspace lists hundreds of repositories; each row as its own tab stop
  // put them all between the search field and the fields after the list.
  it("is one tab stop, moved through with the arrow keys", async () => {
    const user = userEvent.setup();
    renderPicker(scriptedClient(() => ({ data: { repos } })));
    const first = await screen.findByRole("button", { name: /acme\/api/ });
    const second = screen.getByRole("button", { name: /puncsky\/site/ });
    expect(first).toHaveAttribute("tabindex", "0");
    expect(second).toHaveAttribute("tabindex", "-1");
    expect(
      screen.getByRole("group", { name: "Repositories" }),
    ).toContainElement(first);

    first.focus();
    await user.keyboard("{ArrowDown}");
    expect(second).toHaveFocus();
    // The tab stop follows the focus, so Shift+Tab then Tab returns here.
    expect(second).toHaveAttribute("tabindex", "0");
    expect(first).toHaveAttribute("tabindex", "-1");
    await user.keyboard("{ArrowDown}");
    expect(second).toHaveFocus();
    await user.keyboard("{Home}");
    expect(first).toHaveFocus();
    await user.keyboard("{End}");
    expect(second).toHaveFocus();
  });

  // Update Source knows the workspace is connected before the connection read
  // answers: the tab takes the list's shape at once, not the connect prompt's.
  it("takes the list's shape while the connection read is out for a repository in use", async () => {
    const apollo = gatedClient();
    apollo.holdResponses();
    render(
      <ApolloProvider client={apollo.client}>
        <ServiceSourcePicker
          tab="github"
          onTabChange={() => {}}
          selectedRepo={null}
          onSelectRepo={() => {}}
          currentRepo="https://github.com/acme/api.git"
          gitUrl=""
          onGitUrlChange={() => {}}
        />
      </ApolloProvider>,
    );

    expect(document.querySelector('[data-preview="git-connect"]')).toBeNull();
    expect(screen.getByRole("textbox", SEARCH)).toBeInTheDocument();
    const pending = document.querySelector('[aria-busy="true"]');
    expect(pending).not.toBeNull();
    expect(pending).toHaveAttribute("inert");
    // The loaded list sits in a box of the same height, with the repository
    // in use marked.
    expect(pending).toHaveClass("h-52", "sm:h-[17rem]", "rounded-md", "border");
    await act(async () => {
      apollo.releaseResponses();
    });
    const row = await screen.findByRole("button", { name: /acme\/api/ });
    expect(row).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("group", { name: "Repositories" })).toHaveClass(
      "h-52",
      "sm:h-[17rem]",
      "rounded-md",
      "border",
    );
  });

  it("shows the connect prompt once the read says disconnected, whatever is cached", async () => {
    renderPicker(scriptedClient(() => ({ data: { repos } }), []));
    expect(
      await screen.findByRole("button", { name: /Connect GitHub/ }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /acme\/api/ })).toBeNull();
  });
});

// w21/052: the Public Git URL field is marked and announced like the image
// reference field, so a screen-reader user who pastes `github.com/owner/repo`
// (no scheme) hears that it is invalid and why.
describe("ServiceSourcePicker Public Git URL (w21/052)", () => {
  function GitUrlPicker() {
    const [gitUrl, setGitUrl] = useState("");
    return (
      <ServiceSourcePicker
        tab="git"
        onTabChange={() => {}}
        selectedRepo={null}
        onSelectRepo={() => {}}
        gitUrl={gitUrl}
        onGitUrlChange={setGitUrl}
      />
    );
  }

  const renderPicker = () =>
    render(
      <ApolloProvider client={gatedClient().client}>
        <GitUrlPicker />
      </ApolloProvider>,
    );

  it.each(["github.com/bex-co/bex", "http://github.com/bex-co/bex"])(
    "marks %s invalid and describes it with an alert",
    async (typed) => {
      const user = userEvent.setup();
      renderPicker();
      const input = await screen.findByRole("textbox", {
        name: "Repository URL",
      });
      await user.type(input, typed);

      expect(input).toHaveAttribute("aria-invalid", "true");
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/Enter an https:\/\/ URL/);
      expect(input).toHaveAttribute("aria-describedby", alert.id);
      expect(alert.id).not.toBe("");
      expect(input).toHaveAccessibleDescription(/Enter an https:\/\/ URL/);
    },
  );

  it.each([
    "https://github.com/bex-co/bex",
    "https://github.com/bex-co/bex.git",
    "https://github.com/bex-co/bex/",
    "git@github.com:bex-co/bex.git",
  ])("clears the marks for %s", async (typed) => {
    const user = userEvent.setup();
    renderPicker();
    const input = await screen.findByRole("textbox", {
      name: "Repository URL",
    });
    await user.type(input, "github.com/bex-co/bex");
    expect(input).toHaveAttribute("aria-invalid", "true");
    await user.clear(input);
    await user.type(input, typed);

    expect(input).not.toHaveAttribute("aria-invalid", "true");
    expect(input).not.toHaveAttribute("aria-describedby");
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
