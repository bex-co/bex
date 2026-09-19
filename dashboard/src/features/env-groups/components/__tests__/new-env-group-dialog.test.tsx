import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createGroup = vi.fn();

vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroupMutations: () => ({ createGroup, busy: false }),
}));

import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";

beforeEach(() => {
  createGroup.mockReset().mockResolvedValue("eg-new");
});

describe("NewEnvGroupDialog", () => {
  it("blocks a blank name, then returns the created group id", async () => {
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={onCreated} />);

    const submit = screen.getByRole("button", {
      name: "Create Environment Group",
    });
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText("Group name"), "   ");
    expect(submit).toBeDisabled();
    expect(createGroup).not.toHaveBeenCalled();

    await user.clear(screen.getByLabelText("Group name"));
    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(submit);

    expect(createGroup).toHaveBeenCalledWith({
      name: "Shared production",
      envVars: [],
      secretFiles: [],
      serviceIds: [],
      environmentId: null,
    });
    expect(onCreated).toHaveBeenCalledWith("eg-new");
  });

  it("submits initial variables, secret files, and service links once", async () => {
    const user = userEvent.setup();
    render(
      <NewEnvGroupDialog
        open
        onCreated={vi.fn()}
        services={[
          { id: "srv-web", name: "Web API" } as never,
          { id: "srv-worker", name: "Worker" } as never,
        ]}
      />,
    );

    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(
      screen.getByRole("button", { name: "Add Environment Variable" }),
    );
    await user.type(screen.getByLabelText("Key"), "API_TOKEN");
    await user.type(screen.getByLabelText("Value"), "secret");
    await user.click(screen.getByRole("button", { name: "Add Secret File" }));
    await user.type(screen.getByLabelText("File name"), "ca.pem");
    await user.type(screen.getByLabelText("Contents"), "CERT");
    await user.click(screen.getByLabelText(/Web API/));
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).toHaveBeenCalledTimes(1);
    expect(createGroup).toHaveBeenCalledWith({
      name: "Shared production",
      envVars: [
        { key: "API_TOKEN", value: "secret", generateValue: undefined },
      ],
      secretFiles: [{ name: "ca.pem", content: "CERT" }],
      serviceIds: ["srv-web"],
      environmentId: null,
    });
  });

  // w2/m95 t003: bex owns PORT — in a group it would be dropped for every
  // linked service at once — so the dialog says so as the key is typed.
  it("flags a reserved key inline and refuses to create", async () => {
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={vi.fn()} services={[]} />);

    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(
      screen.getByRole("button", { name: "Add Environment Variable" }),
    );
    await user.type(screen.getByLabelText("Key"), "PORT");

    expect(
      await screen.findByText(
        "PORT is set by bex from the service port. Change the service port instead.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Key")).toHaveAttribute(
      "aria-invalid",
      "true",
    );

    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );
    expect(createGroup).not.toHaveBeenCalled();
  });

  // w2/m95 t002: the value field was a single-line `type="password"` input, so
  // a pasted PEM key lost its newlines on the way into state. It is now a
  // masked auto-growing textarea — still dots, but it keeps what was pasted.
  it("keeps a pasted multi-line value and stays masked", async () => {
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={vi.fn()} services={[]} />);

    await user.type(screen.getByLabelText("Group name"), "Shared production");
    await user.click(
      screen.getByRole("button", { name: "Add Environment Variable" }),
    );
    await user.type(screen.getByLabelText("Key"), "PEM");
    const value = screen.getByLabelText("Value");
    expect(value.tagName).toBe("TEXTAREA");
    expect(value).toHaveStyle({ WebkitTextSecurity: "disc" });

    await user.click(value);
    await user.paste("-----BEGIN KEY-----\nline-one\n-----END KEY-----");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).toHaveBeenCalledWith({
      name: "Shared production",
      envVars: [
        {
          key: "PEM",
          value: "-----BEGIN KEY-----\nline-one\n-----END KEY-----",
          generateValue: undefined,
        },
      ],
      secretFiles: [],
      serviceIds: [],
      environmentId: null,
    });
  });

  it("sends generateValue without a conflicting literal", async () => {
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={vi.fn()} />);

    await user.type(screen.getByLabelText("Group name"), "Generated secrets");
    await user.click(
      screen.getByRole("button", { name: "Add Environment Variable" }),
    );
    await user.type(screen.getByLabelText("Key"), "SESSION_SECRET");
    await user.click(screen.getByRole("button", { name: "Generate" }));
    expect(screen.getByLabelText("Value")).toBeDisabled();
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).toHaveBeenCalledWith({
      name: "Generated secrets",
      envVars: [
        { key: "SESSION_SECRET", value: undefined, generateValue: true },
      ],
      secretFiles: [],
      serviceIds: [],
      environmentId: null,
    });
  });

  it("imports dotenv entries into the unsaved create draft", async () => {
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={vi.fn()} />);

    await user.type(screen.getByLabelText("Group name"), "Imported values");
    await user.click(screen.getByRole("button", { name: "Import from .env" }));
    await user.type(
      screen.getByRole("textbox", { name: "Dotenv contents" }),
      "API_URL=https://example.test\nTOKEN=opaque",
    );
    await user.click(screen.getByRole("button", { name: "Add variables" }));

    expect(screen.getByDisplayValue("API_URL")).toBeInTheDocument();
    expect(screen.getByDisplayValue("TOKEN")).toBeInTheDocument();
    expect(createGroup).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );
    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({
        envVars: [
          {
            key: "API_URL",
            value: "https://example.test",
            generateValue: undefined,
          },
          { key: "TOKEN", value: "opaque", generateValue: undefined },
        ],
      }),
    );
  });

  it("blocks invalid variable keys before calling create", async () => {
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={vi.fn()} />);

    await user.type(screen.getByLabelText("Group name"), "shared");
    await user.click(
      screen.getByRole("button", { name: "Add Environment Variable" }),
    );
    await user.type(screen.getByLabelText("Key"), "bad key");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Fix invalid variable keys or file names",
    );
  });

  it("stays open and does not navigate when creation fails", async () => {
    createGroup.mockResolvedValue(null);
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<NewEnvGroupDialog open onCreated={onCreated} />);

    await user.type(screen.getByLabelText("Group name"), "shared");
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(onCreated).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("Group name")).toHaveValue("shared");
  });

  // --- w4/m111: scope-filtered link candidates ---
  //
  // Live on 2026-09-17 this dialog listed every workspace service with no
  // scope control at all, so checking a service that lives in an Environment
  // produced a create the backend deterministically refused:
  // "linked services must have the same Environment scope as the environment
  // group". The only way through was create-unlinked -> Move -> link.

  const ENVIRONMENTS = [
    { id: "evm-qa", name: "qa-env" },
    { id: "evm-prod", name: "prod-env" },
  ] as never;
  const SERVICES = [
    { id: "srv-ws", name: "workspace-svc" },
    { id: "srv-qa", name: "qa-svc" },
  ] as never;
  const SERVICE_ENVIRONMENTS = new Map([["srv-qa", "evm-qa"]]);

  function renderScoped(props: Record<string, unknown> = {}) {
    return render(
      <NewEnvGroupDialog
        open
        onCreated={vi.fn()}
        services={SERVICES}
        environments={ENVIRONMENTS}
        serviceEnvironmentById={SERVICE_ENVIRONMENTS}
        {...props}
      />,
    );
  }

  it("offers only workspace-scoped services under Workspace scope", () => {
    renderScoped();

    expect(screen.getByRole("checkbox", { name: /workspace-svc/ })).toBeTruthy();
    // The in-Environment service is the one the backend would refuse.
    expect(screen.queryByRole("checkbox", { name: /qa-svc/ })).toBeNull();
  });

  it("offers only that Environment's services once one is picked", async () => {
    const user = userEvent.setup();
    renderScoped();

    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "qa-env" }));

    expect(screen.getByRole("checkbox", { name: /qa-svc/ })).toBeTruthy();
    expect(screen.queryByRole("checkbox", { name: /workspace-svc/ })).toBeNull();
  });

  it("creates in the picked Environment with its links attached", async () => {
    const user = userEvent.setup();
    renderScoped();

    await user.type(screen.getByLabelText("Group name"), "qa-evg");
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "qa-env" }));
    await user.click(screen.getByRole("checkbox", { name: /qa-svc/ }));
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    expect(createGroup).toHaveBeenCalledWith({
      name: "qa-evg",
      envVars: [],
      secretFiles: [],
      serviceIds: ["srv-qa"],
      environmentId: "evm-qa",
    });
  });

  it("drops checks the new scope has made incompatible", async () => {
    const user = userEvent.setup();
    renderScoped();

    await user.type(screen.getByLabelText("Group name"), "qa-evg");
    await user.click(screen.getByRole("checkbox", { name: /workspace-svc/ }));
    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "qa-env" }));
    await user.click(
      screen.getByRole("button", { name: "Create Environment Group" }),
    );

    // The workspace service must not ride along into the qa-env create.
    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({ serviceIds: [], environmentId: "evm-qa" }),
    );
  });

  it("opens on the initial scope the service page hands it", () => {
    renderScoped({ initialEnvironmentId: "evm-qa" });

    expect(
      screen.getByRole("combobox", { name: "Environment" }),
    ).toHaveTextContent("qa-env");
    expect(screen.getByRole("checkbox", { name: /qa-svc/ })).toBeTruthy();
  });

  it("says an Environment has no services rather than pretending none exist", async () => {
    const user = userEvent.setup();
    renderScoped();

    await user.click(screen.getByRole("combobox", { name: "Environment" }));
    await user.click(screen.getByRole("option", { name: "prod-env" }));

    expect(
      screen.getByText(/No services live in this Environment/),
    ).toBeTruthy();
  });
});
