import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WebShellTerminal } from "@/features/services/components/web-shell-terminal";

// xterm.js is DOM/canvas-heavy and imported dynamically inside the effect;
// replace it with a spy so the test drives the connection logic, not rendering.
const h = vi.hoisted(() => {
  const writes: Uint8Array[] = [];
  let dataCb: (d: string) => void = () => {};
  const term = {
    loadAddon: () => {},
    open: () => {},
    onData: (cb: (d: string) => void) => {
      dataCb = cb;
    },
    write: (d: Uint8Array) => {
      writes.push(d);
    },
    dispose: () => {},
    cols: 90,
    rows: 30,
  };
  class ShellUnavailableError extends Error {}
  const createShellSession = vi.fn(
    async (_id: string, _instanceId?: string) => ({
      ticket: "tkt-123",
      url: "wss://ssh.bex.co/shell",
      expiresAt: "",
    }),
  );
  return {
    term,
    writes,
    typeInto: (d: string) => dataCb(d),
    ShellUnavailableError,
    createShellSession,
  };
});
const { createShellSession, ShellUnavailableError } = h;

vi.mock("@xterm/xterm", () => ({
  Terminal: function Terminal() {
    return h.term;
  },
}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: function FitAddon() {
    return { fit: () => {} };
  },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

vi.mock("@/features/services/hooks/use-shell-session", () => ({
  useShellSession: () => ({ createShellSession: h.createShellSession }),
  ShellUnavailableError: h.ShellUnavailableError,
}));

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];
  url: string;
  protocols: string[];
  readyState = 0;
  sent: unknown[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(url: string, protocols?: string | string[]) {
    this.url = url;
    this.protocols =
      protocols === undefined
        ? []
        : Array.isArray(protocols)
          ? protocols
          : [protocols];
    FakeWebSocket.instances.push(this);
  }
  send(data: unknown) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
  }
}

beforeEach(() => {
  FakeWebSocket.instances = [];
  h.writes.length = 0;
  createShellSession.mockClear();
  vi.stubGlobal("WebSocket", FakeWebSocket);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

async function openSocket() {
  const ws = await waitFor(() => {
    expect(FakeWebSocket.instances).toHaveLength(1);
    return FakeWebSocket.instances[0];
  });
  await act(async () => {
    ws.readyState = FakeWebSocket.OPEN;
    ws.onopen?.();
  });
  return ws;
}

describe("WebShellTerminal", () => {
  it("mints a ticket and opens the gateway WebSocket carrying it in a subprotocol, never the URL", async () => {
    render(<WebShellTerminal serviceId="srv-x" />);
    const ws = await openSocket();
    expect(createShellSession).toHaveBeenCalledWith("srv-x", undefined);
    // w1/042 L8: the URL lands in the edge proxy's access log, so the ticket
    // must ride the Sec-WebSocket-Protocol offer instead.
    expect(ws.url).toBe("wss://ssh.bex.co/shell");
    expect(ws.protocols).toEqual(["bex.shell", "bex.ticket.tkt-123"]);
    // On open it sends an initial resize control frame.
    expect(
      ws.sent.some(
        (m) => typeof m === "string" && m.includes('"type":"resize"'),
      ),
    ).toBe(true);
    expect(await screen.findByText("Connected")).toBeInTheDocument();
  });

  it("passes a pinned instance id to the ticket mint", async () => {
    render(<WebShellTerminal serviceId="srv-x" instanceId="srv-x-pod01" />);
    await openSocket();
    expect(createShellSession).toHaveBeenCalledWith("srv-x", "srv-x-pod01");
  });

  it("writes binary frames to the terminal and sends keystrokes as binary stdin", async () => {
    render(<WebShellTerminal serviceId="srv-x" />);
    const ws = await openSocket();

    act(() => {
      ws.onmessage?.({ data: new TextEncoder().encode("hello").buffer });
    });
    expect(h.writes).toHaveLength(1);
    expect(new TextDecoder().decode(h.writes[0])).toBe("hello");

    act(() => h.typeInto("ls\n"));
    const stdin = ws.sent.find((m) => typeof m !== "string");
    expect(
      stdin,
      "keystrokes should be sent as a binary stdin frame",
    ).toBeDefined();
    expect(new TextDecoder().decode(stdin as ArrayBufferView)).toBe("ls\n");
  });

  it("surfaces a server error control frame and offers reconnect", async () => {
    render(<WebShellTerminal serviceId="srv-x" />);
    const ws = await openSocket();

    act(() => {
      ws.onmessage?.({
        data: JSON.stringify({
          type: "error",
          message: "session limit reached; close another shell and try again",
        }),
      });
    });

    expect(
      await screen.findByText(/session limit reached/i),
    ).toBeInTheDocument();
    const reconnect = screen.getByRole("button", { name: /reconnect/i });

    createShellSession.mockClear();
    await userEvent.click(reconnect);
    await waitFor(() => expect(createShellSession).toHaveBeenCalled());
  });

  it("explains when the browser transport is unconfigured (503)", async () => {
    createShellSession.mockRejectedValueOnce(
      new ShellUnavailableError("web shell transport not configured"),
    );
    render(<WebShellTerminal serviceId="srv-x" />);
    expect(
      await screen.findByText(/isn't enabled on this platform/i),
    ).toBeInTheDocument();
    expect(FakeWebSocket.instances).toHaveLength(0);
  });
});
