import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { GlobalSearch } from "../global-search";

// w4/265: the mounted palette kept its last query after a result selection or
// a Cmd/Ctrl+K close, because both flip the parent's `open` directly instead
// of going through Radix's onOpenChange (which only Escape/Close use). These
// drive the real parent + lazy dialog + Radix/cmdk; the resource hooks are
// mocked only as data sources.
const navigate = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [], loading: false }),
}));
vi.mock("@/features/databases/hooks/use-databases", () => ({
  useDatabases: () => ({ databases: [], loading: false }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-values", () => ({
  useKeyValues: () => ({ keyValues: [], loading: false }),
}));
vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [{ id: "prj-a254", name: "qa-a254-search-雪" }],
    loading: false,
  }),
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: [], loading: false }),
}));
vi.mock("@/features/blueprints/hooks/use-blueprints", () => ({
  useBlueprints: () => ({ blueprints: [], loading: false }),
}));

function pressCtrlK() {
  fireEvent.keyDown(document, { key: "k", ctrlKey: true });
}

async function openWithKeyboard() {
  pressCtrlK();
  return screen.findByRole("combobox");
}

function type(value: string) {
  const input = screen.getByRole("combobox");
  fireEvent.change(input, { target: { value } });
  return input;
}

async function waitClosed() {
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
}

function expectUnfiltered() {
  const input = screen.getByRole("combobox");
  expect(input).toHaveValue("");
  // Every Navigation destination is back, plus the resource.
  expect(screen.getByRole("option", { name: /Webhooks/ })).toBeInTheDocument();
  expect(
    screen.getByRole("option", { name: /Environment Groups/ }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("option", { name: /qa-a254-search-雪/ }),
  ).toBeInTheDocument();
}

beforeEach(() => {
  navigate.mockReset();
});

describe("GlobalSearch close/reopen", () => {
  it("starts empty after a project is selected by click", async () => {
    render(<GlobalSearch />);
    await openWithKeyboard();
    type("a254-search");
    fireEvent.click(screen.getByRole("option", { name: /qa-a254-search-雪/ }));
    expect(navigate).toHaveBeenCalledWith({
      to: "/project/$projectId",
      params: { projectId: "prj-a254" },
    });
    await waitClosed();

    await openWithKeyboard();
    expectUnfiltered();
  });

  it("starts empty after a Navigation result is selected with Enter, reopened from the topbar", async () => {
    render(<GlobalSearch />);
    const input = await openWithKeyboard();
    type("Environment Groups");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(navigate).toHaveBeenCalledWith({ to: "/env-groups" });
    await waitClosed();

    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    await screen.findByRole("combobox");
    expectUnfiltered();
  });

  it("starts empty after Ctrl+K closes it", async () => {
    render(<GlobalSearch />);
    await openWithKeyboard();
    type("Environment Groups");
    pressCtrlK();
    await waitClosed();

    await openWithKeyboard();
    expectUnfiltered();
  });

  it("still starts empty after Escape (control)", async () => {
    render(<GlobalSearch />);
    const input = await openWithKeyboard();
    type("QA_NO_MATCH");
    fireEvent.keyDown(input, { key: "Escape" });
    await waitClosed();

    await openWithKeyboard();
    expectUnfiltered();
  });

  it("keeps the typed query while the dialog stays open", async () => {
    render(<GlobalSearch />);
    await openWithKeyboard();
    const input = type("Webhooks");
    expect(input).toHaveValue("Webhooks");
    expect(
      screen.queryByRole("option", { name: /Environment Groups/ }),
    ).toBeNull();
  });
});

// w4/291: the Search button is not the dialog's Radix trigger, so dismissing
// the palette restored focus to nothing: it fell to <body>, and Enter could
// not reopen the control just used.
describe("GlobalSearch returns focus when dismissed (w4/291)", () => {
  const searchButton = () => screen.getByRole("button", { name: "Search" });
  async function openFromButton() {
    searchButton().focus();
    fireEvent.click(searchButton());
    return screen.findByRole("combobox");
  }

  it("returns to the Search button after Escape, and Enter opens it again", async () => {
    render(<GlobalSearch />);
    const input = await openFromButton();
    expect(input).toHaveFocus();
    fireEvent.keyDown(input, { key: "Escape" });
    await waitClosed();
    await waitFor(() => expect(searchButton()).toHaveFocus());

    fireEvent.click(document.activeElement!); // Enter on a focused button
    expect(await screen.findByRole("combobox")).toHaveFocus();
  });

  it("returns to the Search button after Ctrl+K closes a button-opened palette", async () => {
    render(<GlobalSearch />);
    await openFromButton();
    pressCtrlK();
    await waitClosed();
    await waitFor(() => expect(searchButton()).toHaveFocus());
  });

  it("returns to whatever held focus when the shortcut opened it", async () => {
    render(
      <>
        <button>elsewhere</button>
        <GlobalSearch />
      </>,
    );
    const elsewhere = screen.getByRole("button", { name: "elsewhere" });
    elsewhere.focus();
    const input = await openWithKeyboard();
    fireEvent.keyDown(input, { key: "Escape" });
    await waitClosed();
    await waitFor(() => expect(elsewhere).toHaveFocus());
  });

  it("falls back to the Search button when the shortcut's opener is gone or was the page itself", async () => {
    // GlobalSearch keeps its place in the tree; only the opener comes and goes.
    const page = (withOpener: boolean) => (
      <>
        {withOpener ? <button>elsewhere</button> : null}
        <GlobalSearch />
      </>
    );
    const { rerender } = render(page(true));
    screen.getByRole("button", { name: "elsewhere" }).focus();
    await openWithKeyboard();
    rerender(page(false));
    let input = screen.getByRole("combobox");
    fireEvent.keyDown(input, { key: "Escape" });
    await waitClosed();
    await waitFor(() => expect(searchButton()).toHaveFocus());

    // Opened with nothing focused: the button is the logical place to land.
    searchButton().blur();
    input = await openWithKeyboard();
    fireEvent.keyDown(input, { key: "Escape" });
    await waitClosed();
    await waitFor(() => expect(searchButton()).toHaveFocus());
  });

  it("leaves focus to the destination when a result is chosen", async () => {
    render(<GlobalSearch />);
    const input = await openFromButton();
    type("Environment Groups");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(navigate).toHaveBeenCalledWith({ to: "/env-groups" });
    await waitClosed();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(searchButton()).not.toHaveFocus();

    // The next dismissal is a dismissal again.
    const reopened = await openFromButton();
    fireEvent.keyDown(reopened, { key: "Escape" });
    await waitClosed();
    await waitFor(() => expect(searchButton()).toHaveFocus());
  });
});
