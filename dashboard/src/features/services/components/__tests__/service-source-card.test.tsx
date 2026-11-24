import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceSourceCard } from "@/features/services/components/service-source-card";
import type { SourceMutationResult } from "@/features/services/hooks/use-source-mutation";

const navigate = vi.fn();
let currentBase: "/services" | "/static" = "/services";

const setImage = vi.fn(
  async (): Promise<SourceMutationResult> => ({
    ok: true,
    deployId: "dep-img-1",
  }),
);
const setRepo = vi.fn(
  async (): Promise<SourceMutationResult> => ({
    ok: true,
    deployId: "dep-git-1",
  }),
);
const linkBuildSource = vi.fn(
  async (): Promise<SourceMutationResult> => ({
    ok: true,
    deployId: "dep-bds-1",
  }),
);
let repoLastError: string | null = null;
let imageLastError: string | null = null;
let linkLastError: string | null = null;
let buildSourcesError: Error | undefined;
const buildSources = [
  {
    id: "bds-mono",
    name: "Mono",
    source: "https://github.com/acme/mono",
    kind: "git",
    nativeRuntime: "node",
  },
  {
    id: "bds-docker",
    name: "Docker",
    source: "https://github.com/acme/d",
    native: false,
  },
];

const repos = [
  {
    id: 1,
    fullName: "acme/api",
    htmlUrl: "https://github.com/acme/api",
    cloneUrl: "https://github.com/acme/api.git",
    accountLogin: "acme",
    private: true,
    defaultBranch: "main",
  },
  {
    id: 2,
    fullName: "puncsky/site",
    htmlUrl: "https://github.com/puncsky/site",
    cloneUrl: "https://github.com/puncsky/site.git",
    accountLogin: "puncsky",
    private: false,
    defaultBranch: "trunk",
  },
];

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));
vi.mock("@/features/services/lib/service-base", () => ({
  useServiceBase: () => currentBase,
}));
vi.mock(
  "@/features/services/hooks/use-build-sources",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/services/hooks/use-build-sources")
    >()),
    useBuildSources: () =>
      buildSourcesError
        ? { sources: [], loading: false, error: buildSourcesError }
        : { sources: buildSources, loading: false },
    useBuildSource: (id: string | null) => ({
      source: buildSources.find((source) => source.id === id) ?? null,
      loading: false,
    }),
  }),
);
vi.mock("@/features/services/hooks/use-link-build-source", () => ({
  useLinkBuildSource: () => ({
    linkBuildSource,
    busy: false,
    lastError: linkLastError,
  }),
}));
vi.mock("@/features/services/hooks/use-set-image", () => ({
  useSetImage: () => ({ setImage, busy: false, lastError: imageLastError }),
}));
vi.mock("@/features/services/hooks/use-set-repo", () => ({
  useSetRepo: () => ({ setRepo, busy: false, lastError: repoLastError }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/services/hooks/use-repo-branches", () => ({
  useRepoBranches: () => ({ branches: ["main", "release"], loading: false }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  // ServiceSourcePicker's disconnected gate reads the singular view.
  useGitConnection: () => ({
    connection: { connected: true, accountLogin: "acme", installUrl: "" },
    loading: false,
    refetch: vi.fn(),
  }),
  // GitCredentialsMenu (w8/m31), rendered in the picker's GitHub tab header.
  useGitConnections: () => ({
    connections: [
      {
        accountLogin: "acme",
        installationId: 42,
        createdAt: "",
        installUrl: "https://github.com/settings/installations/42",
      },
    ],
    connected: true,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGitAccount: () => ({ connect: vi.fn(), busy: false }),
}));
vi.mock("@/features/git/hooks/use-disconnect-git", () => ({
  useDisconnectGit: () => ({ disconnect: vi.fn(), busy: false }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", async () => {
  const { mockCapabilities } = await import("@/test/mocks/capabilities");
  return {
    useCapabilities: () => mockCapabilities({ canCreate: true }),
  };
});
vi.mock("@/features/services/components/registry-credential-select", () => ({
  RegistryCredentialSelect: () => null,
}));

beforeEach(() => {
  currentBase = "/services";
  navigate.mockClear();
  setImage.mockClear();
  setRepo.mockClear();
  linkBuildSource.mockClear();
  repoLastError = null;
  imageLastError = null;
  linkLastError = null;
  buildSourcesError = undefined;
});

describe("ServiceSourceCard", () => {
  it("keeps the source draft open and navigates nowhere when the backend rejects access", async () => {
    const user = userEvent.setup();
    setRepo.mockResolvedValueOnce({ ok: false });
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByDisplayValue("https://github.com/acme/api");
    await user.clear(input);
    await user.type(input, "https://github.com/unconnected/private");
    await user.click(screen.getByRole("button", { name: "Deploy" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-1", {
        repo: "https://github.com/unconnected/private",
        branch: "main",
        runtime: "docker",
        deploy: true,
      }),
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(input).toHaveValue("https://github.com/unconnected/private");
    expect(setImage).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
  });

  // w10/m490: a repository the GitHub app connected and can no longer read
  // (moved to an account without the app, or its grant revoked) gets no
  // pushes, and the card says so instead of looking connected.
  it("warns when the GitHub app can no longer read the repository, and only then", () => {
    const props = {
      serviceId: "srv-1",
      repo: "https://github.com/acme/api",
      branch: "main",
      imagePath: null,
      registryCredentialId: null,
      buildSourceId: null,
      startCommand: null,
    };
    const { rerender } = render(
      <ServiceSourceCard {...props} repoAccessLost />,
    );
    const warning = screen.getByTestId("source-repo-access-lost");
    expect(warning).toHaveTextContent(
      "bex's GitHub app can't read this repository",
    );
    expect(warning).toHaveTextContent("install the GitHub app on that account");

    rerender(<ServiceSourceCard {...props} repoAccessLost={false} />);
    expect(
      screen.queryByTestId("source-repo-access-lost"),
    ).not.toBeInTheDocument();

    rerender(
      <ServiceSourceCard
        {...props}
        repo={null}
        imagePath="ghcr.io/acme/api:1"
        repoAccessLost
      />,
    );
    expect(
      screen.queryByTestId("source-repo-access-lost"),
    ).not.toBeInTheDocument();
  });

  it("shows a repo and branch, repoints both from the grouped picker, deploys, and navigates", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
      />,
    );

    expect(screen.getByText("github.com · acme / api")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Edit" }));

    expect(
      screen.getByText(/Submitting starts a new deploy/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Deploy" })).toBeDisabled();
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(screen.getByText("puncsky")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));

    const branch = screen.getByRole("combobox", { name: "Branch" });
    expect(branch).toHaveValue("trunk");
    await user.clear(branch);
    await user.type(branch, "release");
    await user.click(screen.getByRole("button", { name: "Deploy" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-1", {
        repo: "https://github.com/puncsky/site",
        branch: "release",
        runtime: "docker",
        deploy: true,
      }),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "srv-1", deployId: "dep-git-1" },
    });
  });

  // A service on a repository the workspace's GitHub connection grants used
  // to open Update Source on the Public Git URL tab with its private URL in
  // the field, as if it had been pasted there, and nothing on the GitHub tab.
  describe("a repository the workspace's GitHub connection grants", () => {
    const connected = (overrides: { repo?: string; branch?: string } = {}) => (
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo={overrides.repo ?? "https://github.com/acme/api"}
        branch={overrides.branch ?? "main"}
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />
    );

    it("opens on the GitHub tab with the repository selected and nothing to save", async () => {
      const user = userEvent.setup();
      render(connected());
      await user.click(screen.getByRole("button", { name: "Edit" }));

      expect(screen.getByRole("tab", { name: "GitHub" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      expect(
        screen.getByRole("button", { name: /acme\/api/i }),
      ).toHaveAttribute("aria-pressed", "true");
      expect(
        screen.getByRole("button", { name: /puncsky\/site/i }),
      ).toHaveAttribute("aria-pressed", "false");
      expect(screen.getByRole("combobox", { name: "Branch" })).toHaveValue(
        "main",
      );
      expect(screen.getByRole("button", { name: "Deploy" })).toBeDisabled();

      // Its URL is not offered as a public one.
      await user.click(screen.getByRole("tab", { name: /Public Git/ }));
      expect(
        screen.getByRole("textbox", {
          name: /Git repository URL|Repository URL/,
        }),
      ).toHaveValue("");
      expect(screen.getByRole("button", { name: "Deploy" })).toBeDisabled();
    });

    it("changes the branch alone, keeping the URL the service stores", async () => {
      const user = userEvent.setup();
      render(connected({ repo: "https://github.com/acme/api.git" }));
      await user.click(screen.getByRole("button", { name: "Edit" }));

      // The stored clone URL ends in .git; GitHub's page URL does not.
      expect(
        screen.getByRole("button", { name: /acme\/api/i }),
      ).toHaveAttribute("aria-pressed", "true");
      const branch = screen.getByRole("combobox", { name: "Branch" });
      await user.clear(branch);
      await user.type(branch, "release");
      await user.click(screen.getByRole("button", { name: "Deploy" }));

      await waitFor(() =>
        expect(setRepo).toHaveBeenCalledWith("srv-1", {
          repo: "https://github.com/acme/api.git",
          branch: "release",
          runtime: "docker",
          deploy: true,
        }),
      );
    });

    it("picking the same repository again changes nothing, and another one is a change", async () => {
      const user = userEvent.setup();
      render(
        connected({
          repo: "https://github.com/acme/api.git",
          branch: "release",
        }),
      );
      await user.click(screen.getByRole("button", { name: "Edit" }));
      const save = screen.getByRole("button", { name: "Deploy" });

      // Its own repository keeps the branch it deploys, not the default one.
      await user.click(screen.getByRole("button", { name: /acme\/api/i }));
      expect(screen.getByRole("combobox", { name: "Branch" })).toHaveValue(
        "release",
      );
      expect(save).toBeDisabled();

      await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));
      expect(
        screen.getByRole("button", { name: /acme\/api/i }),
      ).toHaveAttribute("aria-pressed", "false");
      expect(screen.getByRole("combobox", { name: "Branch" })).toHaveValue(
        "trunk",
      );
      await user.click(save);
      await waitFor(() =>
        expect(setRepo).toHaveBeenCalledWith("srv-1", {
          repo: "https://github.com/puncsky/site",
          branch: "trunk",
          runtime: "docker",
          deploy: true,
        }),
      );
    });
  });

  // A disabled primary action with no reason beside it read as a broken form,
  // and an enabled one never said what it was about to save.
  it("says why Deploy is disabled, then what it will deploy", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    const save = screen.getByRole("button", { name: "Deploy" });
    expect(save).toBeDisabled();
    expect(save).toHaveAccessibleDescription("No changes to save.");

    const branch = screen.getByRole("combobox", { name: "Branch" });
    await user.clear(branch);
    await user.type(branch, "release");
    expect(save).toBeEnabled();
    expect(screen.queryByText("No changes to save.")).toBeNull();
    expect(save).toHaveAccessibleDescription("New source: acme/api · release");
  });

  // The choice is a check mark on one row of a list that scrolls and filters.
  // Filtered out of sight, the footer is the only thing that still names it.
  it("still names the picked repository when a search hides its row", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));
    await user.type(
      screen.getByRole("textbox", { name: "Search repositories…" }),
      "zzzz",
    );
    expect(screen.queryByRole("button", { name: /puncsky\/site/i })).toBeNull();
    const save = screen.getByRole("button", { name: "Deploy" });
    expect(save).toBeEnabled();
    expect(save).toHaveAccessibleDescription(
      "New source: puncsky/site · trunk",
    );
  });

  it("names a typed URL with its host, which a GitHub pick does not need", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: /Public Git/ }));
    await user.type(
      screen.getByRole("textbox", {
        name: /Git repository URL|Repository URL/,
      }),
      "https://forge.example/acme/api.git",
    );
    expect(
      screen.getByRole("button", { name: "Deploy" }),
    ).toHaveAccessibleDescription(
      "New source: forge.example · acme / api · main",
    );
  });

  // The Branch field came and went with each keystroke of the URL, and the
  // buttons under it moved with it. It stays, and waits for a repository.
  it("keeps the Branch field in place, disabled until there is a repository", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: /Public Git/ }));
    const url = screen.getByRole("textbox", {
      name: /Git repository URL|Repository URL/,
    });
    const branch = () => screen.getByRole("combobox", { name: "Branch" });
    const save = screen.getByRole("button", { name: "Deploy" });
    expect(branch()).toBeDisabled();
    expect(save).toHaveAccessibleDescription("Enter a repository URL.");

    await user.type(url, "not a repository");
    expect(branch()).toBeDisabled();
    expect(save).toHaveAccessibleDescription(
      "Complete the fields above to continue.",
    );
    await user.clear(url);
    await user.type(url, "https://github.com/elsewhere/public");
    expect(branch()).toBeEnabled();
    expect(save).toBeEnabled();
  });

  // Each source is a different height. The dialog's height is `auto` before
  // and after a switch, which CSS does not ease between, so it is animated
  // from the height it had, once the browser reports the new one.
  it("eases the dialog from one source's height to the next, unless motion is reduced", async () => {
    const user = userEvent.setup();
    const height = vi
      .spyOn(HTMLElement.prototype, "offsetHeight", "get")
      .mockImplementation(function (this: HTMLElement) {
        if (!this.matches('[role="dialog"]')) return 0;
        const selected = this.querySelector(
          '[role="tab"][aria-selected="true"]',
        );
        return selected?.textContent === "GitHub" ? 696 : 438;
      });
    const animate = vi.fn();
    const originalAnimate = Element.prototype.animate;
    Element.prototype.animate = animate;
    const resized: (() => void)[] = [];
    const OriginalResizeObserver = globalThis.ResizeObserver;
    globalThis.ResizeObserver = class {
      constructor(callback: ResizeObserverCallback) {
        resized.push(() => callback([], this as unknown as ResizeObserver));
      }
      observe() {}
      unobserve() {}
      disconnect() {}
    };
    const layout = () => act(() => resized.forEach((report) => report()));
    const matchMedia = window.matchMedia;
    try {
      render(
        <ServiceSourceCard
          serviceId="srv-1"
          serviceType="web_service"
          repo="https://github.com/acme/api"
          branch="main"
          runtime="docker"
          imagePath={null}
          registryCredentialId={null}
          buildSourceId={null}
          startCommand={null}
          pushDeliveryMethod="github_app"
        />,
      );
      await user.click(screen.getByRole("button", { name: "Edit" }));
      const dialog = screen.getByRole("dialog");
      layout();
      expect(animate).not.toHaveBeenCalled();

      await user.click(screen.getByRole("tab", { name: /Public Git/ }));
      layout();
      expect(animate).toHaveBeenCalledWith(
        [{ height: "696px" }, { height: "438px" }],
        { duration: 200, easing: "ease-out" },
      );
      expect(animate.mock.contexts[0]).toBe(dialog);

      animate.mockClear();
      window.matchMedia = (query) =>
        ({
          matches: query.includes("prefers-reduced-motion"),
          media: query,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
        }) as unknown as MediaQueryList;
      await user.click(screen.getByRole("tab", { name: "GitHub" }));
      layout();
      expect(animate).not.toHaveBeenCalled();
    } finally {
      height.mockRestore();
      window.matchMedia = matchMedia;
      globalThis.ResizeObserver = OriginalResizeObserver;
      Element.prototype.animate = originalAnimate;
    }
  });

  it("asks for a repository on the GitHub tab before one is picked", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-2"
        serviceType="web_service"
        repo={null}
        branch={null}
        imagePath="nginx:stable"
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    expect(
      screen.getByRole("button", { name: "Deploy" }),
    ).toHaveAccessibleDescription("Select a repository.");
    await user.click(screen.getByRole("button", { name: /acme\/api/i }));
    expect(screen.queryByText("Select a repository.")).toBeNull();
    expect(screen.getByRole("button", { name: "Deploy" })).toBeEnabled();
  });

  it("opens a repository the GitHub connection does not grant as its URL", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        serviceType="web_service"
        repo="https://github.com/elsewhere/public"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="manual_webhook"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));

    expect(screen.getByRole("tab", { name: /Public Git/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByDisplayValue("https://github.com/elsewhere/public"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    for (const row of [/acme\/api/i, /puncsky\/site/i]) {
      expect(screen.getByRole("button", { name: row })).toHaveAttribute(
        "aria-pressed",
        "false",
      );
    }
  });

  it("repoints an image, forwards its registry credential, deploys, and navigates", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-2"
        serviceType="web_service"
        repo={null}
        branch={null}
        imagePath="nginx:stable"
        registryCredentialId="rgc-1"
        buildSourceId={null}
        startCommand={null}
      />,
    );

    expect(screen.getByText("nginx:stable")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByDisplayValue("nginx:stable");
    await user.clear(input);
    await user.type(input, "nginx:1.27");
    await user.click(screen.getByRole("button", { name: "Connect" }));

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith(
        "srv-2",
        "nginx:1.27",
        "rgc-1",
        true,
      ),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "srv-2", deployId: "dep-img-1" },
    });
  });

  it("switches an image-backed service to a connected Docker repo and deploys", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-3"
        serviceType="web_service"
        repo={null}
        branch={null}
        imagePath="nginx:stable"
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    await user.click(screen.getByRole("button", { name: /acme\/api/i }));
    await user.click(screen.getByRole("button", { name: "Deploy" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-3", {
        repo: "https://github.com/acme/api",
        branch: "main",
        runtime: "docker",
        deploy: true,
      }),
    );
    expect(
      screen.queryByTestId("source-native-commands"),
    ).not.toBeInTheDocument();
  });

  // w32/052 + w4/m593: an image service that built natively defaults back to a
  // native runtime and requires both build and start commands.
  it("asks an image service that built natively for its commands to switch back to Git", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-5"
        serviceType="web_service"
        repo=""
        branch=""
        imagePath="nginx:stable"
        registryCredentialId={null}
        buildSourceId={null}
        startCommand=""
        builder="native"
        buildCommand=""
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(
      screen.queryByTestId("source-native-commands"),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    await user.click(screen.getByRole("button", { name: /acme\/api/i }));

    expect(screen.getByTestId("source-native-commands")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Branch" })).toHaveValue(
      "main",
    );
    const deploy = screen.getByRole("button", { name: "Deploy" });
    expect(deploy).toBeDisabled();
    await user.type(screen.getByLabelText("Build Command"), "yarn");
    expect(deploy).toBeDisabled();
    await user.type(screen.getByLabelText("Start Command"), "node app.js");
    await user.click(deploy);

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-5", {
        repo: "https://github.com/acme/api",
        branch: "main",
        runtime: "node",
        buildCommand: "yarn",
        startCommand: "node app.js",
        deploy: true,
      }),
    );
  });

  // w4/m593: switching runtime in Update Source toggles native commands vs Docker.
  it("reconfirms Runtime and commands when moving between Docker and native runtimes", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-rt"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        builder="dockerfile"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        buildCommand={null}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(
      screen.queryByTestId("source-native-commands"),
    ).not.toBeInTheDocument();

    // Switch Runtime from Docker to Python: native commands appear pre-seeded.
    await user.click(screen.getByRole("combobox", { name: "Runtime" }));
    await user.click(screen.getByRole("option", { name: "Python 3" }));
    const build = screen.getByLabelText("Build Command");
    const start = screen.getByLabelText("Start Command");
    expect(build).toHaveValue("pip install -r requirements.txt");
    expect(start).toHaveValue("gunicorn app:app");

    await user.click(screen.getByRole("button", { name: "Deploy" }));
    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-rt", {
        repo: "https://github.com/acme/api",
        branch: "main",
        runtime: "python",
        buildCommand: "pip install -r requirements.txt",
        startCommand: "gunicorn app:app",
        deploy: true,
      }),
    );
  });

  // w4/m593: cron jobs label the native command "Command" and submit `command`.
  it("labels a cron job's native command field as Command and deploys on submit", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-cron"
        serviceType="cron_job"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="node"
        builder="native"
        buildCommand="npm ci"
        startCommand="node cron.js"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        pushDeliveryMethod="github_app"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    const cmd = screen.getByLabelText("Command");
    expect(cmd).toHaveValue("node cron.js");
    await user.clear(cmd);
    await user.type(cmd, "node nightly.js");
    await user.click(screen.getByRole("button", { name: "Deploy" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-cron", {
        repo: "https://github.com/acme/api",
        branch: "main",
        runtime: "node",
        buildCommand: "npm ci",
        command: "node nightly.js",
        deploy: true,
      }),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "srv-cron", deployId: "dep-git-1" },
    });
  });

  // w4/m593: static sites show only Git tabs, reconfirm Build Command (optional)
  // + Publish Directory (required), deploy on submit, and navigate under /static.
  it("limits static-site Update Source to Git tabs, validates Publish Directory, and deploys under /static", async () => {
    currentBase = "/static";
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-site"
        serviceType="static_site"
        repo="https://github.com/acme/api"
        branch="main"
        buildCommand="npm run build"
        publishPath="dist"
        rootDir=""
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(
      screen.getByText(/Select a Git repository to back this static site/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "GitHub" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Public Git/ })).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: /Existing Image/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: /Build source/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("combobox", { name: "Runtime" }),
    ).not.toBeInTheDocument();

    const build = screen.getByLabelText("Build Command");
    const publish = screen.getByLabelText("Publish directory");
    const deploy = screen.getByRole("button", { name: "Deploy" });
    expect(build).toHaveValue("npm run build");
    expect(publish).toHaveValue("dist");
    expect(deploy).toBeDisabled();

    // Clearing Build Command is allowed (optional), clearing Publish Directory is refused.
    await user.clear(build);
    expect(deploy).toBeEnabled();

    await user.clear(publish);
    expect(deploy).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter a publish directory.",
    );

    // Escaping repo root via ../ is rejected inline.
    await user.type(publish, "../outside");
    expect(deploy).toBeDisabled();
    expect(screen.getByRole("alert")).toBeInTheDocument();

    await user.clear(publish);
    await user.type(publish, "public");
    expect(deploy).toBeEnabled();
    await user.click(deploy);

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-site", {
        repo: "https://github.com/acme/api",
        branch: "main",
        buildCommand: "",
        publishPath: "public",
        deploy: true,
      }),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/static/$serviceId/deploys/$deployId",
      params: { serviceId: "srv-site", deployId: "dep-git-1" },
    });
  });

  // Suspended service or unbuilt build source: server saves cleanly and returns
  // deployId: null — dialog closes without navigating.
  it("closes without navigating when the backend saves without opening a deploy", async () => {
    const user = userEvent.setup();
    setRepo.mockResolvedValueOnce({ ok: true, deployId: null });
    render(
      <ServiceSourceCard
        serviceId="srv-susp"
        serviceType="web_service"
        repo="https://github.com/acme/api"
        branch="main"
        runtime="docker"
        imagePath={null}
        registryCredentialId={null}
        buildSourceId={null}
        startCommand={null}
        pushDeliveryMethod="github_app"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));
    await user.click(screen.getByRole("button", { name: "Deploy" }));

    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(navigate).not.toHaveBeenCalled();
  });
});

// w4/275: bex-api sends an image-backed service's repo as "", which the
// dirty check read as a repository, so an untouched image draft enabled
// Connect and re-saved the same image.
describe("ServiceSourceCard — unchanged image source (w4/275)", () => {
  const card = (repo: string | null) => (
    <ServiceSourceCard
      serviceId="srv-4"
      serviceType="web_service"
      repo={repo}
      branch={repo === null ? null : ""}
      imagePath="docker.io/library/busybox:1.37.0"
      registryCredentialId={null}
      buildSourceId={null}
      startCommand={null}
    />
  );
  const connect = () => screen.getByRole("button", { name: "Connect" });

  it.each([
    ["an empty-string repo", ""],
    ["a null repo", null],
  ])(
    "keeps Connect disabled for %s until the image really changes",
    async (_label, repo) => {
      const user = userEvent.setup();
      render(card(repo));

      await user.click(screen.getByRole("button", { name: "Edit" }));
      const input = screen.getByDisplayValue(
        "docker.io/library/busybox:1.37.0",
      );
      expect(connect()).toBeDisabled();

      await user.type(input, "  ");
      expect(connect()).toBeDisabled();

      await user.clear(input);
      expect(connect()).toBeDisabled();

      await user.type(input, "docker.io/library/busybox:1.36.1");
      expect(connect()).toBeEnabled();

      await user.click(screen.getByRole("button", { name: "Cancel" }));
      await user.click(screen.getByRole("button", { name: "Edit" }));
      expect(
        screen.getByDisplayValue("docker.io/library/busybox:1.37.0"),
      ).toBeInTheDocument();
      expect(connect()).toBeDisabled();
      expect(setImage).not.toHaveBeenCalled();
    },
  );

  it("keeps a refused image change open for a retry", async () => {
    const user = userEvent.setup();
    setImage.mockResolvedValueOnce({ ok: false });
    render(card(""));

    await user.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByDisplayValue("docker.io/library/busybox:1.37.0");
    await user.clear(input);
    await user.type(input, "docker.io/library/busybox:1.36.1");
    await user.click(connect());

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith(
        "srv-4",
        "docker.io/library/busybox:1.36.1",
        undefined,
        true,
      ),
    );
    expect(
      screen.getByDisplayValue("docker.io/library/busybox:1.36.1"),
    ).toBeInTheDocument();
    expect(connect()).toBeEnabled();
    expect(navigate).not.toHaveBeenCalled();
  });

  it("still enables a switch from an empty-string repo image service to a repo", async () => {
    const user = userEvent.setup();
    render(card(""));

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    await user.click(screen.getByRole("button", { name: /acme\/api/i }));
    expect(screen.getByRole("button", { name: "Deploy" })).toBeEnabled();
  });
});

// w2/m254 t008 + w4/m593: Update Source links a service to a build source and deploys.
describe("ServiceSourceCard — build sources", () => {
  const card = (buildSourceId: string | null) => (
    <ServiceSourceCard
      serviceId="srv-5"
      serviceType="web_service"
      repo={buildSourceId ? null : "https://github.com/acme/api"}
      branch={buildSourceId ? null : "main"}
      runtime="docker"
      imagePath={null}
      registryCredentialId={null}
      buildSourceId={buildSourceId}
      startCommand="node server.js"
    />
  );

  it("shows the linked build source and what it builds from", () => {
    render(card("bds-mono"));
    expect(screen.getByText("Build source")).toBeInTheDocument();
    expect(screen.getByText("Mono")).toBeInTheDocument();
    expect(screen.getByText("github.com · acme / mono")).toBeInTheDocument();
  });

  it("links a native build source with the service's start command, deploys, and navigates", async () => {
    const user = userEvent.setup();
    render(card(null));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "Build source" }));
    await user.click(screen.getByRole("combobox", { name: "Build source" }));
    await user.click(screen.getByRole("option", { name: /Mono/ }));
    const start = screen.getByLabelText("Start Command");
    expect(start).toHaveValue("node server.js");
    await user.clear(start);
    const save = screen.getByRole("button", { name: "Deploy" });
    expect(save).toBeDisabled();
    await user.type(start, "npm start");
    await user.click(save);
    await waitFor(() =>
      expect(linkBuildSource).toHaveBeenCalledWith(
        "srv-5",
        "bds-mono",
        "npm start",
        true,
      ),
    );
    expect(navigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "srv-5", deployId: "dep-bds-1" },
    });
  });

  it("unlinks by switching to a public Git repository", async () => {
    const user = userEvent.setup();
    render(card("bds-mono"));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("tab", { name: "Build source" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    const save = screen.getByRole("button", { name: "Deploy" });
    expect(save).toBeDisabled();
    await user.click(screen.getByRole("tab", { name: /Public Git/ }));
    await user.type(
      screen.getByRole("textbox", {
        name: /Git repository URL|Repository URL/,
      }),
      "https://github.com/acme/api",
    );
    await user.click(save);
    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-5", {
        repo: "https://github.com/acme/api",
        branch: "main",
        runtime: "docker",
        deploy: true,
      }),
    );
    expect(linkBuildSource).not.toHaveBeenCalled();
  });

  it("says the list failed rather than that there are no sources", async () => {
    buildSourcesError = new Error("network");
    const user = userEvent.setup();
    render(card(null));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "Build source" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load this workspace's build sources",
    );
  });

  it("links a Docker build source without a start command", async () => {
    const user = userEvent.setup();
    render(card(null));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "Build source" }));
    await user.click(screen.getByRole("combobox", { name: "Build source" }));
    await user.click(screen.getByRole("option", { name: /Docker/ }));
    expect(screen.queryByLabelText("Start Command")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Deploy" }));
    await waitFor(() =>
      expect(linkBuildSource).toHaveBeenCalledWith(
        "srv-5",
        "bds-docker",
        undefined,
        true,
      ),
    );
  });

  it("displays linked Build Source name, kind badge, copyable ID, and Builds from source (w2/m612 t005)", () => {
    render(
      <ServiceSourceCard
        serviceId="srv-linked"
        serviceType="web_service"
        repo={null}
        branch={null}
        imagePath={null}
        registryCredentialId={null}
        buildSourceId="bds-mono"
        startCommand="node server.js"
      />,
    );
    expect(screen.getByText("Mono")).toBeInTheDocument();
    expect(screen.getByText("Git repository")).toBeInTheDocument();
    expect(screen.getByText("bds-mono")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy Build Source ID" }),
    ).toBeInTheDocument();
    expect(screen.getByText("github.com · acme / mono")).toBeInTheDocument();
  });
});
