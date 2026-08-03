import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewBlueprintPage } from "../blueprints.new";
import type { RepoView } from "@/features/services/hooks/use-repos";
import type { BlueprintPreviewResult } from "@/features/blueprints/types";

// --- git connection mock ---
const connectionState: {
  connection: {
    connected: boolean;
    accountLogin: string | null;
    installUrl: string;
  } | null;
  loading: boolean;
} = {
  connection: { connected: true, accountLogin: "acme", installUrl: "" },
  loading: false,
};
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => connectionState,
}));

// --- repos mock ---
const reposState: { repos: RepoView[]; loading: boolean; error: undefined } = {
  repos: [],
  loading: false,
  error: undefined,
};
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => reposState,
}));

// --- branches mock ---
const branchesState: { branches: string[]; loading: boolean } = {
  branches: [],
  loading: false,
};
vi.mock("@/features/services/hooks/use-repo-branches", () => ({
  useRepoBranches: () => branchesState,
}));

// --- preview mock ---
const previewState: {
  preview: BlueprintPreviewResult | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  preview: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/blueprints/hooks/use-blueprint-preview", () => ({
  useBlueprintPreview: () => previewState,
}));

// --- create mock ---
const create = vi.fn(async () => ({
  status: "success" as const,
  blueprint: { id: "blp-new1" },
}));
vi.mock("@/features/blueprints/hooks/use-create-blueprint", () => ({
  useCreateBlueprint: () => ({ create, busy: false }),
}));

function repo(overrides: Partial<RepoView> = {}): RepoView {
  return {
    id: 1,
    fullName: "acme/hello-go",
    private: false,
    defaultBranch: "main",
    htmlUrl: "https://github.com/acme/hello-go",
    cloneUrl: "https://github.com/acme/hello-go.git",
    ...overrides,
  };
}

function validPreview(): BlueprintPreviewResult {
  return {
    found: true,
    commitId: "abc1234",
    error: null,
    validation: {
      valid: true,
      errors: [],
      plan: {
        services: ["api", "worker"],
        databases: ["db"],
        keyValue: [],
        envGroups: [],
        totalActions: 3,
      },
    },
  };
}

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints/new",
    component: NewBlueprintPage,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/blueprints/$blueprintId",
    component: () => <div>blueprint detail</div>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/blueprints/new"] }),
    context: { client: {} as never, session: null },
  });
  render(<RouterProvider router={router} />);
  return router;
}

beforeEach(() => {
  connectionState.connection = {
    connected: true,
    accountLogin: "acme",
    installUrl: "",
  };
  connectionState.loading = false;
  reposState.repos = [];
  reposState.loading = false;
  branchesState.branches = [];
  previewState.preview = null;
  previewState.loading = false;
  previewState.error = undefined;
  create.mockClear();
});

describe("NewBlueprintPage", () => {
  it("shows the install-GitHub-App CTA when the workspace is not connected", async () => {
    connectionState.connection = {
      connected: false,
      accountLogin: null,
      installUrl: "https://github.com/apps/bex/installations/new",
    };
    renderPage();

    const cta = await screen.findByRole("link", { name: /connect github/i });
    expect(cta).toHaveAttribute(
      "href",
      "https://github.com/apps/bex/installations/new",
    );
  });

  it("lists connected repos, filters by search, and auto-fills name + branch on select", async () => {
    reposState.repos = [
      repo(),
      repo({ id: 2, fullName: "acme/other", private: true }),
    ];
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText("acme/hello-go")).toBeInTheDocument();
    expect(screen.getByText("acme/other")).toBeInTheDocument();
    expect(screen.getByText("Private")).toBeInTheDocument();

    await user.type(
      screen.getByRole("textbox", { name: /search repositories/i }),
      "hello",
    );
    expect(screen.queryByText("acme/other")).not.toBeInTheDocument();

    await user.click(screen.getByText("acme/hello-go"));
    expect(screen.getByLabelText("Blueprint Name")).toHaveValue("hello-go");
    expect(screen.getByRole("combobox", { name: /branch/i })).toHaveValue(
      "main",
    );
  });

  it("shows the fetch error with Retry and disables Deploy when the file is missing", async () => {
    reposState.repos = [repo()];
    previewState.preview = {
      found: false,
      commitId: null,
      error: "bex.yml not found on main",
      validation: null,
    };
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByText("acme/hello-go"));

    expect(
      await screen.findByText("bex.yml not found on main"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /deploy blueprint/i }),
    ).toBeDisabled();
  });

  it("shows the parsed resource plan, then creates and navigates on Deploy", async () => {
    reposState.repos = [repo()];
    previewState.preview = validPreview();
    const user = userEvent.setup();
    const router = renderPage();

    await user.click(await screen.findByText("acme/hello-go"));

    expect(await screen.findByText(/parsed successfully/i)).toBeInTheDocument();
    expect(screen.getByText("api")).toBeInTheDocument();
    expect(screen.getByText("worker")).toBeInTheDocument();
    expect(screen.getByText("db")).toBeInTheDocument();

    const deploy = screen.getByRole("button", { name: /deploy blueprint/i });
    expect(deploy).toBeEnabled();
    await user.click(deploy);

    expect(create).toHaveBeenCalledWith(
      "https://github.com/acme/hello-go.git",
      "main",
      "render.yaml",
      "hello-go",
      undefined,
    );
    expect(await screen.findByText("blueprint detail")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/blueprints/blp-new1");
  });

  it("accepts a public Git URL on the Public Git tab", async () => {
    previewState.preview = validPreview();
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("tab", { name: /public git/i }));
    await user.type(
      screen.getByLabelText("Repository URL"),
      "https://github.com/acme/public-app",
    );
    await user.type(screen.getByRole("combobox", { name: /branch/i }), "main");

    const deploy = screen.getByRole("button", { name: /deploy blueprint/i });
    expect(deploy).toBeEnabled();
    await user.click(deploy);
    expect(create).toHaveBeenCalledWith(
      "https://github.com/acme/public-app",
      "main",
      "render.yaml",
      "public-app",
      undefined,
    );
  });

  it("rejects an invalid public Git URL", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("tab", { name: /public git/i }));
    await user.type(screen.getByLabelText("Repository URL"), "not-a-url");

    expect(screen.getByText(/enter a valid/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /deploy blueprint/i }),
    ).toBeDisabled();
  });
});
