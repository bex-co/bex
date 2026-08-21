import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { OryNodeInputProps } from "@ory/elements-react";
import {
  extractOtp,
  oryAuthFormOverrides,
  OtpCodeInput,
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

function codeProps(
  overrides: Partial<OryNodeInputProps["inputProps"]> = {},
): OryNodeInputProps {
  return {
    attributes: {
      name: "code",
      type: "text",
      node_type: "input",
      disabled: false,
      required: false,
    } as OryNodeInputProps["attributes"],
    node: {
      type: "input",
      group: "code",
      attributes: {
        name: "code",
        type: "text",
        node_type: "input",
        disabled: false,
        required: false,
      },
      messages: [],
      meta: {},
    } as OryNodeInputProps["node"],
    inputProps: {
      id: "code",
      name: "code",
      // Required key; undefined keeps the input uncontrolled so the tests
      // can type into it like a browser would.
      value: undefined,
      onClick: () => undefined,
      onChange: () => undefined,
      onBlur: () => undefined,
      ref: () => undefined,
      type: "text",
      placeholder: "Enter the code",
      ...overrides,
    },
  };
}

/** Render the OTP input inside a flow-shaped form with the method submit. */
function renderOtpForm(props: OryNodeInputProps, onSubmit: () => void) {
  return render(
    <form
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <OtpCodeInput {...props} />
      <button
        type="submit"
        name="method"
        value="code"
        data-testid="ory/form/node/button/method"
      >
        Continue
      </button>
    </form>,
  );
}

describe("OtpCodeInput", () => {
  it("auto-submits the flow when the last digit of the code lands", async () => {
    let submits = 0;
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });

    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "12345" } });
    await Promise.resolve(); // flush the auto-submit microtask
    expect(submits).toBe(0); // one digit short — no submit

    fireEvent.change(input, { target: { value: "123456" } });
    await Promise.resolve();
    expect(submits).toBe(1);
  });

  it("submits once per code value and ignores non-numeric input", async () => {
    let submits = 0;
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });

    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "abc123" } });
    await Promise.resolve();
    expect(submits).toBe(0); // six chars but not a numeric code

    fireEvent.change(input, { target: { value: "987654" } });
    await Promise.resolve();
    fireEvent.change(input, { target: { value: "987654" } });
    await Promise.resolve();
    expect(submits).toBe(1); // same value never double-submits
  });

  it("still forwards changes to the form library handler", () => {
    const seen: string[] = [];
    renderOtpForm(
      codeProps({
        onChange: (event) =>
          seen.push((event.target as HTMLInputElement).value),
      }),
      () => undefined,
    );
    const input = screen.getByTestId("ory/form/node/input/code");
    fireEvent.change(input, { target: { value: "42" } });
    expect(seen).toEqual(["42"]);
  });

  it("uses one-time-code semantics for OS autofill and numeric keyboards", () => {
    renderOtpForm(codeProps(), () => undefined);
    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;
    expect(input.autocomplete).toBe("one-time-code");
    expect(input.inputMode).toBe("numeric");
    expect(input.maxLength).toBe(6);
  });
});

const KRATOS_EMAIL_TEXT =
  "Verify your account with the following code: 513889 or clicking the " +
  "following link: https://auth.bex.co/self-service/verification?code=513889" +
  "&flow=47545c52-77ef-481c-9ec5-7697f0904b56 If this was not you, do " +
  "nothing. This code / link expires in 60 minutes.";

describe("extractOtp", () => {
  it("finds the exact six-digit code inside the whole Kratos email", () => {
    expect(extractOtp(KRATOS_EMAIL_TEXT)).toBe("513889");
    expect(extractOtp("513889")).toBe("513889");
  });

  it("never promotes UUID fragments, durations, or longer runs", () => {
    expect(extractOtp("flow=47545c52-77ef 60 minutes")).toBeNull();
    expect(extractOtp("1234567")).toBeNull(); // seven digits is not a code
    expect(extractOtp("")).toBeNull();
  });
});

describe("OtpCodeInput page-wide paste", () => {
  function pasteOn(target: Element | Document, text: string) {
    fireEvent.paste(target, {
      clipboardData: { getData: () => text },
    });
  }

  it("captures a paste anywhere on the page and auto-submits", async () => {
    let submits = 0;
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });
    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;

    pasteOn(document.body, KRATOS_EMAIL_TEXT); // input NOT focused
    await Promise.resolve();
    expect(input.value).toBe("513889");
    expect(submits).toBe(1);
  });

  it("sanitizes a paste aimed at the input itself to just the code", async () => {
    let submits = 0;
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });
    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;
    input.focus();

    pasteOn(input, KRATOS_EMAIL_TEXT);
    await Promise.resolve();
    expect(input.value).toBe("513889");
    expect(submits).toBe(1);
  });

  it("never hijacks a paste aimed at another text-entry surface", async () => {
    let submits = 0;
    render(
      <div>
        <input data-testid="other-field" />
      </div>,
    );
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });
    const other = screen.getByTestId("other-field") as HTMLInputElement;
    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;

    pasteOn(other, KRATOS_EMAIL_TEXT);
    await Promise.resolve();
    expect(input.value).toBe("");
    expect(submits).toBe(0);
  });

  it("leaves pastes without a six-digit code untouched", async () => {
    let submits = 0;
    renderOtpForm(codeProps(), () => {
      submits += 1;
    });
    const input = screen.getByTestId(
      "ory/form/node/input/code",
    ) as HTMLInputElement;

    pasteOn(document.body, "no code here, expires in 60 minutes");
    await Promise.resolve();
    expect(input.value).toBe("");
    expect(submits).toBe(0);
  });
});
