import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecoveryPanel } from "@/features/databases/components/recovery-panel";
import { formatDateTime } from "@/common/lib/format";
import { toLocalInput } from "@/features/databases/lib/restore-time";
import type {
  BackupItem,
  ExportItem,
  RecoveryInfo,
} from "@/features/databases/hooks/use-recovery";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

const createExport = vi.fn();
const downloadExport = vi.fn();
const recover = vi.fn();

function windowInfo(): RecoveryInfo {
  return {
    enabled: true,
    earliestRecoveryTime: "2026-07-13T12:00:00Z",
    latestRecoveryTime: "2026-07-14T12:00:00Z",
    backups: [] as BackupItem[],
  };
}

const state: {
  exports: ExportItem[];
  exportInProgress: boolean;
  info: RecoveryInfo;
  error: Error | undefined;
  downloadingExportId: string | null;
} = {
  exports: [],
  exportInProgress: false,
  info: windowInfo(),
  error: undefined,
  downloadingExportId: null,
};

vi.mock("@/features/databases/hooks/use-recovery", () => ({
  useRecovery: () => ({
    info: state.info,
    exports: state.exports,
    exportInProgress: state.exportInProgress,
    loading: false,
    error: state.error,
    exporting: false,
    recovering: false,
    createExport,
    downloadExport,
    downloadingExportId: state.downloadingExportId,
    recover,
  }),
}));

beforeEach(() => {
  state.exports = [];
  state.exportInProgress = false;
  state.info = windowInfo();
  state.error = undefined;
  state.downloadingExportId = null;
  createExport.mockReset();
  downloadExport.mockReset();
  recover.mockReset();
  navigate.mockReset();
});

describe("RecoveryPanel restore window", () => {
  it("shows exact restore-point boundaries once the window is established", () => {
    render(<RecoveryPanel id="db" />);

    expect(
      screen.getByText(formatDateTime("2026-07-13T12:00:00Z")!),
    ).toBeInTheDocument();
    expect(
      screen.getByText(formatDateTime("2026-07-14T12:00:00Z")!),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Restore to new instance" }),
    ).toBeEnabled();
  });

  it("shows honest boundaries and disables restore until a window is established", async () => {
    // The backend now omits latestRecoveryTime until the PITR window is actually
    // open (w6/m117): the card must never name a precise "latest restore point"
    // beside an unestablished earliest boundary and an empty backup list. Both
    // boundaries share one fallback so they can never contradict each other.
    state.info = {
      enabled: true,
      earliestRecoveryTime: null,
      latestRecoveryTime: null,
      backups: [],
    };
    render(<RecoveryPanel id="db" />);

    expect(screen.getAllByText("Not established yet")).toHaveLength(2);
    const restore = screen.getByRole("button", {
      name: "Restore to new instance",
    });
    expect(restore).toBeDisabled();
    expect(restore).toHaveAttribute(
      "title",
      "Restore becomes available once the first recoverability point is established.",
    );
    expect(
      screen.getByText(
        "Restore becomes available once the first recoverability point is established.",
      ),
    ).toBeInTheDocument();
    await userEvent.setup().click(restore);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("distinguishes an unreadable recovery source from a plan without backups", () => {
    state.info = {
      enabled: false,
      earliestRecoveryTime: null,
      latestRecoveryTime: null,
      backups: [],
    };
    state.error = new Error("forbidden");

    render(<RecoveryPanel id="db" />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Could not read recovery information. Please try again.",
    );
    expect(
      screen.queryByText(/Backups aren't enabled for this plan/),
    ).not.toBeInTheDocument();
  });
});

describe("RecoveryPanel logical exports", () => {
  it("renders an available export with a Download button and a failed reason honestly", async () => {
    state.exports = [
      {
        id: "exp-available",
        status: "available",
        createdAt: "2026-07-14T11:00:00Z",
        expiresAt: "2026-07-21T11:00:00Z",
        filename: "2026-07-14T11_00Z.dir.tar.gz",
        failureReason: null,
      },
      {
        id: "exp-failed",
        status: "failed",
        createdAt: "2026-07-14T10:00:00Z",
        expiresAt: "2026-07-21T10:00:00Z",
        filename: "2026-07-14T10_00Z.dir.tar.gz",
        failureReason: "S3 credentials were rejected",
      },
    ];

    render(<RecoveryPanel id="db" />);

    // The list holds no URL: pressing Download is what asks for one, and what
    // bex-api records as a download (w21/105). A link with an href would have
    // been minted by every poll.
    expect(screen.queryByRole("link", { name: "Download" })).toBeNull();
    const download = screen.getAllByRole("button", { name: "Download" })[0]!;
    expect(download).toBeEnabled();
    expect(downloadExport).not.toHaveBeenCalled();
    await userEvent.setup().click(download);
    expect(downloadExport).toHaveBeenCalledTimes(1);
    expect(downloadExport).toHaveBeenCalledWith(
      expect.objectContaining({ id: "exp-available" }),
    );
    // The failed export's Download stays disabled and says why.
    expect(
      screen.getAllByRole("button", { name: "Download" })[1],
    ).toBeDisabled();
    expect(screen.getByText("Available")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(
      screen.getByText("Export failed: S3 credentials were rejected"),
    ).toBeInTheDocument();
  });

  it("disables the Download that is fetching its link", () => {
    state.downloadingExportId = "exp-available";
    state.exports = [
      {
        id: "exp-available",
        status: "available",
        createdAt: "2026-07-14T11:00:00Z",
        expiresAt: "2026-07-21T11:00:00Z",
        filename: "2026-07-14T11_00Z.dir.tar.gz",
        failureReason: null,
      },
    ];

    render(<RecoveryPanel id="db" />);

    expect(screen.getByRole("button", { name: "Download" })).toBeDisabled();
  });

  it("disables creating and downloading while another export is running", () => {
    state.exportInProgress = true;
    state.exports = [
      {
        id: "exp-running",
        status: "running",
        createdAt: "2026-07-14T11:00:00Z",
        expiresAt: "2026-07-21T11:00:00Z",
        filename: "2026-07-14T11_00Z.dir.tar.gz",
        failureReason: null,
      },
    ];

    render(<RecoveryPanel id="db" />);

    expect(
      screen.getByRole("button", { name: "Create export" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Download" })).toBeDisabled();
    expect(
      screen.getByText("The export is still being prepared."),
    ).toBeInTheDocument();
  });

  it("opens the recovered database by its returned immutable id", async () => {
    recover.mockResolvedValue("dpg-recovered-id");
    const user = userEvent.setup();
    render(<RecoveryPanel id="dpg-source" />);

    await user.click(
      screen.getByRole("button", { name: "Restore to new instance" }),
    );
    await user.type(screen.getByLabelText("New database name"), "recovered");
    await user.click(screen.getByRole("button", { name: "Restore" }));

    expect(recover).toHaveBeenCalledWith({
      name: "recovered",
      targetTime: undefined,
    });
    expect(navigate).toHaveBeenCalledWith({
      to: "/databases/$databaseId",
      params: { databaseId: "dpg-recovered-id" },
    });
  });

  // w21/020: the picked wall time is the viewer's, not UTC.
  it("sends the instant the picked local time denotes and bounds the field", async () => {
    recover.mockResolvedValue("dpg-recovered-id");
    const user = userEvent.setup();
    render(<RecoveryPanel id="dpg-source" />);

    await user.click(
      screen.getByRole("button", { name: "Restore to new instance" }),
    );
    await user.type(screen.getByLabelText("New database name"), "recovered");
    const time = screen.getByLabelText(/Point in time/);
    expect(time).toHaveAttribute(
      "min",
      toLocalInput(Date.parse("2026-07-13T12:00:00Z")),
    );
    expect(time).toHaveAttribute("max");
    expect(screen.getByText(/local time zone/)).toBeInTheDocument();

    fireEvent.change(time, {
      target: { value: toLocalInput(Date.parse("2026-07-13T18:30:00Z")) },
    });
    await user.click(screen.getByRole("button", { name: "Restore" }));
    expect(recover).toHaveBeenCalledWith({
      name: "recovered",
      targetTime: "2026-07-13T18:30:00Z",
    });
  });

  it("disables Restore for a time outside the recovery window", async () => {
    const user = userEvent.setup();
    render(<RecoveryPanel id="dpg-source" />);

    await user.click(
      screen.getByRole("button", { name: "Restore to new instance" }),
    );
    await user.type(screen.getByLabelText("New database name"), "recovered");
    fireEvent.change(screen.getByLabelText(/Point in time/), {
      target: { value: "2026-06-01T12:00" },
    });
    expect(screen.getByRole("button", { name: "Restore" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "earliest restore point",
    );
  });

  it("keeps the recovery dialog in place when recovery fails", async () => {
    recover.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<RecoveryPanel id="dpg-source" />);

    await user.click(
      screen.getByRole("button", { name: "Restore to new instance" }),
    );
    await user.type(screen.getByLabelText("New database name"), "recovered");
    await user.click(screen.getByRole("button", { name: "Restore" }));

    expect(navigate).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByLabelText("New database name")).toHaveValue("recovered");
  });

  // w8/258: a user can create more databases in an instance (w8/m155), and a
  // logical export dumps the primary one only. The panel says which.
  it("names the one database an export contains", () => {
    const { rerender } = render(<RecoveryPanel id="db" databaseName="shop" />);
    expect(
      screen.getByText(
        "An export contains the database shop only. Other databases created in this instance are not included.",
      ),
    ).toBeInTheDocument();
    rerender(<RecoveryPanel id="db" />);
    expect(screen.queryByText(/An export contains the database/)).toBeNull();
  });
});
