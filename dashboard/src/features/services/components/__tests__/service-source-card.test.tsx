import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceSourceCard } from "@/features/services/components/service-source-card";

const setImage = vi.fn(async () => true);
const setRepo = vi.fn(async () => true);
const linkBuildSource = vi.fn(async () => true);
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
  useLinkBuildSource: () => ({ linkBuildSource, busy: false }),
}));
vi.mock("@/features/services/hooks/use-set-image", () => ({
  useSetImage: () => ({ setImage, busy: false }),
}));
vi.mock("@/features/services/hooks/use-set-repo", () => ({
  useSetRepo: () => ({ setRepo, busy: false }),
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
  useConnectGit: () => ({ connect: vi.fn(), busy: false }),
}));
vi.mock("@/features/git/hooks/use-claim-git", () => ({
  useClaimGit: () => ({ claim: vi.fn(), busy: false }),
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
  setImage.mockClear();
  setRepo.mockClear();
  linkBuildSource.mockClear();
  buildSourcesError = undefined;
});

describe("ServiceSourceCard", () => {
  it("keeps the source draft open when the backend rejects access", async () => {
    const user = userEvent.setup();
    setRepo.mockResolvedValueOnce(false);
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        repo="https://github.com/acme/api"
        branch="main"
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
    await user.click(screen.getByRole("button", { name: "Update source" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-1", {
        repo: "https://github.com/unconnected/private",
        branch: "main",
      }),
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(input).toHaveValue("https://github.com/unconnected/private");
    expect(setImage).not.toHaveBeenCalled();
  });

  it("shows a repo and branch, then atomically repoints both from the grouped picker", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        repo="https://github.com/acme/api"
        branch="main"
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
      screen.getByText(/changes aren't deployed automatically/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Update source" }),
    ).toBeDisabled();
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(screen.getByText("puncsky")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));

    const branch = screen.getByRole("combobox", { name: "Branch" });
    expect(branch).toHaveValue("trunk");
    await user.clear(branch);
    await user.type(branch, "release");
    await user.click(screen.getByRole("button", { name: "Update source" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-1", {
        repo: "https://github.com/puncsky/site",
        branch: "release",
      }),
    );
  });

  it("repoints an image and forwards its registry credential", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-2"
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
    await user.click(screen.getByRole("button", { name: "Update source" }));

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith("srv-2", "nginx:1.27", "rgc-1"),
    );
  });

  it("switches an image-backed service to a connected repo and its default branch", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-3"
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
    await user.click(screen.getByRole("button", { name: "Update source" }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith("srv-3", {
        repo: "https://github.com/acme/api",
        branch: "main",
      }),
    );
  });
});

// w4/275: bex-api sends an image-backed service's repo as "", which the
// dirty check read as a repository, so an untouched image draft enabled
// Update source and re-saved the same image.
describe("ServiceSourceCard — unchanged image source (w4/275)", () => {
  const card = (repo: string | null) => (
    <ServiceSourceCard
      serviceId="srv-4"
      repo={repo}
      branch={repo === null ? null : ""}
      imagePath="docker.io/library/busybox:1.37.0"
      registryCredentialId={null}
      buildSourceId={null}
      startCommand={null}
    />
  );
  const update = () => screen.getByRole("button", { name: "Update source" });

  it.each([
    ["an empty-string repo", ""],
    ["a null repo", null],
  ])(
    "keeps Update source disabled for %s until the image really changes",
    async (_label, repo) => {
      const user = userEvent.setup();
      render(card(repo));

      await user.click(screen.getByRole("button", { name: "Edit" }));
      const input = screen.getByDisplayValue(
        "docker.io/library/busybox:1.37.0",
      );
      expect(update()).toBeDisabled();

      await user.type(input, "  ");
      expect(update()).toBeDisabled();

      await user.clear(input);
      expect(update()).toBeDisabled();

      await user.type(input, "docker.io/library/busybox:1.36.1");
      expect(update()).toBeEnabled();

      await user.click(screen.getByRole("button", { name: "Cancel" }));
      await user.click(screen.getByRole("button", { name: "Edit" }));
      expect(
        screen.getByDisplayValue("docker.io/library/busybox:1.37.0"),
      ).toBeInTheDocument();
      expect(update()).toBeDisabled();
      expect(setImage).not.toHaveBeenCalled();
    },
  );

  it("keeps a refused image change open for a retry", async () => {
    const user = userEvent.setup();
    setImage.mockResolvedValueOnce(false);
    render(card(""));

    await user.click(screen.getByRole("button", { name: "Edit" }));
    const input = screen.getByDisplayValue("docker.io/library/busybox:1.37.0");
    await user.clear(input);
    await user.type(input, "docker.io/library/busybox:1.36.1");
    await user.click(update());

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith(
        "srv-4",
        "docker.io/library/busybox:1.36.1",
        undefined,
      ),
    );
    expect(
      screen.getByDisplayValue("docker.io/library/busybox:1.36.1"),
    ).toBeInTheDocument();
    expect(update()).toBeEnabled();
  });

  it("still enables a switch from an empty-string repo image service to a repo", async () => {
    const user = userEvent.setup();
    render(card(""));

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "GitHub" }));
    await user.click(screen.getByRole("button", { name: /acme\/api/i }));
    expect(update()).toBeEnabled();
  });
});

// w2/m254 t008: Update Source links a service to a build source, as on Render.
describe("ServiceSourceCard — build sources", () => {
  const card = (buildSourceId: string | null) => (
    <ServiceSourceCard
      serviceId="srv-5"
      repo={buildSourceId ? null : "https://github.com/acme/api"}
      branch={buildSourceId ? null : "main"}
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

  it("links a native build source with the service's start command", async () => {
    const user = userEvent.setup();
    render(card(null));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("tab", { name: "Build source" }));
    await user.click(screen.getByRole("combobox", { name: "Build source" }));
    await user.click(screen.getByRole("option", { name: /Mono/ }));
    const start = screen.getByLabelText("Start Command");
    expect(start).toHaveValue("node server.js");
    await user.clear(start);
    const save = screen.getByRole("button", { name: "Update source" });
    expect(save).toBeDisabled();
    await user.type(start, "npm start");
    await user.click(save);
    await waitFor(() =>
      expect(linkBuildSource).toHaveBeenCalledWith(
        "srv-5",
        "bds-mono",
        "npm start",
      ),
    );
  });

  it("unlinks by switching to a public Git repository", async () => {
    const user = userEvent.setup();
    render(card("bds-mono"));
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("tab", { name: "Build source" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    const save = screen.getByRole("button", { name: "Update source" });
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
    await user.click(screen.getByRole("button", { name: "Update source" }));
    await waitFor(() =>
      expect(linkBuildSource).toHaveBeenCalledWith(
        "srv-5",
        "bds-docker",
        undefined,
      ),
    );
  });
});
