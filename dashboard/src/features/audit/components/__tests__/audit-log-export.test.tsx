import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { AuditLogExport } from "@/features/audit/components/audit-log-export";

const fetchAuditRange = vi.fn();
vi.mock("@/features/audit/lib/audit-csv", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/audit/lib/audit-csv")>()),
  fetchAuditRange: (...args: unknown[]) => fetchAuditRange(...args),
}));
vi.mock("@apollo/client/react", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@apollo/client/react")>()),
  useApolloClient: () => ({ client: true }),
}));

const createObjectURL = vi.fn(() => "blob:audit");
const click = vi
  .spyOn(HTMLAnchorElement.prototype, "click")
  .mockImplementation(() => {});

beforeEach(() => {
  fetchAuditRange.mockReset();
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
    render(<AuditLogExport ownerId="tea-1" />);
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

  it("downloads the range as Render's CSV", async () => {
    fetchAuditRange.mockResolvedValue([]);
    render(<AuditLogExport ownerId="tea-1" />);
    fireEvent.change(field("Start date"), { target: { value: "2026-11-01" } });
    fireEvent.change(field("End date"), { target: { value: "2026-11-10" } });
    fireEvent.click(screen.getByRole("button", { name: /export as csv/i }));

    await waitFor(() => expect(click).toHaveBeenCalledTimes(1));
    expect(fetchAuditRange).toHaveBeenCalledWith(
      { client: true },
      "tea-1",
      "2026-11-01",
      "2026-11-10",
      expect.any(AbortSignal),
    );
    const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
    expect(await blob.text()).toBe("timestamp,actor,event,status,metadata\r\n");
  });

  it("says so when the range has too many events", async () => {
    const { AuditExportTooLargeError } =
      await import("@/features/audit/lib/audit-csv");
    fetchAuditRange.mockRejectedValue(new AuditExportTooLargeError());
    render(<AuditLogExport ownerId="tea-1" />);
    fireEvent.click(screen.getByRole("button", { name: /export as csv/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Choose a shorter range",
    );
    expect(click).not.toHaveBeenCalled();
  });
});
