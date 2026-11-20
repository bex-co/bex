import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { EditViewDialog, type EditViewTarget } from "../edit-view-dialog";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";

// Render's Edit view modal (w3/m507): Name, Filters rows (Field, Operator,
// Values, trash) with an add button, the shared-view notice, Close and Done.
// Done writes the view through bex-api; Close writes nothing.

const mutations = {
  create: vi.fn(),
  update: vi.fn(),
  duplicate: vi.fn(),
  remove: vi.fn(),
  busy: null,
};
vi.mock("@/features/projects/hooks/use-saved-search-mutations", () => ({
  useSavedSearchMutations: () => mutations,
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({
    environments: [
      { id: "evm-prod", name: "production" },
      { id: "evm-stage", name: "staging" },
    ],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [
      {
        id: "prj-1",
        name: "observed",
        ownerId: "tea-1",
        serviceIds: ["srv-web", "srv-cron", "srv-site"],
        databaseIds: ["dpg-db"],
        keyValueIds: [],
      },
    ],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
// The modal reads the workspace's lists once, without polling.
const listReads: unknown[] = [];
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: (options: unknown) => {
    listReads.push(options);
    return {
      services: [
        { id: "srv-web", name: "storefront", type: "web_service" },
        { id: "srv-cron", name: "nightly", type: "cron_job" },
        { id: "srv-site", name: "docs", type: "static_site" },
        { id: "srv-other", name: "elsewhere", type: "web_service" },
      ],
    };
  },
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: (options: unknown) => {
    listReads.push(options);
    return { databases: [{ id: "dpg-db", name: "orders-db" }] };
  },
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: (options: unknown) => {
    listReads.push(options);
    return { keyValues: [] };
  },
}));

// The Instance picker's project-wide read (w3/079): what the project's logs
// hold, by the call's project, label and log type.
const discovery = vi.hoisted(() => ({
  calls: [] as string[][],
  answer: {
    values: [] as string[],
    status: "ready" as "loading" | "ready" | "failed",
  },
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useProjectLogLabelDiscovery: (
    project: string,
    label: string,
    logType: string,
  ) => {
    discovery.calls.push([project, label, logType]);
    return discovery.answer;
  },
}));

const WEB_A = "srv-web-0123456789abcdefghij";
const WEB_B = "srv-web-klmnopqrstuv01234567";
const DB_A = "dpg-db-89abcdefghijklmnopqr";

const view = (
  filters: Record<string, unknown> = { filters: [], version: 1 },
): SavedSearch => ({
  __typename: "SavedSearch",
  id: "svs-app",
  name: "Application Logs",
  type: SavedSearchType.ApplicationLogs,
  filters,
});

function open(target: EditViewTarget) {
  const onOpenChange = vi.fn();
  const onSaved = vi.fn();
  render(
    <EditViewDialog
      projectId="prj-1"
      target={target}
      onOpenChange={onOpenChange}
      onSaved={onSaved}
    />,
  );
  return { onOpenChange, onSaved, dialog: screen.getByRole("dialog") };
}

beforeEach(() => {
  for (const fn of [mutations.create, mutations.update]) fn.mockReset();
  discovery.calls = [];
  discovery.answer = { values: [WEB_A, WEB_B, DB_A], status: "ready" };
});

/** The rows a project's Instance row starts from: one, moved to Instance. */
async function instanceRow(
  user: ReturnType<typeof userEvent.setup>,
  dialog: HTMLElement,
) {
  await user.click(within(dialog).getByRole("button", { name: "Add filter" }));
  await user.click(within(dialog).getByRole("combobox", { name: "Field" }));
  await user.click(screen.getByRole("option", { name: "Instance" }));
  return within(dialog).getByRole("button", { name: "Values for Instance" });
}

/** The open picker's listed values, as they read. */
const pickerOptions = () =>
  within(screen.getByRole("listbox", { name: "Values for Instance" }))
    .getAllByRole("option")
    .map((option) => option.textContent);

describe("EditViewDialog", () => {
  it("shows Render's editor for a view and saves its name and filters on Done", async () => {
    const user = userEvent.setup();
    const saved = { ...view(), name: "Errors" };
    mutations.update.mockResolvedValue(saved);
    const { onOpenChange, onSaved, dialog } = open({
      kind: "edit",
      view: view({
        filters: [
          { field: "ENVIRONMENT", operator: "IS", values: ["evm-prod"] },
        ],
        version: 1,
      }),
    });

    expect(
      within(dialog).getByRole("heading", { name: "Edit view" }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "Editing filters will update the view for everyone on the project.",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("combobox", { name: "Values for Environment" }),
    ).toHaveTextContent("production");

    const name = within(dialog).getByLabelText("Name");
    await user.clear(name);
    await user.type(name, "Errors");
    await user.click(
      within(dialog).getByRole("button", { name: "Add filter" }),
    );
    // The next unused field, Resource; its values are the project's own
    // resources a log view reads (no static site, nothing outside it).
    await user.click(
      within(dialog).getByRole("button", { name: "Values for Resource" }),
    );
    const menu = screen.getByRole("menu");
    expect(
      within(menu)
        .getAllByRole("menuitemcheckbox")
        .map((item) => item.textContent),
    ).toEqual(["storefront", "nightly", "orders-db"]);
    await user.click(
      within(menu).getByRole("menuitemcheckbox", { name: "nightly" }),
    );
    await user.keyboard("{Escape}");
    const [environmentOperator, resourceOperator] = within(dialog).getAllByRole(
      "combobox",
      { name: "Operator" },
    );
    // Environment takes `is` only, as Render's editor does.
    expect(environmentOperator).toBeDisabled();
    await user.click(resourceOperator);
    await user.click(screen.getByRole("option", { name: "is not one of" }));

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.update).toHaveBeenCalledWith("svs-app", "Errors", {
      filters: [
        { field: "ENVIRONMENT", operator: "IS", values: ["evm-prod"] },
        { field: "RESOURCE", operator: "IS_NOT", values: ["srv-cron"] },
      ],
      version: 1,
    });
    expect(onSaved).toHaveBeenCalledWith(saved);
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(listReads).not.toHaveLength(0);
    expect(listReads.every((o) => JSON.stringify(o) === '{"poll":false}')).toBe(
      true,
    );
  });

  it("reopens a stored is-not-one-of row on a line field and saves it as IS_NOT", async () => {
    const user = userEvent.setup();
    mutations.update.mockResolvedValue(view());
    const { dialog } = open({
      kind: "edit",
      view: view({
        filters: [
          { field: "LEVEL", operator: "IS_NOT", values: ["info"] },
          { field: "INSTANCE", operator: "IS", values: ["srv-web-abcde"] },
        ],
        version: 1,
      }),
    });
    const [levelOperator, instanceOperator] = within(dialog).getAllByRole(
      "combobox",
      { name: "Operator" },
    );
    // Render's editor offers `is not one of` on the line fields (w3/m523).
    expect(levelOperator).toHaveTextContent("is not one of");
    expect(levelOperator).toBeEnabled();
    expect(instanceOperator).toHaveTextContent("is one of");
    await user.click(instanceOperator);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["is one of", "is not one of"]);
    await user.click(screen.getByRole("option", { name: "is not one of" }));

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.update).toHaveBeenCalledWith(
      "svs-app",
      "Application Logs",
      {
        filters: [
          { field: "LEVEL", operator: "IS_NOT", values: ["info"] },
          { field: "INSTANCE", operator: "IS_NOT", values: ["srv-web-abcde"] },
        ],
        version: 1,
      },
    );
  });

  it("lists the instances the project's logs hold, each named with its resource, and adds one that has gone as typed (w3/079)", async () => {
    const user = userEvent.setup();
    mutations.update.mockResolvedValue(view());
    const { dialog } = open({ kind: "edit", view: view() });
    const trigger = await instanceRow(user, dialog);
    expect(discovery.calls).toContainEqual(["prj-1", "instance", "app"]);

    await user.click(trigger);
    expect(pickerOptions()).toEqual([
      "storefront · 01234",
      "storefront · klmno",
      "orders-db · 89abc",
    ]);
    await user.click(screen.getByRole("option", { name: "orders-db · 89abc" }));
    expect(
      screen.getByRole("option", { name: "orders-db · 89abc" }),
    ).toHaveAttribute("aria-checked", "true");

    // The search narrows the list by resource name or id ...
    const search = screen.getByPlaceholderText("Search, or type a value");
    await user.type(search, "storefront");
    expect(pickerOptions()).toEqual([
      "storefront · 01234",
      "storefront · klmno",
      "Add “storefront”",
    ]);
    // ... and a value the logs no longer hold is added as typed.
    const gone = "srv-web-vvvvvvvvvvvvvvvvvvvv";
    await user.clear(search);
    await user.type(search, gone);
    await user.click(screen.getByRole("option", { name: `Add “${gone}”` }));
    expect(search).toHaveValue("");
    expect(pickerOptions()).toContain("storefront · vvvvv");
    await user.keyboard("{Escape}");
    expect(trigger).toHaveTextContent("orders-db · 89abc, storefront · vvvvv");

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.update).toHaveBeenCalledWith(
      "svs-app",
      "Application Logs",
      {
        filters: [{ field: "INSTANCE", operator: "IS", values: [DB_A, gone] }],
        version: 1,
      },
    );
  });

  it("keeps a stored instance the logs no longer hold, unchecks it, and reads a Request Logs view's instances", async () => {
    const user = userEvent.setup();
    mutations.update.mockResolvedValue(view());
    // Request Logs reads the web services alone.
    discovery.answer = { values: [WEB_A, WEB_B], status: "ready" };
    const gone = "srv-web-vvvvvvvvvvvvvvvvvvvv";
    const { dialog } = open({
      kind: "edit",
      view: {
        ...view({
          filters: [{ field: "INSTANCE", operator: "IS_NOT", values: [gone] }],
          version: 1,
        }),
        type: SavedSearchType.RequestLogs,
      },
    });
    expect(discovery.calls).toContainEqual(["prj-1", "instance", "request"]);
    const trigger = within(dialog).getByRole("button", {
      name: "Values for Instance",
    });
    expect(trigger).toHaveTextContent("storefront · vvvvv");
    await user.click(trigger);
    expect(pickerOptions()).toEqual([
      "storefront · 01234",
      "storefront · klmno",
      "storefront · vvvvv",
    ]);
    await user.click(
      screen.getByRole("option", { name: "storefront · vvvvv" }),
    );
    await user.click(
      screen.getByRole("option", { name: "storefront · 01234" }),
    );
    await user.keyboard("{Escape}");
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.update).toHaveBeenCalledWith(
      "svs-app",
      "Application Logs",
      {
        filters: [{ field: "INSTANCE", operator: "IS_NOT", values: [WEB_A] }],
        version: 1,
      },
    );
  });

  it("says when the project's logs cannot be read or hold no instance, and still takes a typed one", async () => {
    const user = userEvent.setup();
    discovery.answer = { values: [], status: "failed" };
    const { dialog } = open({ kind: "edit", view: view() });
    await user.click(await instanceRow(user, dialog));
    expect(screen.getByRole("status")).toHaveTextContent(
      "Couldn't read the project's logs. Type a value to add it.",
    );
    await user.type(
      screen.getByPlaceholderText("Search, or type a value"),
      `${WEB_A}{Enter}`,
    );
    expect(pickerOptions()).toEqual(["storefront · 01234"]);
    await user.keyboard("{Escape}");

    discovery.answer = { values: [], status: "ready" };
    await user.click(
      within(dialog).getByRole("button", { name: "Values for Instance" }),
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "None in the project's logs from the last 7 days. Type a value to add it.",
    );
  });

  it("writes nothing on Close", async () => {
    const user = userEvent.setup();
    const { onOpenChange, dialog } = open({ kind: "edit", view: view() });
    await user.type(within(dialog).getByLabelText("Name"), " changed");
    await user.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(mutations.update).not.toHaveBeenCalled();
    expect(mutations.create).not.toHaveBeenCalled();
  });

  it("asks for a value before saving a row without one", async () => {
    const user = userEvent.setup();
    const { dialog } = open({ kind: "edit", view: view() });
    await user.click(
      within(dialog).getByRole("button", { name: "Add filter" }),
    );
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(
      within(dialog).getByText("Choose at least one value."),
    ).toBeInTheDocument();
    expect(mutations.update).not.toHaveBeenCalled();

    // A typed value (an instance, a search term, …) is added with Enter and
    // removed with its own button.
    await user.click(
      within(dialog).getByRole("button", { name: "Remove Environment filter" }),
    );
    for (let i = 0; i < 6; i++) {
      await user.click(
        within(dialog).getByRole("button", { name: "Add filter" }),
      );
    }
    const text = within(dialog).getByRole("textbox", {
      name: "Values for Text Search",
    });
    await user.type(text, "timeout{Enter}refused,");
    expect(
      within(dialog).getByRole("button", { name: "Remove timeout" }),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Remove refused" }),
    );
    expect(
      within(dialog).queryByRole("button", { name: "Remove refused" }),
    ).toBeNull();
  });

  it("creates a new view of its type on Done, and shows a refusal in the modal", async () => {
    const user = userEvent.setup();
    mutations.create.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        errors: [
          {
            message:
              'a view named "Request Logs" already exists in this project',
            extensions: { code: "duplicate_saved_search_name" },
          },
        ],
      }),
    );
    const { onSaved, dialog } = open({
      kind: "create",
      type: SavedSearchType.RequestLogs,
      name: "Request Logs",
    });
    expect(
      within(dialog).getByRole("heading", { name: "New view" }),
    ).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Name")).toHaveValue("Request Logs");
    expect(
      within(dialog).getByText(
        "No filters: the view reads every resource in the project.",
      ),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.create).toHaveBeenCalledWith(
      SavedSearchType.RequestLogs,
      "Request Logs",
      {
        filters: [],
        version: 1,
      },
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      /view named "Request Logs" already exists in this project/,
    );
    expect(onSaved).not.toHaveBeenCalled();

    // Request Logs reads web services only.
    await user.click(
      within(dialog).getByRole("button", { name: "Add filter" }),
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Add filter" }),
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Values for Resource" }),
    );
    expect(
      within(screen.getByRole("menu"))
        .getAllByRole("menuitemcheckbox")
        .map((item) => item.textContent),
    ).toEqual(["storefront"]);
  });

  it("creates a Service Metrics view with its own fields, its static sites among the resources (w3/m508)", async () => {
    const user = userEvent.setup();
    mutations.create.mockResolvedValueOnce({
      ...view(),
      id: "svs-met",
      name: "Service Metrics 2",
      type: SavedSearchType.Metrics,
    });
    const { dialog, onSaved } = open({
      kind: "create",
      type: SavedSearchType.Metrics,
      name: "Service Metrics 2",
    });
    await user.click(
      within(dialog).getByRole("button", { name: "Add filter" }),
    );
    // A metrics view narrows its resources and its range, and has no line
    // to filter.
    await user.click(within(dialog).getByRole("combobox", { name: "Field" }));
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual(["Environment", "Resource", "Resource Type", "Time Range"]);
    await user.click(screen.getByRole("option", { name: "Resource" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Values for Resource" }),
    );
    expect(
      within(screen.getByRole("menu"))
        .getAllByRole("menuitemcheckbox")
        .map((item) => item.textContent),
    ).toEqual(["storefront", "nightly", "docs", "orders-db"]);
    await user.click(
      within(screen.getByRole("menu")).getByRole("menuitemcheckbox", {
        name: "docs",
      }),
    );
    await user.keyboard("{Escape}");
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(mutations.create).toHaveBeenCalledWith(
      SavedSearchType.Metrics,
      "Service Metrics 2",
      {
        filters: [{ field: "RESOURCE", operator: "IS", values: ["srv-site"] }],
        version: 1,
      },
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("refuses a blank name before writing", async () => {
    const user = userEvent.setup();
    const { dialog } = open({
      kind: "create",
      type: SavedSearchType.ApplicationLogs,
      name: "",
    });
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(within(dialog).getByText("Name the view.")).toBeInTheDocument();
    expect(mutations.create).not.toHaveBeenCalled();
  });
});
