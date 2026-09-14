import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ConnectionField } from "@/common/components/connection-field";

describe("ConnectionField", () => {
  it("gives the copy control a verb-shaped name derived from the field label (m101/t014)", () => {
    render(
      <ConnectionField
        label="Password"
        value="s3cret"
        copiedText="Copied"
        copyErrorText="Copy failed"
      />,
    );

    expect(
      screen.getByRole("button", { name: /Copy .*[Pp]assword/ }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Password$/ })).not.toBeInTheDocument();
    // Visible column label is unchanged.
    expect(screen.getByText("Password")).toBeInTheDocument();
  });

  it("honors an explicit copyLabel when the visible label is already an action", () => {
    render(
      <ConnectionField
        label="Copy 203.0.113.10"
        copyLabel="Copy 203.0.113.10"
        value="203.0.113.10"
        copiedText="Copied"
        copyErrorText="Copy failed"
      />,
    );

    expect(
      screen.getByRole("button", { name: "Copy 203.0.113.10" }),
    ).toBeInTheDocument();
  });
});
