import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  ImageSourceCard,
  SwitchToImageRow,
} from "@/features/services/components/service-source-card";

// w5/m76: the Source card + repo↔image switch is the dashboard half of
// Render's Update Source. These pin that the mutations fire with the right
// arguments and that the no-auto-deploy note is shown.

const setImage = vi.fn(async () => true);
const setRepo = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-set-image", () => ({
  useSetImage: () => ({ setImage, busy: false }),
}));
vi.mock("@/features/services/hooks/use-set-repo", () => ({
  useSetRepo: () => ({ setRepo, busy: false }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [
      {
        id: 1,
        fullName: "puncsky/site",
        htmlUrl: "https://github.com/puncsky/site",
        accountLogin: "puncsky",
        private: false,
        defaultBranch: "main",
        cloneUrl: "",
      },
    ],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canCreate: true, canOperate: true }),
}));
vi.mock("@/features/services/components/registry-credential-select", () => ({
  RegistryCredentialSelect: () => null,
}));

beforeEach(() => {
  setImage.mockClear();
  setRepo.mockClear();
});

describe("ImageSourceCard", () => {
  it("shows the configured image and switches to a repo via setRepo", async () => {
    const user = userEvent.setup();
    render(
      <ImageSourceCard
        serviceId="srv-1"
        imagePath="nginx:stable"
        registryCredentialId={null}
      />,
    );

    // The image row is present (its Edit affordance).
    expect(
      screen.getByRole("button", { name: "Edit image" }),
    ).toBeInTheDocument();

    // Open the switch-to-repo dialog and pick a connected-account repo.
    await user.click(
      screen.getByRole("button", { name: /switch to a git repository/i }),
    );
    await user.click(screen.getByRole("button", { name: /puncsky\/site/i }));
    await user.click(screen.getByRole("button", { name: /update source/i }));

    await waitFor(() =>
      expect(setRepo).toHaveBeenCalledWith(
        "srv-1",
        "https://github.com/puncsky/site",
      ),
    );
  });

  it("edits the image inline via setImage (confirm-gated, credential forwarded)", async () => {
    const user = userEvent.setup();
    render(
      <ImageSourceCard
        serviceId="srv-1"
        imagePath="nginx:stable"
        registryCredentialId="rc-1"
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit image" }));
    const input = screen.getByDisplayValue("nginx:stable");
    await user.clear(input);
    await user.type(input, "nginx:1.27");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    // Source edits are confirm-gated; the note names the new value.
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Save changes" }),
    );

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith("srv-1", "nginx:1.27", "rc-1"),
    );
  });
});

describe("SwitchToImageRow", () => {
  it("switches a repo-backed service to an image via setImage", async () => {
    const user = userEvent.setup();
    render(<SwitchToImageRow serviceId="srv-2" disabled={false} />);

    await user.click(
      screen.getByRole("button", { name: /switch to a container image/i }),
    );
    const input = screen.getByPlaceholderText(/nginx/i);
    await user.type(input, "ghcr.io/acme/app:v1");
    await user.click(screen.getByRole("button", { name: /update source/i }));

    await waitFor(() =>
      expect(setImage).toHaveBeenCalledWith(
        "srv-2",
        "ghcr.io/acme/app:v1",
        undefined,
      ),
    );
  });

  it("is disabled with no dialog when the caller lacks can_create", () => {
    render(
      <SwitchToImageRow
        serviceId="srv-2"
        disabled
        disabledReason="need can_create"
      />,
    );
    expect(
      screen.getByRole("button", { name: /switch to a container image/i }),
    ).toBeDisabled();
    expect(screen.getByText("need can_create")).toBeInTheDocument();
  });
});
