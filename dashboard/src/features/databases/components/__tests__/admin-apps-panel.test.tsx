import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { AdminAppsPanel } from "@/features/databases/components/admin-apps-panel";
import { AdminAppsPanelSkeleton } from "@/features/databases/components/admin-apps-panel-skeleton";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { AdminAppView } from "@/features/databases/hooks/use-postgres-admin-apps";
import type { ServiceView } from "@/features/services/types";
import { PaymentOnboardingCancelledError } from "@/features/usage/context/payment-required-error";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
    params,
  }: {
    children: ReactNode;
    to: string;
    params: { serviceId: string };
  }) => <a href={to.replace("$serviceId", params.serviceId)}>{children}</a>,
}));

vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({
    session: { identity: { traits: { email: "ada@example.com" } } },
  }),
}));

const environments = vi.hoisted(() => ({
  list: [] as { id: string; protectedStatus: string }[],
}));
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => ({ environments: environments.list, loading: false }),
}));

const deploy = vi.fn();
const state = vi.hoisted(() => ({
  apps: [] as AdminAppView[],
  environmentId: null as string | null,
  loading: false,
  error: undefined as Error | undefined,
}));
vi.mock("@/features/databases/hooks/use-postgres-admin-apps", () => ({
  usePostgresAdminApps: () => ({
    ...state,
    projectId: state.environmentId ? "prj-1" : null,
    deploy,
  }),
}));

function catalog(service: Partial<ServiceView> | null = null): AdminAppView[] {
  return [
    {
      id: "pgadmin",
      name: "pgAdmin",
      plan: "starter",
      diskSizeGB: 1,
      monthlyUsd: "4.90",
      diskMonthlyUsd: "0.175",
      service: null,
    },
    {
      id: "pghero",
      name: "PgHero",
      plan: "free",
      diskSizeGB: null,
      monthlyUsd: "0.00",
      diskMonthlyUsd: null,
      service: service
        ? ({
            id: "srv-hero",
            name: "PgHero-dpg-1",
            type: "web_service",
            suspended: false,
            phase: "Running",
            url: "https://pghero.example.com",
            plan: "free",
            ...service,
          } as ServiceView)
        : null,
    },
  ];
}

beforeEach(() => {
  state.apps = catalog();
  state.environmentId = null;
  state.loading = false;
  state.error = undefined;
  environments.list = [];
  deploy.mockReset();
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("AdminAppsPanel", () => {
  it("lists Render's two apps with their copy and pgAdmin's docs link", () => {
    render(<AdminAppsPanel databaseId="dpg-1" />);
    const pgadmin = screen.getByTestId("admin-app-pgadmin");
    expect(within(pgadmin).getByText("pgAdmin")).toBeInTheDocument();
    expect(
      within(pgadmin).getByText(/manage schemas, tables, and indexes/i),
    ).toBeInTheDocument();
    expect(
      within(pgadmin).getByRole("link", { name: /read the docs/i }),
    ).toHaveAttribute("href", "https://www.pgadmin.org/docs/");
    const pghero = screen.getByTestId("admin-app-pghero");
    expect(
      within(pghero).getByText(/performance dashboard/i),
    ).toBeInTheDocument();
    expect(
      within(pghero).queryByRole("link", { name: /read the docs/i }),
    ).toBeNull();
    expect(screen.getAllByRole("button", { name: "Deploy app" })).toHaveLength(
      2,
    );
  });

  it("opens the dialog with Render's generated login and the app's billing lines, then deploys it", async () => {
    deploy.mockResolvedValue({ id: "srv-new", name: "pgAdmin-dpg-1" });
    render(<AdminAppsPanel databaseId="dpg-1" />);
    await userEvent.click(
      within(screen.getByTestId("admin-app-pgadmin")).getByRole("button", {
        name: "Deploy app",
      }),
    );

    const dialog = screen.getByRole("dialog", { name: "Create a pgAdmin app" });
    const username = within(dialog).getByLabelText(
      "Username",
    ) as HTMLInputElement;
    const password = within(dialog).getByLabelText(
      "Password",
    ) as HTMLInputElement;
    // pgAdmin logs in by email: the signed-in user's, as on Render.
    expect(username.value).toBe("ada@example.com");
    expect(password.value).toMatch(/^[0-9a-f]{32}$/);
    expect(password.type).toBe("password");
    expect(
      within(dialog).getByText("Web service - Starter instance"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("$4.90 / month")).toBeInTheDocument();
    expect(within(dialog).getByText("Disk - 1GB")).toBeInTheDocument();
    expect(within(dialog).getByText("$0.175 / month")).toBeInTheDocument();

    await userEvent.click(
      within(dialog).getByRole("button", { name: "Show Password" }),
    );
    expect(password.type).toBe("text");
    await userEvent.clear(password);
    await userEvent.type(password, "chosen-pass");
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Deploy pgAdmin" }),
    );

    expect(deploy).toHaveBeenCalledWith("pgadmin", {
      username: "ada@example.com",
      password: "chosen-pass",
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("generates PgHero's username and shows the free instance line", async () => {
    render(<AdminAppsPanel databaseId="dpg-1" />);
    await userEvent.click(
      within(screen.getByTestId("admin-app-pghero")).getByRole("button", {
        name: "Deploy app",
      }),
    );
    const dialog = screen.getByRole("dialog", { name: "Create a PgHero app" });
    expect(
      (within(dialog).getByLabelText("Username") as HTMLInputElement).value,
    ).toMatch(/^pghero-[0-9a-f]{8}$/);
    expect(
      within(dialog).getByText("Web service - Free instance"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("$0.00 / month")).toBeInTheDocument();
    expect(within(dialog).queryByText(/^Disk/)).toBeNull();
  });

  it("keeps a refused deploy's reason in the dialog", async () => {
    deploy.mockRejectedValue(new Error("boom"));
    render(<AdminAppsPanel databaseId="dpg-1" />);
    await userEvent.click(
      within(screen.getByTestId("admin-app-pghero")).getByRole("button", {
        name: "Deploy app",
      }),
    );
    const dialog = screen.getByRole("dialog");
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Deploy PgHero" }),
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      within(screen.getByRole("dialog")).getByRole("alert"),
    ).toHaveTextContent(/boom|unable to create the app/i);
  });

  // A 402 opens billing onboarding (the hook's payment gate); closing it
  // without a card leaves Render's sentence in the dialog.
  it("asks for a payment method when onboarding closes without one", async () => {
    deploy.mockRejectedValue(new PaymentOnboardingCancelledError());
    render(<AdminAppsPanel databaseId="dpg-1" />);
    await userEvent.click(
      within(screen.getByTestId("admin-app-pgadmin")).getByRole("button", {
        name: "Deploy app",
      }),
    );
    await userEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Deploy pgAdmin",
      }),
    );
    expect(
      within(screen.getByRole("dialog")).getByRole("alert"),
    ).toHaveTextContent(
      "Add a payment method to this workspace to deploy pgAdmin.",
    );
  });

  it("disables Deploy app for a caller who cannot read the database's connection", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({
        role: "CONTRIBUTOR",
        canViewSensitive: false,
        reasonKey: (action) =>
          action === "can_view_sensitive"
            ? "capabilities.reasonRole"
            : undefined,
      }),
    );
    render(<AdminAppsPanel databaseId="dpg-1" />);
    for (const button of screen.getAllByRole("button", {
      name: "Deploy app",
    })) {
      expect(button).toBeDisabled();
    }
  });

  it("disables Deploy app for a Developer when the database's environment is protected", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    state.environmentId = "env-prod";
    environments.list = [{ id: "env-prod", protectedStatus: "protected" }];
    render(<AdminAppsPanel databaseId="dpg-1" />);
    for (const button of screen.getAllByRole("button", {
      name: "Deploy app",
    })) {
      expect(button).toBeDisabled();
    }
    // An Admin may.
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    render(<AdminAppsPanel databaseId="dpg-1" />);
    expect(
      screen
        .getAllByRole("button", { name: "Deploy app" })
        .some((b) => !b.hasAttribute("disabled")),
    ).toBe(true);
  });

  it("shows a deployed app's status, Open app and Manage instead of Deploy app", () => {
    state.apps = catalog({});
    render(<AdminAppsPanel databaseId="dpg-1" />);
    const pghero = screen.getByTestId("admin-app-pghero");
    expect(
      within(pghero).queryByRole("button", { name: "Deploy app" }),
    ).toBeNull();
    expect(
      within(pghero).getByRole("link", { name: /open app/i }),
    ).toHaveAttribute("href", "https://pghero.example.com");
    expect(
      within(pghero).getByRole("link", { name: /manage/i }),
    ).toHaveAttribute("href", "/services/srv-hero");
    // pgAdmin is still undeployed.
    expect(
      within(screen.getByTestId("admin-app-pgadmin")).getByRole("button", {
        name: "Deploy app",
      }),
    ).toBeEnabled();
  });

  it("disables Open app while the app deploys or is suspended", () => {
    for (const service of [{ phase: "Building" }, { suspended: true }]) {
      state.apps = catalog(service);
      const { unmount } = render(<AdminAppsPanel databaseId="dpg-1" />);
      const pghero = screen.getByTestId("admin-app-pghero");
      expect(
        within(pghero).getByRole("button", { name: /open app/i }),
      ).toBeDisabled();
      unmount();
    }
  });

  it("draws the skeleton's card frame while loading", () => {
    state.loading = true;
    render(<AdminAppsPanel databaseId="dpg-1" />);
    expect(screen.getByTestId("admin-apps-skeleton")).toBeInTheDocument();
  });
});

describe("AdminAppsPanelSkeleton", () => {
  it("previews the ready section: its title, description and two app cards", () => {
    render(<AdminAppsPanelSkeleton />);
    const skeleton = screen.getByTestId("admin-apps-skeleton");
    expect(within(skeleton).getByText("Apps")).toBeInTheDocument();
    expect(within(skeleton).getByText(/private network/i)).toBeInTheDocument();
    expect(skeleton.querySelectorAll(".rounded-lg.border.p-4")).toHaveLength(2);
  });
});
