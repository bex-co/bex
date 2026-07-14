import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mockUseDeployHook = vi.fn();
vi.mock("@/features/services/hooks/use-deploy-hook", () => ({
  useDeployHook: (...args: unknown[]) => mockUseDeployHook(...args),
}));

const mockCopy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy: mockCopy }),
}));

import { DeployHookSection } from "@/features/services/components/deploy-hook-section";

const hookURL = "https://api.bex.co/v1/deploy-hooks/dhk-secret";
const regenerate = vi.fn();

function hookResult(
  over: Partial<{
    url: string | null;
    loading: boolean;
    error: Error | undefined;
    regenerating: boolean;
  }> = {},
) {
  return {
    url: hookURL,
    loading: false,
    error: undefined,
    regenerate,
    regenerating: false,
    ...over,
  };
}

beforeEach(() => {
  mockUseDeployHook.mockReset();
  regenerate.mockReset().mockResolvedValue(true);
  mockCopy.mockReset().mockResolvedValue(undefined);
  mockUseDeployHook.mockReturnValue(hookResult());
});

describe("DeployHookSection", () => {
  it("masks the credential until the user reveals it", async () => {
    const user = userEvent.setup();
    render(<DeployHookSection serviceId="web" />);

    const field = screen.getByLabelText("Deploy Hook URL");
    expect(field).not.toHaveValue(hookURL);
    expect(screen.queryByDisplayValue(hookURL)).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Reveal Deploy Hook URL" }),
    );
    expect(field).toHaveValue(hookURL);
    expect(
      screen.getByRole("button", { name: "Hide Deploy Hook URL" }),
    ).toBeInTheDocument();
  });

  it("copies the full URL even while the field is masked", async () => {
    const user = userEvent.setup();
    render(<DeployHookSection serviceId="web" />);

    await user.click(
      screen.getByRole("button", { name: "Copy Deploy Hook URL" }),
    );
    await waitFor(() => expect(mockCopy).toHaveBeenCalledWith(hookURL));
  });

  it("warns that integrations break before rotating", async () => {
    const user = userEvent.setup();
    render(<DeployHookSection serviceId="web" />);

    await user.click(screen.getByRole("button", { name: "Regenerate Hook" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText(/current URL will stop working immediately/i),
    ).toBeInTheDocument();

    await user.click(
      within(dialog).getByRole("button", { name: "Regenerate" }),
    );
    await waitFor(() => expect(regenerate).toHaveBeenCalledOnce());
  });

  it("shows an honest load failure without a copyable value", () => {
    mockUseDeployHook.mockReturnValue(
      hookResult({ url: null, error: new Error("forbidden") }),
    );
    render(<DeployHookSection serviceId="web" />);

    expect(
      screen.getByText("Couldn't load the Deploy Hook URL."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy Deploy Hook URL" }),
    ).not.toBeInTheDocument();
  });
});
