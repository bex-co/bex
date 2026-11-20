import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceLogsPage } from "@/routes/services.$serviceId.logs";
import type { LogLine } from "../../types";
import { setupVirtualGeometry } from "@/test/virtual-geometry";

// SSH into instance on the service Logs tab (w3/m528): offered only on a
// service whose Shell can open (`sshAddress`, as the Shell page decides), and
// the running instances are read only then.

const line = (instance: string): LogLine => ({
  key: `id:${instance}`,
  timestamp: new Date(Date.now() - 60_000).toISOString(),
  instance,
  message: `from ${instance}`,
  spans: null,
  resource: "",
  type: "app",
  level: "",
  method: "",
  statusCode: "",
  host: "",
  path: "",
});

const server = vi.hoisted(() => ({
  service: { id: "srv-web", type: "web_service", sshAddress: "" as string },
}));
const instancesSpy = vi.hoisted(() => vi.fn());
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service: server.service, loading: false }),
}));
vi.mock("@/features/services/hooks/use-service-instances", () => ({
  useServiceInstances: (...args: unknown[]) => {
    instancesSpy(...args);
    return {
      instances: [{ id: "srv-web-up", createdAt: "" }],
      loading: false,
      refetch: () => undefined,
    };
  },
}));
vi.mock("@/features/logs/hooks/use-log-history", () => ({
  useLogHistory: () => ({
    lines: [line("srv-web-up"), line("srv-web-gone")],
    loading: false,
    error: undefined,
    storeUnavailable: false,
    timedOut: false,
    hasMore: false,
    loadingOlder: false,
    loadOlder: () => undefined,
    resources: undefined,
    retry: () => undefined,
  }),
}));
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
  useNewLogsWatch: () => ({ available: false, dismiss: () => undefined }),
}));
vi.mock("@/features/logs/hooks/use-log-retention", () => ({
  useLogRetentionBound: () => undefined,
}));
vi.mock("@/features/logs/hooks/use-log-label-values", () => ({
  useLogLabelDiscovery: () => ({ values: [], resolved: false }),
  useLogLabelValues: () => [],
}));

setupVirtualGeometry();

beforeEach(() => {
  instancesSpy.mockReset();
});

async function sshItem(index: number) {
  const user = userEvent.setup();
  fireEvent.pointerEnter(document.querySelector(`[data-index="${index}"]`)!);
  await user.click(screen.getByRole("button", { name: "Log line options" }));
  return screen.queryByRole("menuitem", { name: "SSH into instance" });
}

describe("ServiceLogsPage SSH into instance (w3/m528)", () => {
  it("links a running instance's line to the Shell on it, and disables a gone one's", async () => {
    server.service.sshAddress = "srv-web@ssh.bex.co";
    render(<ServiceLogsPage serviceId="srv-web" />);
    expect(instancesSpy).toHaveBeenLastCalledWith("srv-web", { skip: false });
    expect(await sshItem(0)).toHaveAttribute(
      "href",
      "/services/srv-web/shell?instance=srv-web-up",
    );
    await userEvent.keyboard("{Escape}");
    expect(await sshItem(1)).toHaveAttribute("aria-disabled", "true");
  });

  it("offers none on a service whose Shell cannot open, and reads no instances", async () => {
    server.service.sshAddress = "";
    render(<ServiceLogsPage serviceId="srv-web" />);
    expect(instancesSpy).toHaveBeenLastCalledWith("srv-web", { skip: true });
    expect(await sshItem(0)).toBeNull();
  });
});
