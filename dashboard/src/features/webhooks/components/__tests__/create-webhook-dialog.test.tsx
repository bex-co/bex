import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateWebhookDialog } from "@/features/webhooks/components/create-webhook-dialog";

const create = vi.fn();
vi.mock("@/features/webhooks/hooks/use-create-webhook", () => ({
  useCreateWebhook: () => ({ create, busy: false }),
}));

vi.mock("@/features/webhooks/hooks/use-webhook-event-types", () => ({
  useWebhookEventTypes: () => ({
    eventTypes: ["deploy_started", "deploy_ended", "service_suspended"],
    loading: false,
  }),
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

beforeEach(() => {
  create.mockReset();
  copy.mockReset();
});

async function fillAndSubmit(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Add webhook" }));
  const dialog = await screen.findByRole("dialog");
  await user.type(within(dialog).getByLabelText("Name"), "slack-bot");
  await user.type(
    within(dialog).getByLabelText("Destination URL"),
    "https://example.com/hook",
  );
  await user.click(within(dialog).getByRole("checkbox", { name: /deploy_started/ }));
  await user.click(within(dialog).getByRole("button", { name: "Create" }));
  return dialog;
}

describe("CreateWebhookDialog — secret reveal-once (w3/m11/t007)", () => {
  it("shows the signing secret exactly once after create, with a copy affordance", async () => {
    create.mockResolvedValue({ id: "whk-1", name: "slack-bot", secret: "whsec_s3cret" });
    const user = userEvent.setup();
    render(<CreateWebhookDialog onCreated={vi.fn()} />);

    const dialog = await fillAndSubmit(user);

    expect(await within(dialog).findByText("whsec_s3cret")).toBeInTheDocument();
    expect(create).toHaveBeenCalledWith("slack-bot", "https://example.com/hook", [
      "deploy_started",
    ]);

    await user.click(within(dialog).getByLabelText("Copy"));
    expect(copy).toHaveBeenCalledWith("whsec_s3cret");
  });

  it("the secret exists nowhere after the dialog is dismissed and reopened", async () => {
    create.mockResolvedValue({ id: "whk-1", name: "slack-bot", secret: "whsec_s3cret" });
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<CreateWebhookDialog onCreated={onCreated} />);

    const dialog = await fillAndSubmit(user);
    await within(dialog).findByText("whsec_s3cret");

    await user.click(within(dialog).getByRole("button", { name: "Done" }));
    expect(screen.queryByText("whsec_s3cret")).not.toBeInTheDocument();
    expect(onCreated).toHaveBeenCalled();

    // Reopen: back to the entry step, fields and checkboxes reset, no secret.
    await user.click(screen.getByRole("button", { name: "Add webhook" }));
    const reopened = await screen.findByRole("dialog");
    expect(within(reopened).getByLabelText("Destination URL")).toHaveValue("");
    expect(
      within(reopened).getByRole("checkbox", { name: /deploy_started/ }),
    ).not.toBeChecked();
    expect(screen.queryByText("whsec_s3cret")).not.toBeInTheDocument();
  });

  it("a create failure (null from the hook) keeps the dialog on the entry step", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<CreateWebhookDialog onCreated={vi.fn()} />);

    const dialog = await fillAndSubmit(user);

    expect(within(dialog).getByLabelText("Destination URL")).toBeInTheDocument();
    expect(
      within(dialog).queryByText(/won't be able to see it again/i),
    ).not.toBeInTheDocument();
  });

  it("keeps submit disabled until a URL and at least one event type are picked", async () => {
    const user = userEvent.setup();
    render(<CreateWebhookDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Add webhook" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Create" });
    expect(submit).toBeDisabled();

    await user.type(
      within(dialog).getByLabelText("Destination URL"),
      "https://example.com/hook",
    );
    expect(submit).toBeDisabled(); // URL alone isn't enough

    const checkbox = within(dialog).getByRole("checkbox", { name: /deploy_ended/ });
    await user.click(checkbox);
    expect(submit).toBeEnabled();

    await user.click(checkbox); // unchecking the only event disables again
    expect(submit).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });
});
