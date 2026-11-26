import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OrganizationAuditLogKeys } from "@/features/organizations/components/organization-audit-log-keys";
import {
  refusal,
  scriptedClient,
} from "@/features/organizations/__tests__/org-fixtures";

const KEY = "bex_aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa_s3cret";

function apiToken(id: string, label: string, lastUsedAt: string | null) {
  return {
    __typename: "APIToken",
    id,
    label,
    createdAt: "2026-11-20T00:00:00Z",
    lastUsedAt,
    expiresAt: null,
    type: "AUDIT_LOG",
  };
}

function tokens(...list: ReturnType<typeof apiToken>[]) {
  return {
    organization: {
      __typename: "Organization",
      id: "org-1",
      organizationAPITokens: list,
    },
  };
}

function renderKeys(
  canManage: boolean,
  answers: Parameters<typeof scriptedClient>[0],
) {
  const script = scriptedClient(answers);
  render(
    <OrganizationAuditLogKeys organizationId="org-1" canManage={canManage} />,
    { wrapper: script.wrapper },
  );
  return { ...script, user: userEvent.setup() };
}

// w1/m657: Render's Audit Log API Keys section, its strings and its states.
describe("OrganizationAuditLogKeys", () => {
  it("lists an Owner's keys with Render's columns", async () => {
    renderKeys(true, {
      organizationAPITokens: () =>
        tokens(
          apiToken("aaaaaaaa-1111", "siem", null),
          apiToken("bbbbbbbb-2222", "archive", "2026-11-25T00:00:00Z"),
        ),
    });
    const table = await screen.findByRole("table");
    expect(
      within(table)
        .getAllByRole("columnheader")
        .map((head) => head.textContent),
    ).toEqual(["Name", "Key", "Created", "Last used", "Actions"]);
    // Render's table opens sorted by last use; a key never used comes last.
    expect(
      within(table)
        .getAllByRole("row")
        .slice(1)
        .map((row) => row.querySelector("td")?.textContent),
    ).toEqual(["archive", "siem"]);
    const siem = within(table).getByText("siem").closest("tr")!;
    expect(siem).toHaveTextContent("aaaaaaaa…");
    expect(siem).toHaveTextContent("Never");
    expect(screen.queryByText("No provisioned API keys.")).toBeNull();
  });

  it("says when the org has no keys, and when they could not be read", async () => {
    let fail = true;
    const { user } = renderKeys(true, {
      organizationAPITokens: () => (fail ? refusal("INTERNAL") : tokens()),
    });
    expect(
      await screen.findByText("Unable to load your organization's API keys."),
    ).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(
      await screen.findByText("No provisioned API keys."),
    ).toBeInTheDocument();
  });

  it("creates a key, shows it once, and lists it after", async () => {
    let created = false;
    const { user, sent } = renderKeys(true, {
      organizationAPITokens: () =>
        created ? tokens(apiToken("aaaaaaaa-1111", "siem", null)) : tokens(),
      provisionOrganizationAuditLogToken: () => {
        created = true;
        return { provisionOrganizationAuditLogToken: { token: KEY } };
      },
    });
    await user.click(
      await screen.findByRole("button", { name: "Create API key" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Create an API key",
    });
    expect(dialog).toHaveTextContent(
      "Give your API key a unique name, like “agent” or “laptop”.",
    );
    const generate = within(dialog).getByRole("button", {
      name: "Generate API key",
    });
    expect(generate).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox", { name: "API key name" }),
      "  siem ",
    );
    await user.click(generate);

    const done = await screen.findByRole("dialog", { name: "API key created" });
    expect(done).toHaveTextContent(KEY);
    expect(done).toHaveTextContent(
      "Save this API key now. You won’t be able to see it again.",
    );
    expect(
      sent.find((op) => op.name === "provisionOrganizationAuditLogToken")
        ?.variables,
    ).toEqual({ organizationId: "org-1", label: "siem" });
    await user.click(within(done).getByRole("button", { name: "Done" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(await screen.findByText("siem")).toBeInTheDocument();
    expect(screen.queryByText(KEY)).toBeNull();
  });

  it("answers too many keys with Render's words", async () => {
    const { user } = renderKeys(true, {
      organizationAPITokens: () => tokens(),
      provisionOrganizationAuditLogToken: () =>
        refusal("ORGANIZATION_API_KEY_LIMIT"),
    });
    await user.click(
      await screen.findByRole("button", { name: "Create API key" }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "API key name" }),
      "one too many",
    );
    await user.click(screen.getByRole("button", { name: "Generate API key" }));
    expect(
      await screen.findByText(
        "You have created too many keys. Please try again later.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Create an API key" }),
    ).toBeVisible();
  });

  it("revokes a key after Render's confirmation", async () => {
    let revoked = false;
    const { user, sent } = renderKeys(true, {
      organizationAPITokens: () =>
        revoked ? tokens() : tokens(apiToken("aaaaaaaa-1111", "siem", null)),
      revokeOrganizationAPIToken: () => {
        revoked = true;
        return { revokeOrganizationAPIToken: true };
      },
    });
    await user.click(
      await screen.findByRole("button", { name: "Actions for siem" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Revoke" }));
    const confirm = await screen.findByRole("alertdialog", {
      name: "Revoke API key",
    });
    expect(confirm).toHaveTextContent(
      "“siem” will no longer be able to access your organization's Audit Logs.",
    );
    await user.click(
      within(confirm).getByRole("button", { name: "Revoke API key" }),
    );
    expect(
      await screen.findByText("No provisioned API keys."),
    ).toBeInTheDocument();
    expect(
      sent.find((op) => op.name === "revokeOrganizationAPIToken")?.variables,
    ).toEqual({ organizationId: "org-1", tokenId: "aaaaaaaa-1111" });
  });

  it("keeps a failed revoke in its dialog", async () => {
    const { user } = renderKeys(true, {
      organizationAPITokens: () =>
        tokens(apiToken("aaaaaaaa-1111", "siem", null)),
      revokeOrganizationAPIToken: () => refusal("INTERNAL"),
    });
    await user.click(
      await screen.findByRole("button", { name: "Actions for siem" }),
    );
    await user.click(screen.getByRole("menuitem", { name: "Revoke" }));
    await user.click(screen.getByRole("button", { name: "Revoke API key" }));
    expect(
      await screen.findByText(
        "Unable to revoke your API key. Please try again.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("alertdialog")).toBeVisible();
  });

  it("shows anyone but an Owner the disabled button and asks nothing", async () => {
    const { sent } = renderKeys(false, {});
    expect(
      screen.getByRole("button", { name: "Create API key" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "You do not have permission to manage this organization's API keys.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
    expect(sent).toEqual([]);
  });
});
