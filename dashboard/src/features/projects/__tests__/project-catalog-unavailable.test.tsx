import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApolloClient, ApolloLink, InMemoryCache } from "@apollo/client";
import { ApolloProvider } from "@apollo/client/react";
import { Observable } from "rxjs";
import { apolloCacheConfig } from "@/common/apollo/cache";
import { apolloDefaultOptions } from "@/common/apollo/default-options";
import {
  EnvironmentsDocument,
  KeyValuesDocument,
  ServicesDocument,
} from "@/graphql/definitions";
import type { ProjectResourceFilterState } from "@/features/projects/lib/resource-filter";
import { blankSelection } from "@/test/mocks/blank-selection";
import { mockWorkspaceContext } from "@/test/mocks/workspace";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import { useProjectResourceCatalog } from "@/features/projects/hooks/use-project-resource-catalog";

/**
 * w4/m227 (a883): the Project Overview dropped the resource-list errors, so a
 * failed Services/Key Values read joined against real membership drew "0
 * resources" and the empty invitation. Real Apollo with the app's defaults,
 * the real list hooks, catalog, panel and card; only the network is faked.
 * The move dialog and the row table are stubbed — they are not part of this
 * read — and any operation other than the five list reads is recorded.
 */
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="#">{children}</a>,
}));
vi.mock("@/features/workspaces/context/hooks", () =>
  mockWorkspaceContext("tea-1"),
);
// w4/m238: the stranded-link guard is inert here so the query accounting
// stays about the catalog reads.
vi.mock("@/features/env-groups/hooks/use-stranded-link-guard", () => ({
  useStrandedLinkGuard: () => ({
    groups: [],
    ready: true,
    loading: false,
    error: undefined,
    retry: () => undefined,
    strandedFor: () => [],
    scopeNameFor: () => undefined,
  }),
}));

vi.mock("@/features/environments/components/manage-resources-dialog", () => ({
  ManageResourcesDialog: () => null,
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  resourceSelectionKey: (row: { kind: string; id: string }) =>
    `${row.kind}:${row.id}`,
  ResourceTable: ({ rows }: { rows: Array<{ id: string; name: string }> }) => (
    <ul aria-label="resources">
      {rows.map((row) => (
        <li key={row.id}>{row.name}</li>
      ))}
    </ul>
  ),
}));

/** "held-error": the read waits for `release()`, then fails. */
type Reply = "ok" | "error" | "empty" | "held-error";
type List = "Services" | "Databases" | "KeyValues" | "EnvGroups";

function fakeApi(
  scripts: Partial<Record<List, Reply[]>>,
  { environments = "blue" as "blue" | "none" } = {},
) {
  const reads: Record<List, number> = {
    Services: 0,
    Databases: 0,
    KeyValues: 0,
    EnvGroups: 0,
  };
  const others: string[] = [];
  const held: Array<() => void> = [];
  const item = (
    doc: Parameters<typeof blankSelection>[0],
    field: string,
    fields: Record<string, unknown>,
  ) => ({ ...blankSelection(doc, field), ...fields });
  const rows: Record<List, () => unknown[]> = {
    Services: () => [
      item(ServicesDocument, "services", {
        __typename: "Service",
        id: "srv-1",
        name: "storefront",
      }),
    ],
    KeyValues: () => [
      item(KeyValuesDocument, "keyValues", {
        __typename: "KeyValue",
        id: "red-1",
        name: "sessions",
      }),
    ],
    Databases: () => [],
    EnvGroups: () => [],
  };
  const field: Record<List, string> = {
    Services: "services",
    Databases: "databases",
    KeyValues: "keyValues",
    EnvGroups: "envGroups",
  };
  const link = new ApolloLink(
    (op) =>
      new Observable((sub) => {
        const reply = (result: Record<string, unknown>) =>
          setTimeout(() => {
            sub.next(result);
            sub.complete();
          });
        const name = op.operationName ?? "";
        if (name === "Environments") {
          reply({
            data: {
              environments:
                environments === "none"
                  ? []
                  : [
                      item(EnvironmentsDocument, "environments", {
                        __typename: "Environment",
                        id: "evm-blue",
                        projectId: "prj-1",
                        name: "Blue",
                        serviceIds: ["srv-1"],
                        databaseIds: [],
                        keyValueIds: ["red-1"],
                        envGroupIds: [],
                        ipAllowList: [],
                        ipAllowListEntries: [],
                        ipAllowListProxiedDomains: [],
                      }),
                    ],
            },
          });
          return;
        }
        if (!(name in reads)) {
          others.push(name);
          sub.error(new Error(`unexpected ${name}`));
          return;
        }
        const list = name as List;
        const script = scripts[list] ?? ["ok"];
        const outcome = script[Math.min(reads[list], script.length - 1)];
        reads[list] += 1;
        const failure = {
          data: null,
          errors: [
            { message: "deadline", extensions: { code: "QUERY_TIMEOUT" } },
          ],
        };
        if (outcome === "held-error") held.push(() => reply(failure));
        else if (outcome === "error") reply(failure);
        else
          reply({
            data: { [field[list]]: outcome === "empty" ? [] : rows[list]() },
          });
      }),
  );
  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(apolloCacheConfig),
    defaultOptions: apolloDefaultOptions,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ApolloProvider client={client}>{children}</ApolloProvider>
  );
  return {
    client,
    wrapper,
    reads,
    others,
    release: () => held.splice(0).forEach((send) => send()),
  };
}

// The Project Overview's resource wiring around the real panel and card.
function ProjectOverview({
  environmentId = "evm-blue" as string | null,
  onFilterChange = (() => undefined) as (f: ProjectResourceFilterState) => void,
}) {
  const { services, databases, keyValues, catalog } =
    useProjectResourceCatalog();
  return (
    <EnvironmentsPanel
      projectId="prj-1"
      services={services}
      databases={databases}
      keyValues={keyValues}
      projectRows={[]}
      catalog={catalog}
      servicePending={null}
      onRunServiceAction={vi.fn()}
      onDatabaseDeleted={vi.fn()}
      onKeyValueDeleted={vi.fn()}
      resourceFilter={{ environmentId, query: "", kind: "all" }}
      onResourceFilterChange={onFilterChange}
    />
  );
}

const typeButton = (name: RegExp) =>
  within(screen.getByRole("group", { name: "Resource type" })).getByRole(
    "button",
    { name },
  );

describe("Project Overview resource availability (w4/m227)", () => {
  it("explains failed lists instead of an empty environment, and recovers", async () => {
    const api = fakeApi({
      Services: ["error", "ok"],
      KeyValues: ["error", "ok"],
    });
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<ProjectOverview />, { wrapper: api.wrapper });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/couldn't load/);
    expect(screen.getByText("Count unavailable")).toBeInTheDocument();
    expect(screen.queryByText(/0 resources/)).toBeNull();
    expect(
      screen.queryByText(/No resources in this environment yet/),
    ).toBeNull();
    expect(typeButton(/^All/)).toHaveTextContent("All (—)");
    expect(typeButton(/^Services/)).toHaveTextContent("(—)");

    await user.click(within(alert).getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(screen.getByRole("list", { name: "resources" })).toHaveTextContent(
        /storefront.*sessions/,
      ),
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(typeButton(/^All/)).toHaveTextContent("All (2)");
    expect(typeButton(/^Services/)).toHaveTextContent("(1)");
    expect(typeButton(/^Key Values/)).toHaveTextContent("(1)");
    // Retry re-read the two failed lists once each; nothing else was sent.
    expect(api.reads).toMatchObject({ Services: 2, KeyValues: 2 });
    expect(api.others).toEqual([]);
  });

  it("is busy while retrying and stays unavailable when the retry fails", async () => {
    const api = fakeApi({ Services: ["error", "held-error"] });
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    render(<ProjectOverview />, { wrapper: api.wrapper });
    const alert = await screen.findByRole("alert");
    await user.click(within(alert).getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(
        within(screen.getByRole("alert")).getByRole("button", {
          name: "Try again",
        }),
      ).toBeDisabled(),
    );
    await act(async () => api.release());
    await waitFor(() =>
      expect(
        within(screen.getByRole("alert")).getByRole("button", {
          name: "Try again",
        }),
      ).toBeEnabled(),
    );
    expect(typeButton(/^Services/)).toHaveTextContent("(—)");
    expect(api.reads.Services).toBe(2);
  });

  it("keeps the last rows through a failed refresh, marked possibly out of date", async () => {
    const api = fakeApi({ Services: ["ok", "error"] });
    render(<ProjectOverview />, { wrapper: api.wrapper });
    await waitFor(() =>
      expect(screen.getByRole("list", { name: "resources" })).toHaveTextContent(
        /storefront/,
      ),
    );
    await act(async () => {
      await api.client
        .refetchQueries({ include: [ServicesDocument] })
        .catch(() => []);
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(/out of date/);
    expect(screen.getByRole("list", { name: "resources" })).toHaveTextContent(
      /storefront/,
    );
  });

  it("names only the failed family and keeps the healthy rows", async () => {
    const api = fakeApi({ KeyValues: ["error"] });
    render(<ProjectOverview />, { wrapper: api.wrapper });
    await screen.findByRole("alert");
    expect(screen.getByRole("list", { name: "resources" })).toHaveTextContent(
      "storefront",
    );
    expect(typeButton(/^Services/)).toHaveTextContent("(1)");
    expect(typeButton(/^Key Values/)).toHaveTextContent("(—)");
  });

  it("ignores a family this environment has no members of", async () => {
    const api = fakeApi({ Databases: ["error"], EnvGroups: ["error"] });
    render(<ProjectOverview />, { wrapper: api.wrapper });
    await waitFor(() =>
      expect(typeButton(/^All/)).toHaveTextContent("All (2)"),
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the ordinary empty view for a successful empty read", async () => {
    const api = fakeApi({ Services: ["empty"], KeyValues: ["empty"] });
    render(<ProjectOverview />, { wrapper: api.wrapper });
    expect(
      await screen.findByText(/No resources in this environment yet/),
    ).toBeInTheDocument();
    expect(screen.getByText(/0 resources/)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("does not rewrite the selected scope from failed lists", async () => {
    const api = fakeApi({ Services: ["error"] });
    const onFilterChange = vi.fn();
    render(
      <ProjectOverview environmentId={null} onFilterChange={onFilterChange} />,
      { wrapper: api.wrapper },
    );
    await screen.findByRole("alert");
    expect(onFilterChange).not.toHaveBeenCalled();
  });

  it("does not call Unassigned empty while a list is unavailable", async () => {
    const api = fakeApi({ Services: ["error"] });
    render(<ProjectOverview environmentId="unassigned" />, {
      wrapper: api.wrapper,
    });
    await screen.findByRole("alert");
    expect(
      screen.getByRole("heading", { name: "Unassigned" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/No resources in this environment yet/),
    ).toBeNull();
  });

  it("explains unread lists on a Project with no environments", async () => {
    const api = fakeApi({ Services: ["error"] }, { environments: "none" });
    render(<ProjectOverview environmentId={null} />, { wrapper: api.wrapper });
    expect(await screen.findByRole("alert")).toHaveTextContent(/couldn't load/);
  });
});
