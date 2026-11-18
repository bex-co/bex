import { afterEach, describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { config } from "@/config/config";
import { OrganizationSettingsPage } from "@/features/organizations/pages/organization-settings-page";
import { OrganizationSettingsRouteSkeleton } from "@/features/organizations/components/organization-route-skeletons";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
import type { UseOrganizationResult } from "@/features/organizations/hooks/use-organization";
import {
  organization,
  pageStructure,
  renderAtOrgRoute,
} from "@/features/organizations/__tests__/org-fixtures";
import {
  toOrganizationRole,
  type Organization,
  type OrganizationRole,
} from "@/features/organizations/types";

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => (
    <main>{children}</main>
  ),
}));

// The org's login check reads through Apollo (w7/m466); these pages' tests
// stand for a caller the org does not refuse.
vi.mock("@/features/organizations/hooks/use-organization-login-check", () => ({
  useOrganizationLoginCheck: () => null,
}));

let orgState: UseOrganizationResult;
vi.mock("@/features/organizations/hooks/use-organization", () => ({
  useOrganization: () => orgState,
}));

const rename = vi.fn(async () => true);
vi.mock("@/features/organizations/hooks/use-rename-organization", () => ({
  useRenameOrganization: () => ({ rename, saving: false }),
}));

// The Security switches' mutations are exercised against a real client in
// organization-security-card.test.tsx; here they only record the call.
const setRequired = vi.fn(
  async (_requirement: string, _enabled: boolean) => {},
);
vi.mock(
  "@/features/organizations/hooks/use-set-organization-login-requirement",
  () => ({
    useSetOrganizationLoginRequirement: (requirement: string) => ({
      setRequired: (enabled: boolean) => setRequired(requirement, enabled),
      busy: false,
      error: null,
    }),
  }),
);

const createObjectURL = vi.fn(() => "blob:audit");
const click = vi
  .spyOn(HTMLAnchorElement.prototype, "click")
  .mockImplementation(() => {});

beforeEach(() => {
  rename.mockClear();
  setRequired.mockClear();
  createObjectURL.mockClear();
  click.mockClear();
  URL.createObjectURL = createObjectURL;
  URL.revokeObjectURL = vi.fn();
  vi.mocked(useOrganizations).mockReturnValue({
    organizations: [],
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

async function renderPage(
  role: OrganizationRole,
  settings: Partial<Organization> = {},
) {
  const org = { ...organization(role), ...settings };
  orgState = {
    organization: org,
    role: toOrganizationRole(org.viewerRole),
    loading: false,
    notFound: false,
    workspaceName: () => undefined,
  };
  const view = renderAtOrgRoute(
    "settings",
    <OrganizationSettingsPage organizationId="org-1" />,
  );
  await screen.findByRole("heading", { level: 1, name: "Acme" });
  return view;
}

const field = (label: string) =>
  screen.getByLabelText(`${label} (UTC)`) as HTMLInputElement;

describe("OrganizationSettingsPage (w7/m464)", () => {
  it("lets an Owner rename the org", async () => {
    await renderPage("OWNER");
    const user = userEvent.setup();
    const name = screen.getByLabelText("Name");
    expect(name).toHaveValue("Acme");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    await user.clear(name);
    await user.type(name, " Acme Labs ");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(rename).toHaveBeenCalledWith("Acme Labs");
  });

  // Render: "Only org members with the Org Owner role can export these audit
  // logs", hidden as the workspace card is for non-admins.
  it.each(["MEMBER", "GUEST"] as const)(
    "hides Audit Logs and the rename from a %s",
    async (role) => {
      await renderPage(role);
      expect(screen.queryByText("Audit Logs")).toBeNull();
      expect(
        screen.queryByRole("button", { name: /export as csv/i }),
      ).toBeNull();
      expect(screen.getByLabelText("Name")).toBeDisabled();
      expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
      expect(
        screen.getByText("Only Owners can rename the organization."),
      ).toBeInTheDocument();
    },
  );

  it("refuses an export range longer than 90 days", async () => {
    await renderPage("OWNER");
    expect(screen.getByText("Audit Logs")).toBeInTheDocument();
    fireEvent.change(field("Start date"), { target: { value: "2026-01-01" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-10" } });
    expect(
      screen.getByRole("button", { name: /export as csv/i }),
    ).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("at most 90 days");
  });

  it("downloads the org's REST rows as the workspace export's CSV", async () => {
    const fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            cursor: "aud-1",
            auditLog: {
              timestamp: "2026-11-02T10:00:00Z",
              event: "ChangeOrgRoleEvent",
              status: "success",
              actor: { type: "user", id: "usr-1", email: "ada@example.com" },
              metadata: { from: "member", to: "owner" },
            },
          },
        ]),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetch);
    await renderPage("OWNER");
    fireEvent.change(field("Start date"), { target: { value: "2026-11-01" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-10" } });
    fireEvent.click(screen.getByRole("button", { name: /export as csv/i }));

    await waitFor(() => expect(click).toHaveBeenCalledTimes(1));
    const url = fetch.mock.calls[0][0] as string;
    expect(
      url.startsWith(`${config.apiBaseUrl}/v1/organizations/org-1/audit-logs?`),
    ).toBe(true);
    expect(
      Object.fromEntries(new URL(url, "http://api.invalid").searchParams),
    ).toEqual({
      startTime: "2026-11-01T00:00:00Z",
      endTime: "2026-11-10T23:59:59.999999Z",
      limit: "1000",
    });
    const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
    expect(await blob.text()).toBe(
      "timestamp,actor,event,status,metadata\r\n" +
        '2026-11-02T10:00:00Z,ada@example.com,ChangeOrgRoleEvent,success,"{""from"":""member"",""to"":""owner""}"\r\n',
    );
  });

  // w7/m466: Org Settings -> Security, where Render puts "Require Login
  // Method". Owners get both switches; turning one on asks first.
  it("gives an Owner both Security switches, asking before turning one on", async () => {
    await renderPage("OWNER", { requireGoogleLogin: true });
    const user = userEvent.setup();
    expect(
      screen.getByRole("heading", { level: 2, name: "Security" }),
    ).toBeInTheDocument();
    const twoFactor = screen.getByRole("switch", {
      name: "Require two-factor authentication",
    });
    const google = screen.getByRole("switch", { name: "Require Google login" });
    expect(twoFactor).not.toBeChecked();
    expect(google).toBeChecked();
    expect(
      screen.queryByText("Only Owners can change these settings."),
    ).toBeNull();

    await user.click(twoFactor);
    const dialog = await screen.findByRole("alertdialog", {
      name: "Require two-factor authentication in Acme?",
    });
    expect(dialog).toHaveTextContent(
      "Members without two-factor authentication lose access to the organization and all of its workspaces",
    );
    expect(setRequired).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Require" }));
    expect(setRequired).toHaveBeenCalledWith("twoFactor", true);

    // Turning one off needs no confirmation, as on the workspace cards.
    await user.click(google);
    expect(setRequired).toHaveBeenLastCalledWith("googleLogin", false);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("sends nothing when the Owner cancels the confirmation", async () => {
    await renderPage("OWNER");
    const user = userEvent.setup();
    await user.click(
      screen.getByRole("switch", { name: "Require Google login" }),
    );
    await screen.findByRole("alertdialog", {
      name: "Require Google login in Acme?",
    });
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    expect(setRequired).not.toHaveBeenCalled();
  });

  // Members and Guests read each setting's state; only Owners change it.
  it.each(["MEMBER", "GUEST"] as const)(
    "shows a %s the Security settings read-only",
    async (role) => {
      await renderPage(role, { requireTwoFactor: true });
      expect(screen.queryByRole("switch")).toBeNull();
      const rows = document.querySelectorAll("[data-login-requirement]");
      expect(
        [...rows].map((row) => [
          row.querySelector("label")?.textContent,
          row.querySelector("[data-slot=badge]")?.textContent,
        ]),
      ).toEqual([
        ["Require two-factor authentication", "Required"],
        ["Require Google login", "Not required"],
      ]);
      expect(
        screen.getByText("Only Owners can change these settings."),
      ).toBeInTheDocument();
    },
  );

  // The route's pending frame is the ready page's structure: the name card
  // (Save for an Owner, the Owners-only note otherwise), Security (each
  // setting's label, description and control box; the Owners-only note for
  // a Member or Guest) and, for an Owner, Audit Logs with its export row.
  it.each(["OWNER", "MEMBER", "GUEST"] as const)(
    "previews the ready page for an %s",
    async (role) => {
      const ready = await renderPage(role);
      const readyStructure = pageStructure(ready.container);
      ready.unmount();

      vi.mocked(useOrganizations).mockReturnValue({
        organizations: [organization(role)],
      });
      const pending = renderAtOrgRoute(
        "settings",
        <OrganizationSettingsRouteSkeleton />,
      );
      await waitFor(() =>
        expect(
          pending.container.querySelector(
            '[data-route-skeleton="organization-settings"]',
          ),
        ).not.toBeNull(),
      );
      expect(pageStructure(pending.container)).toEqual(readyStructure);
      expect(readyStructure.cards).toBe(role === "OWNER" ? 3 : 2);
      expect(readyStructure.loginRequirements.map((row) => row.name)).toEqual([
        "two-factor",
        "google-login",
      ]);
      expect(readyStructure.notes).toEqual(
        role === "OWNER"
          ? []
          : [
              "Only Owners can rename the organization.",
              "Only Owners can change these settings.",
            ],
      );
    },
  );
});
