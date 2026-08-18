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
});
