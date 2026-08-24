import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseDisk = vi.fn();
const mockUseDiskSnapshots = vi.fn();
const mockAddDisk = vi.fn();
const mockGrowDisk = vi.fn();
const mockDeleteDisk = vi.fn();
const mockRestoreSnapshot = vi.fn();

vi.mock("@/features/services/hooks/use-disks", () => ({
  useDisk: (...args: unknown[]) => mockUseDisk(...args),
  useDiskSnapshots: (...args: unknown[]) => mockUseDiskSnapshots(...args),
  useDiskMutations: () => ({
    addDisk: mockAddDisk,
    growDisk: mockGrowDisk,
    deleteDisk: mockDeleteDisk,
    restoreSnapshot: mockRestoreSnapshot,
    addError: null,
    clearAddError: vi.fn(),
    busy: false,
  }),
}));

// The usage chart is the datastore metrics panel with a service kind — its own
// suite covers its behavior. Here it is stubbed to a marker so these tests stay
// about the Disk tab's controls, and so they need no Apollo provider.
const mockUseDatastoreMetrics = vi.fn();
vi.mock("@/features/metrics/hooks/use-datastore-metrics", () => ({
  useDatastoreMetrics: (...args: unknown[]) => mockUseDatastoreMetrics(...args),
}));

import { DiskSection } from "@/features/services/components/disk-section";

const noDisk = { disk: null, loading: false, error: undefined, refetch: vi.fn() };
const attached = {
  disk: { id: "dsk-1", name: "data", mountPath: "/var/data", sizeGB: 10, serviceId: "srv-1" },
  loading: false,
  error: undefined,
  refetch: vi.fn(),
};
const noSnapshots = { snapshots: [], loading: false, error: undefined, refetch: vi.fn() };

beforeEach(() => {
  vi.clearAllMocks();
  mockUseDisk.mockReturnValue(noDisk);
  mockUseDiskSnapshots.mockReturnValue(noSnapshots);
  mockAddDisk.mockResolvedValue(true);
  mockUseDatastoreMetrics.mockReturnValue({
    series: [],
    loading: false,
    unavailable: false,
    storeUnavailable: false,
    error: undefined,
    degradedSources: [],
  });
});

describe("DiskSection", () => {
  it("quotes bex's own rate, not Render's", () => {
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    // The 30%-off rate is the deliberate divergence from the captured Render
    // page; showing $0.25 would be quoting a competitor's price.
    expect(screen.getByText(/\$0\.175\/GB per month/)).toBeInTheDocument();
    expect(screen.queryByText(/\$0\.25/)).not.toBeInTheDocument();
  });

  it("refuses to offer a disk on a free service, and says why", () => {
    render(<DiskSection serviceId="srv-1" plan="free" serviceType="web_service" />);
    expect(screen.getByText(/require a paid instance type/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add disk/i })).toBeDisabled();
  });

  it("shows Render's five warnings before the form", async () => {
    const user = userEvent.setup();
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    await user.click(screen.getByRole("button", { name: /add disk/i }));

    for (const warning of [
      /disables zero-downtime deploys/i,
      /can't scale to multiple instances/i,
      /maximum of one disk per service/i,
      /only files under your disk's mount path/i,
      /other services can't access/i,
    ]) {
      expect(screen.getByText(warning)).toBeInTheDocument();
    }
  });

  it("defaults to 10GB and offers Render's quick-select sizes", async () => {
    const user = userEvent.setup();
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    await user.click(screen.getByRole("button", { name: /add disk/i }));

    for (const size of [1, 5, 10, 50, 100]) {
      expect(screen.getByRole("button", { name: `${size} GB` })).toBeInTheDocument();
    }
    expect(screen.getByLabelText(/^size$/i)).toHaveValue(10);
  });

  // These are the mistakes a mount path invites; catching them here means the
  // user reads guidance instead of a server rejection.
  it.each([
    ["data", /must be absolute/i],
    ["/", /cannot be the root directory/i],
    ["/etc/secrets", /reserved by the platform/i],
  ])("refuses %s before submitting", async (path, message) => {
    const user = userEvent.setup();
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    await user.click(screen.getByRole("button", { name: /add disk/i }));
    await user.type(screen.getByLabelText(/mount path/i), path);
    await user.click(screen.getAllByRole("button", { name: /add disk/i }).at(-1)!);

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(mockAddDisk).not.toHaveBeenCalled();
  });

  it("submits a valid disk", async () => {
    const user = userEvent.setup();
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    await user.click(screen.getByRole("button", { name: /add disk/i }));
    await user.type(screen.getByLabelText(/mount path/i), "/var/data");
    await user.click(screen.getByRole("button", { name: "50 GB" }));
    await user.click(screen.getAllByRole("button", { name: /add disk/i }).at(-1)!);

    await waitFor(() =>
      expect(mockAddDisk).toHaveBeenCalledWith({ mountPath: "/var/data", sizeGB: 50 }),
    );
  });

  // Render keeps the size field inert until you press Edit, so a page opened to
  // read cannot be nudged into an irreversible grow (live capture 2026-08-24:
  // its size input ships `disabled`, with an Edit button beside it).
  it("keeps the size field locked until Edit is pressed", async () => {
    const user = userEvent.setup();
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    const size = screen.getByLabelText(/^size$/i);
    expect(size).toBeDisabled();
    expect(screen.queryByRole("button", { name: /increase size/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /edit/i }));
    expect(size).toBeEnabled();
  });

  it("cannot express a shrink on an attached disk", async () => {
    const user = userEvent.setup();
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    await user.click(screen.getByRole("button", { name: /edit/i }));

    // The control's floor is the current size: shrinking is refused by the API,
    // the store and the CRD, so the UI must not even offer it.
    const size = screen.getByLabelText(/^size$/i);
    expect(size).toHaveAttribute("min", "10");
    // Still at the current size, so there is nothing to apply yet.
    expect(screen.getByRole("button", { name: /increase size/i })).toBeDisabled();
  });

  // The mount path is baked into the running pod's volume mount — changing it
  // is a detach and re-attach, not an edit — so it is never writable here.
  it("shows the mount path as a read-only field", () => {
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    const mountPath = screen.getByDisplayValue("/var/data");
    expect(mountPath).toHaveAttribute("readonly");
  });

  // Render's order: Recent Metrics, Disk Configuration, Snapshots, Delete Disk.
  // Delete trails in its own card rather than sitting a mis-click from the size
  // field.
  it("orders the cards the way Render orders them", () => {
    mockUseDisk.mockReturnValue(attached);
    const { container } = render(
      <DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />,
    );

    const titles = [...container.querySelectorAll('[data-slot="card-title"]')].map((el) =>
      el.textContent?.trim(),
    );
    expect(titles).toEqual(["Disk usage", "Disk Configuration", "Snapshots", "Delete disk"]);
  });

  it("warns that deleting destroys the data", async () => {
    const user = userEvent.setup();
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    await user.click(screen.getByRole("button", { name: /delete disk/i }));

    expect(await screen.findByText(/all data on the disk will be lost/i)).toBeInTheDocument();
    expect(mockDeleteDisk).not.toHaveBeenCalled();

    // Same typed-phrase gate as restore: deleting a disk destroys a volume the
    // tenant is paying for and cannot get back.
    const confirm = screen.getAllByRole("button", { name: /delete disk/i }).at(-1)!;
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText(/type/i), "sudo delete disk /var/data");
    await waitFor(() => expect(confirm).toBeEnabled());
    await user.click(confirm);
    await waitFor(() => expect(mockDeleteDisk).toHaveBeenCalledWith("dsk-1"));
  });

  it("carries Render's database-recovery warning on the snapshots card", () => {
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);
    expect(screen.getByText(/don't restore a disk to recover a database/i)).toBeInTheDocument();
  });

  it("confirms before restoring, and says what is lost", async () => {
    const user = userEvent.setup();
    mockUseDisk.mockReturnValue(attached);
    mockUseDiskSnapshots.mockReturnValue({
      ...noSnapshots,
      snapshots: [
        { createdAt: "2026-08-23T02:00:00Z", snapshotKey: "key-1", instanceId: "srv-1" },
      ],
    });
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    await user.click(screen.getByRole("button", { name: /^restore$/i }));
    expect(
      await screen.findByText(/everything written after the snapshot is lost/i),
    ).toBeInTheDocument();
    expect(mockRestoreSnapshot).not.toHaveBeenCalled();

    // ADR082 D6: a restore is irreversible and, unlike a delete, leaves no
    // vanished resource behind to signal that it happened — so it is gated on
    // the typed phrase, not just an "are you sure".
    const confirm = screen.getByRole("button", { name: /restore disk/i });
    expect(confirm).toBeDisabled();

    await user.type(screen.getByLabelText(/type/i), "sudo restore disk /var/data");
    await waitFor(() => expect(confirm).toBeEnabled());
    await user.click(confirm);
    await waitFor(() => expect(mockRestoreSnapshot).toHaveBeenCalledWith("dsk-1", "key-1"));
  });

  // A near-miss must not run: the phrase is compared exactly.
  it("refuses a restore when the typed phrase is not exact", async () => {
    const user = userEvent.setup();
    mockUseDisk.mockReturnValue(attached);
    mockUseDiskSnapshots.mockReturnValue({
      ...noSnapshots,
      snapshots: [
        { createdAt: "2026-08-23T02:00:00Z", snapshotKey: "key-1", instanceId: "srv-1" },
      ],
    });
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    await user.click(screen.getByRole("button", { name: /^restore$/i }));
    await user.type(screen.getByLabelText(/type/i), "sudo restore disk /var/dat");

    expect(screen.getByRole("button", { name: /restore disk/i })).toBeDisabled();
    expect(mockRestoreSnapshot).not.toHaveBeenCalled();
  });

  // The chart must ask for THIS service's disk, not a datastore's. Getting the
  // kind wrong would still render — it would just quietly graph nothing.
  it("graphs the attached disk's usage against its capacity", () => {
    mockUseDisk.mockReturnValue(attached);
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    expect(screen.getByText("Disk usage")).toBeInTheDocument();
    const kinds = mockUseDatastoreMetrics.mock.calls.map((c) => [c[0], c[1], c[2]]);
    expect(kinds).toContainEqual(["service", "srv-1", "disk"]);
    expect(kinds).toContainEqual(["service", "srv-1", "disk_capacity"]);
  });

  // Observability is not billing: the chart appears only once a disk exists,
  // and never stands in for the provisioned size the invoice uses.
  it("shows no usage chart before a disk is attached", () => {
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="web_service" />);

    expect(screen.queryByText("Disk usage")).not.toBeInTheDocument();
    expect(mockUseDatastoreMetrics).not.toHaveBeenCalled();
  });

  // The API refuses a disk on a cron job outright, so the tab must not offer
  // one — before w1/m86 a paid cron showed an enabled Add Disk that always
  // 400'd.
  it("explains why a cron job cannot have a disk, instead of offering one", () => {
    render(<DiskSection serviceId="srv-1" plan="starter" serviceType="cron_job" />);

    expect(
      screen.getByText(/need a long-running instance to mount on/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /add disk/i })).toBeDisabled();
  });

  // Wrong type is reported ahead of the plan, because upgrading cannot fix it.
  it("reports the type problem before the plan problem", () => {
    render(<DiskSection serviceId="srv-1" plan="free" serviceType="cron_job" />);

    expect(
      screen.getByText(/need a long-running instance to mount on/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/paid instance type/i)).not.toBeInTheDocument();
  });
});
