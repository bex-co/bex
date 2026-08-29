import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceSourceCard } from "@/features/services/components/service-source-card";

const setImage = vi.fn(async () => true);
const setRepo = vi.fn(async () => true);

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

vi.mock("@/features/services/hooks/use-set-image", () => ({
  useSetImage: () => ({ setImage, busy: false }),
}));
vi.mock("@/features/services/hooks/use-set-repo", () => ({
  useSetRepo: () => ({ setRepo, busy: false }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({ repos, loading: false, error: undefined }),
}));
vi.mock("@/features/services/hooks/use-repo-branches", () => ({
  useRepoBranches: () => ({ branches: ["main", "release"], loading: false }),
}));
vi.mock("@/features/git/hooks/use-git-connection", () => ({
  useGitConnection: () => ({
    connection: { connected: true },
    loading: false,
  }),
}));
vi.mock("@/features/git/hooks/use-connect-git", () => ({
  useConnectGit: () => ({ connect: vi.fn(), busy: false }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canCreate: true }),
}));
vi.mock("@/features/services/components/registry-credential-select", () => ({
  RegistryCredentialSelect: () => null,
}));

beforeEach(() => {
  setImage.mockClear();
  setRepo.mockClear();
});

describe("ServiceSourceCard", () => {
  it("shows a repo and branch, then atomically repoints both from the grouped picker", async () => {
    const user = userEvent.setup();
    render(
      <ServiceSourceCard
        serviceId="srv-1"
        repo="https://github.com/acme/api"
        branch="main"
        imagePath={null}
        registryCredentialId={null}
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
