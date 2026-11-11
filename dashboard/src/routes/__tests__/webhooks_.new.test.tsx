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

let workspacePlan = "pro";
vi.mock("@/features/workspaces/context", () => ({
  useWorkspace: () => ({
    currentWorkspace: { id: "tea-1", plan: workspacePlan },
  }),
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
  workspacePlan = "pro";
  mutationError = null;
  existingNames = [];
  clearError.mockReset();
  create.mockReset();
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

  // w21/046: Render accepts any URL, so a destination bex can never deliver to
  // (a loopback, private or cluster-internal address) is noted, not refused.
  it("notes an unreachable destination beside the URL without blocking Create", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });
    const url = screen.getByLabelText("Destination URL");
    const notice = /not reachable from the internet/;

    await user.type(url, "https://example.com/hook");
    expect(screen.queryByText(notice)).toBeNull();

    await user.clear(url);
    await user.type(url, "https://localhost:3000/hook");
    expect(screen.getByText(notice)).toBeInTheDocument();
    expect(url).not.toHaveAttribute("aria-invalid", "true");
    expect(url).toHaveAccessibleDescription(
      expect.stringContaining("not reachable from the internet"),
    );

    await user.type(screen.getByLabelText("Name"), "local-hook");
    await user.click(screen.getByRole("checkbox", { name: "Deploy Ended" }));
    await user.click(screen.getByRole("button", { name: "Create webhook" }));
    expect(create).toHaveBeenCalledTimes(1);
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

  it("a successful create lands on the webhook's detail page with no secret step, as on Render", async () => {
    create.mockResolvedValue({
      id: "whk-1",
      name: "slack-bot",
      secret: "whsec_s3cret",
    });
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });

    await fillAndSubmit(user);

    expect(create).toHaveBeenCalledWith(
      "slack-bot",
      "https://example.com/hook",
      ["deploy_started"],
      true,
    );
    expect(await screen.findByTestId("detail-page")).toBeInTheDocument();
    expect(screen.queryByText("whsec_s3cret")).not.toBeInTheDocument();
  });

  it("a create failure (null from the hook) stays on the form", async () => {
    create.mockResolvedValue(null);
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("button", { name: "Create webhook" });

    await fillAndSubmit(user);

    expect(screen.getByLabelText("Destination URL")).toBeInTheDocument();
    expect(screen.queryByTestId("detail-page")).not.toBeInTheDocument();
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

  // w4/425: the server's plan refusal names the plan in its own focused alert,
  // never "Fix the highlighted fields" with nothing highlighted.
  it("names the plan for WEBHOOK_PLAN_LIMIT, without the field summary", async () => {
    mutationError = new WebhookMutationError(
      "WEBHOOK_PLAN_LIMIT",
      "webhooks require the Pro plan or higher; this workspace is on Hobby",
      "form",
    );
    renderPage();
    const alert = await screen.findByText(
      /available on the Pro plan and higher/,
    );
    expect(alert).toHaveFocus();
    expect(
      screen.queryByText("Fix the highlighted fields and try again."),
    ).not.toBeInTheDocument();
  });

  it("keeps the field summary for a field refusal", async () => {
    mutationError = new WebhookMutationError(
      "WEBHOOK_NAME_CONFLICT",
      "server prose",
      "name",
    );
    renderPage();
    expect(
      await screen.findByText("Fix the highlighted fields and try again."),
    ).toBeInTheDocument();
  });

  it("shows a Hobby workspace the plan notice instead of the form", async () => {
    workspacePlan = "hobby";
    renderPage();
    expect(
      await screen.findByText(/Upgrade this workspace to add an endpoint/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(
      screen.queryByRole("button", { name: "Create webhook" }),
    ).not.toBeInTheDocument();
  });
});
