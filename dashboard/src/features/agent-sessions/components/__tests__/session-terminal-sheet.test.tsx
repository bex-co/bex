import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useState } from "react";
import { SessionTerminalSheet } from "@/features/agent-sessions/components/session-terminal-sheet";
import { useSessionTerminalOffered } from "@/features/agent-sessions/hooks/use-agent-session-terminal";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import { ShellUnavailableError } from "@/features/services/hooks/use-shell-session";
import type { AgentSessionView } from "@/features/agent-sessions/types";

// w10/m615 (ADR054 D10): the session page's Terminal. xterm.js is replaced
// with a spy and the WebSocket with a fake, so the test drives what the
// terminal says and sends, not its rendering.
const h = vi.hoisted(() => {
  let dataCb: (d: string) => void = () => {};
  const term = {
    loadAddon: () => {},
    open: () => {},
    onData: (cb: (d: string) => void) => {
      dataCb = cb;
    },
    write: () => {},
    dispose: () => {},
    cols: 100,
    rows: 30,
  };
  return {
    term,
    typeInto: (d: string) => dataCb(d),
    createTerminal: vi.fn(async (_id: string) => ({
      ticket: "tkt-ags",
      url: "wss://ssh.bex.co/shell",
      starting: false as boolean,
    })),
    capabilities: { terminalEnabled: true } as {
      terminalEnabled: boolean;
    } | null,
  };
});

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
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-terminal",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/agent-sessions/hooks/use-agent-session-terminal")
    >()),
    useAgentSessionTerminal: () => ({ createTerminal: h.createTerminal }),
  }),
);
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => ({ capabilities: h.capabilities }),
  }),
);

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];
  readyState = 0;
  sent: unknown[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(
    readonly url: string,
    readonly protocols: string[],
  ) {
    FakeWebSocket.instances.push(this);
  }
  send(data: unknown) {
    this.sent.push(data);
  }
  close() {
    this.readyState = 3;
  }
  frame(msg: object) {
    act(() => this.onmessage?.({ data: JSON.stringify(msg) }));
  }
  stdin(): string[] {
    return this.sent
      .filter((m) => typeof m !== "string")
      .map((m) => new TextDecoder().decode(m as ArrayBufferView));
  }
}

class FakeResizeObserver {
  observe() {}
  disconnect() {}
}

/** The header's use: offered where the deployment serves it, opened by its button. */
function SessionTerminalButton({ session }: { session: AgentSessionView }) {
  const [open, setOpen] = useState(false);
  if (!useSessionTerminalOffered(session)) return null;
  return (
    <SessionTerminalSheet
      session={session}
      open={open}
      onOpenChange={setOpen}
    />
  );
}

function session(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return {
    id: "ags-d4ae2ab0f8cs73a9ejo0",
    ownerId: "tea-a",
    phase: "completed",
    isArchived: false,
    ...over,
  } as AgentSessionView;
}

async function openTerminal(view = session()) {
  render(<SessionTerminalButton session={view} />);
  await userEvent.click(screen.getByRole("button", { name: "Terminal" }));
}

async function socket() {
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

beforeEach(() => {
  FakeWebSocket.instances = [];
  h.capabilities = { terminalEnabled: true };
  h.createTerminal.mockReset();
  h.createTerminal.mockResolvedValue({
    ticket: "tkt-ags",
    url: "wss://ssh.bex.co/shell",
    starting: false,
  });
  vi.stubGlobal("WebSocket", FakeWebSocket);
  vi.stubGlobal("ResizeObserver", FakeResizeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("SessionTerminalSheet", () => {
  it("is offered only where the deployment serves the browser terminal", () => {
    h.capabilities = { terminalEnabled: false };
    const { rerender } = render(<SessionTerminalButton session={session()} />);
    expect(screen.queryByRole("button", { name: "Terminal" })).toBeNull();
    h.capabilities = { terminalEnabled: true };
    rerender(<SessionTerminalButton session={session({ phase: "running" })} />);
    expect(screen.getByRole("button", { name: "Terminal" })).toBeVisible();
    // Nothing is minted, and no sandbox started, until it is opened.
    expect(h.createTerminal).not.toHaveBeenCalled();
  });

  it("opens a shell in a live sandbox with the ticket in a subprotocol", async () => {
    await openTerminal(session({ phase: "running" }));
    const ws = await socket();
    expect(h.createTerminal).toHaveBeenCalledWith("ags-d4ae2ab0f8cs73a9ejo0");
    expect(ws.url).toBe("wss://ssh.bex.co/shell");
    expect(ws.protocols).toEqual(["bex.shell", "bex.ticket.tkt-ags"]);
    expect(await screen.findByText("Connected")).toBeInTheDocument();
    act(() => h.typeInto("pwd\n"));
    expect(ws.stdin()).toEqual(["pwd\n"]);
  });

  // DoD 2: a closed sandbox the mint started reads as starting, keeps
  // saying so, and the shell opens once the gateway says it is ready.
  it("says the sandbox is starting until the gateway opens the shell", async () => {
    h.createTerminal.mockResolvedValue({
      ticket: "tkt-ags",
      url: "wss://ssh.bex.co/shell",
      starting: true,
    });
    await openTerminal();
    const ws = await socket();
    expect(await screen.findByText("Starting the sandbox…")).toBeVisible();
    expect(
      screen.getByText(
        "The shell opens once the sandbox is up, usually within a minute.",
      ),
    ).toBeVisible();
    // Nothing is typed into a shell that has not opened.
    act(() => h.typeInto("early\n"));
    ws.frame({ type: "starting" });
    ws.frame({ type: "starting", elapsed: 90 });
    expect(
      screen.getByText(
        "Still starting the sandbox (1m 30s). The shell opens once it is up.",
      ),
    ).toBeVisible();
    expect(screen.queryByText("Connected")).toBeNull();
    ws.frame({ type: "ready" });
    expect(await screen.findByText("Connected")).toBeVisible();
    expect(screen.queryByText(/Still starting/)).toBeNull();
    act(() => h.typeInto("uname -a\n"));
    expect(ws.stdin()).toEqual(["uname -a\n"]);
  });

  it("says a start that ended without a sandbox, and tries again on Reconnect", async () => {
    h.createTerminal.mockResolvedValue({
      ticket: "tkt-ags",
      url: "wss://ssh.bex.co/shell",
      starting: true,
    });
    await openTerminal();
    const ws = await socket();
    ws.frame({
      type: "error",
      reason: "no_sandbox",
      message: "the sandbox did not start",
    });
    expect(
      await screen.findByText(
        "The sandbox didn't start. Reconnect to try again.",
      ),
    ).toBeVisible();
    expect(screen.getByText("Session ended")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Reconnect" }));
    await waitFor(() => expect(h.createTerminal).toHaveBeenCalledTimes(2));
  });

  // w10/060: the page reopens a Terminal it remembers from an earlier visit.
  // A page load never starts a closed sandbox, as a sleeping Devin's pane
  // offers "Wake up Devin" instead of waking it; a sandbox that is up opens
  // its shell at once, as Devin's live Terminal tab shows it.
  describe("reopened from an earlier visit", () => {
    const closedSandbox = () =>
      session({ phase: "completed", isSteerable: true, sandboxId: null });

    function Restored({ view }: { view: AgentSessionView }) {
      const [open, setOpen] = useState(true);
      return (
        <SessionTerminalSheet
          session={view}
          open={open}
          restored
          onOpenChange={setOpen}
        />
      );
    }

    it("asks before it starts a sandbox that closed", async () => {
      render(<Restored view={closedSandbox()} />);
      const sheet = await screen.findByRole("dialog", { name: "Terminal" });
      expect(
        within(sheet).getByText(
          "This session's sandbox has closed. Start it to open a shell.",
        ),
      ).toBeVisible();
      expect(h.createTerminal).not.toHaveBeenCalled();
      expect(FakeWebSocket.instances).toHaveLength(0);

      h.createTerminal.mockResolvedValue({
        ticket: "tkt-ags",
        url: "wss://ssh.bex.co/shell",
        starting: true,
      });
      await userEvent.click(
        within(sheet).getByRole("button", { name: "Start the sandbox" }),
      );
      await socket();
      expect(h.createTerminal).toHaveBeenCalledTimes(1);
      expect(await screen.findByText("Starting the sandbox…")).toBeVisible();
    });

    it("opens the shell at once while the sandbox is up", async () => {
      render(
        <Restored
          view={session({
            phase: "completed",
            isSteerable: true,
            sandboxId: "sbx-1",
          })}
        />,
      );
      await socket();
      expect(h.createTerminal).toHaveBeenCalledTimes(1);
      expect(
        screen.queryByRole("button", { name: "Start the sandbox" }),
      ).toBeNull();
    });

    it("opens the shell once a turn brings the sandbox up meanwhile", async () => {
      const { rerender } = render(<Restored view={closedSandbox()} />);
      expect(
        await screen.findByRole("button", { name: "Start the sandbox" }),
      ).toBeVisible();
      rerender(
        <Restored view={session({ phase: "running", sandboxId: "sbx-2" })} />,
      );
      await socket();
      expect(h.createTerminal).toHaveBeenCalledTimes(1);
    });

    it("starts a closed sandbox at once when the person opens it", async () => {
      await openTerminal(closedSandbox());
      await socket();
      expect(h.createTerminal).toHaveBeenCalledTimes(1);
      expect(
        screen.queryByRole("button", { name: "Start the sandbox" }),
      ).toBeNull();
    });
  });

  it.each([
    ["canceled", session({ phase: "canceled" }), /was canceled/],
    ["archived", session({ isArchived: true }), /is archived/],
  ])("says why a %s session has no Terminal", async (_, view, text) => {
    await openTerminal(view);
    expect(await screen.findByText(text)).toBeVisible();
    expect(h.createTerminal).not.toHaveBeenCalled();
    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  it.each([
    [
      "a member who may view the sandbox but not start it",
      new AgentSessionError("AGENT_SESSION_NOT_ATTACHABLE", "not running", {
        phase: "completed",
        sandbox: "closed",
      }),
      "This session isn't running, and its sandbox has closed. To start it again, send the session a message, then open the Terminal again.",
    ],
    [
      "the live-sandbox limit",
      new AgentSessionError(
        "AGENT_SESSION_LIVE_LIMIT",
        "workspace has 3 live sandboxes",
        {},
      ),
      /^The sandbox couldn't start: /,
    ],
    [
      "a missing payment method",
      new CombinedGraphQLErrors({
        errors: [
          {
            message: "add a payment method to start compute",
            extensions: { code: "PAYMENT_REQUIRED" },
          },
        ],
      }),
      "The sandbox couldn't start: Add a payment method to start compute",
    ],
    [
      "a platform without the browser terminal",
      new ShellUnavailableError("web shell transport not configured"),
      /isn't enabled on this platform/,
    ],
  ])("says why the mint was refused: %s", async (_, err, text) => {
    h.createTerminal.mockRejectedValue(err);
    await openTerminal();
    expect(await screen.findByText(text)).toBeVisible();
    expect(FakeWebSocket.instances).toHaveLength(0);
  });
});
