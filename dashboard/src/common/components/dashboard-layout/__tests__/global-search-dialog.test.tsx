import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { GlobalSearchDialog } from "../global-search-dialog";

// The dialog resolves resources through six feature hooks + the router; mock
// them so the test drives only the service-result rendering. Only services are
// populated (with distinct types); the other kinds stay empty.
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));
// Mutable per test: each list carries its rows and whether its first read is
// still pending.
const lists = vi.hoisted(() => ({
  services: { rows: [] as object[], loading: false },
  databases: { rows: [] as object[], loading: false },
  keyValues: { rows: [] as object[], loading: false },
  projects: { rows: [] as object[], loading: false },
  envGroups: { rows: [] as object[], loading: false },
  blueprints: { rows: [] as object[], loading: false },
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: lists.services.rows,
    loading: lists.services.loading,
  }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({
    databases: lists.databases.rows,
    loading: lists.databases.loading,
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({
    keyValues: lists.keyValues.rows,
    loading: lists.keyValues.loading,
  }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: lists.projects.rows,
    loading: lists.projects.loading,
  }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({
    groups: lists.envGroups.rows,
    loading: lists.envGroups.loading,
  }),
}));
vi.mock("@/features/blueprints/hooks/use-blueprints", () => ({
  useBlueprints: () => ({
    blueprints: lists.blueprints.rows,
    loading: lists.blueprints.loading,
  }),
}));

const SERVICES = [
  { id: "srv-web", name: "web-app", type: "web_service" },
  { id: "srv-cron", name: "nightly", type: "cron_job" },
  { id: "srv-priv", name: "internal-api", type: "private_service" },
];

beforeEach(() => {
  for (const list of Object.values(lists)) {
    list.rows = [];
    list.loading = false;
  }
  lists.services.rows = SERVICES;
});

describe("GlobalSearchDialog", () => {
  // w4/049: the palette must name each service by its specific type + icon, not
  // collapse every service to a generic "Service" + Globe (sibling of w4/047).
  it("labels each service result by its specific type, not a generic Service", () => {
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);

    expect(screen.getByText("Web Service")).toBeInTheDocument();
    expect(screen.getByText("Cron Job")).toBeInTheDocument();
    expect(screen.getByText("Private Service")).toBeInTheDocument();
    // The generic collapse label must be gone.
    expect(screen.queryByText("Service")).not.toBeInTheDocument();

    // Per-type icons replace the old hardcoded Globe2: the cron row gets a
    // clock, and no result carries the old generic glyph. cmdk portals the
    // dialog to document.body, so query the whole document, not the root.
    expect(document.querySelector(".lucide-clock")).not.toBeNull();
    expect(document.querySelector(".lucide-globe-2")).toBeNull();
  });

  // w4/229: with cached, unrelated services and a Blueprint list still on its
  // first read, "discourse" declared "No matching pages or resources." until
  // the Blueprint arrived.
  const EMPTY = "No matching pages or resources.";
  const MATCHES = {
    services: { id: "srv-d", name: "discourse-web", type: "web_service" },
    databases: { id: "dpg-d", name: "discourse-db" },
    keyValues: { id: "red-d", name: "discourse-cache" },
    projects: { id: "prj-d", name: "discourse-project" },
    envGroups: { id: "evg-d", name: "discourse-env" },
    blueprints: { id: "blp-d", name: "discourse_docker" },
  } as const;

  function typeQuery(value: string) {
    const input = screen.getByRole("combobox");
    input.focus();
    fireEvent.change(input, { target: { value } });
    return input;
  }

  it.each(Object.keys(MATCHES) as (keyof typeof MATCHES)[])(
    "shows Loading, never the empty message, while %s is pending — then its match",
    (kind) => {
      lists[kind].loading = true;
      const { rerender } = render(
        <GlobalSearchDialog open onOpenChange={vi.fn()} />,
      );
      const input = typeQuery("discourse");

      expect(screen.getByText("Loading…")).toBeInTheDocument();
      expect(screen.queryByText(EMPTY)).toBeNull();

      lists[kind].loading = false;
      lists[kind].rows = [...lists[kind].rows, MATCHES[kind]];
      rerender(<GlobalSearchDialog open onOpenChange={vi.fn()} />);

      expect(screen.getByText(MATCHES[kind].name)).toBeInTheDocument();
      expect(screen.queryByText("Loading…")).toBeNull();
      expect(screen.queryByText(EMPTY)).toBeNull();
      expect(input).toHaveValue("discourse");
      expect(input).toHaveFocus();
    },
  );

  it("keeps an already-matching cached row selectable beside Loading", () => {
    lists.blueprints.loading = true;
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
    typeQuery("web-app");

    const row = screen.getByRole("option", { name: /web-app/ });
    expect(row).toHaveAttribute("aria-selected", "true");
    expect(row).not.toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("option", { name: "Loading…" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.queryByText(EMPTY)).toBeNull();
  });

  it("declares no matches only once every list has settled", () => {
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
    typeQuery("QA_NO_MATCH");

    expect(screen.getByText(EMPTY)).toBeInTheDocument();
    expect(screen.queryByText("Loading…")).toBeNull();
    // No floating headings or separator beside it (w6/046).
    expect(screen.queryByText("Resources")).toBeNull();
    expect(screen.queryByText("Navigation")).toBeNull();
    expect(document.querySelector("[cmdk-separator]")).toBeNull();
  });

  // w4/312: cmdk names the results listbox "Suggestions" unless the wrapper
  // passes its own label, so the name must follow the selected language.
  it("names the results list in the selected language", async () => {
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
    expect(
      screen.getByRole("listbox", { name: "Suggestions" }),
    ).toBeInTheDocument();
    await act(() => i18n.changeLanguage("zh"));
    expect(screen.getByRole("listbox", { name: "建议" })).toBeInTheDocument();
    await act(() => i18n.changeLanguage("en"));
  });
});
