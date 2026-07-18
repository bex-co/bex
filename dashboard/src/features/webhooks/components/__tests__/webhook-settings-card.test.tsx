import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { WebhookSettingsCard } from "@/features/webhooks/components/webhook-settings-card";
import type { WebhookEndpointView } from "@/features/webhooks/types";

const update = vi.fn();
vi.mock("@/features/webhooks/hooks/use-update-webhook", () => ({
  useUpdateWebhook: () => ({ update, busy: false }),
}));

const setEnabled = vi.fn();
vi.mock("@/features/webhooks/hooks/use-set-webhook-enabled", () => ({
  useSetWebhookEnabled: () => ({ setEnabled, toggling: null }),
}));

const remove = vi.fn();
vi.mock("@/features/webhooks/hooks/use-delete-webhook", () => ({
  useDeleteWebhook: () => ({ remove, deleting: null }),
}));

vi.mock("@/features/webhooks/hooks/use-webhook-event-types", () => ({
  useWebhookEventTypes: () => ({
    eventTypes: ["deploy_started", "deploy_ended", "service_suspended"],
    loading: false,
  }),
}));

const ENDPOINT: WebhookEndpointView = {
  id: "whk-1",
  name: "slack-bot",
  url: "https://example.com/hook",
  eventTypes: ["deploy_started"],
  enabled: true,
  disabledReason: "",
  createdAt: "2026-07-17T00:00:00Z",
  createdBy: "user@example.com",
};

function renderCard(endpoint: WebhookEndpointView = ENDPOINT) {
  const rootRoute = createRootRoute();
  const route = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <WebhookSettingsCard endpoint={endpoint} />,
  });
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/webhooks",
    component: () => <div data-testid="list-page" />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([route, listRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  update.mockReset();
  update.mockResolvedValue(true);
  setEnabled.mockReset();
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("WebhookSettingsCard (w1/m49/t005)", () => {
  it("Save stays disabled until something changes, then sends only the changed fields", async () => {
    const user = userEvent.setup();
    renderCard();
    const save = await screen.findByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();

    // Change the subscription only — name/url must be omitted from the patch.
    await user.click(screen.getByRole("checkbox", { name: "Deploy Ended" }));
    expect(save).toBeEnabled();
    await user.click(save);

    expect(update).toHaveBeenCalledTimes(1);
    const [id, patch] = update.mock.calls[0];
    expect(id).toBe("whk-1");
    expect(patch).toEqual({
      eventTypes: expect.arrayContaining(["deploy_started", "deploy_ended"]),
    });
    expect(patch.eventTypes).toHaveLength(2);
  });

  it("renaming sends the name; reverting an edit disables Save again", async () => {
    const user = userEvent.setup();
    renderCard();
    const save = await screen.findByRole("button", { name: "Save changes" });
    const name = screen.getByLabelText("Name");

    await user.type(name, "-2");
    expect(save).toBeEnabled();

    await user.type(name, "{backspace}{backspace}");
    expect(save).toBeDisabled(); // back to the server value → not dirty

    await user.type(name, "-two");
    await user.click(save);
    expect(update).toHaveBeenCalledWith("whk-1", { name: "slack-bot-two" });
  });

  it("an empty URL or an empty event set blocks Save even when dirty", async () => {
    const user = userEvent.setup();
    renderCard();
    const save = await screen.findByRole("button", { name: "Save changes" });
    const url = screen.getByLabelText("Destination URL");

    await user.clear(url);
    expect(save).toBeDisabled();

    await user.type(url, "https://example.com/hook2");
    expect(save).toBeEnabled();

    // Unchecking the only subscribed event zeroes the set → blocked.
    await user.click(screen.getByRole("checkbox", { name: "Deploy Started" }));
    expect(save).toBeDisabled();
    expect(update).not.toHaveBeenCalled();
  });

  it("the status switch flips through setWebhookEndpointEnabled immediately", async () => {
    const user = userEvent.setup();
    renderCard();
    await user.click(await screen.findByRole("switch", { name: "Status" }));
    expect(setEnabled).toHaveBeenCalledWith("whk-1", "slack-bot", false);
    expect(update).not.toHaveBeenCalled();
  });

  it("delete requires typing the exact sudo command before the button arms", async () => {
    const user = userEvent.setup();
    renderCard();
    await user.click(await screen.findByRole("button", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    const confirm = within(dialog).getByRole("button", { name: "Delete" });
    expect(confirm).toBeDisabled();

    const input = within(dialog).getByPlaceholderText(
      "delete webhook slack-bot",
    );
    await user.type(input, "delete webhook slack-bo"); // near miss
    expect(confirm).toBeDisabled();

    await user.type(input, "t"); // completes the exact command
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    expect(remove).toHaveBeenCalledWith("whk-1", "slack-bot");
    expect(await screen.findByTestId("list-page")).toBeInTheDocument();
  });

  it("shows the mint-once signing-secret note (t006 decision) — no reveal affordance", async () => {
    renderCard();
    expect(
      await screen.findByText(/shown once when this webhook was created/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/show secret/i)).not.toBeInTheDocument();
  });
});
