import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  type RouteComponent,
} from "@tanstack/react-router";

// w3/m707: Render links three sections bex's pages name otherwise —
// `/settings#account-security`, `/billing#free-usage` and
// `/billing#unbilled-charges` — and each opens scrolled to bex's card.
// Every card is a stub; the pages' own sections and the scroll hook run.

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));
vi.mock("@/features/usage/hooks/use-usage", () => ({
  useUsage: () => ({ summary: null, loading: false, error: undefined }),
}));
for (const [path, name] of [
  ["@/features/usage/components/resource-caps", "WorkspaceResourceCaps"],
  ["@/features/usage/components/plan-card", "PlanCard"],
  ["@/features/usage/components/payment-method-card", "PaymentMethodCard"],
  ["@/features/usage/components/billing-email-card", "BillingEmailCard"],
  ["@/features/usage/components/billing-info-card", "BillingInfoCard"],
  ["@/features/usage/components/charges-card", "ChargesCard"],
  ["@/features/usage/components/agent-model-card", "AgentModelCard"],
  ["@/features/usage/components/usage-navigation", "UsageNavigation"],
  ["@/features/api-keys/components/api-keys-panel", "ApiKeysPanel"],
  ["@/features/ssh-keys/components/ssh-keys-panel", "SSHKeysPanel"],
  ["@/features/git/components/connect-github-card", "ConnectGithubCard"],
  [
    "@/features/auth/pages/settings-page/security-compliance-section",
    "SecurityComplianceSection",
  ],
  [
    "@/features/auth/pages/settings-page/account-deletion-card",
    "AccountDeletionCard",
  ],
  ["@/features/auth/pages/settings-page/appearance-card", "AppearanceCard"],
  [
    "@/features/auth/pages/settings-page/account-avatar-card",
    "AccountAvatarCard",
  ],
] as const) {
  vi.doMock(path, () => ({ [name]: () => null }));
}
vi.mock("@/features/usage/components/billing-summary-cards", () => ({
  CreditBalanceCard: () => null,
  InvoiceHistoryCard: () => null,
}));
vi.mock("@/common/hooks/use-ory-flow", () => ({ useOryFlow: () => null }));

const scrolled: string[] = [];

beforeEach(() => {
  scrolled.length = 0;
  Element.prototype.scrollIntoView = vi.fn(function (this: Element) {
    scrolled.push(this.id);
  });
});

afterEach(() => {
  document.body.innerHTML = "";
});

async function open(path: string, hash: string) {
  const { UsagePage } = await import("@/features/usage/components/usage-page");
  const { default: SettingsPage } =
    await import("@/features/auth/pages/settings-page");
  const root = createRootRoute();
  const router = createRouter({
    routeTree: root.addChildren([
      createRoute({
        getParentRoute: () => root,
        path: "/billing",
        component: UsagePage as RouteComponent,
      }),
      createRoute({
        getParentRoute: () => root,
        path: "/settings",
        component: SettingsPage as RouteComponent,
      }),
    ]),
    history: createMemoryHistory({ initialEntries: [`${path}#${hash}`] }),
  });
  await router.load();
  render(<RouterProvider router={router} />);
}

describe("Render's section anchors land on bex's cards (w3/m707)", () => {
  it.each([
    ["/settings", "account-security", "account"],
    ["/billing", "free-usage", "included-usage"],
    ["/billing", "unbilled-charges", "charges"],
  ])("scrolls %s#%s to #%s", async (path, hash, card) => {
    await open(path, hash);
    await vi.waitFor(() => expect(scrolled).toContain(card));
  });

  it("still scrolls bex's own anchor to its card", async () => {
    await open("/billing", "charges");
    await vi.waitFor(() => expect(scrolled).toContain("charges"));
  });
});
