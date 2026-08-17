import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

// The header links out to /agents and drives cancel through the mutations hook;
// neither is under test here, so both are inert.
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
  useNavigate: () => vi.fn(),
}));
const pinMock = vi.fn().mockResolvedValue(undefined);
const unpinMock = vi.fn().mockResolvedValue(undefined);
const archiveMock = vi.fn().mockResolvedValue(undefined);
const unarchiveMock = vi.fn().mockResolvedValue(undefined);
const deleteMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({
    cancel: vi.fn(),
    pin: pinMock,
    unpin: unpinMock,
    archive: archiveMock,
    unarchive: unarchiveMock,
    deleteSession: deleteMock,
  }),
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return agentSessionView({ phase: "completed", turns: 1, ...over });
}

describe("SessionDetailHeader", () => {
  // w5/m65 removed the evidence side panel; the header must not offer a way back
  // to a surface that no longer exists.
  it("offers no evidence toggle", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByText("Evidence")).not.toBeInTheDocument();
  });

  // With the inline PR card gone, this badge is the session's only PR
  // affordance, so it has to survive.
  it("links the draft PR through the #N badge when the session opened one", () => {
    render(
      <SessionDetailHeader
        session={view({
          prNumber: 7,
          prUrl: "https://github.com/acme/widgets/pull/7",
        })}
      />,
    );
    const link = screen.getByRole("link", { name: /#7/ });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/acme/widgets/pull/7",
    );
  });

  // The default (opted-out) session has no PR at all — the badge must stay away
  // rather than render an empty or dead reference.
  it("renders no PR badge for a session that never asked for one", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(screen.queryByText(/#\d/)).not.toBeInTheDocument();
  });

  // ADR059 D5/D6: a hibernated session shows its snapshot storage cost and a Pin
  // toggle; a live/terminal session shows neither.
  it("shows the storage size + Pin control only for a hibernated session", async () => {
    const { rerender } = render(<SessionDetailHeader session={view()} />);
    expect(
      screen.queryByRole("button", { name: /pin/i }),
    ).not.toBeInTheDocument();

    rerender(
      <SessionDetailHeader
        session={view({ phase: "hibernated", snapshotBytes: 5 * 1024 * 1024 })}
      />,
    );
    expect(screen.getByText(/Hibernated · 5\.0 MiB/)).toBeInTheDocument();
    const pinBtn = screen.getByRole("button", { name: /^pin$/i });
    pinBtn.click();
    expect(pinMock).toHaveBeenCalledTimes(1);
  });

  it("offers Unpin (not Pin) for an already-pinned hibernated session", () => {
    render(
      <SessionDetailHeader
        session={view({ phase: "hibernated", pinned: true })}
      />,
    );
    expect(screen.getByRole("button", { name: /unpin/i })).toBeInTheDocument();
    expect(screen.getByText(/Pinned/)).toBeInTheDocument();
  });

  // ADR065 D1/D6: the overflow menu carries Archive (any phase) and, on a
  // finished session, the destructive Delete behind a confirmation dialog.
  it("archives through the overflow menu and badges an archived session", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SessionDetailHeader session={view()} />);
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /^archive$/i }));
    expect(archiveMock).toHaveBeenCalledTimes(1);

    rerender(
      <SessionDetailHeader
        session={view({
          archivedAt: new Date().toISOString(),
          isArchived: true,
        })}
      />,
    );
    expect(screen.getByText("Archived")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /unarchive/i }));
    expect(unarchiveMock).toHaveBeenCalledTimes(1);
  });

  it("offers Delete only on a finished session, behind a confirmation", async () => {
    const user = userEvent.setup();
    // A live session must cancel first — no Delete in the menu.
    const { rerender } = render(
      <SessionDetailHeader session={view({ phase: "running" })} />,
    );
    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(
      screen.queryByRole("menuitem", { name: /delete/i }),
    ).not.toBeInTheDocument();
    await user.keyboard("{Escape}");

    rerender(<SessionDetailHeader session={view()} />); // completed
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("menuitem", { name: /delete/i }));
    // The confirm dialog gates the destructive call.
    expect(deleteMock).not.toHaveBeenCalled();
    expect(screen.getByText("Delete this session?")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Delete session" }));
    expect(deleteMock).toHaveBeenCalledTimes(1);
  });
});
