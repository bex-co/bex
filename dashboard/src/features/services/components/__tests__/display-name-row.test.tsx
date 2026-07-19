import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DisplayNameRow } from "@/features/services/components/display-name-row";

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

    // Render prefills its Name field with the service name; the opaque id
    // stays in the hint only.
    expect(screen.getByText("docs-site")).toBeInTheDocument();
    expect(screen.queryByText("srv-stable-id")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "The service ID remains srv-stable-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("falls back to the immutable id only while the name is unknown", () => {
    render(<DisplayNameRow serviceId="stable-id" displayName={null} />);

    expect(screen.getByText("stable-id")).toBeInTheDocument();
    expect(
      screen.getByText(
        "The service ID remains stable-id; URLs and infrastructure do not change.",
      ),
    ).toBeInTheDocument();
  });

  it("sets a trimmed human label without changing the service id", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    render(
      <DisplayNameRow
        serviceId="stable-id"
        displayName={null}
        onChanged={onChanged}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    const input = screen.getByRole("textbox", { name: "Service Name" });
    await user.clear(input);
    await user.type(input, "  Customer API  ");
    await user.click(screen.getByRole("button", { name: "Save service name" }));

    expect(setDisplayName).toHaveBeenCalledWith("stable-id", "Customer API");
    expect(onChanged).toHaveBeenCalledOnce();
  });

  it("clears a custom label so the immutable-id fallback can return", async () => {
    const user = userEvent.setup();
    render(<DisplayNameRow serviceId="stable-id" displayName="Customer API" />);

    await user.click(screen.getByRole("button", { name: "Edit service name" }));
    await user.clear(screen.getByRole("textbox", { name: "Service Name" }));
    await user.click(screen.getByRole("button", { name: "Save service name" }));

    expect(setDisplayName).toHaveBeenCalledWith("stable-id", "");
  });
});
