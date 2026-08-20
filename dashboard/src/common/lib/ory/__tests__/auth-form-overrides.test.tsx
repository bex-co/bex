import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { OryNodeInputProps } from "@ory/elements-react";
import {
  oryAuthFormOverrides,
  UniqueIdPasswordInput,
} from "../auth-form-overrides";

function passwordProps(
  overrides: Partial<OryNodeInputProps["inputProps"]> = {},
): OryNodeInputProps {
  return {
    attributes: {
      name: "password",
      type: "password",
      node_type: "input",
      disabled: false,
      required: false,
    } as OryNodeInputProps["attributes"],
    node: {
      type: "input",
      group: "password",
      attributes: {
        name: "password",
        type: "password",
        node_type: "input",
        disabled: false,
        required: false,
      },
      messages: [],
      meta: {},
    } as OryNodeInputProps["node"],
    inputProps: {
      id: "password",
      name: "password",
      value: "",
      onClick: () => undefined,
      onChange: () => undefined,
      onBlur: () => undefined,
      ref: () => undefined,
      type: "password",
      placeholder: "Enter your Password",
      autoComplete: "current-password",
      ...overrides,
    },
  };
}

describe("UniqueIdPasswordInput", () => {
  it("keeps a unique id on the password input and its visibility toggle", () => {
    render(<UniqueIdPasswordInput {...passwordProps()} />);

    const matches = document.querySelectorAll("#password");
    expect(matches).toHaveLength(1);
    expect(matches[0]?.tagName).toBe("INPUT");
    expect(document.getElementById("password-visibility")?.tagName).toBe(
      "BUTTON",
    );
    expect(
      screen.getByRole("button", { name: /show password/i }),
    ).toBeInTheDocument();
  });

  it("marks the password input required for empty-submit validation", () => {
    render(<UniqueIdPasswordInput {...passwordProps()} />);
    expect(
      document.querySelector<HTMLInputElement>("#password")?.required,
    ).toBe(true);
  });
});

describe("oryAuthFormOverrides", () => {
  it("wires Form.Root and Node.Input overrides alongside the card logo hide", () => {
    expect(oryAuthFormOverrides.Card?.Logo).toBeTypeOf("function");
    expect(oryAuthFormOverrides.Form?.Root).toBeTypeOf("function");
    expect(oryAuthFormOverrides.Node?.Input).toBeTypeOf("function");
  });

  it("renders identifier nodes as required email inputs", () => {
    const Input = oryAuthFormOverrides.Node!.Input!;
    render(
      <Input
        {...passwordProps({
          id: "identifier",
          name: "identifier",
          type: "text",
          autoComplete: undefined,
          placeholder: "Enter your E-Mail",
        })}
      />,
    );
    const el = document.querySelector<HTMLInputElement>("#identifier");
    expect(el?.type).toBe("email");
    expect(el?.required).toBe(true);
  });
});
