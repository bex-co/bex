import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueuedMessageList } from "@/features/agent-sessions/components/queued-messages";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import type { QueuedMessageView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

const mutations = vi.hoisted(() => ({
  edit: vi.fn(),
  remove: vi.fn(),
  move: vi.fn(),
  sendNow: vi.fn(),
}));
vi.mock("@/features/agent-sessions/hooks/use-queued-message-mutations", () => ({
  useQueuedMessageMutations: () => mutations,
}));

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (m: string) => toastSuccess(m) },
}));

function message(
  id: string,
  prompt: string,
  position: number,
  over: Partial<QueuedMessageView> = {},
): QueuedMessageView {
  return {
    id,
    position,
    prompt,
    author: "alice",
    attachmentCount: 0,
    blockedReason: null,
    createdAt: null,
    ...over,
  };
}

const THREE = [
  message("aqm-1", "run the linter", 1),
  message("aqm-2", "then the tests", 2),
  message("aqm-3", "then open a PR", 3),
];

const session = agentSessionView({ phase: "running" });

function renderList(
  messages: QueuedMessageView[] = THREE,
  onChanged?: () => void,
) {
  return render(
    <QueuedMessageList
      session={session}
      messages={messages}
      onChanged={onChanged}
    />,
  );
}

/** The row for one message, to scope its controls. */
function row(id: string) {
  const el = document.querySelector(`[data-message-id="${id}"]`);
  if (!(el instanceof HTMLElement)) throw new Error(`no row ${id}`);
  return within(el);
}

beforeEach(() => {
  for (const fn of Object.values(mutations)) fn.mockReset();
  mutations.edit.mockResolvedValue(message("aqm-1", "edited", 1));
  mutations.remove.mockResolvedValue(undefined);
  mutations.move.mockResolvedValue([]);
  mutations.sendNow.mockResolvedValue({ stopped: true });
  toastSuccess.mockReset();
});

describe("QueuedMessageList rendering", () => {
  it("renders nothing, and reserves no space, for an empty queue", () => {
    const { container } = renderList([]);
    expect(container).toBeEmptyDOMElement();
  });

  it("lists each message in order under a counted heading", () => {
    renderList();
    expect(
      screen.getByRole("heading", { name: "3 queued messages" }),
    ).toBeInTheDocument();
    const prompts = screen
      .getAllByTestId("queued-message")
      .map((li) => li.textContent);
    expect(prompts[0]).toContain("run the linter");
    expect(prompts[1]).toContain("then the tests");
    expect(prompts[2]).toContain("then open a PR");
  });

  it("uses the singular heading for one message", () => {
    renderList([message("aqm-1", "only one", 1)]);
    expect(
      screen.getByRole("heading", { name: "1 queued message" }),
    ).toBeInTheDocument();
  });

  it("states the reason the dispatcher could not start a message", () => {
    renderList([
      message("aqm-1", "blocked one", 1, {
        blockedReason: "the session's spend ceiling was reached",
      }),
      message("aqm-2", "free one", 2),
    ]);
    expect(
      screen.getByText(
        "Couldn't start yet: the session's spend ceiling was reached. It is retried automatically.",
      ),
    ).toBeInTheDocument();
    expect(row("aqm-2").queryByText(/Couldn't start yet/)).toBeNull();
  });

  it("shows how many attachments ride with a message", () => {
    renderList([message("aqm-1", "see the shots", 1, { attachmentCount: 2 })]);
    expect(screen.getByText("2 attachments")).toBeInTheDocument();
  });

  it("shortens a very long prompt in the row, not in the edit box", async () => {
    const long = "x".repeat(2000);
    renderList([message("aqm-1", long, 1)]);
    const shown = within(screen.getByTestId("queued-message")).getAllByText(
      /^x+…$/,
    )[0].textContent!;
    expect(shown.length).toBe(601);

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("textbox")).toHaveValue(long);
  });
});

describe("QueuedMessageList reordering and removal", () => {
  it("moves a message up one place", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Move up" }));
    // Place 2 -> place 1.
    await waitFor(() =>
      expect(mutations.move).toHaveBeenCalledWith("aqm-2", 1),
    );
    expect(mutations.move).toHaveBeenCalledTimes(1);
  });

  it("moves a message down one place", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Move down" }));
    await waitFor(() =>
      expect(mutations.move).toHaveBeenCalledWith("aqm-2", 3),
    );
  });

  it("cannot move the first message up or the last one down", () => {
    renderList();
    expect(
      row("aqm-1").getByRole("button", { name: "Move up" }),
    ).toBeDisabled();
    expect(
      row("aqm-1").getByRole("button", { name: "Move down" }),
    ).toBeEnabled();
    expect(row("aqm-3").getByRole("button", { name: "Move up" })).toBeEnabled();
    expect(
      row("aqm-3").getByRole("button", { name: "Move down" }),
    ).toBeDisabled();
  });

  it("removes a message", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(mutations.remove).toHaveBeenCalledWith("aqm-2"));
    expect(mutations.remove).toHaveBeenCalledTimes(1);
  });

  it("rests every control of a row while its change is in flight", async () => {
    let finish: () => void = () => undefined;
    mutations.remove.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Remove" }));

    for (const name of ["Send now", "Move up", "Move down", "Edit", "Remove"]) {
      expect(row("aqm-2").getByRole("button", { name })).toBeDisabled();
    }
    finish();
    await waitFor(() =>
      expect(row("aqm-2").getByRole("button", { name: "Edit" })).toBeEnabled(),
    );
  });
});

describe("QueuedMessageList editing in place", () => {
  it("saves an edit with Enter and calls the edit verb with the new text", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Edit" }));

    const box = screen.getByRole("textbox", { name: "Edit queued message" });
    expect(box).toHaveValue("then the tests");
    await user.clear(box);
    await user.type(box, "then the whole suite{Enter}");

    await waitFor(() =>
      expect(mutations.edit).toHaveBeenCalledWith(
        "aqm-2",
        "then the whole suite",
      ),
    );
    expect(mutations.sendNow).not.toHaveBeenCalled();
    // Back to the row.
    await waitFor(() => expect(screen.queryByRole("textbox")).toBeNull());
  });

  it("saves with the Save button, and Shift+Enter adds a line instead", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    const box = screen.getByRole("textbox");
    await user.type(box, "{Shift>}{Enter}{/Shift}more");
    expect(mutations.edit).not.toHaveBeenCalled();
    expect(box).toHaveValue("run the linter\nmore");

    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(mutations.edit).toHaveBeenCalledWith(
        "aqm-1",
        "run the linter\nmore",
      ),
    );
  });

  it("makes no call when the text did not change", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("textbox")).toBeNull());
    expect(mutations.edit).not.toHaveBeenCalled();
  });

  it("will not save an empty message", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    await user.clear(screen.getByRole("textbox"));
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Save and send now" }),
    ).toBeDisabled();
    await user.type(screen.getByRole("textbox"), "{Enter}");
    expect(mutations.edit).not.toHaveBeenCalled();
  });

  it("discards an edit with Esc or Discard", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    await user.type(screen.getByRole("textbox"), " changed{Escape}");
    expect(screen.queryByRole("textbox")).toBeNull();

    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("textbox")).toHaveValue("run the linter");
    await user.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(mutations.edit).not.toHaveBeenCalled();
  });

  it.each([
    ["Ctrl", "{Control>}{Enter}{/Control}"],
    ["Cmd", "{Meta>}{Enter}{/Meta}"],
  ])(
    "%s+Enter while editing saves the edit, then sends the message now",
    async (_name, keys) => {
      const onChanged = vi.fn();
      const user = userEvent.setup();
      renderList(THREE, onChanged);
      await user.click(row("aqm-3").getByRole("button", { name: "Edit" }));
      await user.type(screen.getByRole("textbox"), " today");
      await user.keyboard(keys);

      await waitFor(() =>
        expect(mutations.sendNow).toHaveBeenCalledWith("aqm-3", 2),
      );
      expect(mutations.edit).toHaveBeenCalledWith(
        "aqm-3",
        "then open a PR today",
      );
      // The text is saved before the stop, in that order.
      expect(mutations.edit.mock.invocationCallOrder[0]).toBeLessThan(
        mutations.sendNow.mock.invocationCallOrder[0],
      );
      expect(toastSuccess).toHaveBeenCalledWith(
        "Stopping the current turn. This message runs next.",
      );
      expect(onChanged).toHaveBeenCalled();
    },
  );

  it("sends an unedited message now with Cmd+Enter without an edit call", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    await waitFor(() =>
      expect(mutations.sendNow).toHaveBeenCalledWith("aqm-1", 0),
    );
    expect(mutations.edit).not.toHaveBeenCalled();
  });

  it("offers Save and send now as a button too", async () => {
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("button", { name: "Save and send now" }));
    await waitFor(() =>
      expect(mutations.sendNow).toHaveBeenCalledWith("aqm-2", 1),
    );
  });
});

describe("QueuedMessageList send now", () => {
  it("stops the turn and runs the message next, then re-reads the session", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    renderList(THREE, onChanged);
    await user.click(row("aqm-3").getByRole("button", { name: "Send now" }));
    await waitFor(() =>
      expect(mutations.sendNow).toHaveBeenCalledWith("aqm-3", 2),
    );
    expect(toastSuccess).toHaveBeenCalledWith(
      "Stopping the current turn. This message runs next.",
    );
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("says only that the message runs next when no turn was running", async () => {
    mutations.sendNow.mockResolvedValue({ stopped: false });
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Send now" }));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith("This message runs next."),
    );
  });
});

describe("QueuedMessageList coded errors", () => {
  async function failWith(code: string, params: Record<string, unknown> = {}) {
    mutations.sendNow.mockRejectedValue(
      new AgentSessionError(code, "server copy", params),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Send now" }));
    return screen.findByRole("alert");
  }

  it("asks to retry while the turn's sandbox is still starting", async () => {
    const alert = await failWith("AGENT_SESSION_TURN_STARTING");
    expect(alert).toHaveTextContent(
      "The turn is still starting, so there is nothing to stop yet. Try again in a few seconds.",
    );
    // Nothing was done, so the controls are back for the retry.
    expect(
      row("aqm-2").getByRole("button", { name: "Send now" }),
    ).toBeEnabled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("says a message that already started can no longer be changed", async () => {
    mutations.edit.mockRejectedValue(
      new AgentSessionError(
        "AGENT_QUEUED_MESSAGE_DISPATCHED",
        "server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Edit" }));
    await user.type(screen.getByRole("textbox"), "!{Enter}");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That message has already started running, so it can no longer be changed.",
    );
  });

  it("says a message that left the queue is gone", async () => {
    mutations.remove.mockRejectedValue(
      new AgentSessionError(
        "AGENT_QUEUED_MESSAGE_NOT_FOUND",
        "server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That message is no longer in the queue.",
    );
  });

  it("says only the sender may edit a message with attachments", async () => {
    mutations.edit.mockRejectedValue(
      new AgentSessionError(
        "AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED",
        "server copy",
        {},
      ),
    );
    const user = userEvent.setup();
    renderList([message("aqm-1", "see the shots", 1, { attachmentCount: 1 })]);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.type(screen.getByRole("textbox"), "!{Enter}");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "only the sender can edit it",
    );
  });

  it("says there is nothing to stop when no turn is running", async () => {
    const alert = await failWith("AGENT_SESSION_NO_TURN_RUNNING");
    expect(alert).toHaveTextContent(
      "No turn is running, so there is nothing to stop.",
    );
  });

  it("falls back to the server's sentence for a code it has no copy for", async () => {
    const alert = await failWith("AGENT_SESSION_SOMETHING_NEW");
    expect(alert).toHaveTextContent("server copy");
  });

  it("falls back to generic copy for a failure with no code", async () => {
    mutations.move.mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-2").getByRole("button", { name: "Move up" }));
    expect(await screen.findByRole("alert")).not.toHaveTextContent("offline");
  });

  it("clears the last error when the next change starts", async () => {
    mutations.remove.mockRejectedValueOnce(
      new AgentSessionError("AGENT_QUEUED_MESSAGE_NOT_FOUND", "x", {}),
    );
    const user = userEvent.setup();
    renderList();
    await user.click(row("aqm-1").getByRole("button", { name: "Remove" }));
    await screen.findByRole("alert");
    await user.click(row("aqm-2").getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });
});
