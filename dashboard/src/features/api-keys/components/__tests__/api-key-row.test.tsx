import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiKeyRow } from "@/features/api-keys/components/api-key-row";
import type { ApiKeyView } from "@/features/api-keys/types";
import { hydrateAcrossBoundary } from "@/test/hydration";

const copy = vi.fn();
vi.mock("@/common/hooks/use-copy-to-clipboard", () => ({
  useCopyToClipboard: () => ({ copied: false, copy }),
}));

vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));

const entry: ApiKeyView = {
  id: "key-1",
  name: "deploy-agent",
  createdAt: "2026-07-01T00:00:00Z",
  createdBy: "user:minter",
  lastUsedAt: "2026-07-05T00:00:00Z",
  permissions: { all: true, grants: [] },
};

beforeEach(() => {
  vi.setSystemTime(new Date("2026-07-08T00:00:00Z"));
});

describe("ApiKeyRow — metadata columns (w4/m13/t003, w4/m105, w9/132)", () => {
  it("shows client_id with a copy affordance beside the ages, and no creator in the caller's own list", () => {
    const { container } = render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={vi.fn()} revoking={false} />
        </tbody>
      </table>,
    );
    expect(screen.getByText("key-1")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy client ID for deploy-agent" }),
    ).toBeInTheDocument();
    expect(screen.getByText("3d")).toBeInTheDocument(); // 2026-07-05 → 2026-07-08
    // Render's columns: name, permissions, key, created, last used, revoke.
    expect(container.querySelectorAll("td")).toHaveLength(6);
    expect(screen.queryByText("user:minter")).not.toBeInTheDocument();
  });

  it("names the creator under the key's name in an Admin's every-member view", () => {
    const { container } = render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={entry}
            namesCreator
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
    const nameCell = container.querySelectorAll("td")[0];
    expect(nameCell).toHaveTextContent("deploy-agentCreated by user:minter");
    expect(within(nameCell).getByText("user:minter")).toBeInTheDocument();
    expect(container.querySelectorAll("td")).toHaveLength(6);
  });

  it("renders a 'Never' last-used and an em dash for a key with no recorded creator", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, createdBy: null, lastUsedAt: null }}
            namesCreator
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
    expect(screen.getByText("Never")).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  // w6/m102: the relative ages in this row are rendered during the settings
  // route's blocking SSR pass, then again at hydration against a fresh
  // Date.now(). A bucket boundary crossed in between ("59m" → "1h" here) used to
  // surface as React error #418; RelativeAge carries the guard.
  it("hydrates across a relative-age bucket boundary without a React #418", () => {
    const lastUsedAt = "2026-07-08T00:00:00Z";
    const row = (
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, lastUsedAt }}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>
    );

    const { html, recovered } = hydrateAcrossBoundary(row, {
      serverNow: Date.parse(lastUsedAt) + 59 * 60_000,
      clientNow: Date.parse(lastUsedAt) + 60 * 60_000,
    });
    expect(html).toContain(">59m<");
    expect(recovered).toEqual([]);
  });
});

describe("ApiKeyRow — revoke with confirmation (w4/m8/t002)", () => {
  it("does not revoke on the row button alone — a confirmation dialog gates it", async () => {
    const onRevoke = vi.fn();
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    expect(onRevoke).not.toHaveBeenCalled();
    expect(await screen.findByText("Revoke deploy-agent?")).toBeInTheDocument();
  });

  it("revokes only after the confirmation is accepted", async () => {
    const onRevoke = vi.fn().mockResolvedValue(true);
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.click(
      within(dialog).getAllByRole("button", { name: "Revoke" })[0],
    );

    expect(onRevoke).toHaveBeenCalledWith("key-1", "deploy-agent");
  });

  it("cancel closes the dialog without revoking", async () => {
    const onRevoke = vi.fn();
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={onRevoke} revoking={false} />
        </tbody>
      </table>,
    );

    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    await screen.findByRole("alertdialog");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onRevoke).not.toHaveBeenCalled();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("disables the row control while this row is revoking", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={vi.fn()} revoking={true} />
        </tbody>
      </table>,
    );
    expect(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    ).toBeDisabled();
  });

  it("names the revoke control after the key (w4/084)", () => {
    render(
      <table>
        <tbody>
          <ApiKeyRow entry={entry} onRevoke={vi.fn()} revoking={false} />
        </tbody>
      </table>,
    );
    expect(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    ).toBeInTheDocument();
  });
});

describe("ApiKeyRow — permissions summary and gear (w9/m496)", () => {
  function renderRow(permissions: ApiKeyView["permissions"]) {
    return render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, permissions }}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
  }

  /** The permissions cell: the row's second, after the name. */
  function permissionsCell(container: HTMLElement) {
    return container.querySelectorAll("td")[1];
  }

  it.each<[ApiKeyView["permissions"], string]>([
    [{ all: true, grants: [] }, "All"],
    [
      {
        all: false,
        grants: [
          { permission: "services", level: "read" },
          { permission: "git", level: "read" },
        ],
      },
      "Read only2 permissions",
    ],
    [{ all: false, grants: [] }, "No permissions"],
    [
      {
        all: false,
        grants: [
          { permission: "services", level: "read_write" },
          { permission: "services_env", level: "read" },
          { permission: "git", level: "read" },
        ],
      },
      "Custom3 permissions",
    ],
    [
      {
        all: false,
        grants: [{ permission: "deploys_run", level: "run" }],
      },
      "Custom1 permission",
    ],
  ])("summarizes %j as %s", (permissions, summary) => {
    const { container } = renderRow(permissions);
    expect(permissionsCell(container)).toHaveTextContent(summary, {
      normalizeWhitespace: true,
    });
  });

  it("puts a gear named after the key beside the summary", () => {
    const { container } = renderRow({ all: true, grants: [] });
    expect(
      within(permissionsCell(container)).getByRole("button", {
        name: "Change permissions for deploy-agent",
      }),
    ).toBeInTheDocument();
  });
});

describe("ApiKeyRow — another member's key in an Admin's every-member view (w9/m539)", () => {
  it("carries no gear, and its revoke confirmation names the creator", async () => {
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={entry}
            own={false}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
    expect(
      screen.queryByRole("button", {
        name: "Change permissions for deploy-agent",
      }),
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "user:minter created this key.",
    );
  });

  it("keeps the plain confirmation for a key with no recorded creator", async () => {
    const user = userEvent.setup();
    render(
      <table>
        <tbody>
          <ApiKeyRow
            entry={{ ...entry, createdBy: null }}
            own={false}
            onRevoke={vi.fn()}
            revoking={false}
          />
        </tbody>
      </table>,
    );
    await user.click(
      screen.getByRole("button", { name: "Revoke API key deploy-agent" }),
    );
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "Anything authenticating with this key will stop working immediately.",
    );
  });
});
