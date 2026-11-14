import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WebShellTerminal } from "@/features/services/components/web-shell-terminal";
import { codedGraphQLError } from "@/test/mocks/apollo";

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
    // The new attempt starts over: the old failure no longer shows.
    expect(
      screen.getByText("Connecting to a running instance…"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/session limit reached/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /reconnect/i })).toBeNull();
  });

  it("starts over at Connecting when pointed at another service", async () => {
    const { rerender } = render(<WebShellTerminal serviceId="srv-x" />);
    await openSocket();
    expect(await screen.findByText("Connected")).toBeInTheDocument();

    rerender(<WebShellTerminal serviceId="srv-y" />);
    expect(
      screen.getByText("Connecting to a running instance…"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Connected")).toBeNull();
    await waitFor(() =>
      expect(createShellSession).toHaveBeenLastCalledWith("srv-y", undefined),
    );
  });

  // w4/m352: a refused mint shows the server's own sentence, not generic copy.
  it("shows a 409 mint refusal's own reason", async () => {
    createShellSession.mockRejectedValueOnce(
      codedGraphQLError(
        "CONFLICT",
        {},
        "conflict: the service has no running instance yet; wait for the current deploy to go live",
      ),
    );
    render(<WebShellTerminal serviceId="srv-x" />);
    expect(
      await screen.findByText(
        "The service has no running instance yet; wait for the current deploy to go live",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Connection error")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't start a shell session.")).toBeNull();
    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  it("keeps the generic copy for a mint failure with no reason", async () => {
    createShellSession.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    render(<WebShellTerminal serviceId="srv-x" />);
    expect(
      await screen.findByText("Couldn't start a shell session."),
    ).toBeInTheDocument();
  });

  // w4/m352: a server-ended session reads "Session ended" with its reason,
  // never "Connection error" or the gateway's raw English.
  const endReasons: [string, string, string][] = [
    [
      "revoked",
      "Your access to this service ended.",
      "你对此服务的访问权限已结束。",
    ],
    [
      "suspended",
      "The service was suspended, which ended this session. Resume it to open a new one.",
      "服务已暂停，此会话随之结束。恢复服务后即可打开新会话。",
    ],
    [
      "ineligible",
      "This service no longer has shell access (its plan or type changed), which ended this session.",
      "此服务已不再支持 Shell 访问（其实例类型或服务类型已变更），此会话随之结束。",
    ],
    [
      "instance_gone",
      "The instance this session ran on stopped. Reconnect to open a shell on a running instance.",
      "此会话所在的实例已停止。重新连接即可在运行中的实例上打开 Shell。",
    ],
    [
      "timeout",
      "The session reached its time limit. Reconnect to keep working.",
      "会话已达到时长上限。重新连接即可继续。",
    ],
    [
      "unverified",
      "Your access couldn't be re-checked, so the session ended. Reconnect to try again.",
      "无法重新验证你的访问权限，会话已结束。请重新连接重试。",
    ],
  ];

  it.each(endReasons)(
    "names a %s end under Session ended, in en and zh",
    async (reason, en, zh) => {
      const frame = JSON.stringify({
        type: "error",
        reason,
        message: "the gateway's english sentence",
      });
      const view = render(<WebShellTerminal serviceId="srv-x" />);
      let ws = await openSocket();
      act(() => ws.onmessage?.({ data: frame }));
      expect(await screen.findByText(en)).toBeInTheDocument();
      expect(screen.getByText("Session ended")).toBeInTheDocument();
      expect(screen.queryByText("Connection error")).toBeNull();
      expect(screen.queryByText(/english sentence/)).toBeNull();
      expect(
        screen.getByRole("button", { name: /reconnect/i }),
      ).toBeInTheDocument();
      view.unmount();

      const i18n = (await import("@/i18n/init")).default;
      await i18n.changeLanguage("zh");
      try {
        FakeWebSocket.instances = [];
        render(<WebShellTerminal serviceId="srv-x" />);
        ws = await openSocket();
        act(() => ws.onmessage?.({ data: frame }));
        expect(await screen.findByText(zh)).toBeInTheDocument();
        expect(screen.getByText("会话已结束")).toBeInTheDocument();
      } finally {
        await i18n.changeLanguage("en");
      }
    },
  );

  it("falls back to the gateway's sentence for a reason it doesn't know", async () => {
    render(<WebShellTerminal serviceId="srv-x" />);
    const ws = await openSocket();
    act(() =>
      ws.onmessage?.({
        data: JSON.stringify({
          type: "error",
          reason: "something_new",
          message: "a reason from a newer gateway",
        }),
      }),
    );
    expect(
      await screen.findByText("a reason from a newer gateway"),
    ).toBeInTheDocument();
    expect(screen.getByText("Session ended")).toBeInTheDocument();
  });

  it("still shows Connection error for a transport failure", async () => {
    render(<WebShellTerminal serviceId="srv-x" />);
    const ws = await openSocket();
    act(() => ws.onerror?.());
    expect(await screen.findByText("Connection error")).toBeInTheDocument();
    expect(screen.queryByText("Session ended")).toBeNull();
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
