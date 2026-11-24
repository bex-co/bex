import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { SERVICE_TYPE_LABEL } from "@/features/services/lib/service-type";
import { GlobalSearchDialog } from "../global-search-dialog";
import { NAV_GROUPS, visibleNavGroups } from "../dashboard-nav";

// The dialog resolves resources through six feature hooks + the router; mock
// them so the test drives only the service-result rendering. Only services are
// populated (with distinct types); the other kinds stay empty.
const navigate = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));
// The caller's role as the capability check last answered (null: unknown).
const role = vi.hoisted(() => ({ admin: true as boolean | null }));
vi.mock("@/features/api-keys/hooks/use-workspace-admin", () => ({
  useWorkspaceAdmin: () => role.admin,
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
  buildSources: { rows: [] as object[], loading: false },
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
vi.mock("@/features/build-sources/hooks/use-build-sources-list", () => ({
  useBuildSourcesList: () => ({
    buildSources: lists.buildSources.rows,
    loading: lists.buildSources.loading,
  }),
}));

// The sidebar's feature gates, mutable per test (w21/031). `router` is what
// useRouterNavAvailable answers: the flag AND the companion-service probe
// (w21/042), which has its own test.
const flags = vi.hoisted(() => ({ agents: true, router: true }));
vi.mock("@/config/use-growthbook", () => ({
  useAgentsFeatureEnabled: () => flags.agents,
}));
vi.mock("../use-router-nav-available", () => ({
  useRouterNavAvailable: () => flags.router,
}));

const SERVICES = [
  { id: "srv-web", name: "web-app", type: "web_service" },
  { id: "srv-cron", name: "nightly", type: "cron_job" },
  { id: "srv-priv", name: "internal-api", type: "private_service" },
];

beforeEach(() => {
  navigate.mockReset();
  role.admin = true;
  flags.agents = true;
  flags.router = true;
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

  // w21/031: the palette's pages come from the sidebar's own list behind the
  // same gates, so a sidebar destination is never missing from ⌘K.
  describe("Navigation follows the sidebar (w21/031)", () => {
    const optionNames = () =>
      screen.getAllByRole("option").map((o) => o.textContent ?? "");

    it("lists every enabled sidebar destination plus the two Settings entries", () => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      const names = optionNames();
      const expected = visibleNavGroups(NAV_GROUPS, {
        agentsEnabled: true,
        routerAvailable: true,
      })
        .flatMap((group) => group.items)
        .map((item) => i18n.t(item.labelKey))
        .filter((label) => label !== i18n.t("common.navSettings"));
      for (const label of expected) {
        expect(names.some((n) => n.includes(label))).toBe(true);
      }
      expect(names.some((n) => n.includes("Workspace Settings"))).toBe(true);
      expect(names.some((n) => n.includes("Account Settings"))).toBe(true);
    });

    it("finds Agents and Automations by name, in en and zh", async () => {
      const { rerender } = render(
        <GlobalSearchDialog open onOpenChange={vi.fn()} />,
      );
      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "agents" },
      });
      expect(
        screen.getByRole("option", { name: /Agents/ }),
      ).toBeInTheDocument();
      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "automations" },
      });
      expect(
        screen.getByRole("option", { name: /Automations/ }),
      ).toBeInTheDocument();

      await act(async () => {
        await i18n.changeLanguage("zh");
      });
      rerender(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "智能体" },
      });
      expect(
        screen.getByRole("option", { name: /智能体/ }),
      ).toBeInTheDocument();
      await act(async () => {
        await i18n.changeLanguage("en");
      });
    });

    it("hides Agents, Automations and Router when their flags are off", () => {
      flags.agents = false;
      flags.router = false;
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      const names = optionNames().join("|");
      expect(names).not.toMatch(/Agents|Automations|Router/);
    });

    // w21/042: Router is offered only where the sidebar offers it. With the
    // gate closed (flag off, or the companion service absent) searching for it
    // finds nothing; with it open the page is there.
    it("finds Router only where the workspace is offered it", () => {
      flags.router = false;
      const { unmount } = render(
        <GlobalSearchDialog open onOpenChange={vi.fn()} />,
      );
      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "router" },
      });
      expect(screen.queryByRole("option", { name: /Router/ })).toBeNull();
      unmount();

      flags.router = true;
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "router" },
      });
      expect(
        screen.getByRole("option", { name: /Router/ }),
      ).toBeInTheDocument();
    });
  });

  // w21/018: a renamed service's original name sits beside the current one with
  // real whitespace, so the option's accessible name does not glue them.
  it("separates a renamed service's names in its accessible name", () => {
    lists.services.rows = [
      {
        id: "srv-r",
        name: "eden-dash-v3",
        immutableName: "block-eden-mono",
        type: "web_service",
      },
    ];
    render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
    expect(
      screen.getByRole("option", { name: /eden-dash-v3 block-eden-mono/ }),
    ).toBeInTheDocument();
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
    buildSources: {
      id: "bds-d",
      name: "discourse-build",
      kind: "git",
      git: { repoUrl: "https://github.com/acme/discourse", branch: "main" },
      image: null,
    },
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

  // w4/711: Render's search shortcuts, read from its dashboard bundle
  // (docs/render-artifacts/search-shortcuts.md), jump to a Settings section
  // or a docs page; a shortcut to a page the palette already lists lends that
  // page its keywords instead.
  describe("Render's search shortcuts (w4/711)", () => {
    const optionNames = () =>
      screen.queryAllByRole("option").map((o) => o.textContent ?? "");

    function enter(input: HTMLElement) {
      fireEvent.keyDown(input, { key: "Enter" });
    }

    it("lands on Account Settings at the API keys card for 'API keys' + Enter", () => {
      const onOpenChange = vi.fn();
      render(<GlobalSearchDialog open onOpenChange={onOpenChange} />);
      const input = typeQuery("API keys");

      const row = screen.getByRole("option", { name: /^API Keys/ });
      expect(row).toHaveAttribute("aria-selected", "true");
      expect(row).toHaveTextContent("Provision API keys");
      enter(input);

      expect(onOpenChange).toHaveBeenCalledWith(false);
      expect(navigate).toHaveBeenCalledWith({
        to: "/settings",
        hash: "api-keys",
      });
    });

    it.each([
      ["git", "Git credentials", "/settings", "integrations"],
      ["audit", "Audit Logs", "/workspace/settings", "audit-logs"],
      ["ssh", "SSH Keys", "/settings", "ssh-public-keys"],
      ["CLI tokens", "CLI Tokens", "/settings", "cli-tokens"],
      [
        "registry",
        "Container Registry Credentials",
        "/workspace/settings",
        "registry-credentials",
      ],
    ])("%s + Enter opens %s at %s#%s", (query, name, to, hash) => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      const input = typeQuery(query);
      expect(screen.getAllByRole("option")[0]).toHaveTextContent(
        new RegExp(`^${name}`),
      );
      enter(input);
      expect(navigate).toHaveBeenCalledWith({ to, hash });
    });

    it.each([
      ["environment variables", "https://bex.co/docs/platform/secrets"],
      ["custom domain", "https://bex.co/docs/platform/custom-domains"],
      ["mcp", "https://bex.co/docs/mcp-server"],
    ])("opens the docs for %s in a new tab", (query, href) => {
      const open = vi.spyOn(window, "open").mockReturnValue(null);
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      const input = typeQuery(query);
      const docs = screen
        .getAllByRole("option")
        .findIndex((o) => o.querySelector(".lucide-book-open"));
      for (let i = 0; i < docs; i++)
        fireEvent.keyDown(input, { key: "ArrowDown" });
      enter(input);
      expect(open).toHaveBeenCalledWith(href, "_blank", "noopener,noreferrer");
      expect(navigate).not.toHaveBeenCalled();
      open.mockRestore();
    });

    it("matches Render's keywords: 'token' finds the three credentials", () => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      typeQuery("token");
      const names = optionNames();
      for (const name of ["API Keys", "SSH Keys", "CLI Tokens"]) {
        expect(names.some((n) => n.startsWith(name))).toBe(true);
      }
    });

    // Render answers no shortcut below three characters, so the open palette
    // and a two-letter query list the pages alone, as before.
    it("offers no shortcut to an empty or two-character query", () => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      expect(optionNames().some((n) => n.startsWith("API Keys"))).toBe(false);
      typeQuery("ap");
      expect(optionNames().some((n) => n.startsWith("API Keys"))).toBe(false);
      typeQuery("api");
      expect(optionNames().some((n) => n.startsWith("API Keys"))).toBe(true);
    });

    it.each([
      ["home", "Projects", "/"],
      ["invoice", "Billing", "/billing"],
      ["secrets", "Environment Groups", "/env-groups"],
      ["integration", "Webhooks", "/webhooks"],
      ["members", "Workspace Settings", "/workspace/settings"],
      ["profile", "Account Settings", "/settings"],
    ])("'%s' finds the %s page by Render's keywords", (query, name, to) => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      const input = typeQuery(query);
      expect(screen.getAllByRole("option")[0]).toHaveTextContent(name);
      enter(input);
      expect(navigate).toHaveBeenCalledWith({ to });
    });

    // Audit Logs is only on an Admin's Workspace Settings.
    it.each([false, null])(
      "offers Audit Logs only to an Admin (admin: %s)",
      (admin) => {
        role.admin = admin;
        render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
        typeQuery("audit");
        expect(screen.queryByRole("option", { name: /Audit Logs/ })).toBeNull();
        expect(
          screen.getByText("No matching pages or resources."),
        ).toBeInTheDocument();
      },
    );

    it("names and finds shortcuts in the selected language", async () => {
      await act(() => i18n.changeLanguage("zh"));
      try {
        render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
        typeQuery("审计日志");
        expect(
          screen.getByRole("option", { name: /审计日志 导出审计日志/ }),
        ).toBeInTheDocument();
        // Render's keywords stay English: they are the product's own terms.
        typeQuery("api key");
        expect(
          screen.getByRole("option", { name: /API 密钥 创建 API 密钥/ }),
        ).toBeInTheDocument();
      } finally {
        await act(() => i18n.changeLanguage("en"));
      }
    });
  });

  // w4/342: every service row shows its kind, so typing that kind must find
  // it. The filter read only names and ids: "Cron Job" said "No matching pages
  // or resources." beside three listed cron jobs, and "cron" missed the one
  // whose name lacks it.
  describe("finds services by their displayed kind", () => {
    // Neutral ids, so only the kind label can produce a match.
    const KINDS = [
      { id: "srv-k1", name: "web-app", type: "web_service" },
      { id: "srv-k2", name: "nightly", type: "cron_job" },
      { id: "srv-k3", name: "backup-cron", type: "cron_job" },
      { id: "srv-k4", name: "internal-api", type: "private_service" },
      { id: "srv-k5", name: "queue", type: "background_worker" },
      { id: "srv-k6", name: "docs", type: "static_site" },
    ];
    const shown = () =>
      KINDS.map((s) => s.name).filter((name) => screen.queryByText(name));

    beforeEach(() => {
      lists.services.rows = KINDS;
    });

    it.each([
      ["Web Service", ["web-app"]],
      ["Cron Job", ["nightly", "backup-cron"]],
      ["cron", ["nightly", "backup-cron"]],
      ["Private Service", ["internal-api"]],
      ["Background Worker", ["queue"]],
      ["Static Site", ["docs"]],
    ])("%s returns exactly its services", (query, names) => {
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      typeQuery(query);
      expect(shown()).toEqual(names);
      expect(screen.queryByText(EMPTY)).toBeNull();
    });

    it("matches the kind word in the selected language", async () => {
      await act(() => i18n.changeLanguage("zh"));
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);
      typeQuery(i18n.t(SERVICE_TYPE_LABEL.cron));
      expect(shown()).toEqual(["nightly", "backup-cron"]);
      await act(() => i18n.changeLanguage("en"));
    });
  });

  describe("Build Sources search (w2/m613)", () => {
    it("finds build sources by name, repo URL, or image URL and navigates to /build-sources", () => {
      lists.buildSources.rows = [
        {
          id: "bds-1",
          name: "monorepo-api",
          kind: "git",
          git: { repoUrl: "https://github.com/acme/payments", branch: "release" },
          image: null,
        },
        {
          id: "bds-2",
          name: "edge-proxy",
          kind: "image",
          git: null,
          image: { imageUrl: "ghcr.io/acme/envoy:1.30" },
        },
      ];
      render(<GlobalSearchDialog open onOpenChange={vi.fn()} />);

      typeQuery("acme/payments");
      expect(
        screen.getByRole("option", { name: /monorepo-api.*Build Source/ }),
      ).toBeInTheDocument();
      expect(screen.queryByText("edge-proxy")).toBeNull();

      typeQuery("envoy:1.30");
      const imageRow = screen.getByRole("option", {
        name: /edge-proxy.*Build Source/,
      });
      fireEvent.click(imageRow);
      expect(navigate).toHaveBeenCalledWith({
        to: "/build-sources",
        search: { q: "edge-proxy", kind: undefined },
      });
    });
  });
});
