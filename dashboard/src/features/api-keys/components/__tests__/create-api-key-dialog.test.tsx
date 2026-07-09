import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateApiKeyDialog } from "@/features/api-keys/components/create-api-key-dialog";

const create = vi.fn();
vi.mock("@/features/api-keys/hooks/use-create-api-key", () => ({
  useCreateApiKey: () => ({ create, busy: false }),
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

beforeEach(() => {
  create.mockReset();
  copy.mockReset();
});

describe("CreateApiKeyDialog — mint-once-visibility (w4/m8/t003)", () => {
  it("shows the secret exactly once after create, with a copy affordance", async () => {
    create.mockResolvedValue({ id: "key-1", name: "deploy-agent", secret: "s3cret-value" });
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(await within(dialog).findByText("s3cret-value")).toBeInTheDocument();
    expect(create).toHaveBeenCalledWith("deploy-agent");

    await user.click(within(dialog).getByLabelText("Copy"));
    expect(copy).toHaveBeenCalledWith("s3cret-value");
  });

  it("the secret exists nowhere after the dialog is dismissed and reopened", async () => {
    create.mockResolvedValue({ id: "key-1", name: "deploy-agent", secret: "s3cret-value" });
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={onCreated} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    let dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
    await user.click(within(dialog).getByRole("button", { name: "Create" }));
    await within(dialog).findByText("s3cret-value");

    // Dismiss (the "Done" button), then reopen — nothing in the DOM carries the
    // secret over; the dialog resets to the name-entry step, not a stale reveal.
    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(screen.queryByText("s3cret-value")).not.toBeInTheDocument();
    expect(onCreated).toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument();
    expect(screen.queryByText("s3cret-value")).not.toBeInTheDocument();
  });

  it("a create failure (null from the hook) keeps the dialog on the name step — no secret to show (t006)", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "deploy-agent");
    await user.click(within(dialog).getByRole("button", { name: "Create" }));

    expect(within(dialog).getByLabelText("Name")).toBeInTheDocument();
    expect(within(dialog).queryByText(/won't be able to see it again/i)).not.toBeInTheDocument();
  });

  it("keeps submit disabled for an empty/whitespace-only name", async () => {
    const user = userEvent.setup();
    render(<CreateApiKeyDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Create API key" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Create" });
    expect(submit).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Name"), "   ");
    expect(submit).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });
});
