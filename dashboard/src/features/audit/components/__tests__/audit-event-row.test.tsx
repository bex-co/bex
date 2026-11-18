import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { AuditEventRow } from "@/features/audit/components/audit-event-row";
import {
  auditLogsToCSV,
  type RenderAuditLog,
} from "@/features/audit/lib/audit-csv";
import type { AuditEvent } from "@/features/audit/types";

// The real roster hook over a mocked member list: the row names a person the
// way the export does, and the roster only fills in an email (w9/101).
const query = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => query(...args),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-primary" }),
}));

const roster = {
  workspaceMembers: [
    {
      subject: "6f1d2c3b-0a9e-4f8d-b7c6-5e4d3c2b1a09",
      userId: "usr-00000000000000000ada",
      email: "ada@example.com",
    },
  ],
};

function event(overrides: Partial<AuditEvent>): AuditEvent {
  return {
    id: "ev-1",
    timestamp: "2026-11-18T00:00:00Z",
    actor: "",
    actorMethod: "session",
    actorId: "",
    actorEmail: "",
    actorDeleted: false,
    sourceIp: "",
    userAgent: "",
    instance: "",
    action: "ViewEnvVarValuesEvent",
    status: "success",
    resource: "service:srv-1",
    targetName: "",
    displayNameFrom: "",
    displayNameTo: "",
    domain: "",
    envGroupId: "",
    envGroupName: "",
    envVarKeys: [],
    relation: "",
    oauthClientId: "",
    oauthAudience: "",
    oauthScopes: [],
    oauthGrantScopes: [],
    ...overrides,
  };
}

/** The actor cell's own label: its first line, without the method line. */
function actorLabel(row: AuditEvent): string {
  render(
    <table>
      <tbody>
        <AuditEventRow event={row} />
      </tbody>
    </table>,
  );
  const cell = within(screen.getByRole("row")).getAllByRole("cell")[1]!;
  const label = cell.querySelector("span[title]")!;
  expect(label).toHaveAttribute("title", label.textContent);
  return label.textContent ?? "";
}

/** The CSV export's actor column for the same row as REST answers it. */
function csvActorColumn(actor: RenderAuditLog["actor"], deleted = false) {
  const csv = auditLogsToCSV([
    {
      timestamp: "2026-11-18T00:00:00Z",
      event: "ViewEnvVarValuesEvent",
      status: "success",
      actor,
      metadata: {},
      actorDeleted: deleted,
    },
  ]);
  return csv.split("\r\n")[1]!.split(",")[1];
}

beforeEach(() => {
  query.mockReset();
  query.mockReturnValue({ data: roster, loading: false });
});

describe("AuditEventRow actor (w9/101)", () => {
  it("names a deleted user as the export does, never the stored marker", () => {
    const label = actorLabel(
      event({
        actor: "deleted:own-dbum0qseoepc73dlkt9g",
        actorId: "usr-dbum0qseoepc73dlkt9g",
        actorDeleted: true,
      }),
    );

    expect(label).toBe("Deleted User ID#usr-dbum0qseoepc73dlkt9g");
    expect(label).toBe(
      csvActorColumn({ type: "user", id: "usr-dbum0qseoepc73dlkt9g" }, true),
    );
    // The label is known from the row itself: no roster read.
    expect(query.mock.calls[0]![1]).toMatchObject({ skip: true });
  });

  it("names a person whose email could not be read by their usr- id, not the Kratos subject", () => {
    const label = actorLabel(
      event({
        actor: "0b7f0e5c-4ad7-4b1f-9d5e-0d1f2a3b4c5d",
        actorId: "usr-00000000000000000007",
      }),
    );

    expect(label).toBe("usr-00000000000000000007");
    expect(label).toBe(
      csvActorColumn({ type: "user", id: "usr-00000000000000000007" }),
    );
  });

  it("names a live member by the email their row carries", () => {
    const label = actorLabel(
      event({
        actor: "6f1d2c3b-0a9e-4f8d-b7c6-5e4d3c2b1a09",
        actorId: "usr-00000000000000000ada",
        actorEmail: "ada@example.com",
      }),
    );

    expect(label).toBe("ada@example.com");
    expect(label).toBe(
      csvActorColumn({
        type: "user",
        id: "usr-00000000000000000ada",
        email: "ada@example.com",
      }),
    );
  });

  it("finds a live member's email in the roster by usr- id when the row has none", () => {
    expect(
      actorLabel(
        event({
          actor: "6f1d2c3b-0a9e-4f8d-b7c6-5e4d3c2b1a09",
          actorId: "usr-00000000000000000ada",
        }),
      ),
    ).toBe("ada@example.com");
  });

  it("reads Unknown for a row with no actor at all", () => {
    render(
      <table>
        <tbody>
          <AuditEventRow event={event({ actorMethod: "" })} />
        </tbody>
      </table>,
    );
    const cell = within(screen.getByRole("row")).getAllByRole("cell")[1]!;
    expect(cell.querySelector("span[title]")).toBeNull();
    expect(cell).toHaveTextContent(/^Unknown$/);
  });
});
