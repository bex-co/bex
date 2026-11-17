import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { AuditLogExport } from "@/features/audit/components/audit-log-export";
import { AuditLogPanelSkeleton } from "@/features/audit/components/audit-log-panel-skeleton";

const fetchAuditLogRange = vi.fn();
vi.mock("@/features/audit/lib/audit-csv", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/audit/lib/audit-csv")>()),
  fetchAuditLogRange: (...args: unknown[]) => fetchAuditLogRange(...args),
}));

const PATH = "/v1/owners/tea-1/audit-logs";

const createObjectURL = vi.fn(() => "blob:audit");
const click = vi
  .spyOn(HTMLAnchorElement.prototype, "click")
  .mockImplementation(() => {});

beforeEach(() => {
  fetchAuditLogRange.mockReset();
  createObjectURL.mockClear();
  click.mockClear();
  URL.createObjectURL = createObjectURL;
  URL.revokeObjectURL = vi.fn();
});

const field = (label: string) =>
  screen.getByLabelText(`${label} (UTC)`) as HTMLInputElement;

// w3/m305: Render's audit export — a start and end date at most 90 days apart.
describe("AuditLogExport", () => {
  it("refuses a range longer than 90 days or ending before it starts", () => {
    render(<AuditLogExport subjectId="tea-1" path={PATH} />);
    fireEvent.change(field("Start date"), { target: { value: "2026-01-01" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-10" } });
    expect(
      screen.getByRole("button", { name: /export as csv/i }),
    ).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("at most 90 days");

    fireEvent.change(field("Start date"), { target: { value: "2026-11-10" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-01" } });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "on or after the start date",
    );
  });

  it("downloads the range's REST rows as Render's CSV", async () => {
    fetchAuditLogRange.mockResolvedValue([
      {
        timestamp: "2026-11-02T10:00:00Z",
        event: "apps.Get",
        status: "error",
        actor: { type: "user", id: "usr-1", email: "ada@example.com" },
        metadata: { outcome: "denied" },
      },
    ]);
    render(<AuditLogExport subjectId="tea-1" path={PATH} />);
    fireEvent.change(field("Start date"), { target: { value: "2026-11-01" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-10" } });
    fireEvent.click(screen.getByRole("button", { name: /export as csv/i }));

    await waitFor(() => expect(click).toHaveBeenCalledTimes(1));
    expect(fetchAuditLogRange).toHaveBeenCalledWith(
      PATH,
      "2026-11-01",
      "2026-11-10",
      expect.any(AbortSignal),
    );
    const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
    expect(await blob.text()).toBe(
      "timestamp,actor,event,status,metadata\r\n" +
        '2026-11-02T10:00:00Z,ada@example.com,apps.Get,error,"{""outcome"":""denied""}"\r\n',
    );
  });

  it("says so when the range has too many events", async () => {
    const { AuditExportTooLargeError } =
      await import("@/features/audit/lib/audit-csv");
    fetchAuditLogRange.mockRejectedValue(new AuditExportTooLargeError());
    render(<AuditLogExport subjectId="tea-1" path={PATH} />);
    fireEvent.click(screen.getByRole("button", { name: /export as csv/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Choose a shorter range",
    );
    expect(click).not.toHaveBeenCalled();
  });

  // w3/m305: the page skeleton previews this row with its own labels, button
  // and hint, so it wraps as the ready row does (production at 390 px: the
  // bars' 144 px row became the ready 208 px one).
  it("is previewed by the page skeleton with its own labels, button and hint", () => {
    const parts = (root: HTMLElement) => ({
      labels: [...root.querySelectorAll("label")].map((l) => l.textContent),
      action: (root.querySelector("button") ?? root.querySelector(".invisible"))
        ?.textContent,
      hint: root.querySelector("p")?.textContent,
    });
    const ready = render(<AuditLogExport subjectId="tea-1" path={PATH} />);
    const readyParts = parts(ready.container);
    ready.unmount();

    render(<AuditLogPanelSkeleton />);
    expect(parts(screen.getByTestId("audit-log-export-frame"))).toEqual(
      readyParts,
    );
    expect(readyParts.labels).toEqual(["Start date (UTC)", "End date (UTC)"]);
  });
});
