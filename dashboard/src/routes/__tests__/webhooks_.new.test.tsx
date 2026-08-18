import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRouter,
  createRootRoute,
  createRoute,
  createMemoryHistory,
} from "@tanstack/react-router";
import { NewWebhookPage } from "../webhooks_.new";
import { WebhookMutationError } from "@/features/webhooks/lib/errors";

const create = vi.fn();
let canManage = true;
let mutationError: Error | null = null;
let existingNames: string[] = [];
const clearError = vi.fn();
vi.mock("@/features/webhooks/hooks/use-create-webhook", () => ({
  useCreateWebhook: () => ({
    create,
    busy: false,
    error: mutationError,
    clearError,
  }),
}));

vi.mock("@/features/webhooks/hooks/use-webhook-event-types", () => ({
  useWebhookEventTypes: () => ({
    eventTypes: ["deploy_started", "deploy_ended", "service_suspended"],
    loading: false,
    error: undefined,
    retry: vi.fn(),
  }),
}));

vi.mock("@/features/webhooks/hooks/use-webhooks", () => ({
  useWebhooks: () => ({
    endpoints: existingNames.map((name) => ({ name })),
    loading: false,
  }),
}));

vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => ({ canManage, loaded: true }),
}));

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

function renderPage() {
  const rootRoute = createRootRoute();
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: NewWebhookPage,
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/webhook/$webhookId",
    component: () => <div data-testid="detail-page" />,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([newRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  canManage = true;
  mutationError = null;
  existingNames = [];
  clearError.mockReset();
  create.mockReset();
  copy.mockReset();
});

async function fillAndSubmit(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Name"), "slack-bot");
  await user.type(
    screen.getByLabelText("Destination URL"),
    "https://example.com/hook",
  );
  await user.click(screen.getByRole("checkbox", { name: "Deploy Started" }));
  await user.click(screen.getByRole("button", { name: "Create webhook" }));
}

describe("NewWebhookPage — /webhooks/new (w1/m49/t003)", () => {
  it("reports every missing field inline and focuses the first failure", async () => {
    const user = userEvent.setup();
    renderPage();
    const submit = await screen.findByRole("button", {
      name: "Create webhook",
    });
    expect(submit).toBeEnabled();
    await user.click(submit);
    expect(screen.getByText("Enter a webhook name.")).toBeInTheDocument();
    expect(screen.getByText("Enter a destination URL.")).toBeInTheDocument();
    expect(screen.getByText("Select at least one event.")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveFocus();
    expect(create).not.toHaveBeenCalled();
  });

  it("reports malformed and non-HTTPS destinations beside the URL", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });
    await user.type(screen.getByLabelText("Name"), "deploy-hook");
    await user.type(screen.getByLabelText("Destination URL"), "http://bad");
    await user.click(screen.getByRole("checkbox", { name: "Deploy Ended" }));
    await user.click(screen.getByRole("button", { name: "Create webhook" }));
    expect(
      screen.getByText(
        "Enter an absolute HTTPS URL without embedded credentials.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Destination URL")).toHaveFocus();
  });

  it("preflights duplicate names and anchors an authoritative server race to Name", async () => {
    existingNames = ["Existing Hook"];
    const user = userEvent.setup();
    const first = renderPage();
    await screen.findByRole("button", { name: "Create webhook" });
    await user.type(screen.getByLabelText("Name"), " existing hook ");
    await user.type(
      screen.getByLabelText("Destination URL"),
      "https://example.com/hook",
    );
    await user.click(screen.getByRole("checkbox", { name: "Deploy Ended" }));
    await user.click(screen.getByRole("button", { name: "Create webhook" }));
    expect(
      screen.getByText("A webhook with this name already exists."),
    ).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();

    first.unmount();
    existingNames = [];
    mutationError = new WebhookMutationError(
      "WEBHOOK_NAME_CONFLICT",
      "server prose",
      "name",
    );
    renderPage();
    expect(
      await screen.findByText("A webhook with this name already exists."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveFocus();
  });

  it("shows the signing secret exactly once after create, with a copy affordance", async () => {
    create.mockResolvedValue({
      id: "whk-1",
      name: "slack-bot",
      secret: "whsec_s3cret",
    });
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });

    await fillAndSubmit(user);

    expect(await screen.findByText("whsec_s3cret")).toBeInTheDocument();
    expect(create).toHaveBeenCalledWith(
      "slack-bot",
      "https://example.com/hook",
      ["deploy_started"],
      true,
    );

    await user.click(screen.getByLabelText("Copy"));
    expect(copy).toHaveBeenCalledWith("whsec_s3cret");
  });

  it("'View webhook' leaves the secret step for the detail page — the secret is gone", async () => {
    create.mockResolvedValue({
      id: "whk-1",
      name: "slack-bot",
      secret: "whsec_s3cret",
    });
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });

    await fillAndSubmit(user);
    await screen.findByText("whsec_s3cret");

    await user.click(screen.getByRole("button", { name: "View webhook" }));
    expect(await screen.findByTestId("detail-page")).toBeInTheDocument();
    expect(screen.queryByText("whsec_s3cret")).not.toBeInTheDocument();
  });

  it("a create failure (null from the hook) keeps the form step", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });

    await fillAndSubmit(user);

    expect(screen.getByLabelText("Destination URL")).toBeInTheDocument();
    expect(
      screen.queryByText(/won't be able to see it again/i),
    ).not.toBeInTheDocument();
  });

  it("does not expose the create form to a read-only member", async () => {
    canManage = false;
    renderPage();
    expect(
      await screen.findByText(/only workspace managers/i),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Create webhook" }),
    ).not.toBeInTheDocument();
  });
});
