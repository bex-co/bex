import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DisplayNameRow } from "@/features/services/components/display-name-row";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CREATE_REASON, mockCapabilities } from "@/test/mocks/capabilities";

const setDisplayName = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-display-name", () => ({
  useDisplayName: () => ({ setDisplayName, busy: false }),
}));

beforeEach(() => {
  setDisplayName.mockClear();
  setDisplayName.mockResolvedValue(true);
});

describe("DisplayNameRow", () => {
  it("falls back to the service name, never the id, when no display name is set (w5/m48)", () => {
    render(
      <DisplayNameRow
        serviceId="srv-stable-id"
        displayName={null}
        name="docs-site"
      />,
    );

    // Render prefills its Name field with the service name (in a disabled
    // input); the opaque id stays in the hint only.
    const input = screen.getByRole("textbox", { name: "Service Name" });
    expect(input).toHaveValue("docs-site");
    expect(input).toBeDisabled();
    expect(input).not.toHaveValue("srv-stable-id");
    expect(
      screen.getByText(
        "The service ID remains srv-stable-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("falls back to the immutable id only while the name is unknown", () => {
    render(<DisplayNameRow serviceId="stable-id" displayName={null} />);

    expect(screen.getByRole("textbox", { name: "Service Name" })).toHaveValue(
      "stable-id",
    );
    expect(
      screen.getByText(
        "The service ID remains stable-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("clearing an already-empty label back to the id fallback is not a change", async () => {
    const user = userEvent.setup();
    render(
      <DisplayNameRow
        serviceId="stable-id"
        displayName={null}
        name="docs-site"
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    await user.clear(screen.getByRole("textbox", { name: "Service Name" }));
    // Emptying the prefilled fallback isn't a rename, so Save stays disabled.
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("sets a trimmed human label without changing the service id", async () => {
    const user = userEvent.setup();
    render(<DisplayNameRow serviceId="stable-id" displayName={null} />);

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    const input = screen.getByRole("textbox", { name: "Service Name" });
    await user.clear(input);
    await user.type(input, "  Customer API  ");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setDisplayName).toHaveBeenCalledWith("stable-id", "Customer API");
  });

  it("clears a custom label so the immutable-id fallback can return", async () => {
    const user = userEvent.setup();
    render(<DisplayNameRow serviceId="stable-id" displayName="Customer API" />);

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    await user.clear(screen.getByRole("textbox", { name: "Service Name" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setDisplayName).toHaveBeenCalledWith("stable-id", "");
  });
});

// A rename is a service setting: can_create, as Render's role table gives a
// Contributor no "Modify service configuration" (w9/m619).
describe("DisplayNameRow role gating (w9/m619)", () => {
  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("holds the rename for a contributor with the role's reason", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    render(<DisplayNameRow serviceId="srv-1" displayName="API" name="api" />);
    expect(
      screen.queryByRole("button", { name: "Edit service name" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(CREATE_REASON)).toBeInTheDocument();
  });

  it("offers the rename to a developer", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    render(<DisplayNameRow serviceId="srv-1" displayName="API" name="api" />);
    expect(
      screen.getByRole("button", { name: "Edit service name" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/can’t make this change/)).toBeNull();
  });

  it("holds the row without claiming a role while the check runs", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canCreate: false, loading: true, loaded: false }),
    );
    render(<DisplayNameRow serviceId="srv-1" displayName="API" name="api" />);
    expect(
      screen.queryByRole("button", { name: "Edit service name" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/can’t make this change/)).toBeNull();
  });
});
