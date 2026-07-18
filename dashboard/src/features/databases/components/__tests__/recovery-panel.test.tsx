import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecoveryPanel } from "@/features/databases/components/recovery-panel";
import type { ExportItem } from "@/features/databases/hooks/use-recovery";

const navigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

const createExport = vi.fn();
const recover = vi.fn();
const state: {
  exports: ExportItem[];
  exportInProgress: boolean;
} = { exports: [], exportInProgress: false };

vi.mock("@/features/databases/hooks/use-recovery", () => ({
  useRecovery: () => ({
    info: {
      enabled: true,
      earliestRecoveryTime: null,
      latestRecoveryTime: "2026-07-14T12:00:00Z",
      backups: [],
    },
    exports: state.exports,
    exportInProgress: state.exportInProgress,
    loading: false,
    exporting: false,
    recovering: false,
    createExport,
    recover,
  }),
}));

beforeEach(() => {
  state.exports = [];
  state.exportInProgress = false;
  createExport.mockReset();
  recover.mockReset();
  navigate.mockReset();
});

describe("RecoveryPanel logical exports", () => {
  it("renders an available export as a real download and a failed reason honestly", () => {
    state.exports = [
      {
        id: "exp-available",
        status: "available",
        createdAt: "2026-07-14T11:00:00Z",
        url: "https://objects.example.com/signed-export",
        urlExpiresAt: "2026-07-14T12:15:00Z",
        expiresAt: "2026-07-21T11:00:00Z",
        filename: "2026-07-14T11_00Z.dir.tar.gz",
        failureReason: null,
      },
      {
        id: "exp-failed",
        status: "failed",
        createdAt: "2026-07-14T10:00:00Z",
        url: null,
        urlExpiresAt: null,
        expiresAt: "2026-07-21T10:00:00Z",
        filename: "2026-07-14T10_00Z.dir.tar.gz",
        failureReason: "S3 credentials were rejected",
      },
    ];

    render(<RecoveryPanel id="db" />);

    const download = screen.getByRole("link", { name: "Download" });
    expect(download).toHaveAttribute(
      "href",
      "https://objects.example.com/signed-export",
    );
    expect(download).toHaveAttribute(
      "download",
      "2026-07-14T11_00Z.dir.tar.gz",
    );
    expect(screen.getByText("Available")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(
      screen.getByText("Export failed: S3 credentials were rejected"),
    ).toBeInTheDocument();
  });

  it("disables creating and downloading while another export is running", () => {
    state.exportInProgress = true;
    state.exports = [
      {
        id: "exp-running",
        status: "running",
        createdAt: "2026-07-14T11:00:00Z",
        url: null,
        urlExpiresAt: null,
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
});
